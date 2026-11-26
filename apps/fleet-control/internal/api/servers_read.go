package api

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/navikt/copilot/fleet-control/internal/store"
)

// --- storage ---

// legacySpecSQL builds a v1alpha1 spec from the /admin/v1 columns, for
// versions written before 005 or by /admin/v1. It has no dataCategory.
const legacySpecSQL = `(CASE WHEN v.route_path IS NULL THEN '{}'::jsonb ELSE jsonb_build_object(
		'upstream', jsonb_build_object('app', split_part(v.target, ':', 3), 'url', v.upstream_url),
		'allowedTools', (SELECT jsonb_agg(t ORDER BY t COLLATE "C") FROM unnest(v.tools) t)) END
	|| CASE WHEN v.registry IS NULL THEN '{}'::jsonb ELSE jsonb_build_object('registry', v.registry) END)`

type serverRow struct {
	NS, Name, UID      string
	RV                 int64
	Labels, Annots     string
	Created            time.Time
	Status             string
	Published, Pending *int
	Latest             int
}

const serverRowCols = `s.namespace, s.name, s.uid::text, s.resource_version, s.labels::text, s.annotations::text, s.created_at, s.status,
	s.published_version, s.pending_version, (SELECT COALESCE(max(version), 0) FROM server_versions x WHERE x.namespace = s.namespace AND x.name = s.name)`

func scanServerRow(row pgx.Row) (serverRow, error) {
	var s serverRow
	err := row.Scan(&s.NS, &s.Name, &s.UID, &s.RV, &s.Labels, &s.Annots, &s.Created, &s.Status, &s.Published, &s.Pending, &s.Latest)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, errV1NotFound
	}
	return s, err
}

func lockServerRow(ctx context.Context, tx pgx.Tx, ns, name string) (serverRow, error) {
	return scanServerRow(tx.QueryRow(ctx, `SELECT `+serverRowCols+` FROM servers s WHERE s.namespace = $1 AND s.name = $2 FOR UPDATE`, ns, name))
}

type versionRow struct {
	Version                    int
	Spec                       MCPServerSpec
	Hash                       *string
	SubKind                    *string
	SubName                    string
	ApprovedBy                 *string
	ApprovedAt, PublishedAt    *time.Time
	RevokedAt                  *time.Time
	CreatedAt                  time.Time
	Blocked                    bool
	SubOid, RegName, ApprovOid *string
}

func (v versionRow) submitter() *Actor { return &Actor{Kind: deref(v.SubKind), Name: v.SubName} }

func (v versionRow) expiresAt() time.Time { return v.CreatedAt.Add(pendingValidity) }

// txNow is the database clock for the transaction. Every expiry decision uses it,
// so a pod clock that drifts from Postgres cannot make the checks disagree.
func txNow(ctx context.Context, tx pgx.Tx) (time.Time, error) {
	var now time.Time
	err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now)
	return now, err
}

func (v versionRow) approver() *Actor {
	if v.ApprovedBy == nil {
		return nil
	}
	// Approval is by a person: /admin/v1 and v1alpha1 both take a person's token.
	return &Actor{Kind: "User", Name: *v.ApprovedBy}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func loadVersion(ctx context.Context, q store.Querier, ns, name string, version int) (versionRow, error) {
	var v versionRow
	var spec string
	err := q.QueryRow(ctx, `SELECT v.version, COALESCE(v.spec, `+legacySpecSQL+`)::text, v.spec_hash, v.submitted_by_kind, v.submitted_by,
		v.approved_by, v.approved_at, v.published_at, v.revoked_at, v.blocked, v.submitted_by_oid::text, v.registry->>'name', v.approved_by_oid::text, v.created_at
		FROM server_versions v WHERE v.namespace = $1 AND v.name = $2 AND v.version = $3`, ns, name, version).
		Scan(&v.Version, &spec, &v.Hash, &v.SubKind, &v.SubName, &v.ApprovedBy, &v.ApprovedAt, &v.PublishedAt, &v.RevokedAt, &v.Blocked, &v.SubOid, &v.RegName, &v.ApprovOid, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, errV1NotFound
	}
	if err != nil {
		return v, err
	}
	return v, json.Unmarshal([]byte(spec), &v.Spec)
}

// lastEvents returns the time of the newest event per action for one
// resource. It reads resource_status, which outlives audit retention.
func lastEvents(ctx context.Context, q store.Querier, kind, ns, name string, legacy []string) (map[string]time.Time, error) {
	rows, err := q.Query(ctx, `SELECT action, max(at) FROM resource_status WHERE namespace = $1 AND name = $2
		AND (kind = $3 OR (kind = '' AND action = ANY($4))) GROUP BY action`, ns, name, kind, legacy)
	if err != nil {
		return nil, err
	}
	out := map[string]time.Time{}
	_, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (struct{}, error) {
		var a string
		var t time.Time
		scanErr := row.Scan(&a, &t)
		out[a] = t
		return struct{}{}, scanErr
	})
	return out, err
}

func latest(ev map[string]time.Time, fallback time.Time, actions ...string) time.Time {
	t := fallback
	for _, a := range actions {
		if e, ok := ev[a]; ok && e.After(t) {
			t = e
		}
	}
	return t
}

func meta(ns, name, uid string, rv int64, gen int, created time.Time, labels, annots string) ObjectMeta {
	c := created.UTC()
	return ObjectMeta{Name: name, Namespace: ns, UID: uid, ResourceVersion: strconv.FormatInt(rv, 10), Generation: int64(gen),
		CreationTimestamp: &c, Labels: jsonMap(labels), Annotations: jsonMap(annots)}
}

// serverObject builds the full MCPServer for its owner team.
func (d *DB) serverObject(ctx context.Context, ns, name string) (MCPServer, error) {
	row, err := scanServerRow(d.pool.QueryRow(ctx, `SELECT `+serverRowCols+` FROM servers s WHERE s.namespace = $1 AND s.name = $2`, ns, name))
	if err != nil {
		return MCPServer{}, err
	}
	return d.buildServer(ctx, row)
}

func (d *DB) serverObjects(ctx context.Context, teams []string, name string) ([]MCPServer, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+serverRowCols+` FROM servers s WHERE s.namespace = ANY($1) AND ($2 = '' OR s.name = $2) ORDER BY s.namespace, s.name`, nonNil(teams), name)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (serverRow, error) { return scanServerRow(row) })
	if err != nil {
		return nil, err
	}
	out := make([]MCPServer, 0, len(list))
	for _, row := range list {
		obj, err := d.buildServer(ctx, row)
		if err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	return out, nil
}

func (d *DB) buildServer(ctx context.Context, row serverRow) (MCPServer, error) {
	obj := MCPServer{APIVersion: apiGroupVersion, Kind: "MCPServer", Metadata: meta(row.NS, row.Name, row.UID, row.RV, row.Latest, row.Created, row.Labels, row.Annots)}
	gen := int64(row.Latest)
	st := &obj.Status
	st.ObservedGeneration = gen
	var pub *versionRow
	if row.Published != nil {
		v, err := loadVersion(ctx, d.pool, row.NS, row.Name, *row.Published)
		if err != nil {
			return obj, err
		}
		pub = &v
		obj.Spec = v.Spec
		st.PublishedGeneration, st.PublishedContentHash = v.Version, deref(v.Hash)
		st.SubmittedBy, st.ApprovedBy, st.ApprovedAt = v.submitter(), v.approver(), v.ApprovedAt
	}
	if row.Pending != nil {
		v, err := loadVersion(ctx, d.pool, row.NS, row.Name, *row.Pending)
		if err != nil {
			return obj, err
		}
		spec := v.Spec
		exp := v.expiresAt()
		st.Pending = &Pending[MCPServerSpec]{Generation: v.Version, ContentHash: deref(v.Hash), SubmittedBy: v.submitter(), ExpiresAt: &exp, Spec: &spec}
		if pub == nil {
			obj.Spec = v.Spec // never published: spec is the latest submitted
		}
	}
	if obj.Spec.Upstream != nil {
		st.Route = &RouteStatus{Path: "/mcp/" + row.NS + "/" + row.Name}
	}
	ev, err := lastEvents(ctx, d.pool, "MCPServer", row.NS, row.Name, serverActions)
	if err != nil {
		return obj, err
	}
	at := func(actions ...string) time.Time { return latest(ev, row.Created, actions...) }
	pubAt := at("approved_published", "auto_published", "rolled_back", "revoked")
	published, revoked := row.Status == "published", row.Status == "revoked"
	pubReason := "NeverApproved"
	switch {
	case revoked:
		pubReason = "Revoked"
	case published && ev["rolled_back"].After(at("approved_published", "auto_published")):
		pubReason = "RolledBack"
	case published:
		pubReason = "Approved"
	}
	pendReason := "NothingPending"
	if row.Pending != nil {
		pendReason = "AwaitingPlatformTeam"
	}
	revReason := "NotRevoked"
	if revoked {
		revReason = "RevokedByOwner"
		var by string
		_ = d.pool.QueryRow(ctx, `SELECT by_role FROM resource_status WHERE namespace = $1 AND name = $2 AND action = 'revoked' ORDER BY at DESC LIMIT 1`, row.NS, row.Name).Scan(&by)
		if by == "platform" {
			revReason = "RevokedByPlatform"
		}
		if pub != nil && pub.Blocked {
			revReason = "Blocked"
		}
	}
	readyReason := "Serving"
	switch {
	case revoked:
		readyReason = "Revoked"
	case !published:
		readyReason = "PendingApproval"
	}
	st.Conditions = []Condition{
		cond("Published", published, pubReason, gen, pubAt),
		cond("PendingApproval", row.Pending != nil, pendReason, gen, at("draft_submitted", "approved_published", "auto_published")),
		cond("Revoked", revoked, revReason, gen, at("revoked", "approved_published", "auto_published")),
		cond("Ready", published, readyReason, gen, pubAt),
	}
	return obj, nil
}
