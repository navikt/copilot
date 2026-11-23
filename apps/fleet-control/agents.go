package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Agent profiles (#2582): a caller app may use listed tools on listed routes
// on behalf of a signed-in user. This slice stores and serves them; the
// gateway does not enforce them yet.

// AgentProfile is the API and snapshot shape.
type AgentProfile struct {
	Namespace string        `json:"namespace"` // owner team
	Name      string        `json:"name"`
	Identity  AgentIdentity `json:"identity"`
	Mode      string        `json:"mode"`
	Grants    []AgentGrant  `json:"grants"`
	Quota     AgentQuota    `json:"quota"`
	DPIA      AgentDPIA     `json:"dpia"`
	Status    string        `json:"status,omitempty"`
	ExpiresAt *time.Time    `json:"expires_at,omitempty"`
	CreatedBy string        `json:"-"`
	// NeedsResubmit marks a seeded draft: no one can approve it until a person
	// revokes and re-submits it. Set on read only (TeamAgents).
	NeedsResubmit bool `json:"needsResubmit,omitempty"`
}

// AgentIdentity is the caller app. StsSubject is reserved for the Nais STS
// workload identity and must stay empty until that issuer is chosen.
type AgentIdentity struct {
	Cluster       string `json:"cluster"`
	Namespace     string `json:"namespace"`
	App           string `json:"app"`
	EntraClientID string `json:"entra_client_id"`
	StsSubject    string `json:"sts_subject,omitempty"`
}

// AgentGrant lists the tools allowed on one route, "<namespace>/<name>".
type AgentGrant struct {
	Route   string   `json:"route"`
	Tools   []string `json:"tools"`
	MaxRisk string   `json:"max_risk"`
}

type AgentQuota struct {
	MaxInFlight       int `json:"max_in_flight"`
	RPM               int `json:"rpm"`
	DailyCallsPerUser int `json:"daily_calls_per_user"`
	DailyCalls        int `json:"daily_calls"`
}

type AgentDPIA struct {
	Owner     string `json:"owner"`
	Reference string `json:"reference"`
}

// Person is a signed-in human and the Nais teams they belong to.
type Person struct {
	Ident string
	Teams []string
}

// personResolver authenticates the human behind a write request.
// errUnauthorized means 401; any other error means 503.
type personResolver interface {
	Person(r *http.Request) (Person, error)
}

func (p Person) in(team string) bool { return slices.Contains(p.Teams, team) }

// identityVerifier checks that an Entra client id belongs to the declared
// cluster/namespace/app, for example from the Nais AzureApp resource. Without
// one every profile stays unverified and out of the gateway snapshot.
type identityVerifier interface {
	Verify(ctx context.Context, id AgentIdentity) (bool, error)
}

// agentStore is implemented by *DB.
type agentStore interface {
	CreateAgentProfile(ctx context.Context, p AgentProfile, actor string, verified bool) (int64, error)
	ApproveAgentRoute(ctx context.Context, id int64, namespace, name, route, actor string) (AgentProfile, error)
	RevokeAgentProfile(ctx context.Context, namespace, name string, actor Person) error
	ActiveAgentProfiles(ctx context.Context) ([]AgentProfile, error)
	AgentProfileDetail(ctx context.Context, namespace, name string) (int64, AgentProfile, map[string]any, error)
}

var (
	errBadRequest = errors.New("bad request")
	errForbidden  = errors.New("forbidden")
	errNotFound   = errors.New("not found")
	errConflict   = errors.New("conflict")
)

// approvalValidity is fixed by styringsmodell v0.3: approvals expire after six months.
const approvalValidity = "6 months"

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func badf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errBadRequest, fmt.Sprintf(format, a...))
}

// validate checks the profile on its own; checkRoutes checks it against the
// published routes.
func (p AgentProfile) validate() error {
	for field, v := range map[string]string{"namespace": p.Namespace, "name": p.Name, "identity.cluster": p.Identity.Cluster,
		"identity.namespace": p.Identity.Namespace, "identity.app": p.Identity.App, "dpia.owner": p.DPIA.Owner} {
		if !dnsLabelRegex.MatchString(v) {
			return badf("%s must be a DNS label", field)
		}
	}
	if p.Identity.Namespace != p.Namespace {
		return badf("identity.namespace must be the owner team's namespace")
	}
	if !uuidRe.MatchString(p.Identity.EntraClientID) {
		return badf("identity.entra_client_id must be a lowercase UUID")
	}
	if p.Identity.StsSubject != "" {
		return badf("identity.sts_subject is not supported yet")
	}
	if p.Mode != "delegated" {
		return badf("mode must be delegated; agent principals are not allowed in phase 1")
	}
	if len(p.Grants) == 0 || len(p.Grants) > 20 {
		return badf("grants must list 1 to 20 routes")
	}
	seen := map[string]bool{}
	for _, g := range p.Grants {
		ns, name, ok := strings.Cut(g.Route, "/")
		if !ok || !dnsLabelRegex.MatchString(ns) || !dnsLabelRegex.MatchString(name) {
			return badf("grant route must be <namespace>/<name>")
		}
		if seen[g.Route] {
			return badf("route %s granted twice", g.Route)
		}
		seen[g.Route] = true
		if len(g.Tools) == 0 || len(g.Tools) > 50 {
			return badf("grant %s must list 1 to 50 tools", g.Route)
		}
		for i, t := range g.Tools {
			if !toolNameRegex.MatchString(t) { // same names as validateTools; no "*" can pass
				return badf("grant %s: tool %q is not a tool name", g.Route, t)
			}
			if slices.Contains(g.Tools[:i], t) {
				return badf("grant %s: tool %s listed twice", g.Route, t)
			}
		}
		if g.MaxRisk != "read" {
			return badf("grant %s: max_risk must be read", g.Route)
		}
	}
	q := p.Quota
	if q.MaxInFlight < 1 || q.MaxInFlight > 64 || q.RPM < 1 || q.RPM > 6000 ||
		q.DailyCallsPerUser < 1 || q.DailyCallsPerUser > 100000 || q.DailyCalls < 1 || q.DailyCalls > 1000000 {
		return badf("quota out of range")
	}
	if n := len(p.DPIA.Reference); n == 0 || n > 300 || strings.ContainsFunc(p.DPIA.Reference, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return badf("dpia.reference must be 1 to 300 printable characters")
	}
	return nil
}

// checkRoutes requires every granted route to be a published gateway route
// that lists every granted tool.
func (p AgentProfile) checkRoutes(servers []Server) error {
	for _, g := range p.Grants {
		i := slices.IndexFunc(servers, func(s Server) bool { return s.Key() == g.Route })
		if i < 0 || servers[i].Status != "published" || servers[i].Route == nil {
			return badf("route %s is not a published gateway route", g.Route)
		}
		for _, t := range g.Tools {
			if !slices.Contains(servers[i].Route.Tools, t) {
				return badf("route %s has no tool %s", g.Route, t)
			}
		}
	}
	return nil
}

func decodeStrict(r *http.Request, v any) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 64<<10+1))
	if err != nil || len(raw) > 64<<10 {
		return badf("body too large or unreadable")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return badf("invalid JSON: %v", err)
	}
	// More() is false before "]" or "}", so require EOF instead.
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return badf("trailing content")
	}
	return nil
}

// tokenPersons resolves a person from a user token obtained by the admin
// client, with teams from the team directory (#2678).
type tokenPersons struct {
	a             *authz
	adminClientID string
	teams         teamDirectory
}

func (t tokenPersons) Person(r *http.Request) (Person, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	ident, teams, err := resolvePerson(ctx, t.a, r.Header.Get("Authorization"), t.adminClientID, t.teams)
	return Person{Ident: ident, Teams: teams}, err
}

// agentRoutes mounts the write routes only with a resolver, which routes()
// builds only when the admin client is set, that is in dev-* clusters.
func (s *server) agentRoutes(mux *http.ServeMux, a *authz, persons personResolver) {
	mux.HandleFunc("GET /internal/v1/agents", a.require(s.listAgents, callerGateway))
	if persons == nil {
		return
	}
	mux.HandleFunc("POST /admin/v1/agents", withPerson(persons, s.createAgent))
	mux.HandleFunc("GET /admin/v1/agents/{namespace}/{name}", withPerson(persons, s.getAgent))
	mux.HandleFunc("POST /admin/v1/agents/{namespace}/{name}/approvals", withPerson(persons, s.approveAgent))
	mux.HandleFunc("POST /admin/v1/agents/{namespace}/{name}/revoke", withPerson(persons, s.revokeAgent))
	s.uiRoutes(mux, persons)
}

func (s *server) agentDB(w http.ResponseWriter, r *http.Request) (agentStore, store, context.Context, context.CancelFunc) {
	st, ctx, cancel := s.db(w, r)
	if st == nil {
		return nil, nil, nil, nil
	}
	as, ok := st.(agentStore)
	if !ok {
		cancel()
		http.Error(w, "agent profiles unavailable", http.StatusServiceUnavailable)
		return nil, nil, nil, nil
	}
	return as, st, ctx, cancel
}

func withPerson(persons personResolver, h func(http.ResponseWriter, *http.Request, Person)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := persons.Person(r)
		switch {
		case errors.Is(err, errUnauthorized) || (err == nil && p.Ident == ""):
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		case err != nil:
			slog.Error("person lookup failed", "error", err)
			http.Error(w, "identity lookup unavailable", http.StatusServiceUnavailable)
			return
		}
		h(w, r, p)
	}
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errBadRequest):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, errForbidden):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, errNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, errConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		// Strip line breaks: err can carry request-derived text (CodeQL go/log-injection).
		slog.Error("agent profile", "error", strings.NewReplacer("\n", " ", "\r", " ").Replace(err.Error()))
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
	}
}

func (s *server) createAgent(w http.ResponseWriter, r *http.Request, who Person) {
	var p AgentProfile
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, err)
		return
	}
	if p.Status != "" || p.ExpiresAt != nil {
		writeErr(w, badf("status and expires_at are set by fleet-control"))
		return
	}
	if err := p.validate(); err != nil {
		writeErr(w, err)
		return
	}
	if !who.in(p.Namespace) {
		writeErr(w, fmt.Errorf("%w: only team %s can create this profile", errForbidden, p.Namespace))
		return
	}
	as, st, ctx, cancel := s.agentDB(w, r)
	if as == nil {
		return
	}
	defer cancel()
	servers, err := st.Servers(ctx)
	if err == nil {
		err = p.checkRoutes(servers)
	}
	verified := false
	if err == nil && s.verifier != nil {
		if verified, err = s.verifier.Verify(ctx, p.Identity); err == nil && !verified {
			err = badf("entra_client_id does not belong to %s:%s:%s", p.Identity.Cluster, p.Identity.Namespace, p.Identity.App)
		}
	}
	var id int64
	if err == nil {
		id, err = as.CreateAgentProfile(ctx, p, who.Ident, verified)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	// Approvers send this id back, so consent binds to this draft, not the name.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id})
}

func (s *server) approveAgent(w http.ResponseWriter, r *http.Request, who Person) {
	var in struct {
		Route string `json:"route"` // a granted route, or the profile's namespace for the owner team's approval
		ID    int64  `json:"id"`    // the draft the approver reviewed
	}
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, err)
		return
	}
	routeTeam, _, _ := strings.Cut(in.Route, "/")
	if !who.in(routeTeam) {
		writeErr(w, fmt.Errorf("%w: only the route's owning team, or the owner team for its namespace, can approve", errForbidden))
		return
	}
	as, _, ctx, cancel := s.agentDB(w, r)
	if as == nil {
		return
	}
	defer cancel()
	p, err := as.ApproveAgentRoute(ctx, in.ID, r.PathValue("namespace"), r.PathValue("name"), in.Route, who.Ident)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"status": p.Status, "expires_at": p.ExpiresAt})
}

// getAgent shows the live profile, its id and per-route approvals, so an
// approver can review it and send the id back. Only the owner team and the
// teams owning a granted route see it; everyone else gets 404.
func (s *server) getAgent(w http.ResponseWriter, r *http.Request, who Person) {
	as, _, ctx, cancel := s.agentDB(w, r)
	if as == nil {
		return
	}
	defer cancel()
	id, p, approvals, err := as.AgentProfileDetail(ctx, r.PathValue("namespace"), r.PathValue("name"))
	if err == nil && !p.visibleTo(who) {
		err = errNotFound
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"id": id, "status": p.Status, "profile": p, "approvals": approvals})
}

// visibleTo is true for the owner team and the owning team of any granted route.
func (p AgentProfile) visibleTo(who Person) bool {
	return who.in(p.Namespace) || slices.ContainsFunc(p.Grants, func(g AgentGrant) bool {
		team, _, _ := strings.Cut(g.Route, "/")
		return who.in(team)
	})
}

func (s *server) revokeAgent(w http.ResponseWriter, r *http.Request, who Person) {
	as, _, ctx, cancel := s.agentDB(w, r)
	if as == nil {
		return
	}
	defer cancel()
	if err := as.RevokeAgentProfile(ctx, r.PathValue("namespace"), r.PathValue("name"), who); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listAgents serves approved, verified, unexpired profiles to fleet-gateway. The
// database filters on expiry at read time, so an expired profile drops out
// without a job.
func (s *server) listAgents(w http.ResponseWriter, r *http.Request) {
	as, _, ctx, cancel := s.agentDB(w, r)
	if as == nil {
		return
	}
	defer cancel()
	agents, err := as.ActiveAgentProfiles(ctx)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"schema_version": 1, "agents": agents})
}

// --- storage ---

func auditAgent(ctx context.Context, tx pgx.Tx, actor, action, namespace, name string, detail any) error {
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_events (actor, action, namespace, name, detail) VALUES ($1, $2, $3, $4, $5::jsonb)`,
		actor, action, namespace, name, string(raw))
	return err
}

func (d *DB) CreateAgentProfile(ctx context.Context, p AgentProfile, actor string, verified bool) (int64, error) {
	var id int64
	grants, err := json.Marshal(p.Grants)
	if err != nil {
		return 0, err
	}
	err = pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		// An expired profile must not block a new one with its name or client id.
		rows, qErr := tx.Query(ctx, `UPDATE agent_profiles SET status = 'expired'
			WHERE status = 'approved' AND expires_at <= now()
			  AND ((namespace = $1 AND name = $2) OR entra_client_id = $3::uuid)
			RETURNING id, namespace, name`, p.Namespace, p.Name, p.Identity.EntraClientID)
		if qErr != nil {
			return qErr
		}
		type expired struct {
			ID       int64
			NS, Name string
		}
		gone, cErr := pgx.CollectRows(rows, pgx.RowToStructByPos[expired])
		if cErr != nil {
			return cErr
		}
		for _, g := range gone {
			if aErr := auditAgent(ctx, tx, actor, "agent_profile_expired", g.NS, g.Name, map[string]any{"id": g.ID}); aErr != nil {
				return aErr
			}
		}
		if execErr := tx.QueryRow(ctx, `INSERT INTO agent_profiles (namespace, name, cluster, app, entra_client_id, mode, grants,
			max_in_flight, rpm, daily_calls_per_user, daily_calls, dpia_owner, dpia_ref, status, created_by, identity_verified_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13, 'draft', $14, CASE WHEN $15 THEN now() END) RETURNING id`,
			p.Namespace, p.Name, p.Identity.Cluster, p.Identity.App, p.Identity.EntraClientID, p.Mode, string(grants),
			p.Quota.MaxInFlight, p.Quota.RPM, p.Quota.DailyCallsPerUser, p.Quota.DailyCalls, p.DPIA.Owner, p.DPIA.Reference, actor, verified).Scan(&id); execErr != nil {
			return execErr
		}
		return auditAgent(ctx, tx, actor, "agent_profile_drafted", p.Namespace, p.Name, map[string]any{"profile": p, "identity_verified": verified})
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		if pgErr.ConstraintName == "agent_profiles_live_client" {
			return 0, fmt.Errorf("%w: entra_client_id is in another verified live profile", errConflict)
		}
		return 0, fmt.Errorf("%w: profile %s/%s exists", errConflict, p.Namespace, p.Name)
	}
	return id, err
}

const agentCols = `id, namespace, name, cluster, app, entra_client_id::text, COALESCE(sts_subject, ''), mode, grants::text,
	max_in_flight, rpm, daily_calls_per_user, daily_calls, dpia_owner, dpia_ref, status, expires_at, created_by`

func scanAgent(row pgx.Row) (int64, AgentProfile, error) {
	var id int64
	var p AgentProfile
	var grants string
	err := row.Scan(&id, &p.Namespace, &p.Name, &p.Identity.Cluster, &p.Identity.App, &p.Identity.EntraClientID, &p.Identity.StsSubject,
		&p.Mode, &grants, &p.Quota.MaxInFlight, &p.Quota.RPM, &p.Quota.DailyCallsPerUser, &p.Quota.DailyCalls,
		&p.DPIA.Owner, &p.DPIA.Reference, &p.Status, &p.ExpiresAt, &p.CreatedBy)
	if err != nil {
		return 0, p, err
	}
	p.Identity.Namespace = p.Namespace
	return id, p, json.Unmarshal([]byte(grants), &p.Grants)
}

// lockLive loads the live (non-revoked) profile for update.
func lockLive(ctx context.Context, tx pgx.Tx, namespace, name string) (int64, AgentProfile, error) {
	id, p, err := scanAgent(tx.QueryRow(ctx, `SELECT `+agentCols+` FROM agent_profiles
		WHERE namespace = $1 AND name = $2 AND status IN ('draft', 'approved') FOR UPDATE`, namespace, name))
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, p, errNotFound
	}
	return id, p, err
}

// ApproveAgentRoute records one approval. route is a granted route (its owner
// approves) or the profile's namespace (a second member of the owner team
// approves, styringsmodell v0.3). The caller has checked that actor belongs
// to that team. The profile becomes approved, with a six-month expiry, once
// the owner team and every granted route have approved.
func (d *DB) ApproveAgentRoute(ctx context.Context, wantID int64, namespace, name, route, actor string) (AgentProfile, error) {
	var out AgentProfile
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		id, p, err := lockLive(ctx, tx, namespace, name)
		if err != nil {
			return err
		}
		if id != wantID {
			return fmt.Errorf("%w: the live profile is %d, not the one approved", errConflict, id)
		}
		if route != p.Namespace && !slices.ContainsFunc(p.Grants, func(g AgentGrant) bool { return g.Route == route }) {
			return badf("profile grants nothing on %s", route)
		}
		// Routes can be revoked or lose tools between draft and approval. FOR
		// SHARE waits for, and blocks, write.go's FOR UPDATE on the same rows.
		for _, g := range p.Grants {
			ns, n, _ := strings.Cut(g.Route, "/")
			if _, err = tx.Exec(ctx, `SELECT 1 FROM servers WHERE namespace = $1 AND name = $2 FOR SHARE`, ns, n); err != nil {
				return err
			}
		}
		servers, err := queryServers(ctx, tx)
		if err != nil {
			return err
		}
		if err = p.checkRoutes(servers); err != nil {
			return err
		}
		if actor == p.CreatedBy {
			return fmt.Errorf("%w: the author cannot approve their own profile", errForbidden)
		}
		// "seed" is not a person, so it cannot be the first of the two people
		// the owner team needs. A human revokes and re-submits the draft.
		if p.CreatedBy == "seed" {
			return fmt.Errorf("%w: a seeded draft must be re-submitted by a person before approval", errForbidden)
		}
		tag, err := tx.Exec(ctx, `INSERT INTO agent_profile_approvals (profile_id, route, approved_by) VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`, id, route, actor)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: %s already approved", errConflict, route)
		}
		if err := auditAgent(ctx, tx, actor, "agent_profile_route_approved", namespace, name, map[string]any{"id": id, "route": route}); err != nil {
			return err
		}
		var approved int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM agent_profile_approvals WHERE profile_id = $1`, id).Scan(&approved); err != nil {
			return err
		}
		p.Status = "draft"
		if approved == len(p.Grants)+1 {
			if err := tx.QueryRow(ctx, `UPDATE agent_profiles SET status = 'approved', approved_at = now(),
				expires_at = now() + $2::interval WHERE id = $1 RETURNING status, expires_at`, id, approvalValidity).Scan(&p.Status, &p.ExpiresAt); err != nil {
				return err
			}
			if err := auditAgent(ctx, tx, actor, "agent_profile_approved", namespace, name, map[string]any{"id": id, "expires_at": p.ExpiresAt}); err != nil {
				return err
			}
		}
		out = p
		return nil
	})
	return out, err
}

// RevokeAgentProfile revokes a draft or approved profile. The owner team or
// the owning team of any granted route may revoke. Revocation is final.
func (d *DB) RevokeAgentProfile(ctx context.Context, namespace, name string, actor Person) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		id, p, err := lockLive(ctx, tx, namespace, name)
		if err != nil {
			return err
		}
		if !p.visibleTo(actor) {
			return fmt.Errorf("%w: only the owner team or a route owner can revoke", errForbidden)
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_profiles SET status = 'revoked', revoked_by = $2, revoked_at = now() WHERE id = $1`, id, actor.Ident); err != nil {
			return err
		}
		return auditAgent(ctx, tx, actor.Ident, "agent_profile_revoked", namespace, name, map[string]any{"id": id, "was": p.Status})
	})
}

// AgentProfileDetail returns the live profile, its id, and per granted route
// and for the owner team (keyed by namespace) either null or who approved it
// and when.
func (d *DB) AgentProfileDetail(ctx context.Context, namespace, name string) (int64, AgentProfile, map[string]any, error) {
	id, p, err := scanAgent(d.pool.QueryRow(ctx, `SELECT `+agentCols+` FROM agent_profiles
		WHERE namespace = $1 AND name = $2 AND status IN ('draft', 'approved')`, namespace, name))
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, p, nil, errNotFound
	}
	if err != nil {
		return 0, p, nil, err
	}
	approvals := map[string]any{p.Namespace: nil}
	for _, g := range p.Grants {
		approvals[g.Route] = nil
	}
	rows, err := d.pool.Query(ctx, `SELECT route, approved_by, approved_at FROM agent_profile_approvals WHERE profile_id = $1`, id)
	if err != nil {
		return 0, p, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var route, by string
		var at time.Time
		if err := rows.Scan(&route, &by, &at); err != nil {
			return 0, p, nil, err
		}
		approvals[route] = map[string]any{"approved_by": by, "approved_at": at}
	}
	return id, p, approvals, rows.Err()
}

// ActiveAgentProfiles returns approved profiles that have not expired.
func (d *DB) ActiveAgentProfiles(ctx context.Context) ([]AgentProfile, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+agentCols+` FROM agent_profiles
		WHERE status = 'approved' AND expires_at > now() AND identity_verified_at IS NOT NULL ORDER BY namespace, name`)
	if err != nil {
		return nil, err
	}
	out := []AgentProfile{}
	_, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (struct{}, error) {
		_, p, scanErr := scanAgent(row)
		out = append(out, p)
		return struct{}{}, scanErr
	})
	return out, err
}

// seedAgentsFromEnv imports SEED_AGENTS_FILE, a JSON list of profiles, as
// drafts by "seed". Set only in dev; refused outside dev-* clusters. A profile is skipped when any profile
// with its name exists. Routes are checked at approval, not here, so a draft
// may name a route the cluster does not have yet.
func seedAgentsFromEnv(ctx context.Context, db *DB) error {
	path := os.Getenv("SEED_AGENTS_FILE")
	if path == "" {
		return nil
	}
	if !devCluster(os.Getenv("NAIS_CLUSTER_NAME")) {
		return errors.New("SEED_AGENTS_FILE is for dev-* clusters only")
	}
	raw, err := os.ReadFile(path) //nolint:gosec // operator-set path baked into the image
	if err != nil {
		return err
	}
	var profiles []AgentProfile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&profiles); err != nil {
		return fmt.Errorf("seed agents: %w", err)
	}
	for _, p := range profiles {
		if err := p.validate(); err != nil {
			return fmt.Errorf("seed agent %s/%s: %w", p.Namespace, p.Name, err)
		}
		var exists bool
		if err := db.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_profiles WHERE namespace = $1 AND name = $2)`, p.Namespace, p.Name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := db.CreateAgentProfile(ctx, p, "seed", false); err != nil {
			return fmt.Errorf("seed agent %s/%s: %w", p.Namespace, p.Name, err)
		}
	}
	return nil
}
