package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-gateway/internal/metrics"
	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

func shadowGateway(t *testing.T, f *fakeFleetControl) (*Gateway, *shadowRoutes) {
	t.Helper()
	gw := newTestEnv(t).gw
	gw.cfg.RoutesShadowSource = routesSourceFleetControl
	gw.cfg.Routes[0].Tools = []string{"whoami"} // as primaryRoute in the snapshot
	// Canonicalize like loadConfigFromEnv does for the Git file.
	routes, err := routing.Validate(gw.cfg.Routes)
	if err != nil {
		t.Fatal(err)
	}
	gw.cfg.Routes = routes
	gw.cfg.FleetControlURL = f.srv.URL
	gw.cfg.TexasTokenEndpoint = f.srv.URL + "/texas"
	gw.cfg.FleetControlAudience = "api://dev-gcp.copilot.fleet-control/.default"
	gw, err = newGateway(gw.cfg, gw.now)
	if err != nil {
		t.Fatal(err)
	}
	return gw, &shadowRoutes{f: &fleetControlRoutes{cfg: gw.cfg, client: f.srv.Client()}}
}

func TestShadowSyncNeverServesAndCountsDiff(t *testing.T) {
	f := newFakeFleetControl(t) // primary + alpha + beta; the Git table has only the primary
	gw, s := shadowGateway(t, f)
	before := gw.table()

	gw.shadowSync(context.Background(), s)
	if gw.table() != before {
		t.Fatal("shadow snapshot replaced the served table")
	}
	if got := testutil.ToFloat64(shadowDiffGauge); got != 2 {
		t.Fatalf("shadow diff = %v, want 2 (alpha, beta)", got)
	}
	if want := []string{"/mcp/copilot/alpha", "/mcp/copilot/beta"}; len(s.last) != 2 || s.last[0] != want[0] || s.last[1] != want[1] {
		t.Fatalf("diff paths = %v, want %v", s.last, want)
	}

	f.body, f.etag = snapBody(2, primaryRoute()), `"e2"`
	gw.shadowSync(context.Background(), s)
	if got := testutil.ToFloat64(shadowDiffGauge); got != 0 {
		t.Fatalf("shadow diff = %v, want 0 for an equal snapshot", got)
	}
}

func TestShadowSyncFailureIsIsolated(t *testing.T) {
	f := newFakeFleetControl(t)
	gw, s := shadowGateway(t, f)
	gw.shadowSync(context.Background(), s)
	before, diff := gw.table(), testutil.ToFloat64(shadowDiffGauge)
	failures, syncFailures := testutil.ToFloat64(shadowFailures), testutil.ToFloat64(metrics.RouteSyncFailures)

	f.status, f.etag = http.StatusInternalServerError, `"e3"`
	gw.shadowSync(context.Background(), s)
	if gw.table() != before {
		t.Fatal("shadow failure changed the served table")
	}
	if got := testutil.ToFloat64(shadowFailures); got != failures+1 {
		t.Fatalf("shadow failures = %v, want %v", got, failures+1)
	}
	if got := testutil.ToFloat64(shadowDiffGauge); got != diff {
		t.Fatalf("shadow failure moved the diff gauge to %v", got)
	}
	if got := testutil.ToFloat64(metrics.RouteSyncFailures); got != syncFailures {
		t.Fatal("shadow failure counted as a serving sync failure")
	}
}

func TestShadowSyncPanicIsRecovered(t *testing.T) {
	gw, _ := shadowGateway(t, newFakeFleetControl(t))
	before, failures := gw.table(), testutil.ToFloat64(shadowFailures)

	gw.shadowSync(context.Background(), &shadowRoutes{}) // nil fetcher panics
	if gw.table() != before {
		t.Fatal("shadow panic changed the served table")
	}
	if got := testutil.ToFloat64(shadowFailures); got != failures+1 {
		t.Fatalf("shadow failures = %v, want %v", got, failures+1)
	}
}
