package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/navikt/copilot/fleet-gateway/internal/metrics"
	"github.com/navikt/copilot/fleet-gateway/internal/policy"
)

// TestRiskEnforcement_EndToEnd sends tools/call through the gateway handler to
// a stub upstream as an agent capped at read, crossing the real policy.CheckRisk (#3004).
func TestRiskEnforcement_EndToEnd(t *testing.T) {
	toolRisk := map[string]string{"get_doc": "read", "draft_pr": "propose", "delete_doc": "write"}
	cases := []struct {
		name, mode, tool, risk, decision string
		forwarded                        bool
	}{
		{"enforce allows read", "enforce", "get_doc", "read", "allow", true},
		{"enforce denies propose", "enforce", "draft_pr", "propose", "would_deny", false},
		{"enforce denies write", "enforce", "delete_doc", "write", "would_deny", false},
		{"enforce treats unknown as write", "enforce", "mystery", "write", "would_deny", false},
		{"report forwards write", "report", "delete_doc", "write", "would_deny", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tc.tool + `"}}`
			var route string
			setup := func(gw *Gateway) {
				gw.cfg.RiskEnforcement = tc.mode
				gw.cfg.Routes[0].ToolRisk = toolRisk
				gw.riskCaller = func(*http.Request) policy.Caller { return policy.Caller{Agent: true, MaxRisk: policy.RiskRead} }
				route = gw.cfg.Routes[0].Route
			}
			counter := func() float64 {
				return testutil.ToFloat64(metrics.RiskDecisions.WithLabelValues(route, "other", tc.risk, "agent", tc.decision))
			}
			var before float64
			audit, got, code, resp := auditRisk(t, body, setup, func() { before = counter() })

			if tc.forwarded {
				if code != http.StatusOK || string(got) != body {
					t.Fatalf("code=%d forwarded=%q, want 200 and the body unchanged", code, got)
				}
			} else {
				if code != http.StatusForbidden || got != nil {
					t.Fatalf("code=%d forwarded=%q, want 403 and nothing upstream", code, got)
				}
				if !strings.Contains(resp, "-32602") || audit["decision_reason"] != policy.ReasonRisk {
					t.Fatalf("resp=%s reason=%v, want -32602 and %s", resp, audit["decision_reason"], policy.ReasonRisk)
				}
			}
			if d := counter() - before; d != 1 {
				t.Fatalf("risk_decisions{risk=%s,caller_kind=agent,decision=%s} grew by %v, want 1", tc.risk, tc.decision, d)
			}
		})
	}
}
