package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
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

func snapBody(gen int, routes ...map[string]any) map[string]any {
	return map[string]any{"schema_version": 1, "generation": gen, "routes": routes}
}

func newFakeFleetControl(t *testing.T) *fakeFleetControl {
	f := &fakeFleetControl{texasStatus: http.StatusOK, status: http.StatusOK, etag: `"e1"`, body: snapBody(1, snapRoute("alpha"), snapRoute("beta"))}
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
	if got := routePaths(gw); len(got) != 2 || got[0] != "/mcp/copilot/alpha" {
		t.Fatalf("routes after first sync = %v", got)
	}
	if _, ok := gw.table().routeByResource(gw.cfg.BaseURL + "/mcp/copilot/beta"); !ok {
		t.Fatal("team tokens cannot resolve a route from the new table")
	}

	f.body = snapBody(2) // would be rejected if read: proves 304 skips the body
	gw.syncRoutes(context.Background(), src)
	if f.lastINM != `"e1"` {
		t.Fatalf("If-None-Match = %q, want the stored ETag", f.lastINM)
	}
	if got := routePaths(gw); len(got) != 2 {
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
		"invalid upstream":    func(f *fakeFleetControl) { f.body = snapBody(2, snapRoute("alpha"), bad) },
		"route without tools": func(f *fakeFleetControl) { f.body = snapBody(2, snapRoute("alpha"), noTools) },
		"path mismatch":       func(f *fakeFleetControl) { f.body = snapBody(2, snapRoute("alpha"), pathMismatch) },
		"older generation":    func(f *fakeFleetControl) { f.body = snapBody(0, snapRoute("alpha")) },
		"unknown schema": func(f *fakeFleetControl) {
			f.body = map[string]any{"schema_version": 2, "generation": 2, "routes": []any{snapRoute("alpha")}}
		},
		"fleet-control down": func(f *fakeFleetControl) {
			f.status, f.body = http.StatusServiceUnavailable, snapBody(2, snapRoute("zeta"))
		},
		"texas down": func(f *fakeFleetControl) {
			f.texasStatus, f.body = http.StatusInternalServerError, snapBody(2, snapRoute("zeta"))
		},
		"malformed json": func(f *fakeFleetControl) { f.body = "not an object" },
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

func TestRouteSyncUnreachableAtStartupKeepsGitRoutes(t *testing.T) {
	f := newFakeFleetControl(t)
	gw, src := fleetControlGateway(t, f)
	git := routePaths(gw)
	f.srv.Close()
	gw.syncRoutes(context.Background(), src)
	if got := routePaths(gw); !slices.Equal(got, git) {
		t.Fatalf("routes = %v, want the Git routes %v", got, git)
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
