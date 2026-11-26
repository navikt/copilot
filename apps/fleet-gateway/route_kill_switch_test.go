package main

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

func postRoute(te *testEnv, path string) int {
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
	return rec.Code
}

// GATEWAY_DISABLED_ROUTES stops one route with a labelled 503, leaves the
// others alone and still holds after a new route snapshot replaces the table.
func TestDisabledRoutesKillSwitch(t *testing.T) {
	te := newTestEnv(t)
	off := te.gw.cfg.Routes[0]
	on := off
	on.Route = "/mcp/other-team/other"
	te.gw.routeTable.Store(routing.NewTable([]routing.Route{off, on}))
	te.gw.cfg.DisabledRoutes, _ = parseDisabledRoutes(off.Route)
	te.handler = te.gw.routes()

	shed := httpSeries("503", "route_disabled")
	outcome := mcpSeries(off.Route, "route_disabled", "", "", "")
	beforeShed, beforeOutcome := testutil.ToFloat64(shed), testutil.ToFloat64(outcome)

	if code := postRoute(te, off.Route); code != http.StatusServiceUnavailable {
		t.Fatalf("disabled route: code = %d, want 503", code)
	}
	if testutil.ToFloat64(shed) != beforeShed+1 || testutil.ToFloat64(outcome) != beforeOutcome+1 {
		t.Fatalf("disabled route 503 not counted with reason/outcome route_disabled")
	}
	if code := postRoute(te, on.Route); code == http.StatusServiceUnavailable {
		t.Fatalf("other route answered 503")
	}

	// A new snapshot: fresh table built from scratch, same paths.
	te.gw.routeTable.Store(routing.NewTable([]routing.Route{off, on}))
	if code := postRoute(te, off.Route); code != http.StatusServiceUnavailable {
		t.Fatalf("after snapshot: code = %d, want 503", code)
	}
}

func TestDisabledRoutesEmptyHasNoEffect(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.DisabledRoutes, _ = parseDisabledRoutes("")
	te.handler = te.gw.routes()
	for _, r := range te.gw.cfg.Routes {
		if code := postRoute(te, r.Route); code == http.StatusServiceUnavailable {
			t.Fatalf("%s answered 503 with an empty list", r.Route)
		}
	}
}

// Malformed entries are skipped and returned for a warning; well-formed ones still apply.
// Each typo class must be reported, or the route silently stays open.
func TestParseDisabledRoutes(t *testing.T) {
	got, malformed := parseDisabledRoutes(" /mcp/copilot/kodeverk , copilot/dolly, /mcp/x, /mcp//y, /api/a/b, /mcp/a/b/c,, /mcp/team/later ," +
		"/mcp/team/a b,/mcp/team/semi;,/mcp/team/q?x=1,/mcp/./b,/mcp/team/..,/mcp/te@m/b")
	want := map[string]bool{"/mcp/copilot/kodeverk": true, "/mcp/team/later": true}
	if !maps.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	wantMalformed := []string{"/api/a/b", "/mcp/./b", "/mcp//y", "/mcp/a/b/c", "/mcp/te@m/b", "/mcp/team/..", "/mcp/team/a b", "/mcp/team/q?x=1", "/mcp/team/semi;", "/mcp/x", "copilot/dolly"}
	if !slices.Equal(malformed, wantMalformed) {
		t.Fatalf("malformed = %q, want %q", malformed, wantMalformed)
	}
}

// A deliberately disabled route must not fire error-rate alerts: every alert
// filter that excludes outcomes must exclude route_disabled. Prometheus anchors
// label regexes, so "disabled" alone does not match.
func TestAlertsExcludeRouteDisabled(t *testing.T) {
	raw, err := os.ReadFile(".nais/app.yaml")
	if err != nil {
		t.Fatal(err)
	}
	filters := regexp.MustCompile(`outcome!~"([^"]+)"`).FindAllStringSubmatch(string(raw), -1)
	if len(filters) == 0 {
		t.Fatal("no outcome!~ filters found in .nais/app.yaml")
	}
	for _, f := range filters {
		if !regexp.MustCompile("^(?:" + f[1] + ")$").MatchString("route_disabled") {
			t.Errorf("alert filter outcome!~%q does not exclude route_disabled", f[1])
		}
	}
}

// Per-user caps answer 429 and must stay out of the availability SLO; a
// separate warning alert watches them (owner decision 2026-11-26).
func TestAlertsExcludeUserCaps(t *testing.T) {
	raw, err := os.ReadFile(".nais/app.yaml")
	if err != nil {
		t.Fatal(err)
	}
	bad := regexp.MustCompile(`outcome=~"([^"]*over_capacity[^"]*)"`).FindAllStringSubmatch(string(raw), -1)
	if len(bad) == 0 {
		t.Fatal("no SLO error filters found in .nais/app.yaml")
	}
	for _, f := range bad {
		re := regexp.MustCompile("^(?:" + f[1] + ")$")
		for _, o := range []string{"user_cap", "user_sse_cap"} {
			if re.MatchString(o) {
				t.Errorf("SLO error filter outcome=~%q counts %s", f[1], o)
			}
		}
		if !re.MatchString("over_capacity") || !re.MatchString("sse_over_capacity") {
			t.Errorf("SLO error filter outcome=~%q dropped a global cap", f[1])
		}
	}
	if !regexp.MustCompile(`(?s)- alert: FleetGatewayUserCapsHit\n\s+expr: [^\n]*outcome=~"user_cap\|user_sse_cap"[^\n]*\n(?:\s+[^\n]*\n)*?\s+severity: warning`).Match(raw) {
		t.Error("FleetGatewayUserCapsHit warning alert on user_cap|user_sse_cap missing")
	}
}
