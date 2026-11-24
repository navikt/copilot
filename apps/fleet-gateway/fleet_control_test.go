package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type fakeFleetControl struct {
	srv         *httptest.Server
	texasStatus int
	status      int
	body        any
	etag        string
	requests    atomic.Int32
	lastINM     string
}

func snapRoute(name string) map[string]any {
	return map[string]any{
		"namespace": "copilot", "name": name, "path": "/mcp/copilot/" + name,
		"target": "dev-gcp:copilot:" + name, "upstream_url": "http://" + name + ".copilot/",
		"enabled": true, "tools": []string{"whoami"},
	}
}

// primaryRoute is newTestEnv's only route, so its primary resource.
func primaryRoute() map[string]any {
	return map[string]any{
		"namespace": "team-etterlevelse", "name": "etterlevelse", "path": "/mcp/team-etterlevelse/etterlevelse",
		"target": "dev-gcp:team-etterlevelse:nav-etterlevelse-mcp", "upstream_url": "http://nav-etterlevelse-mcp.team-etterlevelse",
		"enabled": true, "tools": []string{"whoami"},
	}
}

func snapBody(gen int, routes ...map[string]any) map[string]any {
	return map[string]any{"schema_version": 1, "generation": gen, "routes": routes}
}

func newFakeFleetControl(t *testing.T) *fakeFleetControl {
	f := &fakeFleetControl{texasStatus: http.StatusOK, status: http.StatusOK, etag: `"e1"`, body: snapBody(1, primaryRoute(), snapRoute("alpha"), snapRoute("beta"))}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		switch r.URL.Path {
		case "/texas":
			var in map[string]string
			_ = json.NewDecoder(r.Body).Decode(&in)
			if f.texasStatus != http.StatusOK || in["target"] != "api://dev-gcp.copilot.fleet-control/.default" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "m2m"})
		case "/internal/v1/snapshots/routes":
			if r.Header.Get("Authorization") != "Bearer m2m" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			f.lastINM = r.Header.Get("If-None-Match")
			w.Header().Set("ETag", f.etag)
			if f.lastINM == f.etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.WriteHeader(f.status)
			_ = json.NewEncoder(w).Encode(f.body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func fleetControlGateway(t *testing.T, f *fakeFleetControl) (*Gateway, *fleetControlRoutes) {
	t.Helper()
	gw := newTestEnv(t).gw
	gw.cfg.RoutesSource = routesSourceFleetControl
	gw.cfg.FleetControlURL = f.srv.URL
	gw.cfg.TexasTokenEndpoint = f.srv.URL + "/texas"
	gw.cfg.FleetControlAudience = "api://dev-gcp.copilot.fleet-control/.default"
	// Rebuild through newGateway so the startup route table is the production one.
	gw, err := newGateway(gw.cfg, gw.now)
	if err != nil {
		t.Fatal(err)
	}
	return gw, &fleetControlRoutes{cfg: gw.cfg, client: f.srv.Client()}
}

func routePaths(gw *Gateway) []string {
	var out []string
	for _, r := range gw.table().Routes {
		out = append(out, r.Route)
	}
	return out
}

func TestRouteSyncSwapsThenHonoursETag(t *testing.T) {
	f := newFakeFleetControl(t)
	gw, src := fleetControlGateway(t, f)

	gw.syncRoutes(context.Background(), src)
	if got := routePaths(gw); len(got) != 3 || got[1] != "/mcp/copilot/alpha" {
		t.Fatalf("routes after first sync = %v", got)
	}
	// The new table serves at once: beta answers 401 (auth), not 404 (unknown).
	rec := httptest.NewRecorder()
	gw.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp/copilot/beta", strings.NewReader(`{}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /mcp/copilot/beta = %d, want 401 from the swapped table", rec.Code)
	}

	f.body = snapBody(2) // would be rejected if read: proves 304 skips the body
	gw.syncRoutes(context.Background(), src)
	if f.lastINM != `"e1"` {
		t.Fatalf("If-None-Match = %q, want the stored ETag", f.lastINM)
	}
	if got := routePaths(gw); len(got) != 3 {
		t.Fatalf("304 changed the table: %v", got)
	}
}

func TestRouteSyncFailsClosed(t *testing.T) {
	bad := snapRoute("gamma")
	bad["upstream_url"] = "http://gamma.other-ns/"
	noTools := snapRoute("delta")
	delete(noTools, "tools")
	pathMismatch := snapRoute("eps")
	pathMismatch["path"] = "/mcp/copilot/other"

	cases := map[string]func(f *fakeFleetControl){
		"empty snapshot":      func(f *fakeFleetControl) { f.body = snapBody(2) },
		"invalid upstream":    func(f *fakeFleetControl) { f.body = snapBody(2, primaryRoute(), snapRoute("alpha"), bad) },
		"route without tools": func(f *fakeFleetControl) { f.body = snapBody(2, primaryRoute(), snapRoute("alpha"), noTools) },
		"path mismatch":       func(f *fakeFleetControl) { f.body = snapBody(2, primaryRoute(), snapRoute("alpha"), pathMismatch) },
		"older generation":    func(f *fakeFleetControl) { f.body = snapBody(0, primaryRoute(), snapRoute("alpha")) },
		"unknown schema": func(f *fakeFleetControl) {
			f.body = map[string]any{"schema_version": 2, "generation": 2, "routes": []any{snapRoute("alpha")}}
		},
		"fleet-control down": func(f *fakeFleetControl) {
			f.status, f.body = http.StatusServiceUnavailable, snapBody(2, primaryRoute(), snapRoute("zeta"))
		},
		"texas down": func(f *fakeFleetControl) {
			f.texasStatus, f.body = http.StatusInternalServerError, snapBody(2, primaryRoute(), snapRoute("zeta"))
		},
		"primary removed": func(f *fakeFleetControl) {
			f.body = map[string]any{"schema_version": 1, "generation": 2, "routes": []any{snapRoute("alpha"), snapRoute("beta")}}
		},
		"same generation, new body": func(f *fakeFleetControl) { f.body = snapBody(1, primaryRoute(), snapRoute("alpha")) },
		"malformed json":            func(f *fakeFleetControl) { f.body = "not an object" },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeFleetControl(t)
			gw, src := fleetControlGateway(t, f)
			gw.syncRoutes(context.Background(), src)
			good := routePaths(gw)

			breakIt(f)
			f.etag = `"e2"`
			gw.syncRoutes(context.Background(), src)
			if got := routePaths(gw); !slices.Equal(got, good) {
				t.Fatalf("bad snapshot replaced the table: %v, want %v", got, good)
			}
		})
	}
}

// #2797: a pod that starts while fleet-control, Texas or the snapshot is bad
// must serve no routes (503, not ready), never the Git file's.
func TestRouteSyncNoSnapshotAtStartupFailsClosed(t *testing.T) {
	cases := map[string]func(f *fakeFleetControl){
		"fleet-control down": func(f *fakeFleetControl) { f.srv.Close() },
		"fleet-control 503":  func(f *fakeFleetControl) { f.status = http.StatusServiceUnavailable },
		"texas down":         func(f *fakeFleetControl) { f.texasStatus = http.StatusInternalServerError },
		"unknown schema": func(f *fakeFleetControl) {
			f.body = map[string]any{"schema_version": 2, "generation": 1, "routes": []any{primaryRoute()}}
		},
		"malformed json": func(f *fakeFleetControl) { f.body = "not an object" },
	}
	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeFleetControl(t)
			gw, src := fleetControlGateway(t, f)
			breakIt(f)
			gw.syncRoutes(context.Background(), src)
			if got := routePaths(gw); len(got) != 0 {
				t.Fatalf("routes = %v, want none before the first snapshot", got)
			}
			rec := httptest.NewRecorder()
			gw.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{}`)))
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("Git route = %d, want 503", rec.Code)
			}
			rec = httptest.NewRecorder()
			gw.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("/ready = %d, want 503", rec.Code)
			}

			// First valid snapshot: routes served and ready.
			f2 := newFakeFleetControl(t)
			src.cfg.FleetControlURL, src.cfg.TexasTokenEndpoint, src.client = f2.srv.URL, f2.srv.URL+"/texas", f2.srv.Client()
			gw.syncRoutes(context.Background(), src)
			rec = httptest.NewRecorder()
			gw.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
			if len(routePaths(gw)) != 3 || rec.Code != http.StatusOK {
				t.Fatalf("after snapshot: routes %v, /ready %d", routePaths(gw), rec.Code)
			}
		})
	}
}

func TestRouteSyncFlagOffMakesNoRequest(t *testing.T) {
	f := newFakeFleetControl(t)
	gw, _ := fleetControlGateway(t, f)
	gw.cfg.RoutesSource = routesSourceGit
	before := gw.table()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gw.startRouteSync(ctx, fleetControlSyncInterval)
	if n := f.requests.Load(); n != 0 {
		t.Fatalf("ROUTES_SOURCE=git sent %d requests to fleet-control", n)
	}
	if gw.table() != before {
		t.Fatal("ROUTES_SOURCE=git changed the route table")
	}
}

func TestRouteSyncReorderKeepsPrimary(t *testing.T) {
	f := newFakeFleetControl(t)
	gw, src := fleetControlGateway(t, f)
	primary := gw.table().PrimaryResource
	f.body = map[string]any{"schema_version": 1, "generation": 1, "routes": []any{snapRoute("beta"), snapRoute("alpha"), primaryRoute()}}
	gw.syncRoutes(context.Background(), src)
	if got := routePaths(gw); len(got) != 3 || got[0] != "/mcp/copilot/beta" {
		t.Fatalf("reordered snapshot not applied: %v", got)
	}
	if gw.table().PrimaryResource != primary {
		t.Fatalf("primary resource = %q, want %q", gw.table().PrimaryResource, primary)
	}
}

func TestRouteSyncAppliesLargeShrink(t *testing.T) {
	f := newFakeFleetControl(t)
	gw, src := fleetControlGateway(t, f)
	many := []map[string]any{primaryRoute()}
	for _, n := range []string{"a", "b", "c", "d", "e", "g", "h", "i"} {
		many = append(many, snapRoute(n))
	}
	f.body = map[string]any{"schema_version": 1, "generation": 1, "routes": many}
	gw.syncRoutes(context.Background(), src)
	good := routePaths(gw)
	if len(good) != 9 {
		t.Fatalf("setup: %v", good)
	}
	f.body, f.etag = map[string]any{"schema_version": 1, "generation": 2, "routes": many[:5]}, `"e2"` // drops 4 of 9, limit 3
	gw.syncRoutes(context.Background(), src)
	// fleet-control owns the shrink rule and exempts revocations: a big drop it publishes must apply.
	if got := routePaths(gw); len(got) != 5 {
		t.Fatalf("shrinking snapshot not applied: %v", got)
	}
}

func TestRouteSyncInitialisesMetricsForNewRoutes(t *testing.T) {
	f := newFakeFleetControl(t)
	gw, src := fleetControlGateway(t, f)
	f.body = map[string]any{"schema_version": 1, "generation": 1, "routes": []any{primaryRoute(), snapRoute("metricsonly")}}
	gw.syncRoutes(context.Background(), src)
	got := 0
	ch := make(chan prometheus.Metric, 4096)
	go func() { mcpRequestsTotal.Collect(ch); close(ch) }()
	for m := range ch {
		var d dto.Metric
		_ = m.Write(&d)
		for _, l := range d.GetLabel() {
			if l.GetValue() == "/mcp/copilot/metricsonly" {
				got++
				break
			}
		}
	}
	if got == 0 {
		t.Fatal("no mcp_requests series for a route added by a snapshot")
	}
}

// fleet-control publishes a revoked route as disabled (#2688). Revoking the
// primary route must pass the primary-resource guard and stop serving it.
func TestRouteSyncRevokedPrimaryAnswers503(t *testing.T) {
	f := newFakeFleetControl(t)
	gw, src := fleetControlGateway(t, f)
	revoked := primaryRoute()
	revoked["enabled"] = false
	f.body = snapBody(1, revoked)
	gw.syncRoutes(context.Background(), src)
	if got := routePaths(gw); len(got) != 1 || !gw.table().Routes[0].isDisabled() {
		t.Fatalf("revoked primary not applied as disabled: %v", got)
	}
	rec := httptest.NewRecorder()
	gw.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp/team-etterlevelse/etterlevelse", strings.NewReader(`{}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("revoked route = %d, want 503", rec.Code)
	}
}
