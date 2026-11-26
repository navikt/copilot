package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/navikt/copilot/fleet-control/internal/canon"
	"github.com/navikt/copilot/fleet-control/internal/model"
	"github.com/navikt/copilot/fleet-control/internal/store"
)

// TypedRef points at another resource, as targetRefs in Gateway API.
type TypedRef struct {
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (r TypedRef) key() string { return r.Namespace + "/" + r.Name }

// AgentIdentityV1 is the v1 wire form of AgentIdentity.
type AgentIdentityV1 struct {
	App           string `json:"app"`
	Cluster       string `json:"cluster"`
	EntraClientID string `json:"entraClientID"`
	StsSubject    string `json:"stsSubject,omitempty"`
}

// AgentGrantV1 is the v1 wire form of AgentGrant.
type AgentGrantV1 struct {
	ServerRef    TypedRef `json:"serverRef"`
	AllowedTools []string `json:"allowedTools"`
	MaxRisk      string   `json:"maxRisk"`
}

// AgentQuotaV1 is the v1 wire form of AgentQuota.
type AgentQuotaV1 struct {
	MaxInFlight       int `json:"maxInFlight"`
	RequestsPerMinute int `json:"requestsPerMinute"`
	DailyCallsPerUser int `json:"dailyCallsPerUser"`
	DailyCalls        int `json:"dailyCalls"`
}

// AgentDPIAV1 is the v1 wire form of AgentDPIA.
type AgentDPIAV1 struct {
	Owner     string `json:"owner"`
	Reference string `json:"reference"`
}

// AgentProfileSpec is the spec of an AgentProfile resource.
type AgentProfileSpec struct {
	Identity AgentIdentityV1 `json:"identity"`
	Mode     string          `json:"mode"`
	Grants   []AgentGrantV1  `json:"grants,omitempty"`
	Quota    AgentQuotaV1    `json:"quota"`
	DPIA     *AgentDPIAV1    `json:"dpia,omitempty"`
}

// GrantStatus is the approval state of one grant.
type GrantStatus struct {
	ServerRef    TypedRef   `json:"serverRef"`
	ContentHash  string     `json:"contentHash,omitempty"`
	Granted      bool       `json:"granted"`
	GrantedBy    *Actor     `json:"grantedBy,omitempty"`
	GrantedAt    *time.Time `json:"grantedAt,omitempty"`
	MissingTools []string   `json:"missingTools,omitempty"`
	Reason       string     `json:"reason,omitempty"`
}

// AgentProfileStatus is the status of an AgentProfile resource.
type AgentProfileStatus struct {
	ObservedGeneration   int64                      `json:"observedGeneration,omitempty"`
	PublishedGeneration  int                        `json:"publishedGeneration,omitempty"`
	PublishedContentHash string                     `json:"publishedContentHash,omitempty"`
	ContentHash          string                     `json:"contentHash,omitempty"` // generations/{n} only
	SubmittedBy          *Actor                     `json:"submittedBy,omitempty"`
	ApprovedAt           *time.Time                 `json:"approvedAt,omitempty"`
	IdentityVerifiedAt   *time.Time                 `json:"identityVerifiedAt,omitempty"`
	ExpiresAt            *time.Time                 `json:"expiresAt,omitempty"`
	Pending              *Pending[AgentProfileSpec] `json:"pending,omitempty"`
	// NeedsResubmit: the pending generation has no person as submitter (a
	// seeded draft), so nobody can approve it until a person submits it again.
	NeedsResubmit bool          `json:"needsResubmit,omitempty"`
	Grants        []GrantStatus `json:"grants,omitempty"`
	Conditions    []Condition   `json:"conditions,omitempty"`
}

// AgentProfileV1 is the AgentProfile resource.
type AgentProfileV1 struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Metadata   ObjectMeta         `json:"metadata"`
	Spec       AgentProfileSpec   `json:"spec"`
	Status     AgentProfileStatus `json:"status"`
}

// validateProfileSpec checks spec on its own and returns it normalised:
// grants sorted by serverRef, tools sorted.
func validateProfileSpec(spec AgentProfileSpec) (AgentProfileSpec, error) {
	id := spec.Identity
	switch {
	case !dnsLabelRegex.MatchString(id.App):
		return spec, invalid("spec.identity.app", "must be a lowercase DNS label")
	case !dnsLabelRegex.MatchString(id.Cluster):
		return spec, invalid("spec.identity.cluster", "must be a lowercase DNS label")
	case !uuidRe.MatchString(id.EntraClientID):
		return spec, invalid("spec.identity.entraClientID", "must be a lowercase UUID")
	case id.StsSubject != "":
		return spec, invalid("spec.identity.stsSubject", "is reserved until the Nais STS issuer is chosen")
	case spec.Mode != "Delegated":
		return spec, invalid("spec.mode", "must be Delegated in phase 1")
	case len(spec.Grants) == 0 || len(spec.Grants) > 20:
		return spec, invalid("spec.grants", "must list 1 to 20 grants")
	}
	grants := make([]AgentGrantV1, 0, len(spec.Grants))
	seen := map[string]bool{}
	for _, g := range spec.Grants {
		ref := g.ServerRef
		if ref.Group != apiGroup || ref.Kind != "MCPServer" || !dnsLabelRegex.MatchString(ref.Namespace) || !dnsLabelRegex.MatchString(ref.Name) {
			return spec, invalid("spec.grants.serverRef", "serverRef must be group fleet.nav.no, kind MCPServer, and DNS-label namespace and name")
		}
		if seen[ref.key()] {
			return spec, invalid("spec.grants.serverRef", "MCP server %s is granted twice", ref.key())
		}
		seen[ref.key()] = true
		if len(g.AllowedTools) == 0 || len(g.AllowedTools) > 50 {
			return spec, invalid("spec.grants.allowedTools", "grant %s must list 1 to 50 tools", ref.key())
		}
		for i, t := range g.AllowedTools {
			if !toolNameRegex.MatchString(t) || slices.Contains(g.AllowedTools[:i], t) {
				return spec, invalid("spec.grants.allowedTools", "grant %s: %q is not a tool name, or is listed twice", ref.key(), t)
			}
		}
		if g.MaxRisk != "Read" {
			return spec, invalid("spec.grants.maxRisk", "must be Read in phase 1")
		}
		g.AllowedTools = slices.Sorted(slices.Values(g.AllowedTools))
		grants = append(grants, g)
	}
	slices.SortFunc(grants, func(a, b AgentGrantV1) int { return strings.Compare(a.ServerRef.key(), b.ServerRef.key()) })
	spec.Grants = grants
	q := spec.Quota
	if q.MaxInFlight < 1 || q.MaxInFlight > 64 || q.RequestsPerMinute < 1 || q.RequestsPerMinute > 6000 ||
		q.DailyCallsPerUser < 1 || q.DailyCallsPerUser > 100000 || q.DailyCalls < 1 || q.DailyCalls > 1000000 {
		return spec, invalid("spec.quota", "quota out of range")
	}
	if spec.DPIA == nil || !dnsLabelRegex.MatchString(spec.DPIA.Owner) {
		return spec, invalid("spec.dpia.owner", "must be a team (DNS label)")
	}
	if n := len(spec.DPIA.Reference); n == 0 || n > 300 || strings.ContainsFunc(spec.DPIA.Reference, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return spec, invalid("spec.dpia.reference", "must be 1 to 300 printable characters")
	}
	return spec, nil
}

// grantHash covers what the server's owner team sees: identity, mode,
// quota and their one grant (owner decision 13).
func grantHash(spec AgentProfileSpec, g AgentGrantV1) (string, error) {
	return canon.Hash(struct {
		Identity AgentIdentityV1 `json:"identity"`
		Mode     string          `json:"mode"`
		Quota    AgentQuotaV1    `json:"quota"`
		Grant    AgentGrantV1    `json:"grant"`
	}{spec.Identity, spec.Mode, spec.Quota, g})
}

// checkGrantServers requires every grant to name a published, unrevoked
// gateway server that lists every granted tool.
func checkGrantServers(ctx context.Context, q store.Querier, grants []AgentGrantV1) error {
	servers, err := store.QueryServers(ctx, q)
	if err != nil {
		return err
	}
	for _, g := range grants {
		i := slices.IndexFunc(servers, func(s model.Server) bool { return s.Key() == g.ServerRef.key() })
		if i < 0 || servers[i].Status != "published" || servers[i].Route == nil {
			return invalid("spec.grants.serverRef", "MCP server %s is not published behind fleet-gateway", g.ServerRef.key())
		}
		for _, t := range g.AllowedTools {
			if !slices.Contains(servers[i].Route.Tools, t) {
				return invalid("spec.grants.allowedTools", "MCP server %s has no tool %s", g.ServerRef.key(), t)
			}
		}
	}
	return nil
}

// --- handlers ---

func listProfilesNS(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns := r.PathValue("namespace")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	items, err := db.profileObjects(r.Context(), ns)
	if err != nil {
		return err
	}
	writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "AgentProfileList", "items": emptyIfNil(items)})
	return nil
}

// listAwaitingProfiles answers ?awaitingApprovalBy=me with excerpts of the
// profiles whose pending generation waits for a grant from the caller's teams.
func listAwaitingProfiles(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	if r.URL.Query().Get("awaitingApprovalBy") != "me" {
		return stErr(http.StatusBadRequest, "BadRequest", "only ?awaitingApprovalBy=me is supported")
	}
	items, err := db.awaitingProfiles(r.Context(), c)
	if err != nil {
		return err
	}
	writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "AgentProfileList", "items": emptyIfNil(items)})
	return nil
}

func getProfile(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	// A non-member learns nothing from timing: a hidden profile and a missing
	// one both cost this one query (#2696). Like excerpt, it reads the pending
	// generation when there is one.
	if !c.in(ns) {
		var shared bool
		if err := db.pool.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM agent_profile_resources r
			JOIN agent_profile_generations g ON g.namespace = r.namespace AND g.name = r.name AND g.generation = COALESCE(r.pending_generation, r.published_generation)
			CROSS JOIN jsonb_object_keys(g.grant_hashes) k
			WHERE r.namespace = $1 AND r.name = $2 AND split_part(k, '/', 1) = ANY($3))`, ns, name, c.Teams).Scan(&shared); err != nil {
			return err
		}
		if !shared {
			return errV1NotFound
		}
	}
	obj, err := db.profileObject(r.Context(), ns, name)
	switch {
	case errors.Is(err, errV1NotFound):
		return errV1NotFound
	case err != nil:
		return err
	case c.in(ns):
		w.Header().Set("ETag", `"`+obj.Metadata.ResourceVersion+`"`)
		writeObject(w, http.StatusOK, obj)
		return nil
	}
	ex, ok := excerpt(obj, c)
	if !ok {
		return errV1NotFound
	}
	writeObject(w, http.StatusOK, ex)
	return nil
}

// excerpt is what a server's owner team sees of another team's profile:
// identity, mode, quota and only their own grants, with the pending
// generation and the hash for those grants. No actors, history or other
// teams' grants.
func excerpt(obj AgentProfileV1, c caller) (AgentProfileV1, bool) {
	spec := obj.Spec
	if p := obj.Status.Pending; p != nil && p.Spec != nil {
		spec = *p.Spec
	}
	out := AgentProfileV1{APIVersion: obj.APIVersion, Kind: obj.Kind, Metadata: ObjectMeta{Name: obj.Metadata.Name, Namespace: obj.Metadata.Namespace}}
	out.Spec = AgentProfileSpec{Identity: spec.Identity, Mode: spec.Mode, Quota: spec.Quota}
	for _, g := range spec.Grants {
		if c.in(g.ServerRef.Namespace) {
			out.Spec.Grants = append(out.Spec.Grants, g)
		}
	}
	if len(out.Spec.Grants) == 0 {
		return out, false
	}
	for _, g := range obj.Status.Grants {
		if c.in(g.ServerRef.Namespace) {
			g.GrantedBy = nil
			out.Status.Grants = append(out.Status.Grants, g)
		}
	}
	if p := obj.Status.Pending; p != nil {
		out.Status.Pending = &Pending[AgentProfileSpec]{Generation: p.Generation}
	}
	// An expired profile stays visible with its expiry, as the owner team sees it (#2696).
	out.Status.ExpiresAt = obj.Status.ExpiresAt
	out.Status.NeedsResubmit = obj.Status.NeedsResubmit
	return out, true
}

func getProfileGeneration(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	n, err := pathGeneration(r)
	if err != nil {
		return err
	}
	obj, err := db.profileObject(r.Context(), ns, name)
	if err != nil {
		return err
	}
	g, err := loadProfileGen(r.Context(), db.pool, ns, name, n)
	if err != nil {
		return err
	}
	obj.Spec = g.Spec
	obj.Metadata.Generation = int64(n)
	obj.Status = AgentProfileStatus{ObservedGeneration: obj.Status.ObservedGeneration, ContentHash: g.Hash, SubmittedBy: g.submitter(), ApprovedAt: g.ApprovedAt,
		IdentityVerifiedAt: g.VerifiedAt, ExpiresAt: g.ExpiresAt, Grants: grantHashStatus(g)}
	writeObject(w, http.StatusOK, obj)
	return nil
}

func grantHashStatus(g profileGen) []GrantStatus {
	out := make([]GrantStatus, 0, len(g.Spec.Grants))
	for _, gr := range g.Spec.Grants {
		out = append(out, GrantStatus{ServerRef: gr.ServerRef, ContentHash: g.GrantHashes[gr.ServerRef.key()]})
	}
	return out
}

func (s *server) createProfile(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	return s.submitProfileHTTP(w, r, c, db, true)
}

func (s *server) putProfile(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	return s.submitProfileHTTP(w, r, c, db, false)
}

func (s *server) submitProfileHTTP(w http.ResponseWriter, r *http.Request, c caller, db *DB, create bool) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	in, err := decodeResource[AgentProfileSpec](w, r, "AgentProfile", ns, name)
	if err != nil {
		return err
	}
	spec, err := validateProfileSpec(in.Spec)
	if err != nil {
		return err
	}
	if err = checkGrantServers(r.Context(), db.pool, spec.Grants); err != nil {
		return err
	}
	verified := false
	if s.verifier != nil {
		id := AgentIdentity{Cluster: spec.Identity.Cluster, Namespace: ns, App: spec.Identity.App, EntraClientID: spec.Identity.EntraClientID}
		if verified, err = s.verifier.Verify(r.Context(), id); err != nil {
			return err
		}
		if !verified {
			return invalid("spec.identity.entraClientID", "the client id does not belong to %s:%s:%s", id.Cluster, ns, id.App)
		}
	}
	if err = db.submitProfile(r.Context(), c, in.Metadata, spec, verified, create); err != nil {
		return err
	}
	obj, err := db.profileObject(r.Context(), ns, in.Metadata.Name)
	if err != nil {
		return err
	}
	code := http.StatusOK
	if create {
		code = http.StatusCreated
	}
	w.Header().Set("ETag", `"`+obj.Metadata.ResourceVersion+`"`)
	writeObject(w, code, obj)
	return nil
}

// grantTeamGuard lets only the owner team of ref act on its grant. A member
// of the profile's team gets 403, anyone else 404.
func grantTeamGuard(c caller, ns string, ref TypedRef) error {
	switch {
	case !dnsLabelRegex.MatchString(ref.Namespace) || !dnsLabelRegex.MatchString(ref.Name):
		return stErr(http.StatusBadRequest, "BadRequest", "serverRef must have DNS-label namespace and name")
	case c.in(ref.Namespace):
		return nil
	case c.in(ns):
		return stErr(http.StatusForbidden, "Forbidden", "only the owner team of MCP server %s can act on its grant", ref.key())
	}
	return errV1NotFound
}

func approveProfile(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	var body struct {
		Generation  int      `json:"generation"`
		ServerRef   TypedRef `json:"serverRef"`
		ContentHash string   `json:"contentHash"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		return stErr(http.StatusBadRequest, "BadRequest", "%v", err)
	}
	if err := grantTeamGuard(c, ns, body.ServerRef); err != nil {
		return err
	}
	if err := db.grantProfile(r.Context(), c, ns, name, body.Generation, body.ServerRef, body.ContentHash); err != nil {
		return err
	}
	return writeProfileFor(r.Context(), w, db, c, ns, name)
}

func (s *server) revokeProfile(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	var body struct {
		Reason    string    `json:"reason"`
		ServerRef *TypedRef `json:"serverRef"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		return stErr(http.StatusBadRequest, "BadRequest", "%v", err)
	}
	by := ""
	var err error
	if body.ServerRef != nil {
		by, err = "server-owner", grantTeamGuard(c, ns, *body.ServerRef)
	} else {
		by, err = s.operatorOr(c, ns)
	}
	if err != nil {
		return err
	}
	reason, err := reasonText(body.Reason)
	if err != nil {
		return err
	}
	if err := db.revokeProfileV1(r.Context(), c, by, ns, name, reason, body.ServerRef); err != nil {
		return err
	}
	return writeProfileFor(r.Context(), w, db, c, ns, name)
}

func writeProfileFor(ctx context.Context, w http.ResponseWriter, db *DB, c caller, ns, name string) error {
	obj, err := db.profileObject(ctx, ns, name)
	if err != nil {
		return err
	}
	if !c.in(ns) {
		ex, ok := excerpt(obj, c)
		if !ok {
			// The platform team stopping a profile sees only its identity and state.
			ex = AgentProfileV1{APIVersion: obj.APIVersion, Kind: obj.Kind, Metadata: ObjectMeta{Name: name, Namespace: ns}}
			ex.Status.Conditions = obj.Status.Conditions
		}
		obj = ex
	}
	writeObject(w, http.StatusOK, obj)
	return nil
}

// --- storage ---

type profileRow struct {
	NS, Name, UID      string
	RV                 int64
	Labels, Annots     string
	Created            time.Time
	Published, Pending *int
	Revoked            bool
	Latest             int
}

const profileRowCols = `r.namespace, r.name, r.uid::text, r.resource_version, r.labels::text, r.annotations::text, r.created_at,
	r.published_generation, r.pending_generation, r.revoked,
	(SELECT COALESCE(max(generation), 0) FROM agent_profile_generations g WHERE g.namespace = r.namespace AND g.name = r.name)`

func scanProfileRow(row pgx.Row) (profileRow, error) {
	var p profileRow
	err := row.Scan(&p.NS, &p.Name, &p.UID, &p.RV, &p.Labels, &p.Annots, &p.Created, &p.Published, &p.Pending, &p.Revoked, &p.Latest)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, errV1NotFound
	}
	return p, err
}

type profileGen struct {
	Gen                   int
	Spec                  AgentProfileSpec
	Hash                  string
	ClientID              string
	SubName               string
	SubKind               string
	SubOid                *string
	ApprovedAt, ExpiresAt *time.Time
	VerifiedAt, RevokedAt *time.Time
	CreatedAt             time.Time
	Blocked               bool
	GrantHashes           map[string]string
	grants                map[string]grantRow
}

type grantRow struct {
	By        string
	Kind      string
	At        time.Time
	RevokedAt *time.Time
}

func (g profileGen) submitter() *Actor { return &Actor{Kind: g.SubKind, Name: g.SubName} }

func (g profileGen) pendingExpiresAt() time.Time { return g.CreatedAt.Add(pendingValidity) }

func loadProfileGen(ctx context.Context, q store.Querier, ns, name string, gen int) (profileGen, error) {
	var g profileGen
	var spec, hashes string
	err := q.QueryRow(ctx, `SELECT generation, spec::text, spec_hash, entra_client_id::text, submitted_by, submitted_by_kind, submitted_by_oid::text,
		approved_at, expires_at, identity_verified_at, revoked_at, blocked, grant_hashes::text, created_at
		FROM agent_profile_generations WHERE namespace = $1 AND name = $2 AND generation = $3`, ns, name, gen).
		Scan(&g.Gen, &spec, &g.Hash, &g.ClientID, &g.SubName, &g.SubKind, &g.SubOid, &g.ApprovedAt, &g.ExpiresAt, &g.VerifiedAt, &g.RevokedAt, &g.Blocked, &hashes, &g.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, errV1NotFound
	}
	if err != nil {
		return g, err
	}
	if err = json.Unmarshal([]byte(spec), &g.Spec); err != nil {
		return g, err
	}
	if err = json.Unmarshal([]byte(hashes), &g.GrantHashes); err != nil {
		return g, err
	}
	rows, err := q.Query(ctx, `SELECT server, granted_by, granted_by_kind, granted_at, revoked_at FROM agent_profile_grants
		WHERE namespace = $1 AND name = $2 AND generation = $3`, ns, name, gen)
	if err != nil {
		return g, err
	}
	g.grants = map[string]grantRow{}
	_, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (struct{}, error) {
		var key string
		var gr grantRow
		scanErr := row.Scan(&key, &gr.By, &gr.Kind, &gr.At, &gr.RevokedAt)
		g.grants[key] = gr
		return struct{}{}, scanErr
	})
	return g, err
}

// grantStatus is the live state of each grant in g.
func (d *DB) grantStatus(ctx context.Context, g profileGen) ([]GrantStatus, error) {
	servers, err := store.QueryServers(ctx, d.pool)
	if err != nil {
		return nil, err
	}
	out := make([]GrantStatus, 0, len(g.Spec.Grants))
	for _, gr := range g.Spec.Grants {
		key := gr.ServerRef.key()
		st := GrantStatus{ServerRef: gr.ServerRef, ContentHash: g.GrantHashes[key]}
		row, granted := g.grants[key]
		if granted {
			at := row.At.UTC()
			st.Granted, st.GrantedBy, st.GrantedAt = row.RevokedAt == nil, &Actor{Kind: row.Kind, Name: row.By}, &at
			if row.RevokedAt != nil {
				st.Reason = "AccessRevoked"
			}
			// A server revoked after the grant stops it; republishing does not bring it back.
			var revokedSince bool
			if err := d.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM server_versions WHERE namespace = $1 AND name = $2 AND revoked_at > $3)`,
				gr.ServerRef.Namespace, gr.ServerRef.Name, row.At).Scan(&revokedSince); err != nil {
				return nil, err
			}
			if revokedSince {
				st.Granted, st.Reason = false, "ServerRevoked"
			}
		}
		if i := slices.IndexFunc(servers, func(s model.Server) bool { return s.Key() == key }); i >= 0 && servers[i].Route != nil {
			for _, t := range gr.AllowedTools {
				if !slices.Contains(servers[i].Route.Tools, t) {
					st.MissingTools = append(st.MissingTools, t)
				}
			}
		}
		out = append(out, st)
	}
	return out, nil
}

func (d *DB) profileObject(ctx context.Context, ns, name string) (AgentProfileV1, error) {
	row, err := scanProfileRow(d.pool.QueryRow(ctx, `SELECT `+profileRowCols+` FROM agent_profile_resources r WHERE r.namespace = $1 AND r.name = $2`, ns, name))
	if err != nil {
		return AgentProfileV1{}, err
	}
	return d.buildProfile(ctx, row)
}

func (d *DB) profileObjects(ctx context.Context, ns string) ([]AgentProfileV1, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+profileRowCols+` FROM agent_profile_resources r WHERE r.namespace = $1 ORDER BY r.name`, ns)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (profileRow, error) { return scanProfileRow(row) })
	if err != nil {
		return nil, err
	}
	out := make([]AgentProfileV1, 0, len(list))
	for _, row := range list {
		obj, err := d.buildProfile(ctx, row)
		if err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	return out, nil
}

// awaitingProfiles finds pending generations with a grant the caller's teams
// have not given. shortcut: filters in Go over every pending profile; fine
// at dev scale, move the filter to SQL when profiles number in the hundreds.
func (d *DB) awaitingProfiles(ctx context.Context, c caller) ([]AgentProfileV1, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+profileRowCols+` FROM agent_profile_resources r WHERE r.pending_generation IS NOT NULL ORDER BY r.namespace, r.name`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (profileRow, error) { return scanProfileRow(row) })
	if err != nil {
		return nil, err
	}
	out := []AgentProfileV1{}
	for _, row := range list {
		obj, err := d.buildProfile(ctx, row)
		if err != nil {
			return nil, err
		}
		ex, ok := excerpt(obj, c)
		if !ok || !slices.ContainsFunc(ex.Status.Grants, func(g GrantStatus) bool { return !g.Granted && g.GrantedAt == nil }) {
			continue
		}
		out = append(out, ex)
	}
	return out, nil
}

func (d *DB) buildProfile(ctx context.Context, row profileRow) (AgentProfileV1, error) {
	gen := int64(row.Latest)
	obj := AgentProfileV1{APIVersion: apiGroupVersion, Kind: "AgentProfile", Metadata: meta(row.NS, row.Name, row.UID, row.RV, row.Latest, row.Created, row.Labels, row.Annots)}
	st := &obj.Status
	st.ObservedGeneration = gen
	var pub, pend *profileGen
	if row.Published != nil {
		g, err := loadProfileGen(ctx, d.pool, row.NS, row.Name, *row.Published)
		if err != nil {
			return obj, err
		}
		pub = &g
		obj.Spec = g.Spec
		st.PublishedGeneration, st.PublishedContentHash = g.Gen, g.Hash
		st.SubmittedBy, st.ApprovedAt, st.ExpiresAt, st.IdentityVerifiedAt = g.submitter(), g.ApprovedAt, g.ExpiresAt, g.VerifiedAt
	}
	if row.Pending != nil {
		g, err := loadProfileGen(ctx, d.pool, row.NS, row.Name, *row.Pending)
		if err != nil {
			return obj, err
		}
		pend = &g
		spec := g.Spec
		exp := g.pendingExpiresAt()
		st.Pending = &Pending[AgentProfileSpec]{Generation: g.Gen, ContentHash: g.Hash, SubmittedBy: g.submitter(), ExpiresAt: &exp, Spec: &spec}
		st.NeedsResubmit = g.SubKind != "User" || g.SubOid == nil
		if pub == nil {
			obj.Spec = g.Spec
			st.IdentityVerifiedAt = g.VerifiedAt
		}
	}
	shown := pend
	if shown == nil {
		shown = pub
	}
	if shown != nil {
		grants, err := d.grantStatus(ctx, *shown)
		if err != nil {
			return obj, err
		}
		st.Grants = grants
	}
	ev, err := lastEvents(ctx, d.pool, "AgentProfile", row.NS, row.Name, nil)
	if err != nil {
		return obj, err
	}
	at := func(actions ...string) time.Time { return latest(ev, row.Created, actions...) }
	published := pub != nil && !row.Revoked
	expired := published && pub.ExpiresAt != nil && !pub.ExpiresAt.After(time.Now())
	verified := st.IdentityVerifiedAt != nil
	pubReason := "NeverApproved"
	switch {
	case row.Revoked:
		pubReason = "Revoked"
	case published:
		pubReason = "Approved"
	}
	pendReason := "NothingPending"
	if pend != nil {
		pendReason = "AwaitingGrant"
	}
	revReason := "NotRevoked"
	if row.Revoked {
		revReason = "RevokedByOwner"
		var by string
		_ = d.pool.QueryRow(ctx, `SELECT by_role FROM resource_status WHERE namespace = $1 AND name = $2 AND kind = 'AgentProfile' AND action = 'agent_profile_revoked'`, row.NS, row.Name).Scan(&by)
		if by == "platform" {
			revReason = "RevokedByPlatform"
		}
	}
	verReason := "NotVerified"
	if verified {
		verReason = "ClientIDMatchesApp"
	}
	ready, readyReason := false, "PendingApproval"
	switch {
	case row.Revoked:
		readyReason = "Revoked"
	case expired:
		readyReason = "Expired"
	case published && !verified:
		readyReason = "NotVerified"
	case published:
		ready, readyReason = true, "Serving"
		if pend == nil && slices.ContainsFunc(st.Grants, func(g GrantStatus) bool { return len(g.MissingTools) > 0 }) {
			readyReason = "ToolsMissing"
		}
	}
	expReason := "NotExpired"
	if expired {
		expReason = "ApprovalExpired"
	}
	pubAt := at("agent_profile_published", "agent_profile_revoked")
	st.Conditions = []Condition{
		cond("Published", published, pubReason, gen, pubAt),
		cond("PendingApproval", pend != nil, pendReason, gen, at("agent_profile_submitted", "agent_profile_published")),
		cond("Revoked", row.Revoked, revReason, gen, at("agent_profile_revoked", "agent_profile_published")),
		cond("Expired", expired, expReason, gen, pubAt),
		cond("IdentityVerified", verified, verReason, gen, at("agent_profile_submitted")),
		cond("Ready", ready, readyReason, gen, pubAt),
	}
	return obj, nil
}

// releaseExpiredClientIDs frees client ids of profiles whose approval has
// expired, in the caller's transaction. shortcut: done lazily when a client
// id is claimed, not by a job; a job is needed once expiry must stop
// gateway traffic on time (it does not today: /internal/v1/agents filters).
func releaseExpiredClientIDs(ctx context.Context, tx pgx.Tx, clientID string) error {
	_, err := tx.Exec(ctx, `UPDATE agent_profile_resources r SET published_client_id = NULL
		FROM agent_profile_generations g
		WHERE g.namespace = r.namespace AND g.name = r.name AND g.generation = r.published_generation
		  AND r.published_client_id = $1::uuid AND g.expires_at <= now()`, clientID)
	return err
}

func (d *DB) submitProfile(ctx context.Context, c caller, m ObjectMeta, spec AgentProfileSpec, verified, create bool) error {
	return d.insertProfileGeneration(ctx, c, m, spec, verified, create)
}

func (d *DB) insertProfileGeneration(ctx context.Context, c caller, m ObjectMeta, spec AgentProfileSpec, verified, create bool) error {
	hashes, err := canon.Hashes(spec)
	if err != nil {
		return err
	}
	hash := hashes[0]
	grantHashes := map[string]string{}
	for _, g := range spec.Grants {
		if grantHashes[g.ServerRef.key()], err = grantHash(spec, g); err != nil {
			return err
		}
	}
	ns, name := m.Namespace, m.Name
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if create {
			tag, err := tx.Exec(ctx, `INSERT INTO agent_profile_resources (namespace, name, labels, annotations) VALUES ($1, $2, $3::jsonb, $4::jsonb) ON CONFLICT DO NOTHING`,
				ns, name, jsonText(m.Labels), jsonText(m.Annotations))
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return stErr(http.StatusConflict, "AlreadyExists", "agentprofiles %q already exists", name)
			}
		}
		row, err := scanProfileRow(tx.QueryRow(ctx, `SELECT `+profileRowCols+` FROM agent_profile_resources r WHERE r.namespace = $1 AND r.name = $2 FOR UPDATE`, ns, name))
		if err != nil {
			return err
		}
		if !create {
			if err := requireResourceVersion(m.ResourceVersion, row.RV); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET labels = $3::jsonb, annotations = $4::jsonb WHERE namespace = $1 AND name = $2`,
				ns, name, jsonText(m.Labels), jsonText(m.Annotations)); err != nil {
				return err
			}
			var same bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_profile_generations WHERE namespace = $1 AND name = $2
				AND spec_hash = $5 AND (generation = $4 OR (generation = $3 AND created_at > now() - $6::interval)))`,
				ns, name, row.Pending, row.Published, hash, pendingValidity).Scan(&same); err != nil {
				return err
			}
			if same && !row.Revoked {
				return auditV1(ctx, tx, c, "metadata_updated", "AgentProfile", ns, name, nil, map[string]any{"labels": m.Labels, "annotations": m.Annotations})
			}
		}
		if err := blockedContent(ctx, tx, "agent_profile_generations", ns, name, hashes); err != nil {
			return err
		}
		if err := releaseExpiredClientIDs(ctx, tx, spec.Identity.EntraClientID); err != nil {
			return err
		}
		var taken bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_profile_resources WHERE published_client_id = $1::uuid AND NOT (namespace = $2 AND name = $3))`,
			spec.Identity.EntraClientID, ns, name).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return conflict("spec.identity.entraClientID", "FieldValueDuplicate", "the client id is in another live profile")
		}
		gen := row.Latest + 1
		specJSON, _ := json.Marshal(spec)
		hashJSON, _ := json.Marshal(grantHashes)
		var oid *string
		if c.Oid != "" {
			oid = &c.Oid
		}
		// Written as both entra_client_id and {issuer, subject} (#2896).
		if entraIssuer == "" {
			return stErr(http.StatusServiceUnavailable, "ServiceUnavailable", "the identity issuer is not configured")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO agent_profile_generations (namespace, name, generation, spec, spec_hash, entra_client_id, submitted_by, submitted_by_kind,
			submitted_by_oid, identity_verified_at, grant_hashes, identity_issuer, identity_subject)
			VALUES ($1, $2, $3, $4::jsonb, $5, $6::uuid, $7, $8, $9::uuid, CASE WHEN $10 THEN now() END, $11::jsonb, $12, $13)`,
			ns, name, gen, string(specJSON), hash, spec.Identity.EntraClientID, c.Ident, c.actor().Kind, oid, verified, string(hashJSON), entraIssuer, spec.Identity.EntraClientID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET pending_generation = $3 WHERE namespace = $1 AND name = $2`, ns, name, gen); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "agent_profile_submitted", "AgentProfile", ns, name, &gen, map[string]any{"contentHash": hash, "grantHashes": grantHashes, "identityVerified": verified})
	})
}

// grantVisible is errV1NotFound unless the caller is in the profile's team
// or ref is granted in its pending or published generation. It runs right
// after the row lock, before any check that reveals state, so a stranger
// gets the same 404 for a missing profile and one that is not theirs.
func grantVisible(ctx context.Context, tx pgx.Tx, c caller, row profileRow, ref TypedRef) error {
	if c.in(row.NS) {
		return nil
	}
	var granted bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_profile_generations WHERE namespace = $1 AND name = $2
		AND generation IN ($3, $4) AND grant_hashes ? $5)`, row.NS, row.Name, row.Pending, row.Published, ref.key()).Scan(&granted); err != nil {
		return err
	}
	if !granted {
		return errV1NotFound
	}
	return nil
}

// grantProfile gives one grant on the pending generation. The profile is
// published once every grant in that generation is given.
func (d *DB) grantProfile(ctx context.Context, c caller, ns, name string, gen int, ref TypedRef, hash string) error {
	expired := false
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		row, err := scanProfileRow(tx.QueryRow(ctx, `SELECT `+profileRowCols+` FROM agent_profile_resources r WHERE r.namespace = $1 AND r.name = $2 FOR UPDATE`, ns, name))
		if err != nil {
			return err
		}
		if err = grantVisible(ctx, tx, c, row, ref); err != nil {
			return err
		}
		if row.Pending == nil || *row.Pending != gen {
			return conflict("generation", "FieldValueInvalid", "generation %d is not the one awaiting approval", gen)
		}
		g, err := loadProfileGen(ctx, tx, ns, name, gen)
		if err != nil {
			return err
		}
		if time.Now().After(g.pendingExpiresAt()) {
			// Commit only the audit row; the caller gets 410. Earlier grants lapse with the generation.
			expired = true
			return auditV1(ctx, tx, c, "approval_expired", "AgentProfile", ns, name, &gen, map[string]any{"outcome": "expired", "server": ref.key(), "contentHash": hash, "expiresAt": g.pendingExpiresAt()})
		}
		want, ok := g.GrantHashes[ref.key()]
		i := slices.IndexFunc(g.Spec.Grants, func(gr AgentGrantV1) bool { return gr.ServerRef.key() == ref.key() })
		switch {
		case !ok || i < 0:
			return conflict("serverRef", "FieldValueNotFound", "generation %d grants nothing on %s", gen, ref.key())
		case want != hash || !strings.HasPrefix(hash, canon.Prefix):
			return conflict("contentHash", "FieldValueInvalid", "contentHash does not match the grant on %s", ref.key())
		case g.SubKind != "User" || g.SubOid == nil:
			return stErr(http.StatusForbidden, "Forbidden", "the pending generation has no person as submitter; a person must submit it again")
		case *g.SubOid == c.Oid || g.SubName == c.Ident:
			return stErr(http.StatusForbidden, "Forbidden", "the submitter cannot grant access to their own profile")
		}
		// FOR SHARE waits for, and blocks, a concurrent server write.
		if _, err = tx.Exec(ctx, `SELECT 1 FROM servers WHERE namespace = $1 AND name = $2 FOR SHARE`, ref.Namespace, ref.Name); err != nil {
			return err
		}
		if err = checkGrantServers(ctx, tx, g.Spec.Grants[i:i+1]); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO agent_profile_grants (namespace, name, generation, server, content_hash, granted_by, granted_by_kind, granted_by_oid, granted_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'User', $7::uuid, clock_timestamp()) ON CONFLICT DO NOTHING`, ns, name, gen, ref.key(), hash, c.Ident, c.Oid)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return conflict("serverRef", "FieldValueDuplicate", "access to %s is already granted", ref.key())
		}
		if err := auditV1(ctx, tx, c, "agent_profile_access_granted", "AgentProfile", ns, name, &gen, map[string]any{"server": ref.key(), "contentHash": hash}); err != nil {
			return err
		}
		var granted int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM agent_profile_grants WHERE namespace = $1 AND name = $2 AND generation = $3`, ns, name, gen).Scan(&granted); err != nil {
			return err
		}
		if granted < len(g.Spec.Grants) {
			// Touch the resource so resourceVersion and the ETag move.
			_, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET revoked = revoked WHERE namespace = $1 AND name = $2`, ns, name)
			return err
		}
		// Every server may have changed since its grant: lock them all in key
		// order and check each grant is still valid before publishing.
		for _, gr := range g.Spec.Grants { // sorted by serverRef at validation
			if _, err := tx.Exec(ctx, `SELECT 1 FROM servers WHERE namespace = $1 AND name = $2 FOR SHARE`, gr.ServerRef.Namespace, gr.ServerRef.Name); err != nil {
				return err
			}
		}
		if err := checkGrantServers(ctx, tx, g.Spec.Grants); err != nil {
			return err
		}
		var stale bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM agent_profile_grants a JOIN server_versions v
			ON v.namespace = split_part(a.server, '/', 1) AND v.name = split_part(a.server, '/', 2)
			WHERE a.namespace = $1 AND a.name = $2 AND a.generation = $3 AND (a.revoked_at IS NOT NULL OR v.revoked_at > a.granted_at))`, ns, name, gen).Scan(&stale); err != nil {
			return err
		}
		if stale {
			return conflict("serverRef", "InvalidState", "a grant was revoked, or its server revoked, after it was given; submit the profile again")
		}
		if err := releaseExpiredClientIDs(ctx, tx, g.ClientID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET published_generation = $3, pending_generation = NULL, revoked = false,
			published_client_id = CASE WHEN $5 THEN $4::uuid END WHERE namespace = $1 AND name = $2`, ns, name, gen, g.ClientID, g.VerifiedAt != nil); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
				return conflict("spec.identity.entraClientID", "FieldValueDuplicate", "the client id is in another live profile")
			}
			return err
		}
		// The six months run from the oldest grant, not the last one (#2731).
		if _, err := tx.Exec(ctx, `UPDATE agent_profile_generations SET approved_at = now(), expires_at = (SELECT min(granted_at) FROM agent_profile_grants
			WHERE namespace = $1 AND name = $2 AND generation = $3) + $4::interval
			WHERE namespace = $1 AND name = $2 AND generation = $3`, ns, name, gen, approvalValidity); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "agent_profile_published", "AgentProfile", ns, name, &gen, map[string]any{"contentHash": g.Hash})
	})
	if err == nil && expired {
		return errPendingExpired(gen)
	}
	return err
}

// revokeProfileV1 revokes the whole published profile, or with ref only
// that grant. Final for the generation: a restart is a new submission.
func (d *DB) revokeProfileV1(ctx context.Context, c caller, by, ns, name, reason string, ref *TypedRef) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		row, err := scanProfileRow(tx.QueryRow(ctx, `SELECT `+profileRowCols+` FROM agent_profile_resources r WHERE r.namespace = $1 AND r.name = $2 FOR UPDATE`, ns, name))
		if err != nil {
			return err
		}
		if ref != nil {
			if err := grantVisible(ctx, tx, c, row, *ref); err != nil {
				return err
			}
		}
		if row.Published == nil || row.Revoked {
			return conflict("", "InvalidState", "profile is not published")
		}
		if ref != nil {
			tag, err := tx.Exec(ctx, `UPDATE agent_profile_grants SET revoked_at = clock_timestamp() WHERE namespace = $1 AND name = $2 AND generation = $3 AND server = $4 AND revoked_at IS NULL`,
				ns, name, *row.Published, ref.key())
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return conflict("serverRef", "InvalidState", "no active grant on %s", ref.key())
			}
			// Touch the resource so resourceVersion moves.
			if _, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET revoked = revoked WHERE namespace = $1 AND name = $2`, ns, name); err != nil {
				return err
			}
			return auditV1(ctx, tx, c, "agent_profile_access_revoked", "AgentProfile", ns, name, row.Published, map[string]any{"reason": reason, "server": ref.key()})
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_profile_resources SET revoked = true, published_client_id = NULL WHERE namespace = $1 AND name = $2`, ns, name); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE agent_profile_generations SET revoked_at = now() WHERE namespace = $1 AND name = $2 AND generation = $3`, ns, name, *row.Published); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "agent_profile_revoked", "AgentProfile", ns, name, row.Published, map[string]any{"reason": reason, "by": by})
	})
}

// seedProfileV1 stores a seeded profile as generation 1 by kind System. A
// seeded author is not a person, so nobody can grant it until a person
// submits it again.
func (d *DB) seedProfileV1(ctx context.Context, p AgentProfile) error {
	spec := AgentProfileSpec{
		Identity: AgentIdentityV1{App: p.Identity.App, Cluster: p.Identity.Cluster, EntraClientID: p.Identity.EntraClientID},
		Mode:     "Delegated",
		Quota:    AgentQuotaV1{MaxInFlight: p.Quota.MaxInFlight, RequestsPerMinute: p.Quota.RPM, DailyCallsPerUser: p.Quota.DailyCallsPerUser, DailyCalls: p.Quota.DailyCalls},
		DPIA:     &AgentDPIAV1{Owner: p.DPIA.Owner, Reference: p.DPIA.Reference},
	}
	for _, g := range p.Grants {
		ns, name, _ := strings.Cut(g.Route, "/")
		spec.Grants = append(spec.Grants, AgentGrantV1{ServerRef: TypedRef{Group: apiGroup, Kind: "MCPServer", Namespace: ns, Name: name}, AllowedTools: g.Tools, MaxRisk: "Read"})
	}
	spec, err := validateProfileSpec(spec)
	if err != nil {
		return err
	}
	err = d.insertProfileGeneration(ctx, caller{Ident: "seed", Kind: "System"}, ObjectMeta{Namespace: p.Namespace, Name: p.Name}, spec, false, true)
	if se, ok := errors.AsType[*statusError](err); ok && se.reason == "AlreadyExists" {
		return nil
	}
	return err
}
