package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MCPServerSpec is the v1alpha1 spec. target and path are derived; enabled
// is gone (revocation is an action with a trail).
type MCPServerSpec struct {
	DataCategory string          `json:"dataCategory,omitempty"`
	Upstream     *Upstream       `json:"upstream,omitempty"`
	AllowedTools []string        `json:"allowedTools,omitempty"`
	Registry     json.RawMessage `json:"registry,omitempty"`
	// Readme is markdown from the owner team, stored and returned as is.
	// Untrusted: clients must render it without raw HTML.
	Readme string `json:"readme,omitempty"`
	// Tools describes allowed tools for people. Optional per tool.
	Tools []ToolDoc `json:"tools,omitempty"`
	// Authorization is reserved (owner decision 2) and refused with 422.
	Authorization json.RawMessage `json:"authorization,omitempty"`
}

type ToolDoc struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Caps on v0.8 text fields, in bytes. Both are part of the contentHash; empty
// fields are omitted, so specs from before v0.8 keep their hash.
const (
	maxReadmeBytes          = 64 << 10
	maxToolDescriptionBytes = 1024
	// maxServerTools bounds allowedTools and so tools: 256 x 1 KiB keeps a
	// spec well under 1 MiB, and real MCP servers list tens of tools.
	maxServerTools = 256
)

type Upstream struct {
	App string `json:"app"`
	URL string `json:"url"`
}

type RouteStatus struct {
	Path string `json:"path"`
}

// Pending is the generation awaiting approval. Shown to the owner team only.
type Pending[S any] struct {
	Generation  int    `json:"generation"`
	ContentHash string `json:"contentHash,omitempty"`
	SubmittedBy *Actor `json:"submittedBy,omitempty"`
	Spec        *S     `json:"spec,omitempty"`
}

type MCPServerStatus struct {
	ObservedGeneration   int64  `json:"observedGeneration"`
	PublishedGeneration  int    `json:"publishedGeneration,omitempty"`
	PublishedContentHash string `json:"publishedContentHash,omitempty"`
	// ContentHash is set only by GET .../generations/{n}.
	ContentHash string                  `json:"contentHash,omitempty"`
	Route       *RouteStatus            `json:"route,omitempty"`
	SubmittedBy *Actor                  `json:"submittedBy,omitempty"`
	ApprovedBy  *Actor                  `json:"approvedBy,omitempty"`
	ApprovedAt  *time.Time              `json:"approvedAt,omitempty"`
	Pending     *Pending[MCPServerSpec] `json:"pending,omitempty"`
	Conditions  []Condition             `json:"conditions,omitempty"`
}

type MCPServer struct {
	APIVersion string          `json:"apiVersion"`
	Kind       string          `json:"kind"`
	Metadata   ObjectMeta      `json:"metadata"`
	Spec       MCPServerSpec   `json:"spec"`
	Status     MCPServerStatus `json:"status"`
}

var dataCategories = []string{"Aapen", "Intern", "Fortrolig"}

// validateServerSpec checks spec with the same rules as /admin/v1 drafts
// and returns it normalised (allowedTools sorted) with the Server the
// snapshot columns are written from.
func validateServerSpec(cluster, platform, ns, name string, spec MCPServerSpec) (MCPServerSpec, Server, error) {
	if len(spec.Authorization) > 0 {
		return spec, Server{}, invalid("spec.authorization", "spec.authorization is reserved and not supported in v1alpha1")
	}
	switch {
	case spec.DataCategory == "StrengtFortrolig":
		return spec, Server{}, invalid("spec.dataCategory", "StrengtFortrolig needs its own risk assessment and approval first")
	case !slices.Contains(dataCategories, spec.DataCategory):
		return spec, Server{}, invalid("spec.dataCategory", "spec.dataCategory must be one of %s", strings.Join(dataCategories, ", "))
	}
	if len(spec.AllowedTools) > maxServerTools {
		return spec, Server{}, invalid("spec.allowedTools", "spec.allowedTools lists at most %d tools", maxServerTools)
	}
	if len(spec.Readme) > maxReadmeBytes {
		return spec, Server{}, invalid("spec.readme", "spec.readme is at most %d bytes", maxReadmeBytes)
	}
	if len(spec.Tools) > len(spec.AllowedTools) {
		return spec, Server{}, invalid("spec.tools", "spec.tools has more entries than allowedTools")
	}
	allowed := make(map[string]bool, len(spec.AllowedTools))
	for _, t := range spec.AllowedTools {
		allowed[t] = true
	}
	seen := make(map[string]bool, len(spec.Tools))
	for _, t := range spec.Tools {
		switch {
		case !allowed[t.Name] || seen[t.Name]:
			return spec, Server{}, invalid("spec.tools", "%q is not in allowedTools, or is listed twice", t.Name)
		case len(t.Description) > maxToolDescriptionBytes:
			return spec, Server{}, invalid("spec.tools.description", "tool %s: description is at most %d bytes", t.Name, maxToolDescriptionBytes)
		}
		seen[t.Name] = true
	}
	spec.Tools = slices.SortedFunc(slices.Values(spec.Tools), func(a, b ToolDoc) int { return strings.Compare(a.Name, b.Name) })
	if len(spec.Tools) == 0 {
		spec.Tools = nil
	}
	var draft draftSpec
	draft.Registry = spec.Registry
	if u := spec.Upstream; u != nil {
		if !dnsLabelRegex.MatchString(u.App) {
			return spec, Server{}, invalid("spec.upstream.app", "must be a lowercase DNS label")
		}
		if len(spec.AllowedTools) == 0 {
			return spec, Server{}, invalid("spec.allowedTools", "upstream needs at least one allowed tool")
		}
		spec.AllowedTools = slices.Sorted(slices.Values(spec.AllowedTools))
		draft.Route = &struct {
			Target      string   `json:"target"`
			UpstreamURL string   `json:"upstream_url"`
			Enabled     *bool    `json:"enabled"`
			Tools       []string `json:"tools"`
		}{Target: cluster + ":" + ns + ":" + u.App, UpstreamURL: u.URL, Tools: spec.AllowedTools}
	} else if len(spec.AllowedTools) > 0 {
		return spec, Server{}, invalid("spec.allowedTools", "allowedTools needs upstream")
	}
	sv, err := validateDraft(cluster, ns, name, draft)
	if err != nil {
		return spec, Server{}, invalid("spec", "%v", err)
	}
	if sv.RegistryName != "" && !registryNameAllowed(platform, ns, sv.RegistryName) {
		return spec, Server{}, invalid("spec.registry.name", "%q is not yours: team %s may use no.nav.%s/<name> or io.github.navikt/%s; other names are registered by the platform team", sv.RegistryName, ns, ns, ns)
	}
	return spec, sv, nil
}

// ForeignRegistryNames lists "<namespace>/<name>=<registry name>" for
// servers whose stored registry name the prefix rule would now refuse.
func (d *DB) ForeignRegistryNames(ctx context.Context, platform string) ([]string, error) {
	rows, err := d.pool.Query(ctx, `SELECT namespace, name, registry_name FROM servers WHERE registry_name IS NOT NULL ORDER BY 1, 2`)
	if err != nil {
		return nil, err
	}
	var out []string
	for rows.Next() {
		var ns, name, reg string
		if err := rows.Scan(&ns, &name, &reg); err != nil {
			return nil, err
		}
		if !registryNameAllowed(platform, ns, reg) {
			out = append(out, ns+"/"+name+"="+reg)
		}
	}
	return out, rows.Err()
}

// registryNameAllowed binds a registry name prefix to the team (#2853). The
// platform team may register any name, including vendor names such as
// com.microsoft/*. An unset platform team grants no one that.
func registryNameAllowed(platform, ns, regName string) bool {
	if platform != "" && ns == platform {
		return true
	}
	n := strings.ToLower(regName)
	return strings.HasPrefix(n, "no.nav."+ns+"/") || n == "io.github.navikt/"+ns
}

// --- handlers ---

func listServersNS(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns := r.PathValue("namespace")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	return writeServerList(r.Context(), w, db, []string{ns})
}

func listMyServers(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	return writeServerList(r.Context(), w, db, c.Teams)
}

func writeServerList(ctx context.Context, w http.ResponseWriter, db *DB, teams []string) error {
	items, err := db.serverObjects(ctx, teams, "")
	if err != nil {
		return err
	}
	writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "MCPServerList", "items": emptyIfNil(items)})
	return nil
}

func getServerV1(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	return writeServer(r.Context(), w, db, ns, name, http.StatusOK)
}

func writeServer(ctx context.Context, w http.ResponseWriter, db *DB, ns, name string, code int) error {
	obj, err := db.serverObject(ctx, ns, name)
	if err != nil {
		return err
	}
	w.Header().Set("ETag", `"`+obj.Metadata.ResourceVersion+`"`)
	writeObject(w, code, obj)
	return nil
}

func getServerGeneration(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	n, err := pathGeneration(r)
	if err != nil {
		return err
	}
	obj, err := db.serverObject(r.Context(), ns, name)
	if err != nil {
		return err
	}
	v, err := loadVersion(r.Context(), db.pool, ns, name, n)
	if err != nil {
		return err
	}
	obj.Spec = v.Spec
	obj.Metadata.Generation = int64(n)
	obj.Status = MCPServerStatus{ObservedGeneration: obj.Status.ObservedGeneration, ContentHash: deref(v.Hash), SubmittedBy: v.submitter(), ApprovedBy: v.approver(), ApprovedAt: v.ApprovedAt}
	writeObject(w, http.StatusOK, obj)
	return nil
}

func (s *server) createServer(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns := r.PathValue("namespace")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	in, err := decodeResource[MCPServerSpec](w, r, "MCPServer", ns, "")
	if err != nil {
		return err
	}
	spec, sv, err := validateServerSpec(s.cluster, s.operatorTeam, ns, in.Metadata.Name, in.Spec)
	if err != nil {
		return err
	}
	if err := db.submitServer(r.Context(), c, in.Metadata, spec, sv, true); err != nil {
		return err
	}
	return writeServer(r.Context(), w, db, ns, in.Metadata.Name, http.StatusCreated)
}

func (s *server) putServer(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	in, err := decodeResource[MCPServerSpec](w, r, "MCPServer", ns, name)
	if err != nil {
		return err
	}
	spec, sv, err := validateServerSpec(s.cluster, s.operatorTeam, ns, name, in.Spec)
	if err != nil {
		return err
	}
	if err := db.submitServer(r.Context(), c, in.Metadata, spec, sv, false); err != nil {
		return err
	}
	return writeServer(r.Context(), w, db, ns, name, http.StatusOK)
}

func (s *server) approveServer(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	if err := memberOf(c, ns); err != nil {
		return err
	}
	var body struct {
		Generation  int    `json:"generation"`
		ContentHash string `json:"contentHash"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		return stErr(http.StatusBadRequest, "BadRequest", "%v", err)
	}
	if err := db.approveServer(r.Context(), c, s.operatorTeam, ns, name, body.Generation, body.ContentHash); err != nil {
		return err
	}
	s.notifyServer(r.Context(), db, "approve", ns, name, "owner")
	return writeServer(r.Context(), w, db, ns, name, http.StatusOK)
}

// operatorOr lets members of ns, and of the platform team, stop a resource.
// Anyone else gets 404. It returns who acted: "owner" or "platform".
func (s *server) operatorOr(c caller, ns string) (string, error) {
	switch {
	case c.in(ns):
		return "owner", nil
	case s.operatorTeam != "" && c.in(s.operatorTeam):
		return "platform", nil
	}
	return "", errV1NotFound
}

func reasonText(raw string) (string, error) {
	if t := strings.TrimSpace(raw); t != "" && len(t) <= 500 {
		return t, nil
	}
	return "", stErr(http.StatusBadRequest, "BadRequest", "reason must be 1 to 500 characters")
}

func (s *server) revokeServer(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	by, err := s.operatorOr(c, ns)
	if err != nil {
		return err
	}
	var body struct {
		Reason string `json:"reason"`
		Block  bool   `json:"block"`
	}
	if err = decodeBody(w, r, &body); err != nil {
		return stErr(http.StatusBadRequest, "BadRequest", "%v", err)
	}
	reason, err := reasonText(body.Reason)
	if err != nil {
		return err
	}
	if err := db.revokeServer(r.Context(), c, by, ns, name, reason, body.Block); err != nil {
		return err
	}
	s.notifyServer(r.Context(), db, "revoke", ns, name, by)
	return writeServerFor(r.Context(), w, db, c, ns, name)
}

func (s *server) rollbackServer(w http.ResponseWriter, r *http.Request, c caller, db *DB) error {
	ns, name := r.PathValue("namespace"), r.PathValue("name")
	by, err := s.operatorOr(c, ns)
	if err != nil {
		return err
	}
	var body struct {
		FromGeneration int    `json:"fromGeneration"`
		ToGeneration   *int   `json:"toGeneration"`
		Reason         string `json:"reason"`
	}
	if err = decodeBody(w, r, &body); err != nil {
		return stErr(http.StatusBadRequest, "BadRequest", "%v", err)
	}
	reason, err := reasonText(body.Reason)
	if err != nil {
		return err
	}
	if err := db.rollbackServer(r.Context(), c, s.operatorTeam, by, ns, name, body.FromGeneration, body.ToGeneration, reason); err != nil {
		return err
	}
	s.notifyServer(r.Context(), db, "rollback", ns, name, by)
	return writeServerFor(r.Context(), w, db, c, ns, name)
}

// writeServerFor answers the platform team, which is not in the owner team,
// without the owner-only parts.
func writeServerFor(ctx context.Context, w http.ResponseWriter, db *DB, c caller, ns, name string) error {
	obj, err := db.serverObject(ctx, ns, name)
	if err != nil {
		return err
	}
	if !c.in(ns) {
		obj.Status.Pending, obj.Status.SubmittedBy, obj.Status.ApprovedBy = nil, nil, nil
		obj.Spec.Upstream = nil
	}
	writeObject(w, http.StatusOK, obj)
	return nil
}

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
	Blocked                    bool
	SubOid, RegName, ApprovOid *string
}

func (v versionRow) submitter() *Actor { return &Actor{Kind: deref(v.SubKind), Name: v.SubName} }

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

func loadVersion(ctx context.Context, q querier, ns, name string, version int) (versionRow, error) {
	var v versionRow
	var spec string
	err := q.QueryRow(ctx, `SELECT v.version, COALESCE(v.spec, `+legacySpecSQL+`)::text, v.spec_hash, v.submitted_by_kind, v.submitted_by,
		v.approved_by, v.approved_at, v.published_at, v.revoked_at, v.blocked, v.submitted_by_oid::text, v.registry->>'name', v.approved_by_oid::text
		FROM server_versions v WHERE v.namespace = $1 AND v.name = $2 AND v.version = $3`, ns, name, version).
		Scan(&v.Version, &spec, &v.Hash, &v.SubKind, &v.SubName, &v.ApprovedBy, &v.ApprovedAt, &v.PublishedAt, &v.RevokedAt, &v.Blocked, &v.SubOid, &v.RegName, &v.ApprovOid)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, errV1NotFound
	}
	if err != nil {
		return v, err
	}
	return v, json.Unmarshal([]byte(spec), &v.Spec)
}

// lastEvents returns the time of the newest audit event per action for one resource.
func lastEvents(ctx context.Context, q querier, kind, ns, name string, legacy []string) (map[string]time.Time, error) {
	rows, err := q.Query(ctx, `SELECT action, max(at) FROM audit_events WHERE namespace = $1 AND name = $2
		AND (resource_kind = $3 OR (resource_kind IS NULL AND action = ANY($4))) GROUP BY action`, ns, name, kind, legacy)
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
		st.Pending = &Pending[MCPServerSpec]{Generation: v.Version, ContentHash: deref(v.Hash), SubmittedBy: v.submitter(), Spec: &spec}
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
	pubAt := at("approved_published", "rolled_back", "revoked")
	published, revoked := row.Status == "published", row.Status == "revoked"
	pubReason := "NeverApproved"
	switch {
	case revoked:
		pubReason = "Revoked"
	case published && ev["rolled_back"].After(ev["approved_published"]):
		pubReason = "RolledBack"
	case published:
		pubReason = "Approved"
	}
	pendReason := "NothingPending"
	if row.Pending != nil {
		pendReason = "AwaitingSecondPerson"
	}
	revReason := "NotRevoked"
	if revoked {
		revReason = "RevokedByOwner"
		var by string
		_ = d.pool.QueryRow(ctx, `SELECT COALESCE(detail->>'by', '') FROM audit_events WHERE namespace = $1 AND name = $2 AND action = 'revoked' ORDER BY id DESC LIMIT 1`, row.NS, row.Name).Scan(&by)
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
		cond("PendingApproval", row.Pending != nil, pendReason, gen, at("draft_submitted", "approved_published")),
		cond("Revoked", revoked, revReason, gen, at("revoked", "approved_published")),
		cond("Ready", published, readyReason, gen, pubAt),
	}
	return obj, nil
}

// auditV1 writes an audit row with the actor's form and oid in their own
// columns. detail never holds an oid. version mirrors generation, so the
// /admin/v1 audit view keeps working.
func auditV1(ctx context.Context, tx pgx.Tx, c caller, action, kind, ns, name string, gen *int, detail map[string]any) error {
	return insertAudit(ctx, tx, auditRow{Actor: c.Ident, ActorKind: c.actor().Kind, ActorOid: c.Oid, ActorClientID: c.Azp,
		Action: action, ResourceKind: kind, Namespace: ns, Name: name, Version: gen, Generation: gen, Detail: detail})
}

// blockedContent is 422 when any of hashes equals a blocked generation's
// hash on this resource. Hashes compare as whole strings (see specHashes).
func blockedContent(ctx context.Context, tx pgx.Tx, table, ns, name string, hashes []string) error {
	var blocked bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE namespace = $1 AND name = $2 AND blocked AND spec_hash = ANY($3))`, ns, name, hashes).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return invalid("spec", "this content is blocked and can never be published again")
	}
	return nil
}

// submitServer is POST (create) and PUT. A new generation is made only when
// the spec differs from both the pending and the published one, or when the
// server is revoked (restart). Otherwise only labels and annotations change.
func (d *DB) submitServer(ctx context.Context, c caller, m ObjectMeta, spec MCPServerSpec, sv Server, create bool) error {
	hashes, herr := specHashes(spec)
	if herr != nil {
		return herr
	}
	hash := hashes[0]
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if create {
			tag, err := tx.Exec(ctx, `INSERT INTO servers (namespace, name, status, labels, annotations) VALUES ($1, $2, 'draft', $3::jsonb, $4::jsonb) ON CONFLICT DO NOTHING`,
				sv.Namespace, sv.Name, jsonText(m.Labels), jsonText(m.Annotations))
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return stErr(http.StatusConflict, "AlreadyExists", "mcpservers %q already exists", sv.Name)
			}
		}
		row, err := lockServerRow(ctx, tx, sv.Namespace, sv.Name)
		if err != nil {
			return err
		}
		if !create {
			if err = requireResourceVersion(m.ResourceVersion, row.RV); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE servers SET labels = $3::jsonb, annotations = $4::jsonb WHERE namespace = $1 AND name = $2`,
				sv.Namespace, sv.Name, jsonText(m.Labels), jsonText(m.Annotations)); err != nil {
				return err
			}
			same := false
			for _, v := range []*int{row.Pending, row.Published} {
				if v == nil {
					continue
				}
				var h *string
				if err = tx.QueryRow(ctx, `SELECT spec_hash FROM server_versions WHERE namespace = $1 AND name = $2 AND version = $3`, sv.Namespace, sv.Name, *v).Scan(&h); err != nil {
					return err
				}
				same = same || deref(h) == hash
			}
			if same && row.Status != "revoked" {
				return auditV1(ctx, tx, c, "metadata_updated", "MCPServer", sv.Namespace, sv.Name, nil, map[string]any{"labels": m.Labels, "annotations": m.Annotations})
			}
		}
		if err = blockedContent(ctx, tx, "server_versions", sv.Namespace, sv.Name, hashes); err != nil {
			return err
		}
		version := row.Latest + 1
		var registry, path, target, upstream *string
		var tools []string
		if sv.Registry != nil {
			r := string(sv.Registry)
			registry = &r
		}
		if r := sv.Route; r != nil {
			path, target, upstream, tools = &r.Path, &r.Target, &r.UpstreamURL, r.Tools
		}
		specJSON, err := json.Marshal(spec)
		if err != nil {
			return err
		}
		// content_hash too, so an image from before 005 rolled back can still approve it.
		if _, err := tx.Exec(ctx, `INSERT INTO server_versions (namespace, name, version, registry, route_path, target, upstream_url, enabled, tools,
			submitted_by, submitted_by_kind, submitted_by_oid, content_hash, spec, spec_hash)
			VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, true, $8, $9, 'User', $10::uuid, $11, $12::jsonb, $13)`,
			sv.Namespace, sv.Name, version, registry, path, target, upstream, tools, c.Ident, c.Oid, contentHash(sv), string(specJSON), hash); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET pending_version = $3, updated_at = now() WHERE namespace = $1 AND name = $2`, sv.Namespace, sv.Name, version); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "draft_submitted", "MCPServer", sv.Namespace, sv.Name, &version, map[string]any{"contentHash": hash, "replaced": row.Pending})
	})
}

// approveServer publishes the pending generation. The approver is another
// person: different oid and NAVident, and the submitter a User with an oid.
func (d *DB) approveServer(ctx context.Context, c caller, platform, ns, name string, gen int, hash string) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		row, err := lockServerRow(ctx, tx, ns, name)
		if err != nil {
			return err
		}
		if row.Pending == nil || *row.Pending != gen {
			return conflict("generation", "FieldValueInvalid", "generation %d is not the one awaiting approval", gen)
		}
		v, err := loadVersion(ctx, tx, ns, name, gen)
		if err != nil {
			return err
		}
		switch {
		case v.Hash == nil || *v.Hash != hash || !strings.HasPrefix(hash, hashPrefix):
			return conflict("contentHash", "FieldValueInvalid", "contentHash does not match the pending generation")
		case deref(v.SubKind) != "User" || v.SubOid == nil:
			return stErr(http.StatusForbidden, "Forbidden", "the pending generation has no person as submitter; a person must submit it again")
		case *v.SubOid == c.Oid || v.SubName == c.Ident:
			return stErr(http.StatusForbidden, "Forbidden", "the submitter cannot approve their own generation")
		case v.RegName != nil && !registryNameAllowed(platform, ns, *v.RegName):
			return invalid("spec.registry.name", "generation %d has registry name %q, which team %s may not use", gen, *v.RegName, ns)
		}
		hashes := []string{hash}
		if err := blockedContent(ctx, tx, "server_versions", ns, name, hashes); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE server_versions SET approved_by = $4, approved_by_oid = $5::uuid, approved_at = now(), published_at = now()
			WHERE namespace = $1 AND name = $2 AND version = $3`, ns, name, gen, c.Ident, c.Oid); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET published_version = $3, pending_version = NULL, status = 'published', registry_name = $4, updated_at = now()
			WHERE namespace = $1 AND name = $2`, ns, name, gen, v.RegName); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
				return conflict("spec.registry.name", "FieldValueDuplicate", "registry name is used by another server")
			}
			return err
		}
		if err := checkRoutesFit(ctx, tx, ns); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "approved_published", "MCPServer", ns, name, &gen, map[string]any{"contentHash": hash, "submittedBy": v.SubName})
	})
}

// revokeServer stops the published generation. It is final for that
// generation: rollback never undoes it.
func (d *DB) revokeServer(ctx context.Context, c caller, by, ns, name, reason string, block bool) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		row, err := lockServerRow(ctx, tx, ns, name)
		if err != nil {
			return err
		}
		if row.Status != "published" {
			return conflict("", "InvalidState", "server is %s, not published", row.Status)
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET status = 'revoked', updated_at = now() WHERE namespace = $1 AND name = $2`, ns, name); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE server_versions SET revoked_at = clock_timestamp(), blocked = blocked OR $4 WHERE namespace = $1 AND name = $2 AND version = $3`, ns, name, *row.Published, block); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "revoked", "MCPServer", ns, name, row.Published, map[string]any{"reason": reason, "blocked": block, "by": by})
	})
}

// rollbackServer republishes an earlier published generation. fromGeneration
// must be the published one. A refused target is 409, never a skip to an
// older generation the person did not choose.
func (d *DB) rollbackServer(ctx context.Context, c caller, platform, by, ns, name string, from int, to *int, reason string) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		row, err := lockServerRow(ctx, tx, ns, name)
		if err != nil {
			return err
		}
		if row.Status != "published" {
			return conflict("", "InvalidState", "server is %s; rollback never lifts a revocation", row.Status)
		}
		if *row.Published != from {
			return conflict("fromGeneration", "FieldValueInvalid", "the published generation is %d", *row.Published)
		}
		target := 0
		if to != nil {
			target = *to
		} else if err = tx.QueryRow(ctx, `SELECT COALESCE(max(version), 0) FROM server_versions WHERE namespace = $1 AND name = $2 AND published_at IS NOT NULL AND version < $3`, ns, name, from).Scan(&target); err != nil {
			return err
		}
		if target < 1 || target >= from {
			return conflict("toGeneration", "GenerationNotEligible", "rollback goes only to an earlier generation that was published before")
		}
		v, err := loadVersion(ctx, tx, ns, name, target)
		if errors.Is(err, errV1NotFound) || (err == nil && v.PublishedAt == nil) {
			return conflict("toGeneration", "GenerationNotEligible", "generation %d was never published", target)
		}
		if err != nil {
			return err
		}
		var refused bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM server_versions WHERE namespace = $1 AND name = $2 AND (revoked_at IS NOT NULL OR blocked) AND spec_hash = $3)`, ns, name, v.Hash).Scan(&refused); err != nil {
			return err
		}
		switch {
		case v.Blocked:
			return conflict("toGeneration", "GenerationBlocked", "generation %d is blocked", target)
		case v.RevokedAt != nil:
			return conflict("toGeneration", "GenerationRevoked", "generation %d was revoked", target)
		case refused:
			return conflict("toGeneration", "ContentRefused", "generation %d has the same content as a revoked or blocked generation", target)
		}
		if v.RegName != nil && !registryNameAllowed(platform, ns, *v.RegName) {
			return invalid("spec.registry.name", "generation %d has registry name %q, which team %s may not use", target, *v.RegName, ns)
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET published_version = $3, registry_name = $4, updated_at = now() WHERE namespace = $1 AND name = $2`, ns, name, target, v.RegName); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
				return conflict("spec.registry.name", "FieldValueDuplicate", "registry name is used by another server")
			}
			return err
		}
		if err := checkRoutesFit(ctx, tx, ns); err != nil {
			return err
		}
		return auditV1(ctx, tx, c, "rolled_back", "MCPServer", ns, name, &target, map[string]any{"reason": reason, "from": from, "by": by})
	})
}

// BackfillSpecs fills spec and spec_hash for every server version that was
// ever published and has none. It runs on every start and touches only rows
// without spec, so it catches rows that old pods wrote during a rolling
// deploy or after a rollback. Pending versions never get a hash: they must
// be submitted again by a person.
func (d *DB) BackfillSpecs(ctx context.Context) (int, error) {
	n := 0
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT v.namespace, v.name, v.version, `+legacySpecSQL+`::text FROM server_versions v WHERE v.spec IS NULL AND v.published_at IS NOT NULL`)
		if err != nil {
			return err
		}
		type todo struct {
			NS, Name string
			V        int
			Spec     string
		}
		list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[todo])
		if err != nil {
			return err
		}
		for _, t := range list {
			var spec MCPServerSpec
			if err := json.Unmarshal([]byte(t.Spec), &spec); err != nil {
				return err
			}
			spec.AllowedTools = slices.Sorted(slices.Values(spec.AllowedTools))
			if len(spec.AllowedTools) == 0 {
				spec.AllowedTools = nil
			}
			hash, err := specHash(spec)
			if err != nil {
				return err
			}
			raw, _ := json.Marshal(spec)
			if _, err := tx.Exec(ctx, `UPDATE server_versions SET spec = $4::jsonb, spec_hash = $5 WHERE namespace = $1 AND name = $2 AND version = $3 AND spec IS NULL`,
				t.NS, t.Name, t.V, string(raw), hash); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// history returns the newest events for one resource, and whether it exists.
func (d *DB) history(ctx context.Context, kind, ns, name string, legacy []string, limit int) ([]AuditEvent, bool, error) {
	table := "servers"
	if kind == "AgentProfile" {
		table = "agent_profile_resources"
	}
	var found bool
	if err := d.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE namespace = $1 AND name = $2)`, ns, name).Scan(&found); err != nil || !found {
		return nil, found, err
	}
	rows, err := d.pool.Query(ctx, `SELECT at, actor, COALESCE(actor_kind, ''), COALESCE(actor_client_id, ''), action, COALESCE(generation, version), COALESCE(detail, 'null'::jsonb)::text
		FROM audit_events WHERE namespace = $1 AND name = $2 AND (resource_kind = $3 OR (resource_kind IS NULL AND action = ANY($4)))
		ORDER BY id DESC LIMIT $5`, ns, name, kind, legacy, limit)
	if err != nil {
		return nil, true, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AuditEvent, error) {
		var e AuditEvent
		var detail string
		scanErr := row.Scan(&e.At, &e.Actor.Name, &e.Actor.Kind, &e.Actor.ClientID, &e.Action, &e.Generation, &detail)
		if detail != "null" {
			e.Detail = json.RawMessage(detail)
		}
		return e, scanErr
	})
	return emptyIfNil(out), true, err
}

// --- catalog ---

// CatalogEntry has its own type and serialisation, so a new MCPServer field
// never leaks into the catalog by itself. No actors, upstream, labels,
// annotations, uid or resourceVersion.
type CatalogEntry struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		DataCategory string          `json:"dataCategory,omitempty"`
		AllowedTools []string        `json:"allowedTools,omitempty"`
		Registry     json.RawMessage `json:"registry,omitempty"`
	} `json:"spec"`
	Status struct {
		Route               *RouteStatus `json:"route,omitempty"`
		PublishedGeneration int          `json:"publishedGeneration"`
		ApprovedAt          *time.Time   `json:"approvedAt,omitempty"`
	} `json:"status"`
}

func listCatalog(w http.ResponseWriter, r *http.Request, _ caller, db *DB) error {
	items, err := db.catalog(r.Context(), r.PathValue("namespace"), "")
	if err != nil {
		return err
	}
	writeObject(w, http.StatusOK, map[string]any{"apiVersion": apiGroupVersion, "kind": "MCPServerCatalogEntryList", "items": items})
	return nil
}

func getCatalogEntry(w http.ResponseWriter, r *http.Request, _ caller, db *DB) error {
	items, err := db.catalog(r.Context(), r.PathValue("namespace"), r.PathValue("name"))
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return errV1NotFound
	}
	writeObject(w, http.StatusOK, items[0])
	return nil
}

// catalog lists published, unrevoked servers, optionally for one namespace or name.
func (d *DB) catalog(ctx context.Context, ns, name string) ([]CatalogEntry, error) {
	rows, err := d.pool.Query(ctx, `SELECT s.namespace, s.name, v.version, COALESCE(v.spec, `+legacySpecSQL+`)::text, v.approved_at
		FROM servers s JOIN server_versions v ON v.namespace = s.namespace AND v.name = s.name AND v.version = s.published_version
		WHERE s.status = 'published' AND ($1 = '' OR s.namespace = $1) AND ($2 = '' OR s.name = $2)
		ORDER BY s.namespace, s.name`, ns, name)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (CatalogEntry, error) {
		var e CatalogEntry
		var spec string
		if err = row.Scan(&e.Metadata.Namespace, &e.Metadata.Name, &e.Status.PublishedGeneration, &spec, &e.Status.ApprovedAt); err != nil {
			return e, err
		}
		var s MCPServerSpec
		if err = json.Unmarshal([]byte(spec), &s); err != nil {
			return e, err
		}
		e.APIVersion, e.Kind = apiGroupVersion, "MCPServerCatalogEntry"
		e.Spec.DataCategory, e.Spec.AllowedTools, e.Spec.Registry = s.DataCategory, s.AllowedTools, s.Registry
		if s.Upstream != nil {
			e.Status.Route = &RouteStatus{Path: "/mcp/" + e.Metadata.Namespace + "/" + e.Metadata.Name}
		}
		return e, nil
	})
	return emptyIfNil(out), err
}
