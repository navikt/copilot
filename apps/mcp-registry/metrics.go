package main

import (
	"strconv"
	"strings"
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

	registryServerLookupsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "registry_server_lookups_total",
		Help: "Total number of server lookups by server name and result.",
	}, []string{"server", "result"})

	registryDBEntriesDroppedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "registry_db_entries_dropped_total",
		Help: "Database entries dropped at reload, by reason (collision, invalid, parse).",
	}, []string{"reason"})

	registryWritesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "registry_writes_total",
		Help: "Total number of registry write operations.",
	}, []string{"action", "status"})
)

var knownPaths = map[string]bool{
	"/":                         true,
	"/v0.1/servers":             true,
	"/v0.1/servers/":            true,
	"/health":                   true,
	"/ready":                    true,
	"/metrics":                  true,
	"/robots.txt":               true,
	"/favicon.ico":              true,
	"/.well-known/security.txt": true,
	"/api/v1/entries":           true,
}

func normalizePath(path string) string {
	if strings.HasPrefix(path, "/api/v1/entries/") {
		if strings.HasSuffix(path, "/approve") {
			return "/api/v1/entries/{name}/approve"
		}
		if strings.HasSuffix(path, "/reject") {
			return "/api/v1/entries/{name}/reject"
		}
		return "other"
	}
	if strings.HasPrefix(path, "/v0.1/servers/") && path != "/v0.1/servers/" {
		return "/v0.1/servers/{name}"
	}
	if knownPaths[path] {
		return path
	}
	return "other"
}

func recordHTTPMetrics(method, path string, statusCode int, duration time.Duration) {
	normalized := normalizePath(path)
	httpRequestsTotal.WithLabelValues(method, normalized, strconv.Itoa(statusCode)).Inc()
	httpRequestDuration.WithLabelValues(method, normalized).Observe(duration.Seconds())
}

func recordServerLookup(server, result string) {
	registryServerLookupsTotal.WithLabelValues(server, result).Inc()
}

func recordDBDropped(reason string) {
	registryDBEntriesDroppedTotal.WithLabelValues(reason).Inc()
}

func recordWriteMetric(action, status string) {
	registryWritesTotal.WithLabelValues(action, status).Inc()
}
