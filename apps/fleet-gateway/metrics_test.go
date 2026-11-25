package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-gateway/internal/metrics"
	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

func TestMetricsRegisteredAndServed(t *testing.T) {
	metrics.ObserveMCPRequest(routing.Route{Route: "/mcp/ns/test"}, "upstream_token_error", "", "", "", "", time.Second)
	metrics.ObserveHTTPRequest(http.StatusBadGateway, "")
	metrics.RoutesLoaded.Set(3)

	if got := testutil.ToFloat64(metrics.MCPRequests.WithLabelValues("/mcp/ns/test", "upstream_token_error", "", "", "")); got < 1 {
		t.Fatalf("mcp counter = %v, want >= 1", got)
	}

	srv := httptest.NewServer(metrics.Server("").Handler)
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
	before := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("404", ""))
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code == http.StatusOK {
		t.Fatalf("/metrics answered 200 on the public handler")
	}
	if got := testutil.ToFloat64(metrics.HTTPRequests.WithLabelValues("404", "")); got != before+1 {
		t.Fatalf("http counter for 404 = %v, want %v: loggingMiddleware does not count", got, before+1)
	}
}

// The token-error alert uses increase(), which needs a zero sample before the
// first failure.
func TestInitMetricsPrecreatesAlertSeries(t *testing.T) {
	metrics.Init([]routing.Route{{Route: "/mcp/ns/precreated"}})
	srv := httptest.NewServer(metrics.Server("").Handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	for _, want := range []string{
		`fleet_gateway_mcp_requests_total{decision="allow",outcome="upstream_token_error",reason="",route="/mcp/ns/precreated",tool=""} 0`,
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
	shed := metrics.HTTPRequests.WithLabelValues("503", "disabled")
	unlabelled := metrics.HTTPRequests.WithLabelValues("503", "")
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

// Denials count as decision="deny" with a bounded reason; tool is on the label
// only when on the route list, so an attacker-chosen name cannot mint series.
func TestMCPRequestsTotalDecisionReasonTool(t *testing.T) {
	const route = "/mcp/team-etterlevelse/etterlevelse"
	call := func(name string) string {
		return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + name + `"}}`
	}
	mismatch := http.Header{"Mcp-Method": {"tools/call"}, "Mcp-Name": {"delete_all"}}
	cases := []struct {
		name, body, outcome, decision, reason, tool string
		tools                                       map[string]bool
		hdr                                         http.Header
	}{
		{"listed", call("whoami"), "ok", "allow", "", "whoami", policyTools, nil},
		{"not listed", call("evil-1"), "tool_denied", "deny", "not_listed", "other", policyTools, nil},
		{"ambiguous", call("whoami") + call("evil-2"), "tool_denied", "deny", "ambiguous", "other", policyTools, nil},
		{"header mismatch", call("whoami"), "tool_denied", "deny", "header_mismatch", "whoami", policyTools, mismatch},
		{"pass-all", call("evil-3"), "ok", "allow", "", "other", nil, nil},
		{"not a call", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "ok", "allow", "", "", policyTools, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := metrics.MCPRequests.WithLabelValues(route, tc.outcome, tc.decision, tc.reason, tc.tool)
			before := testutil.ToFloat64(c)
			auditToolsHeaders(t, http.MethodPost, tc.tools, tc.body, tc.hdr)
			if got := testutil.ToFloat64(c); got != before+1 {
				t.Fatalf("series {%s,%s,%s,%s} = %v, want %v", tc.outcome, tc.decision, tc.reason, tc.tool, got, before+1)
			}
		})
	}
	// Raw tool names never reach a label.
	body := scrapeMetrics(t)
	if strings.Contains(body, "evil-") {
		t.Errorf("unlisted tool name leaked into /metrics")
	}
}

func TestInitMetricsPrecreatesDenySeries(t *testing.T) {
	metrics.Init([]routing.Route{{Route: "/mcp/ns/deny"}})
	body := scrapeMetrics(t)
	for _, reason := range metrics.DenyReasons {
		if !strings.Contains(body, `decision="deny",outcome=`) || !strings.Contains(body, `reason="`+reason+`"`) {
			t.Errorf("no pre-created deny series for reason %q", reason)
		}
	}
	for _, ep := range metrics.RateLimitedEndpoints {
		if !strings.Contains(body, `fleet_gateway_rate_limited_total{oauth_endpoint="`+ep+`"} `) {
			t.Errorf("no pre-created rate limit series for %q", ep)
		}
	}
	if strings.Contains(body, `fleet_gateway_rate_limited_total{endpoint=`) {
		t.Errorf("old endpoint label still exported")
	}
}

func TestObserveMCPRequestBoundsReason(t *testing.T) {
	r := routing.Route{Route: "/mcp/ns/bound", AllowedTools: map[string]bool{"a": true}}
	metrics.ObserveMCPRequest(r, "tool_denied", "deny", "made-up", "zzz", "zzz", time.Millisecond)
	if got := testutil.ToFloat64(metrics.MCPRequests.WithLabelValues(r.Route, "tool_denied", "deny", "", "other")); got != 1 {
		t.Fatalf("unknown reason/tool not collapsed: %v", got)
	}
}

func scrapeMetrics(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(metrics.Server("").Handler)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// A listed tool whose name is longer than the audit display cap keeps its own label.
func TestMCPToolLabelUsesFullName(t *testing.T) {
	long := strings.Repeat("a", 100)
	tools := map[string]bool{long: true}
	c := metrics.MCPRequests.WithLabelValues("/mcp/team-etterlevelse/etterlevelse", "ok", "allow", "", long)
	before := testutil.ToFloat64(c)
	auditPostTools(t, tools, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+long+`"}}`)
	if got := testutil.ToFloat64(c); got != before+1 {
		t.Fatalf("long listed tool series = %v, want %v", got, before+1)
	}
}

// increase() needs a zero sample per tool before the first call.
func TestInitMetricsPrecreatesToolSeries(t *testing.T) {
	metrics.Init([]routing.Route{{Route: "/mcp/ns/tools", AllowedTools: map[string]bool{"search_x": true}}})
	body := scrapeMetrics(t)
	for _, tool := range []string{"search_x", "other"} {
		want := `outcome="upstream_token_error",reason="",route="/mcp/ns/tools",tool="` + tool + `"} 0`
		if !strings.Contains(body, want) {
			t.Errorf("/metrics lacks %s", want)
		}
	}
}
