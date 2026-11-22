package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
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
		writeJSON(w, http.StatusOK, map[string]any{"access_token": "team-token", "expires_in": 300})
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
		te.gw.routeTable.Store(&RouteTable{Routes: []PilotRoute{swapped}, RoutesByPath: map[string]PilotRoute{swapped.Route: swapped}, AllowedResources: map[string]bool{swapped.FullRouteID: true}, PrimaryResource: swapped.FullRouteID})
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
