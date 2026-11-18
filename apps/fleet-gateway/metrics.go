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
		Help: "HTTP requests answered by fleet-gateway, by status code. Excludes /health and /ready.",
	}, []string{"code"})

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
)

func observeMCPRequest(route, outcome, decision string, d time.Duration) {
	mcpRequestsTotal.WithLabelValues(route, outcome, decision).Inc()
	mcpRequestDuration.WithLabelValues(route).Observe(d.Seconds())
}

func observeHTTPRequest(code int) {
	httpRequestsTotal.WithLabelValues(strconv.Itoa(code)).Inc()
}

// metricsServer serves /metrics on its own port, which has no ingress.
func metricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}
