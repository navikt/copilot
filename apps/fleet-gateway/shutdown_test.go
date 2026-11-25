package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReady_DrainingReturns503(t *testing.T) {
	te := newTestEnv(t)
	te.gw.draining.Store(true)
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "shutting_down") {
		t.Fatalf("/ready while draining = %d %s, want 503 shutting_down", rec.Code, rec.Body.String())
	}
}

func TestShutdownDrainCoversDefaultCalls(t *testing.T) {
	wt := mcpWriteTimeout(serverReadTimeout, defaultOutboundTimeout, defaultUpstreamTimeout, 5*time.Second)
	if got := shutdownDrain(wt); got != wt {
		t.Fatalf("drain %v, want full WriteTimeout %v", got, wt)
	}
	if got := shutdownDrain(10 * time.Minute); got != maxShutdownDrain {
		t.Fatalf("drain %v, want cap %v", got, maxShutdownDrain)
	}
}

func TestGracePeriodCoversShutdownDrain(t *testing.T) {
	b, err := os.ReadFile(".nais/app.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "terminationGracePeriodSeconds: 180") || maxShutdownDrain > 175*time.Second {
		t.Fatal("terminationGracePeriodSeconds must be 180 and exceed maxShutdownDrain plus the preStop sleep")
	}
}

// The never-synced guard only makes sense where routes come from fleet-control;
// with the Git file the age gauge never appears. Template tags stay at column 0 (#2910).
func TestNeverSyncedAlertOnlyWithFleetControlRoutes(t *testing.T) {
	b, err := os.ReadFile(".nais/app.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, "- alert: FleetGatewayRouteNeverSynced")
	open := strings.LastIndex(s[:i], "\n{{#if routes_source}}\n")
	if i < 0 || open < 0 || strings.Contains(s[open:i], "{{/if}}") {
		t.Fatal("FleetGatewayRouteNeverSynced must sit inside a column-0 {{#if routes_source}} block")
	}
	if !strings.Contains(s, "fleet_gateway_route_sync_failures_total{namespace=\"copilot\"}[15m])) > 0") {
		t.Fatal("FleetGatewayRouteSyncFailing alert missing")
	}
}
