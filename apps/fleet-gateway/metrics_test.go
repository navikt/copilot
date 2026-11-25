package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"

	"github.com/navikt/copilot/fleet-gateway/internal/metrics"
	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

// /metrics must not be reachable through the public handler (it has an ingress),
// and the public handler's requests are counted.
func TestMetricsNotOnPublicPort(t *testing.T) {
	te := newTestEnv(t)
	before := testutil.ToFloat64(httpSeries("404", ""))
	rec := httptest.NewRecorder()
	te.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code == http.StatusOK {
		t.Fatalf("/metrics answered 200 on the public handler")
	}
	if got := testutil.ToFloat64(httpSeries("404", "")); got != before+1 {
		t.Fatalf("http counter for 404 = %v, want %v: loggingMiddleware does not count", got, before+1)
	}
}

// Kill-switch 503s carry reason="disabled" so FleetGatewayHigh5xxRate skips them.
func TestKillSwitch503IsLabelled(t *testing.T) {
	te := newTestEnv(t)
	te.gw.cfg.GatewayDisabled = true
	te.handler = te.gw.routes()
	shed := httpSeries("503", "disabled")
	unlabelled := httpSeries("503", "")
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
			c := mcpSeries(route, tc.outcome, tc.decision, tc.reason, tc.tool)
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
	c := mcpSeries("/mcp/team-etterlevelse/etterlevelse", "ok", "allow", "", long)
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

// series is one counter series read from the default registry, the same data
// /metrics serves, so these tests need no access to the collector variables.
// It is a Collector, so testutil.ToFloat64 reads it; a series not yet created reads 0.
type series struct {
	name   string
	labels map[string]string
}

func labelled(name string, kv ...string) series {
	s := series{name: name, labels: map[string]string{}}
	for i := 0; i+1 < len(kv); i += 2 {
		s.labels[kv[i]] = kv[i+1]
	}
	return s
}

func httpSeries(code, reason string) series {
	return labelled("fleet_gateway_http_requests_total", "code", code, "reason", reason)
}

func mcpSeries(route, outcome, decision, reason, tool string) series {
	return labelled("fleet_gateway_mcp_requests_total", "route", route, "outcome", outcome, "decision", decision, "reason", reason, "tool", tool)
}

func rateLimitedSeries(endpoint string) series {
	return labelled("fleet_gateway_rate_limited_total", "oauth_endpoint", endpoint)
}

func riskSeries(route, tool, risk, callerKind, decision string) series {
	return labelled("fleet_gateway_risk_decisions_total", "route", route, "tool", tool, "risk", risk, "caller_kind", callerKind, "decision", decision)
}

func (s series) desc() *prometheus.Desc { return prometheus.NewDesc(s.name, "test view", nil, nil) }

func (s series) Describe(ch chan<- *prometheus.Desc) { ch <- s.desc() }

func (s series) Collect(ch chan<- prometheus.Metric) {
	v := 0.0
	for _, m := range gatherFamily(s.name) {
		if len(m.GetLabel()) != len(s.labels) {
			continue
		}
		match := true
		for _, l := range m.GetLabel() {
			if want, ok := s.labels[l.GetName()]; !ok || want != l.GetValue() {
				match = false
			}
		}
		if match {
			v = m.GetCounter().GetValue()
		}
	}
	ch <- prometheus.MustNewConstMetric(s.desc(), prometheus.CounterValue, v)
}

// gatherFamily returns every series of one metric family in the default registry.
func gatherFamily(name string) []*dto.Metric {
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		panic(err)
	}
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf.GetMetric()
		}
	}
	return nil
}
