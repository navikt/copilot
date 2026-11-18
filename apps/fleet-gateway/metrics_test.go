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
	observeHTTPRequest(http.StatusBadGateway)
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
	before := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("404"))
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code == http.StatusOK {
		t.Fatalf("/metrics answered 200 on the public handler")
	}
	if got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("404")); got != before+1 {
		t.Fatalf("http counter for 404 = %v, want %v: loggingMiddleware does not count", got, before+1)
	}
}
