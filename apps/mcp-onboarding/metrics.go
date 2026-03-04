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

func recordHTTPMetrics(method, path string, statusCode int, duration time.Duration) {
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
	tokenStoreSize.WithLabelValues("client_registrations").Set(float64(len(store.clientRegistrations)))
}
