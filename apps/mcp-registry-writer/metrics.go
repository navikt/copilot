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

	registryWritesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "registry_writes_total",
		Help: "Total number of registry write operations.",
	}, []string{"action", "status"})
)

var knownPaths = map[string]bool{
	"/":               true,
	"/health":         true,
	"/ready":          true,
	"/metrics":        true,
	"/api/v1/entries": true,
}

func normalizePath(path string) string {
	if strings.HasPrefix(path, "/api/v1/entries/") {
		if strings.HasSuffix(path, "/approve") {
			return "/api/v1/entries/{name}/approve"
		}
		if strings.HasSuffix(path, "/reject") {
			return "/api/v1/entries/{name}/reject"
		}
		return "/api/v1/entries/{name}"
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

func recordWriteMetric(action, status string) {
	registryWritesTotal.WithLabelValues(action, status).Inc()
}
