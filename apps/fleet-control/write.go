package main

import (
	"context"
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

	"github.com/navikt/copilot/fleet-control/internal/model"
	"github.com/navikt/copilot/fleet-control/internal/store"
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
func validateDraft(cluster, ns, name string, spec draftSpec) (model.Server, error) {
	s := model.Server{Namespace: ns, Name: name}
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
		s.Route = &model.Route{Path: "/mcp/" + ns + "/" + name, Target: r.Target, UpstreamURL: r.UpstreamURL, Enabled: r.Enabled == nil || *r.Enabled, Tools: r.Tools}
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
		if e.GatewayPath == "" {
			e.Namespace = ns
		}
		if e.Namespace != ns || e.Name != name {
			return s, fmt.Errorf("registry: entry %q maps to %s, not %s", e.RegistryName, e.Key(), s.Key())
		}
		// As in the seed: a registry entry joins a route only through a gateway remote to it.
		if (s.Route == nil) != (e.GatewayPath == "") {
			return s, errors.New("registry: a gateway remote needs a route, and a route needs a gateway remote")
		}
		s.RegistryName, s.Registry = e.RegistryName, e.Registry
	}
	return s, nil
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

// checkRoutesFit runs routesFit on what tx would publish. The routes snapshot
// lock serialises it with concurrent approvals and rollbacks, so two writes
// that each fit cannot together exceed the limit.
func checkRoutesFit(ctx context.Context, tx pgx.Tx, ns string) error {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", store.SnapshotLockKey("routes")); err != nil {
		return err
	}
	servers, err := store.QueryServers(ctx, tx)
	if err != nil {
		return err
	}
	if routesFitPause != nil {
		routesFitPause()
	}
	return routesFit(servers, ns)
}

// routesFitPause lets a test widen the window between read and commit.
var routesFitPause func()

// adminHandler gets the acting person, the team it acts on and the store.
type adminHandler func(w http.ResponseWriter, r *http.Request, js jobStore, actor, ns string) error

// admin guards a write route: valid names (400), a user token from my-copilot
// (401, 503 when Texas fails), membership of the owning team (403, 503 when
// the lookup fails) and a database (503). Every check fails closed.
func (s *server) admin(a *authz, adminClientID string, teams teamDirectory, h adminHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ns := r.PathValue("namespace")
		if !dnsLabelRegex.MatchString(ns) {
			http.Error(w, "namespace must be a lowercase DNS label", http.StatusBadRequest)
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
		if !s.writeAllowed(ns) {
			http.Error(w, "forbidden: writes are not enabled for team "+ns+" in this cluster", http.StatusForbidden)
			return
		}
		js := s.jobs()
		if js == nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		writeAPIError(w, "write "+ns, h(w, r.WithContext(ctx), js, actor, ns))
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
