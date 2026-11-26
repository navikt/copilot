// Package metrics holds the gateway's Prometheus collectors and observe helpers.
package metrics //nolint:revive // the gateway never imports runtime/metrics; name per docs/architecture.md

import (
	"maps"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/navikt/copilot/fleet-gateway/internal/policy"
	"github.com/navikt/copilot/fleet-gateway/internal/routing"
)

// Metrics carry no user identifiers: labels are limited to values from the
// route config, the fixed outcome/decision sets in proxy.go and HTTP status codes.
var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_gateway_http_requests_total",
		Help: "HTTP requests answered by fleet-gateway, by status code. Excludes /health and /ready. reason is empty except for deliberate 503s (disabled: kill switch, cap: in-flight/SSE cap, user_cap: per-user in-flight cap, user_sse_cap: per-user SSE cap, store_full: store quota, refresh_in_flight: concurrent refresh).",
	}, []string{"code", "reason"})

	// IDPErrors counts login failures caused by Entra or Texas, not by the user (#2893).
	IDPErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_gateway_idp_errors_total",
		Help: "Login steps that failed at Entra ID or Texas: metadata (well-known lookup), code_redeem (network error or 5xx), introspection (Texas).",
	}, []string{"step"})

	mcpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_gateway_mcp_requests_total",
		Help: "MCP proxy requests by route, outcome and tool-policy decision. reason is set on deny only (not_listed, ambiguous, header_mismatch, user_cap, user_sse_cap). tool is the called tool when it is on the route's explicit list, else other; empty when the request is not a tool call.",
	}, []string{"route", "outcome", "decision", "reason", "tool"})

	// riskDecisions counts tool-risk checks at tools/call (#2964), refused or not.
	riskDecisions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_gateway_risk_decisions_total",
		Help: "Tool-risk checks at tools/call. risk is read, propose or write (unknown counts as write); caller_kind is human or agent; decision is allow or would_deny. would_deny is refused only with RISK_ENFORCEMENT=enforce. tool is the called tool when on the route's list, else other.",
	}, []string{"route", "tool", "risk", "caller_kind", "decision"})

	mcpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "fleet_gateway_mcp_request_duration_seconds",
		Help:    "Time from request to finished response on MCP routes, including token exchange and upstream. SSE streams count until the stream ends.",
		Buckets: []float64{0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
	}, []string{"route"})

	LockoutAuditOutboxPending = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fleet_gateway_lockout_audit_outbox_pending",
		Help: "Lockout audit entries left in the outbox after the last drain. Above zero for long means the drain keeps failing.",
	})
	RoutesLoaded = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fleet_gateway_routes_loaded",
		Help: "Number of MCP routes in the current route table.",
	})

	// Alerts select on this so they stay correct if deployed in prod or evaluated tenant-wide; today rules run per cluster (#2949).
	routesSource = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "fleet_gateway_routes_source_info",
		Help: "Always 1; the source label says where this pod reads routes from (git or fleet-control).",
	}, []string{"source"})

	RouteSyncFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "fleet_gateway_route_sync_failures_total",
		Help: "fleet-control route snapshots fetched or validated without success; the current table stays.",
	})

	RoutesFromStore = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fleet_gateway_routes_from_stored_snapshot",
		Help: "1 while the route table comes from the snapshot stored in the gateway's own database because fleet-control was unreachable at startup, 0 otherwise.",
	})

	StoreUp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fleet_gateway_store_up",
		Help: "1 if the last session-store ping (on each /ready probe) succeeded, 0 if it failed. /ready does not fail on it (#2893).",
	})

	rateLimited = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_gateway_rate_limited_total",
		Help: "Requests rejected by the per-IP or global rate limiter, by OAuth endpoint.",
	}, []string{"oauth_endpoint"})
)

// OAuth endpoint names on the oauth_endpoint label of rateLimited.
const (
	EndpointAuthorize = "authorize"
	EndpointConsent   = "consent"
	EndpointToken     = "token"
	EndpointRevoke    = "revoke"
	EndpointRegister  = "register"
	EndpointProbe     = "probe"
)

// RateLimitedEndpoints lists every endpoint behind a rate limiter.
var RateLimitedEndpoints = []string{
	EndpointAuthorize,
	EndpointConsent,
	EndpointToken,
	EndpointRevoke,
	EndpointRegister,
	EndpointProbe,
}

// ObserveRateLimited counts one rejected request on endpoint.
func ObserveRateLimited(endpoint string) {
	rateLimited.WithLabelValues(endpoint).Inc()
}

// denyReasons is the closed set of reason values on decision="deny".
var denyReasons = []string{policy.ReasonNotListed, policy.ReasonAmbiguous, policy.ReasonHeaderMismatch, policy.ReasonRisk, "user_cap", "user_sse_cap"}

// ObserveMCPRequest bounds both extra labels: reason only on deny and only from
// denyReasons; tool only when on the route's explicit list, else "other"
// (passAllTools routes always give "other").
func ObserveMCPRequest(route routing.Route, outcome, decision, reason, tool, rawTool string, d time.Duration) {
	if decision != "deny" || !slices.Contains(denyReasons, reason) {
		reason = ""
	}
	// tool is the truncated audit value, only used to tell "not a tool call" from "other".
	switch {
	case tool == "":
	case route.AllowedTools[rawTool]:
		tool = rawTool
	default:
		tool = "other"
	}
	mcpRequests.WithLabelValues(route.Route, outcome, decision, reason, tool).Inc()
	mcpRequestDuration.WithLabelValues(route.Route).Observe(d.Seconds())
}

// ObserveRiskDecision counts one tool-risk check; tool is bounded like ObserveMCPRequest.
func ObserveRiskDecision(route routing.Route, rawTool string, risk policy.Risk, callerKind, decision string) {
	tool := "other"
	if route.AllowedTools[rawTool] {
		tool = rawTool
	}
	riskDecisions.WithLabelValues(route.Route, tool, risk.String(), callerKind, decision).Inc()
}

// ObserveHTTPRequest counts one answered request by status code and shed reason.
func ObserveHTTPRequest(code int, reason string) {
	httpRequests.WithLabelValues(strconv.Itoa(code), reason).Inc()
}

// SetRoutesSource leaves exactly one source series at 1.
func SetRoutesSource(source string) {
	routesSource.Reset()
	routesSource.WithLabelValues(source).Set(1)
}

// Init creates the series the alerts read, at zero, before traffic.
// CounterVec creates a series on first Inc, so increase() would miss a first
// burst that lands between two scrapes.
func Init(routes []routing.Route) {
	for _, code := range []string{"200", "500", "502", "503", "504"} {
		httpRequests.WithLabelValues(code, "")
	}
	for _, reason := range []string{"disabled", "route_disabled", "cap", "user_cap", "user_sse_cap", "store_full", "refresh_in_flight"} {
		httpRequests.WithLabelValues("503", reason)
	}
	for _, step := range []string{"metadata", "code_redeem", "introspection"} {
		IDPErrors.WithLabelValues(step)
	}
	for _, ep := range RateLimitedEndpoints {
		rateLimited.WithLabelValues(ep)
	}
	for _, r := range routes {
		tools := append([]string{"", "other"}, slices.Sorted(maps.Keys(r.AllowedTools))...)
		for _, o := range []string{"ok", "upstream_token_error", "upstream_error"} {
			for _, tool := range tools {
				mcpRequests.WithLabelValues(r.Route, o, "allow", "", tool)
			}
		}
		for _, reason := range denyReasons {
			o := "tool_denied"
			switch reason {
			case "user_cap", "user_sse_cap":
				o = reason
			}
			mcpRequests.WithLabelValues(r.Route, o, "deny", reason, "")
		}
		mcpRequests.WithLabelValues(r.Route, "disabled", "", "", "")
		mcpRequests.WithLabelValues(r.Route, "route_disabled", "", "", "")
	}
}

// Server serves /metrics on its own port, which has no ingress.
func Server(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}
