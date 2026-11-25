package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Same names, labels and buckets (Prometheus DefBuckets) as copilot-cli and
// copilot-survey. Hand-written because this app's /metrics is plain text
// without client_golang.
var httpDurationBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

type httpCounterKey struct {
	method, route string
	code          int
}

type httpHistogram struct {
	buckets []uint64
	count   uint64
	sum     float64
}

var httpStats = struct {
	sync.Mutex
	requests  map[httpCounterKey]uint64
	durations map[string]*httpHistogram
}{requests: map[httpCounterKey]uint64{}, durations: map[string]*httpHistogram{}}

// httpMetrics records every request under the ServeMux pattern that matched
// it, never the raw path, so the route label stays bounded. Nested routers
// show up under their mount pattern (for example /api/v1/).
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
		recordHTTPRequest(metricsMethod(r.Method), route, rec.code, time.Since(start).Seconds())
	})
}

func recordHTTPRequest(method, route string, code int, seconds float64) {
	httpStats.Lock()
	defer httpStats.Unlock()
	httpStats.requests[httpCounterKey{method, route, code}]++
	h := httpStats.durations[route]
	if h == nil {
		h = &httpHistogram{buckets: make([]uint64, len(httpDurationBuckets))}
		httpStats.durations[route] = h
	}
	for i, le := range httpDurationBuckets {
		if seconds <= le {
			h.buckets[i]++
		}
	}
	h.count++
	h.sum += seconds
}

// httpMetricsText is both metrics in Prometheus text format.
func httpMetricsText() string {
	httpStats.Lock()
	defer httpStats.Unlock()
	var b strings.Builder
	b.WriteString("\n# HELP http_requests_total Total number of HTTP requests.\n# TYPE http_requests_total counter\n")
	keys := make([]httpCounterKey, 0, len(httpStats.requests))
	for k := range httpStats.requests {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
	for _, k := range keys {
		fmt.Fprintf(&b, "http_requests_total{method=%q,route=%q,code=\"%d\"} %d\n", k.method, k.route, k.code, httpStats.requests[k])
	}
	b.WriteString("# HELP http_request_duration_seconds HTTP request duration in seconds.\n# TYPE http_request_duration_seconds histogram\n")
	routes := make([]string, 0, len(httpStats.durations))
	for r := range httpStats.durations {
		routes = append(routes, r)
	}
	sort.Strings(routes)
	for _, r := range routes {
		h := httpStats.durations[r]
		for i, le := range httpDurationBuckets {
			fmt.Fprintf(&b, "http_request_duration_seconds_bucket{route=%q,le=\"%g\"} %d\n", r, le, h.buckets[i])
		}
		fmt.Fprintf(&b, "http_request_duration_seconds_bucket{route=%q,le=\"+Inf\"} %d\n", r, h.count)
		fmt.Fprintf(&b, "http_request_duration_seconds_sum{route=%q} %g\n", r, h.sum)
		fmt.Fprintf(&b, "http_request_duration_seconds_count{route=%q} %d\n", r, h.count)
	}
	return b.String()
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
