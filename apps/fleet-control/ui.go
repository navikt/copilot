package main

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Read routes for the self-service pages in my-copilot (#2736). Every route
// takes a person's token from my-copilot and shows only what the person's
// teams own. A list for another team is empty, and an audit trail for another
// team is 404 whether or not the object exists, so neither leaks existence.

// uiStore is implemented by *DB.
type uiStore interface {
	TeamServers(ctx context.Context, teams []string) ([]serverSummary, error)
	TeamAgents(ctx context.Context, teams []string) ([]AgentProfile, error)
	AuditTrail(ctx context.Context, ns, name string, actions []string) ([]auditEvent, error)
	DraftAgents(ctx context.Context, teams []string) ([]pendingAgent, error)
}

type serverSummary struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Published *int   `json:"published_version"`
	Pending   *int   `json:"pending_version"`
}

type auditEvent struct {
	At      time.Time       `json:"at"`
	Actor   string          `json:"actor"`
	Action  string          `json:"action"`
	Version *int            `json:"version,omitempty"`
	Detail  json.RawMessage `json:"detail"`
}

type pendingAgent struct {
	ID       int64        `json:"id"`
	Profile  AgentProfile `json:"profile"`
	Approved []string     `json:"-"`
	Awaiting []string     `json:"awaiting"` // routes the caller's teams still have to approve
}

var (
	serverActions = []string{"draft_submitted", "approved_published", "revoked", "rolled_back"}
	agentActions  = []string{"agent_profile_drafted", "agent_profile_route_approved", "agent_profile_approved", "agent_profile_revoked", "agent_profile_expired"}
)

func (s *server) uiRoutes(mux *http.ServeMux, persons personResolver) {
	mux.HandleFunc("GET /admin/v1/me", withPerson(persons, func(w http.ResponseWriter, _ *http.Request, who Person) {
		writeJSON(w, map[string]any{"ident": who.Ident, "teams": nonNil(who.Teams)})
	}))
	mux.HandleFunc("GET /admin/v1/servers", withPerson(persons, s.uiList("servers", func(ctx context.Context, st uiStore, teams []string) (any, error) {
		return st.TeamServers(ctx, teams)
	})))
	mux.HandleFunc("GET /admin/v1/agents", withPerson(persons, s.uiList("agents", func(ctx context.Context, st uiStore, teams []string) (any, error) {
		return st.TeamAgents(ctx, teams)
	})))
	mux.HandleFunc("GET /admin/v1/servers/{namespace}/{name}/audit", withPerson(persons, s.uiAudit(serverActions)))
	mux.HandleFunc("GET /admin/v1/agents/{namespace}/{name}/audit", withPerson(persons, s.uiAudit(agentActions)))
	mux.HandleFunc("GET /admin/v1/agents/pending-approval", withPerson(persons, s.pendingApproval))
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func (s *server) uiDB(w http.ResponseWriter, r *http.Request) (uiStore, context.Context, context.CancelFunc) {
	st, ctx, cancel := s.db(w, r)
	if st == nil {
		return nil, nil, nil
	}
	us, ok := st.(uiStore)
	if !ok {
		cancel()
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return nil, nil, nil
	}
	return us, ctx, cancel
}

// uiList serves one list, limited to the caller's teams, or to one of them
// with ?namespace=. A namespace the caller is not in gives an empty list.
func (s *server) uiList(field string, load func(context.Context, uiStore, []string) (any, error)) func(http.ResponseWriter, *http.Request, Person) {
	return func(w http.ResponseWriter, r *http.Request, who Person) {
		teams := who.Teams
		if ns := r.URL.Query().Get("namespace"); ns != "" {
			if !dnsLabelRegex.MatchString(ns) {
				http.Error(w, "namespace must be a lowercase DNS label", http.StatusBadRequest)
				return
			}
			teams = nil
			if who.in(ns) {
				teams = []string{ns}
			}
		}
		if len(teams) == 0 {
			writeJSON(w, map[string]any{field: []any{}})
			return
		}
		st, ctx, cancel := s.uiDB(w, r)
		if st == nil {
			return
		}
		defer cancel()
		items, err := load(ctx, st, teams)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, map[string]any{field: items})
	}
}

// uiAudit serves the audit trail of one server or agent profile to members of
// its owning team. Anyone else gets 404 before the database is asked.
func (s *server) uiAudit(actions []string) func(http.ResponseWriter, *http.Request, Person) {
	return func(w http.ResponseWriter, r *http.Request, who Person) {
		ns, name := r.PathValue("namespace"), r.PathValue("name")
		if !dnsLabelRegex.MatchString(ns) || !dnsLabelRegex.MatchString(name) {
			http.Error(w, "namespace and name must be lowercase DNS labels", http.StatusBadRequest)
			return
		}
		if !who.in(ns) {
			writeErr(w, errNotFound)
			return
		}
		st, ctx, cancel := s.uiDB(w, r)
		if st == nil {
			return
		}
		defer cancel()
		events, err := st.AuditTrail(ctx, ns, name, actions)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, map[string]any{"events": events})
	}
}

// pendingApproval is the «Til godkjenning» inbox: live drafts with an open
// slot one of the caller's teams fills: a granted route it owns, or the
// owner-team approval (keyed by namespace) when it owns the profile. The
// author can approve no slot, so never sees their own draft.
func (s *server) pendingApproval(w http.ResponseWriter, r *http.Request, who Person) {
	out := []pendingAgent{}
	if len(who.Teams) == 0 {
		writeJSON(w, map[string]any{"agents": out})
		return
	}
	st, ctx, cancel := s.uiDB(w, r)
	if st == nil {
		return
	}
	defer cancel()
	drafts, err := st.DraftAgents(ctx, who.Teams)
	if err != nil {
		writeErr(w, err)
		return
	}
	for _, d := range drafts {
		// ApproveAgentRoute refuses seeded drafts, so they are not approvable.
		if d.Profile.CreatedBy == who.Ident || d.Profile.CreatedBy == "seed" {
			continue
		}
		d.Awaiting = []string{}
		if who.in(d.Profile.Namespace) && !slices.Contains(d.Approved, d.Profile.Namespace) {
			d.Awaiting = append(d.Awaiting, d.Profile.Namespace)
		}
		for _, g := range d.Profile.Grants {
			team, _, _ := strings.Cut(g.Route, "/")
			if who.in(team) && !slices.Contains(d.Approved, g.Route) {
				d.Awaiting = append(d.Awaiting, g.Route)
			}
		}
		if len(d.Awaiting) > 0 {
			out = append(out, d)
		}
	}
	writeJSON(w, map[string]any{"agents": out})
}

// --- storage ---

func (d *DB) TeamServers(ctx context.Context, teams []string) ([]serverSummary, error) {
	rows, err := d.pool.Query(ctx, `SELECT namespace, name, status, published_version, pending_version
		FROM servers WHERE namespace = ANY($1) ORDER BY namespace, name`, teams)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByPos[serverSummary])
	if out == nil {
		out = []serverSummary{}
	}
	return out, err
}

// TeamAgents returns the live (draft or approved) profiles owned by teams.
func (d *DB) TeamAgents(ctx context.Context, teams []string) ([]AgentProfile, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+agentCols+` FROM agent_profiles
		WHERE namespace = ANY($1) AND status IN ('draft', 'approved') ORDER BY namespace, name`, teams)
	if err != nil {
		return nil, err
	}
	out := []AgentProfile{}
	_, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (struct{}, error) {
		_, p, scanErr := scanAgent(row)
		// The gateway drops approved profiles past expires_at at read time;
		// report what it enforces.
		if p.Status == "approved" && p.ExpiresAt != nil && !p.ExpiresAt.After(time.Now()) {
			p.Status = "expired"
		}
		p.NeedsResubmit = p.Status == "draft" && p.CreatedBy == "seed"
		out = append(out, p)
		return struct{}{}, scanErr
	})
	return out, err
}

// AuditTrail returns the newest 500 events with one of actions on ns/name.
func (d *DB) AuditTrail(ctx context.Context, ns, name string, actions []string) ([]auditEvent, error) {
	rows, err := d.pool.Query(ctx, `SELECT at, actor, action, version, COALESCE(detail, 'null'::jsonb)::text
		FROM audit_events WHERE namespace = $1 AND name = $2 AND action = ANY($3) ORDER BY id DESC LIMIT 500`, ns, name, actions)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (auditEvent, error) {
		var e auditEvent
		var detail string
		scanErr := row.Scan(&e.At, &e.Actor, &e.Action, &e.Version, &detail)
		e.Detail = json.RawMessage(detail)
		return e, scanErr
	})
	if out == nil {
		out = []auditEvent{}
	}
	return out, err
}

// DraftAgents returns the live drafts owned by, or granting a route owned by,
// one of teams, with the slots approved so far. The handler narrows to the
// slots still awaiting those teams.
func (d *DB) DraftAgents(ctx context.Context, teams []string) ([]pendingAgent, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+agentCols+`,
		COALESCE((SELECT array_agg(route) FROM agent_profile_approvals a WHERE a.profile_id = agent_profiles.id), '{}')
		FROM agent_profiles WHERE status = 'draft'
		  AND (namespace = ANY($1) OR EXISTS (SELECT 1 FROM jsonb_array_elements(grants) g WHERE split_part(g->>'route', '/', 1) = ANY($1)))
		ORDER BY namespace, name`, teams)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []pendingAgent{}
	for rows.Next() {
		var pa pendingAgent
		var grants string
		p := &pa.Profile
		if err := rows.Scan(&pa.ID, &p.Namespace, &p.Name, &p.Identity.Cluster, &p.Identity.App, &p.Identity.EntraClientID, &p.Identity.StsSubject,
			&p.Mode, &grants, &p.Quota.MaxInFlight, &p.Quota.RPM, &p.Quota.DailyCallsPerUser, &p.Quota.DailyCalls,
			&p.DPIA.Owner, &p.DPIA.Reference, &p.Status, &p.ExpiresAt, &p.CreatedBy, &pa.Approved); err != nil {
			return nil, err
		}
		p.Identity.Namespace = p.Namespace
		if err := json.Unmarshal([]byte(grants), &p.Grants); err != nil {
			return nil, err
		}
		out = append(out, pa)
	}
	return out, rows.Err()
}
