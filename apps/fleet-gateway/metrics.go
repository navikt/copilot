package main

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
)

// Metrics carry no user identifiers: labels are limited to values from the
// route config, the fixed outcome/decision sets in proxy.go and HTTP status codes.
var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_gateway_http_requests_total",
		Help: "HTTP requests answered by fleet-gateway, by status code. Excludes /health and /ready. reason is empty except for deliberate 503s (disabled: kill switch, cap: in-flight/SSE cap, user_cap: per-user in-flight cap, user_sse_cap: per-user SSE cap, store_full: store quota, refresh_in_flight: concurrent refresh).",
	}, []string{"code", "reason"})

	mcpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_gateway_mcp_requests_total",
		Help: "MCP proxy requests by route, outcome and tool-policy decision. reason is set on deny only (not_listed, ambiguous, header_mismatch, user_cap, user_sse_cap). tool is the called tool when it is on the route's explicit list, else other; empty when the request is not a tool call.",
	}, []string{"route", "outcome", "decision", "reason", "tool"})

	mcpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "fleet_gateway_mcp_request_duration_seconds",
		Help:    "Time from request to finished response on MCP routes, including token exchange and upstream. SSE streams count until the stream ends.",
		Buckets: []float64{0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
	}, []string{"route"})

	lockoutAuditOutboxPending = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fleet_gateway_lockout_audit_outbox_pending",
		Help: "Lockout audit entries left in the outbox after the last drain. Above zero for long means the drain keeps failing.",
	})
	routesLoaded = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fleet_gateway_routes_loaded",
		Help: "Number of MCP routes loaded from the route file at startup.",
	})

	rateLimitedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_gateway_rate_limited_total",
		Help: "Requests rejected by the per-IP or global rate limiter, by OAuth endpoint.",
	}, []string{"oauth_endpoint"})
)

const (
	rateLimitEndpointAuthorize = "authorize"
	rateLimitEndpointConsent   = "consent"
	rateLimitEndpointToken     = "token"
	rateLimitEndpointRevoke    = "revoke"
	rateLimitEndpointRegister  = "register"
)

var rateLimitedEndpoints = []string{
	rateLimitEndpointAuthorize,
	rateLimitEndpointConsent,
	rateLimitEndpointToken,
	rateLimitEndpointRevoke,
	rateLimitEndpointRegister,
}

func observeRateLimited(endpoint string) {
	rateLimitedTotal.WithLabelValues(endpoint).Inc()
}

// denyReasons is the closed set of reason values on decision="deny".
var denyReasons = []string{policy.ReasonNotListed, policy.ReasonAmbiguous, policy.ReasonHeaderMismatch, "user_cap", "user_sse_cap"}

// observeMCPRequest bounds both extra labels: reason only on deny and only from
// denyReasons; tool only when on the route's explicit list, else "other"
// (passAllTools routes always give "other").
func observeMCPRequest(route PilotRoute, outcome, decision, reason, tool, rawTool string, d time.Duration) {
	if decision != "deny" || !slices.Contains(denyReasons, reason) {
		reason = ""
	}
	// tool is the truncated audit value, only used to tell "not a tool call" from "other".
	switch {
	case tool == "":
	case route.allowedTools[rawTool]:
		tool = rawTool
	default:
		tool = "other"
	}
	mcpRequestsTotal.WithLabelValues(route.Route, outcome, decision, reason, tool).Inc()
	mcpRequestDuration.WithLabelValues(route.Route).Observe(d.Seconds())
}

func observeHTTPRequest(code int, reason string) {
	httpRequestsTotal.WithLabelValues(strconv.Itoa(code), reason).Inc()
}

// markShed labels a deliberate 503 so the 5xx alert can tell it apart from
// real failures: "disabled" (kill switch), "cap" (in-flight/SSE limits),
// "user_cap" (per-user in-flight limit), "user_sse_cap" (per-user SSE limit),
// "store_full" (store quota reached), "refresh_in_flight" (concurrent refresh).
// Store errors and upstream failures stay unlabelled and count.
func markShed(w http.ResponseWriter, reason string) {
	for {
		if sw, ok := w.(*statusResponseWriter); ok {
			sw.reason = reason
			return
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return
		}
		w = u.Unwrap()
	}
}

// initMetrics creates the series the alerts read, at zero, before traffic.
// CounterVec creates a series on first Inc, so increase() would miss a first
// burst that lands between two scrapes.
func initMetrics(routes []PilotRoute) {
	for _, code := range []string{"200", "500", "502", "503", "504"} {
		httpRequestsTotal.WithLabelValues(code, "")
	}
	for _, reason := range []string{"disabled", "cap", "user_cap", "user_sse_cap", "store_full", "refresh_in_flight"} {
		httpRequestsTotal.WithLabelValues("503", reason)
	}
	for _, ep := range rateLimitedEndpoints {
		rateLimitedTotal.WithLabelValues(ep)
	}
	for _, r := range routes {
		tools := append([]string{"", "other"}, slices.Sorted(maps.Keys(r.allowedTools))...)
		for _, o := range []string{"ok", "upstream_token_error", "upstream_error"} {
			for _, tool := range tools {
				mcpRequestsTotal.WithLabelValues(r.Route, o, "allow", "", tool)
			}
		}
		for _, reason := range denyReasons {
			o := "tool_denied"
			switch reason {
			case "user_cap", "user_sse_cap":
				o = reason
			}
			mcpRequestsTotal.WithLabelValues(r.Route, o, "deny", reason, "")
		}
		mcpRequestsTotal.WithLabelValues(r.Route, "disabled", "", "", "")
	}
}

// metricsServer serves /metrics on its own port, which has no ingress.
func metricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}
