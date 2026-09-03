package main

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status_code"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	mcpToolCallsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mcp_tool_calls_total",
		Help: "Total number of MCP tool calls.",
	}, []string{"tool", "status"})

	oauthFlowsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "oauth_flows_total",
		Help: "Total number of OAuth flows by stage and result.",
	}, []string{"stage", "result"})

	authenticatedUsersTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "authenticated_users_total",
		Help: "Total number of successful user authentications.",
	})

	tokenStoreSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "token_store_size",
		Help: "Current number of entries in the token store.",
	}, []string{"type"})
)

// knownPaths is the route table as registered on the mux (see RegisterRoutes
// and main). Anything else is bucketed, because "/" is a catch-all: without
// this, a caller requesting made-up paths adds a new series per path to
// http_requests_total and http_request_duration_seconds for as long as the
// process lives. The known routes keep their own label so the shared Grafana
// dashboard, which groups by path, is unaffected.
var knownPaths = map[string]bool{
	"/":                true,
	"/mcp":             true,
	"/register":        true,
	"/oauth/authorize": true,
	"/oauth/callback":  true,
	"/oauth/token":     true,
	"/health":          true,
	"/ready":           true,
	"/metrics":         true,
	"/.well-known/oauth-authorization-server":   true,
	"/.well-known/oauth-protected-resource":     true,
	"/.well-known/oauth-protected-resource/mcp": true,
}

func normalizePath(path string) string {
	if knownPaths[path] {
		return path
	}
	return "other"
}

func recordHTTPMetrics(method, path string, statusCode int, duration time.Duration) {
	path = normalizePath(path)
	httpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(statusCode)).Inc()
	httpRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

func recordToolCall(tool, status string) {
	mcpToolCallsTotal.WithLabelValues(tool, status).Inc()
}

func recordOAuthFlow(stage, result string) {
	oauthFlowsTotal.WithLabelValues(stage, result).Inc()
}

func recordAuthentication() {
	authenticatedUsersTotal.Inc()
}

func updateTokenStoreGauges(store *TokenStore) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	tokenStoreSize.WithLabelValues("active_tokens").Set(float64(len(store.tokens)))
	tokenStoreSize.WithLabelValues("refresh_tokens").Set(float64(len(store.refreshTokens)))
}
