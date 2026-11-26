package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The stuck-pending gauge is alerted on (.nais/app.yaml); /metrics must export it.
func TestMetricsExportsBudgetStuckGauge(t *testing.T) {
	rec := httptest.NewRecorder()
	metricsHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(rec.Body.String(), "copilot_api_budget_raises_stuck_pending ") {
		t.Errorf("/metrics missing copilot_api_budget_raises_stuck_pending: %s", rec.Body.String())
	}
}
