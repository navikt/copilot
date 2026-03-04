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
)

func normalizePath(path string) string {
	if strings.HasPrefix(path, "/v0.1/servers/") && path != "/v0.1/servers/" {
		return "/v0.1/servers/{name}"
	}
	return path
}

func recordHTTPMetrics(method, path string, statusCode int, duration time.Duration) {
	httpRequestsTotal.WithLabelValues(method, normalizePath(path), strconv.Itoa(statusCode)).Inc()
	httpRequestDuration.WithLabelValues(method, normalizePath(path)).Observe(duration.Seconds())
}

func recordServerLookup(server, result string) {
	registryServerLookupsTotal.WithLabelValues(server, result).Inc()
}
