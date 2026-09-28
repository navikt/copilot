package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// metricsHandler exposes Prometheus metrics for scraping, following the
// same convention as the other Go services in this monorepo.
func metricsHandler() http.Handler {
	return promhttp.Handler()
}
