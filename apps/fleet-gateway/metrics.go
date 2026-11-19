package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics carry no user identifiers: labels are limited to values from the
// route config, the fixed outcome/decision sets in proxy.go and HTTP status codes.
var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_gateway_http_requests_total",
		Help: "HTTP requests answered by fleet-gateway, by status code. Excludes /health and /ready. reason is empty except for deliberate 503s (disabled: kill switch, cap: in-flight/SSE cap, store_full: store quota, refresh_in_flight: concurrent refresh).",
	}, []string{"code", "reason"})

	mcpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_gateway_mcp_requests_total",
		Help: "MCP proxy requests by route, outcome and tool-policy decision.",
	}, []string{"route", "outcome", "decision"})

	mcpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "fleet_gateway_mcp_request_duration_seconds",
		Help:    "Time from request to finished response on MCP routes, including token exchange and upstream. SSE streams count until the stream ends.",
		Buckets: []float64{0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
	}, []string{"route"})

	routesLoaded = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fleet_gateway_routes_loaded",
		Help: "Number of MCP routes loaded from the route file at startup.",
	})

	rateLimitedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "fleet_gateway_rate_limited_total",
		Help: "Requests rejected by the per-IP or global rate limiter, by endpoint.",
	}, []string{"endpoint"})
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

func observeMCPRequest(route, outcome, decision string, d time.Duration) {
	mcpRequestsTotal.WithLabelValues(route, outcome, decision).Inc()
	mcpRequestDuration.WithLabelValues(route).Observe(d.Seconds())
}

func observeHTTPRequest(code int, reason string) {
	httpRequestsTotal.WithLabelValues(strconv.Itoa(code), reason).Inc()
}

// markShed labels a deliberate 503 so the 5xx alert can tell it apart from
// real failures: "disabled" (kill switch), "cap" (in-flight/SSE limits),
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
	for _, reason := range []string{"disabled", "cap", "store_full", "refresh_in_flight"} {
		httpRequestsTotal.WithLabelValues("503", reason)
	}
	for _, ep := range rateLimitedEndpoints {
		rateLimitedTotal.WithLabelValues(ep)
	}
	for _, r := range routes {
		for _, o := range []string{"ok", "upstream_token_error", "upstream_error"} {
			mcpRequestsTotal.WithLabelValues(r.Route, o, "allow")
		}
		mcpRequestsTotal.WithLabelValues(r.Route, "disabled", "")
	}
}

// metricsServer serves /metrics on its own port, which has no ingress.
func metricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}
