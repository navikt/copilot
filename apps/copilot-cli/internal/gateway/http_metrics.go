package gateway

import (
	"net/http"
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
	}, []string{"method", "route", "code"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})
)

// httpMetrics records every request under the ServeMux pattern that matched
// it, never the raw path, so the route label stays bounded.
func httpMetrics(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &metricsRecorder{ResponseWriter: w, code: http.StatusOK}
		mux.ServeHTTP(rec, r) // sets r.Pattern on this request
		route := r.Pattern
		if _, p, ok := strings.Cut(route, " "); ok {
			route = p
		}
		if route == "" {
			route = "unmatched"
		}
		httpRequestsTotal.WithLabelValues(metricsMethod(r.Method), route, strconv.Itoa(rec.code)).Inc()
		httpRequestDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
	})
}

// metricsMethod bounds the client-controlled method label.
func metricsMethod(m string) string {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return m
	}
	return "other"
}

type metricsRecorder struct {
	http.ResponseWriter
	code        int
	wroteHeader bool
}

func (m *metricsRecorder) WriteHeader(code int) {
	if !m.wroteHeader {
		m.code, m.wroteHeader = code, true
	}
	m.ResponseWriter.WriteHeader(code)
}

func (m *metricsRecorder) Write(b []byte) (int, error) {
	m.wroteHeader = true
	return m.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach Flush on the real writer.
func (m *metricsRecorder) Unwrap() http.ResponseWriter { return m.ResponseWriter }
