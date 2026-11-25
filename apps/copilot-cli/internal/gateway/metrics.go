package gateway

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func metricsHandler() http.Handler {
	return promhttp.Handler()
}
