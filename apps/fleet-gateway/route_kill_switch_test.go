package main

import (
	"net/http"
	"net/http/httptest"
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
	te.gw.cfg.DisabledRoutes = parseDisabledRoutes(off.Route)
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
	te.gw.cfg.DisabledRoutes = parseDisabledRoutes("")
	te.handler = te.gw.routes()
	for _, r := range te.gw.cfg.Routes {
		if code := postRoute(te, r.Route); code == http.StatusServiceUnavailable {
			t.Fatalf("%s answered 503 with an empty list", r.Route)
		}
	}
}

// Malformed entries are skipped with a warning; well-formed ones still apply.
func TestParseDisabledRoutes(t *testing.T) {
	got := parseDisabledRoutes(" /mcp/copilot/kodeverk , copilot/dolly, /mcp/x, /mcp//y, /api/a/b, /mcp/a/b/c,, /mcp/team/later ")
	want := map[string]bool{"/mcp/copilot/kodeverk": true, "/mcp/team/later": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
