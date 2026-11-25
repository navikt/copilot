package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPMetricsRecordsPatternAndCode(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /things/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	httpMetrics(mux).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/things/abc123", nil))

	text := httpMetricsText()
	if !strings.Contains(text, `http_requests_total{method="GET",route="/things/{id}",code="418"} 1`) {
		t.Fatalf("pattern series missing:\n%s", text)
	}
	if !strings.Contains(text, `http_request_duration_seconds_count{route="/things/{id}"} 1`) {
		t.Fatalf("histogram missing:\n%s", text)
	}
	if strings.Contains(text, "abc123") {
		t.Fatalf("raw path recorded:\n%s", text)
	}
}
