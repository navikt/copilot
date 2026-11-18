package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsRegisteredAndServed(t *testing.T) {
	observeMCPRequest("/mcp/ns/test", "upstream_token_error", "", time.Second)
	observeHTTPRequest(http.StatusBadGateway, "")
	routesLoaded.Set(3)

	if got := testutil.ToFloat64(mcpRequestsTotal.WithLabelValues("/mcp/ns/test", "upstream_token_error", "")); got < 1 {
		t.Fatalf("mcp counter = %v, want >= 1", got)
	}

	srv := httptest.NewServer(metricsServer("").Handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	for _, name := range []string{
		"fleet_gateway_http_requests_total",
		"fleet_gateway_mcp_requests_total",
		"fleet_gateway_mcp_request_duration_seconds_bucket",
		"fleet_gateway_routes_loaded 3",
	} {
		if !strings.Contains(string(body), name) {
			t.Errorf("/metrics lacks %q", name)
		}
	}
}

// /metrics must not be reachable through the public handler (it has an ingress),
// and the public handler's requests are counted.
func TestMetricsNotOnPublicPort(t *testing.T) {
	te := newTestEnv(t)
	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("404", ""))
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code == http.StatusOK {
		t.Fatalf("/metrics answered 200 on the public handler")
	}
	if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("404", "")); got != before+1 {
		t.Fatalf("http counter for 404 = %v, want %v: loggingMiddleware does not count", got, before+1)
	}
}

// The token-error alert uses increase(), which needs a zero sample before the
// first failure.
func TestInitMetricsPrecreatesAlertSeries(t *testing.T) {
	initMetrics([]PilotRoute{{Route: "/mcp/ns/precreated"}})
	srv := httptest.NewServer(metricsServer("").Handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{
		`fleet_gateway_mcp_requests_total{decision="allow",outcome="upstream_token_error",route="/mcp/ns/precreated"} 0`,
		`fleet_gateway_http_requests_total{code="502",reason=""}`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("/metrics lacks %s", want)
		}
	}
}

// Kill-switch 503s carry reason="disabled" so FleetGatewayHigh5xxRate skips them.
func TestKillSwitch503IsLabelled(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.GatewayDisabled = true
	te.handler = te.gw.routes()
	shed := httpRequestsTotal.WithLabelValues("503", "disabled")
	unlabelled := httpRequestsTotal.WithLabelValues("503", "")
	beforeShed, beforeReal := testutil.ToFloat64(shed), testutil.ToFloat64(unlabelled)
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, te.gw.cfg.Routes[0].Route, strings.NewReader(`{}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", rec.Code)
	}
	if testutil.ToFloat64(shed) != beforeShed+1 || testutil.ToFloat64(unlabelled) != beforeReal {
		t.Fatalf("kill-switch 503 not labelled reason=disabled")
	}
}
