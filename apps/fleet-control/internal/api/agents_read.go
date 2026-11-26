package api

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/navikt/copilot/fleet-control/internal/model"
	"github.com/navikt/copilot/fleet-control/internal/store"
)

// --- storage ---

type profileRow struct {
	NS, Name, UID      string
	RV                 int64
	Labels, Annots     string
	Created            time.Time
	Published, Pending *int
	Revoked            bool
	Latest             int
	Now                time.Time // DB clock at read, for expiry status
}

const profileRowCols = `r.namespace, r.name, r.uid::text, r.resource_version, r.labels::text, r.annotations::text, r.created_at,
	r.published_generation, r.pending_generation, r.revoked,
	(SELECT COALESCE(max(generation), 0) FROM agent_profile_generations g WHERE g.namespace = r.namespace AND g.name = r.name), now()`

func scanProfileRow(row pgx.Row) (profileRow, error) {
	var p profileRow
	err := row.Scan(&p.NS, &p.Name, &p.UID, &p.RV, &p.Labels, &p.Annots, &p.Created, &p.Published, &p.Pending, &p.Revoked, &p.Latest, &p.Now)
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
	// DB time from the row read, like enforcement (txNow), so status and refusals agree under pod clock drift.
	expired := published && pub.ExpiresAt != nil && !pub.ExpiresAt.After(row.Now)
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
