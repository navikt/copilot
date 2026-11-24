package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Write API (#2625, #2626). A team member submits a draft version, a second
// member of the same team approves the exact content (by hash), and approval
// publishes it. Any one member can revoke or roll back to the previously
// published version: stopping takes one person, starting takes two.

// draftSpec is the request body for a draft. The route path is not sent: it
// is always /mcp/<namespace>/<name>, so a team can only claim routes in its
// own namespace.
type draftSpec struct {
	Registry json.RawMessage `json:"registry,omitempty"`
	Route    *struct {
		Target      string   `json:"target"`
		UpstreamURL string   `json:"upstream_url"`
		Enabled     *bool    `json:"enabled"`
		Tools       []string `json:"tools"`
	} `json:"route,omitempty"`
}

// validateDraft applies the seed's validation to hostile input and returns
// the server it describes.
func validateDraft(cluster, ns, name string, spec draftSpec) (Server, error) {
	s := Server{Namespace: ns, Name: name}
	if spec.Route == nil && len(spec.Registry) == 0 {
		return s, errors.New("registry or route is required")
	}
	if r := spec.Route; r != nil {
		parts := strings.Split(r.Target, ":")
		if len(parts) != 3 || !dnsLabelRegex.MatchString(parts[0]) || parts[1] != ns || !dnsLabelRegex.MatchString(parts[2]) {
			return s, fmt.Errorf("route.target %q must be <cluster>:%s:<app>", r.Target, ns)
		}
		if parts[0] != cluster {
			return s, fmt.Errorf("route.target must be in cluster %q, where the gateway resolves service DNS", cluster)
		}
		if err := validateUpstreamURL(r.UpstreamURL, ns); err != nil {
			return s, fmt.Errorf("route.upstream_url: %w", err)
		}
		// Unlike the seed, the API allows only the target app's own service DNS:
		// an ingress name proves nothing about which app answers, and the
		// gateway sends the target's token there. fleet-gateway rejects query
		// strings and fragments, so they would block the whole snapshot.
		u, _ := url.Parse(r.UpstreamURL)
		host, _ := normalizeHost(u.Hostname())
		if app := parts[2]; host != app+"."+ns && host != app+"."+ns+".svc.cluster.local" {
			return s, fmt.Errorf("route.upstream_url: host must be %s.%s or %s.%s.svc.cluster.local", app, ns, app, ns)
		}
		if u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
			return s, errors.New("route.upstream_url: query strings and fragments are not allowed")
		}
		if err := validateTools(r.Tools); err != nil {
			return s, fmt.Errorf("route: %w", err)
		}
		s.Route = &Route{Path: "/mcp/" + ns + "/" + name, Target: r.Target, UpstreamURL: r.UpstreamURL, Enabled: r.Enabled == nil || *r.Enabled, Tools: r.Tools}
	}
	if len(spec.Registry) > 0 {
		// mcp-registry rejects the whole snapshot without these.
		// shortcut: only the required fields; a validator shared with
		// mcp-registry is the full fix (#2690).
		var req struct {
			Description any `json:"description"`
			Version     any `json:"version"`
		}
		_ = json.Unmarshal(spec.Registry, &req)
		desc, _ := req.Description.(string)
		ver, _ := req.Version.(string)
		if strings.TrimSpace(desc) == "" || len(desc) > 100 || strings.TrimSpace(ver) == "" {
			return s, errors.New("registry: description (1 to 100 characters) and version are required strings")
		}
		e, err := parseRegistryEntry(spec.Registry)
		if err != nil {
			return s, fmt.Errorf("registry: %w", err)
		}
		// Without a gateway remote the parser defaults the namespace to
		// copilot; the owning namespace is then the authenticated path's.
		if e.gatewayPath == "" {
			e.Namespace = ns
		}
		if e.Namespace != ns || e.Name != name {
			return s, fmt.Errorf("registry: entry %q maps to %s, not %s", e.RegistryName, e.Key(), s.Key())
		}
		// As in the seed: a registry entry joins a route only through a gateway remote to it.
		if (s.Route == nil) != (e.gatewayPath == "") {
			return s, errors.New("registry: a gateway remote needs a route, and a route needs a gateway remote")
		}
		s.RegistryName, s.Registry = e.RegistryName, e.Registry
	}
	return s, nil
}

// contentHash is what an approver reviews and approves.
func contentHash(s Server) string {
	raw, _ := json.Marshal(struct {
		Registry json.RawMessage `json:"registry,omitempty"`
		Route    *Route          `json:"route,omitempty"`
	}{s.Registry, s.Route})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// apiError is a refusal the caller can act on; any other error is a 503.
type apiError struct {
	code int
	msg  string
}

func (e *apiError) Error() string { return e.msg }

func refuse(code int, format string, args ...any) error {
	return &apiError{code, fmt.Sprintf(format, args...)}
}

// writeStore is implemented by *DB.
type writeStore interface {
	ServerDetail(ctx context.Context, ns, name string) (map[string]any, error)
	SaveDraft(ctx context.Context, actor string, s Server, hash string) (int, error)
	Approve(ctx context.Context, actor, ns, name string, version int, hash string) error
	Revoke(ctx context.Context, actor, ns, name, reason string, block bool) error
	Rollback(ctx context.Context, actor, ns, name string) (int, error)
}

type serverState struct {
	Published *int   `json:"published_version"`
	Pending   *int   `json:"pending_version"`
	Status    string `json:"status"`
}

// lockServer locks the server row for the rest of tx, so concurrent writes to
// one server run one at a time.
func lockServer(ctx context.Context, tx pgx.Tx, ns, name string) (serverState, error) {
	var st serverState
	err := tx.QueryRow(ctx, `SELECT published_version, pending_version, status FROM servers WHERE namespace = $1 AND name = $2 FOR UPDATE`, ns, name).Scan(&st.Published, &st.Pending, &st.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, refuse(http.StatusNotFound, "server not found")
	}
	return st, err
}

func audit(ctx context.Context, tx pgx.Tx, actor, action, ns, name string, version *int, before, after serverState, extra map[string]any) error {
	detail := map[string]any{"before": before, "after": after}
	for k, v := range extra {
		detail[k] = v
	}
	return insertAudit(ctx, tx, auditRow{Actor: actor, Action: action, Namespace: ns, Name: name, Version: version, Detail: detail})
}

// ServerDetail returns the published and pending version of a server.
func (d *DB) ServerDetail(ctx context.Context, ns, name string) (map[string]any, error) {
	var st serverState
	err := d.pool.QueryRow(ctx, `SELECT published_version, pending_version, status FROM servers WHERE namespace = $1 AND name = $2`, ns, name).Scan(&st.Published, &st.Pending, &st.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, refuse(http.StatusNotFound, "server not found")
	}
	if err != nil {
		return nil, err
	}
	out := map[string]any{"namespace": ns, "name": name, "state": st}
	for key, v := range map[string]*int{"published": st.Published, "pending": st.Pending} {
		if v == nil {
			continue
		}
		var registry, path, target, upstream, hash, approvedBy *string
		var enabled bool
		var tools []string
		var submittedBy string
		if err := d.pool.QueryRow(ctx, `SELECT registry::text, route_path, target, upstream_url, enabled, tools, submitted_by, content_hash, approved_by
			FROM server_versions WHERE namespace = $1 AND name = $2 AND version = $3`, ns, name, *v).
			Scan(&registry, &path, &target, &upstream, &enabled, &tools, &submittedBy, &hash, &approvedBy); err != nil {
			return nil, err
		}
		ver := map[string]any{"version": *v, "submitted_by": submittedBy, "content_hash": hash, "approved_by": approvedBy}
		if registry != nil {
			ver["registry"] = json.RawMessage(*registry)
		}
		if path != nil {
			ver["route"] = Route{Path: *path, Target: *target, UpstreamURL: *upstream, Enabled: enabled, Tools: tools}
		}
		out[key] = ver
	}
	return out, nil
}

// SaveDraft stores s as a new version and makes it the pending one. A newer
// draft replaces an older pending one, which then needs no approval.
func (d *DB) SaveDraft(ctx context.Context, actor string, s Server, hash string) (int, error) {
	var version int
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO servers (namespace, name, status) VALUES ($1, $2, 'draft') ON CONFLICT DO NOTHING`, s.Namespace, s.Name); err != nil {
			return err
		}
		before, err := lockServer(ctx, tx, s.Namespace, s.Name)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM server_versions WHERE namespace = $1 AND name = $2`, s.Namespace, s.Name).Scan(&version); err != nil {
			return err
		}
		var registry, path, target, upstream *string
		var tools []string
		enabled := true
		if s.Registry != nil {
			r := string(s.Registry)
			registry = &r
		}
		if r := s.Route; r != nil {
			path, target, upstream, tools, enabled = &r.Path, &r.Target, &r.UpstreamURL, r.Tools, r.Enabled
		}
		if _, err := tx.Exec(ctx, `INSERT INTO server_versions (namespace, name, version, registry, route_path, target, upstream_url, enabled, tools, submitted_by, content_hash)
			VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, $8, $9, $10, $11)`, s.Namespace, s.Name, version, registry, path, target, upstream, enabled, tools, actor, hash); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET pending_version = $3, updated_at = now() WHERE namespace = $1 AND name = $2`, s.Namespace, s.Name, version); err != nil {
			return err
		}
		after := before
		after.Pending = &version
		return audit(ctx, tx, actor, "draft_submitted", s.Namespace, s.Name, &version, before, after, map[string]any{"content_hash": hash})
	})
	return version, err
}

// Approve publishes the pending version when actor is not its submitter and
// hash is the content actor reviewed. The caller checks team membership.
func (d *DB) Approve(ctx context.Context, actor, ns, name string, version int, hash string) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		before, err := lockServer(ctx, tx, ns, name)
		if err != nil {
			return err
		}
		if before.Pending == nil || *before.Pending != version {
			return refuse(http.StatusConflict, "version %d is not the pending draft", version)
		}
		var submittedBy string
		var stored, regName *string
		if err := tx.QueryRow(ctx, `SELECT submitted_by, content_hash, registry->>'name' FROM server_versions WHERE namespace = $1 AND name = $2 AND version = $3`, ns, name, version).Scan(&submittedBy, &stored, &regName); err != nil {
			return err
		}
		if submittedBy == actor {
			return refuse(http.StatusForbidden, "the submitter cannot approve their own draft")
		}
		if stored == nil || *stored != hash {
			return refuse(http.StatusConflict, "content_hash does not match the pending draft")
		}
		if _, err := tx.Exec(ctx, `UPDATE server_versions SET approved_by = $4, approved_at = now(), published_at = now() WHERE namespace = $1 AND name = $2 AND version = $3`, ns, name, version, actor); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET published_version = $3, pending_version = NULL, status = 'published', registry_name = $4, updated_at = now() WHERE namespace = $1 AND name = $2`, ns, name, version, regName); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
				return refuse(http.StatusConflict, "registry name is used by another server")
			}
			return err
		}
		if err := checkRoutesFit(ctx, tx); err != nil {
			return err
		}
		after := serverState{Published: &version, Status: "published"}
		return audit(ctx, tx, actor, "approved_published", ns, name, &version, before, after, map[string]any{"content_hash": hash, "submitted_by": submittedBy})
	})
}

// checkRoutesFit runs routesFit on what tx would publish. The routes snapshot
// lock serialises it with concurrent approvals and rollbacks, so two writes
// that each fit cannot together exceed the limit.
func checkRoutesFit(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", snapshotLockKey("routes")); err != nil {
		return err
	}
	servers, err := queryServers(ctx, tx)
	if err != nil {
		return err
	}
	if routesFitPause != nil {
		routesFitPause()
	}
	return routesFit(servers)
}

// routesFitPause lets a test widen the window between read and commit.
var routesFitPause func()

// Revoke stops a published server. One person is enough. block also marks
// the published version so rollback never returns to it.
func (d *DB) Revoke(ctx context.Context, actor, ns, name, reason string, block bool) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		before, err := lockServer(ctx, tx, ns, name)
		if err != nil {
			return err
		}
		if before.Status != "published" {
			return refuse(http.StatusConflict, "server is %s, not published", before.Status)
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET status = 'revoked', updated_at = now() WHERE namespace = $1 AND name = $2`, ns, name); err != nil {
			return err
		}
		// Mark the generation (005), so no rollback through either API returns to it.
		if _, err := tx.Exec(ctx, `UPDATE server_versions SET revoked_at = clock_timestamp() WHERE namespace = $1 AND name = $2 AND version = $3`, ns, name, *before.Published); err != nil {
			return err
		}
		if block {
			if _, err := tx.Exec(ctx, `UPDATE server_versions SET blocked = true WHERE namespace = $1 AND name = $2 AND version = $3`, ns, name, *before.Published); err != nil {
				return err
			}
		}
		after := before
		after.Status = "revoked"
		return audit(ctx, tx, actor, "revoked", ns, name, before.Published, before, after, map[string]any{"reason": reason, "blocked": block})
	})
}

// Rollback republishes the version published before the current one. It
// keeps the status, so a revoked server stays revoked.
func (d *DB) Rollback(ctx context.Context, actor, ns, name string) (int, error) {
	var target int
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		before, err := lockServer(ctx, tx, ns, name)
		if err != nil {
			return err
		}
		if before.Published == nil {
			return refuse(http.StatusConflict, "server has no published version")
		}
		var regName *string
		err = tx.QueryRow(ctx, `SELECT version, registry->>'name' FROM server_versions WHERE namespace = $1 AND name = $2 AND published_at IS NOT NULL AND NOT blocked AND revoked_at IS NULL AND version < $3 ORDER BY version DESC LIMIT 1`, ns, name, *before.Published).Scan(&target, &regName)
		if errors.Is(err, pgx.ErrNoRows) {
			return refuse(http.StatusConflict, "no earlier published, unblocked, unrevoked version")
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE servers SET published_version = $3, registry_name = $4, updated_at = now() WHERE namespace = $1 AND name = $2`, ns, name, target, regName); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
				return refuse(http.StatusConflict, "registry name is used by another server")
			}
			return err
		}
		if err := checkRoutesFit(ctx, tx); err != nil {
			return err
		}
		after := before
		after.Published = &target
		return audit(ctx, tx, actor, "rolled_back", ns, name, &target, before, after, nil)
	})
	return target, err
}

// adminHandler gets the acting person, the server it acts on and the store.
type adminHandler func(w http.ResponseWriter, r *http.Request, ws writeStore, actor, ns, name string) error

// admin guards a write route: valid names (400), a user token from my-copilot
// (401, 503 when Texas fails), membership of the owning team (403, 503 when
// the lookup fails) and a database (503). Every check fails closed.
func (s *server) admin(a *authz, adminClientID string, teams teamDirectory, h adminHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ns, name := r.PathValue("namespace"), r.PathValue("name")
		// Job routes have no {name}.
		if !dnsLabelRegex.MatchString(ns) || (name != "" && !dnsLabelRegex.MatchString(name)) {
			http.Error(w, "namespace and name must be lowercase DNS labels", http.StatusBadRequest)
			return
		}
		actor, email, err := a.user(r.Context(), r.Header.Get("Authorization"), adminClientID)
		if !writeAuthError(w, err) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		member, err := teams.IsMember(ctx, ns, email)
		if err != nil {
			slog.Error("team lookup failed", "team", ns, "error", err)
			http.Error(w, "team lookup unavailable", http.StatusServiceUnavailable)
			return
		}
		if !member {
			http.Error(w, "forbidden: not a member of team "+ns, http.StatusForbidden)
			return
		}
		box := s.store.Load()
		var ws writeStore
		if box != nil {
			ws, _ = box.store.(writeStore)
		}
		if ws == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		writeAPIError(w, "write "+ns+"/"+name, h(w, r.WithContext(ctx), ws, actor, ns, name))
	}
}

// decodeBody reads one JSON object of at most 1 MiB with no unknown fields.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return refuse(http.StatusBadRequest, "invalid body: %v", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return refuse(http.StatusBadRequest, "invalid body: trailing content")
	}
	return nil
}

func getServer(w http.ResponseWriter, r *http.Request, ws writeStore, _, ns, name string) error {
	out, err := ws.ServerDetail(r.Context(), ns, name)
	if err == nil {
		writeJSON(w, out)
	}
	return err
}

func putDraft(w http.ResponseWriter, r *http.Request, ws writeStore, actor, ns, name, cluster string) error {
	var spec draftSpec
	if err := decodeBody(w, r, &spec); err != nil {
		return err
	}
	sv, err := validateDraft(cluster, ns, name, spec)
	if err != nil {
		return refuse(http.StatusBadRequest, "%v", err)
	}
	hash := contentHash(sv)
	version, err := ws.SaveDraft(r.Context(), actor, sv, hash)
	if err == nil {
		writeJSON(w, map[string]any{"version": version, "content_hash": hash})
	}
	return err
}

func postApprove(w http.ResponseWriter, r *http.Request, ws writeStore, actor, ns, name string) error {
	var body struct {
		Version     int    `json:"version"`
		ContentHash string `json:"content_hash"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		return err
	}
	if err := ws.Approve(r.Context(), actor, ns, name, body.Version, body.ContentHash); err != nil {
		return err
	}
	writeJSON(w, map[string]any{"published_version": body.Version})
	return nil
}

func postRevoke(w http.ResponseWriter, r *http.Request, ws writeStore, actor, ns, name string) error {
	var body struct {
		Reason string `json:"reason"`
		Block  bool   `json:"block"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		return err
	}
	if body.Reason = strings.TrimSpace(body.Reason); body.Reason == "" || len(body.Reason) > 500 {
		return refuse(http.StatusBadRequest, "reason must be 1 to 500 characters")
	}
	if err := ws.Revoke(r.Context(), actor, ns, name, body.Reason, body.Block); err != nil {
		return err
	}
	writeJSON(w, map[string]any{"status": "revoked"})
	return nil
}

func postRollback(w http.ResponseWriter, r *http.Request, ws writeStore, actor, ns, name string) error {
	version, err := ws.Rollback(r.Context(), actor, ns, name)
	if err == nil {
		writeJSON(w, map[string]any{"published_version": version})
	}
	return err
}
