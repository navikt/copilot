package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/navikt/copilot/fleet-gateway/internal/httpx"
	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

// A route swap between dispatch and the token exchange must not split one
// request across two snapshots: the token's audience and the upstream it is
// sent to come from the same route entry (#2641).
func TestTokenAndUpstreamFromOneSnapshot(t *testing.T) {
	te := newTestEnv(t)
	var targets []string
	texas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		targets = append(targets, in["target"])
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"access_token": "team-token", "expires_in": 300})
	}))
	t.Cleanup(texas.Close)
	var otherHits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { otherHits.Add(1) }))
	t.Cleanup(other.Close)
	te.gw.cfg.TexasExchangeEndpoint = texas.URL
	var err error
	if te.gw.TeamTokens, err = newTeamTokenSource(te.gw.cfg, te.gw.store, te.gw.now); err != nil {
		t.Fatal(err)
	}

	token := te.obtainAccessTokenForResource(te.resource)
	orig := te.gw.table().Routes[0]
	swapped := orig
	swapped.Target = "dev-gcp:team-etterlevelse:other-app"
	swapped.ParsedURL, _ = url.Parse(other.URL)
	afterRouteLookup = func() {
		te.gw.routeTable.Store(&routing.Table{Routes: []routing.Route{swapped}, RoutesByPath: map[string]routing.Route{swapped.Route: swapped}, AllowedResources: map[string]bool{swapped.FullRouteID: true}, PrimaryResource: swapped.FullRouteID})
	}
	t.Cleanup(func() { afterRouteLookup = func() {} })

	req := httptest.NewRequest(http.MethodPost, orig.Route, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if len(targets) != 1 || targets[0] != orig.Target {
		t.Fatalf("token exchanged for %v, want only %q (the dispatched route)", targets, orig.Target)
	}
	if otherHits.Load() != 0 {
		t.Fatal("request reached the upstream of a later snapshot")
	}
}

// #2967: tool_risk is new in the routes snapshot and may hold levels this
// replica does not know (propose, or later ones). It must decode and load
// the route as before, not reject the snapshot (#2949).
func TestSnapshotIgnoresToolRisk(t *testing.T) {
	raw := `{"schema_version":1,"generation":1,"checked_at":"2026-11-25T00:00:00Z","routes":[{"namespace":"team-a","name":"svc","path":"/mcp/team-a/svc","target":"dev-gcp:team-a:svc","upstream_url":"http://svc.team-a/mcp","enabled":true,"tools":["a","b"],"tool_risk":{"a":"propose","b":"unknown-level"}}]}`
	var snap routesSnapshot
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		t.Fatal(err)
	}
	routes, err := snapshotRoutes(snap)
	if err != nil || len(routes) != 1 || len(routes[0].Tools) != 2 {
		t.Fatalf("snapshot with tool_risk must load: %v %+v", err, routes)
	}
}
