package main

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetrics_NormalizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/health", "/health"},
		{"/ready", "/ready"},
		{"/metrics", "/metrics"},
		{"/api/v1/entries", "/api/v1/entries"},
		{"/api/v1/entries/io.github.navikt/test/approve", "/api/v1/entries/{name}/approve"},
		{"/api/v1/entries/io.github.navikt/test/reject", "/api/v1/entries/{name}/reject"},
		{"/api/v1/entries/io.github.navikt/test", "/api/v1/entries/{name}"},
		{"/unknown/path", "other"},
	}

	for _, tc := range tests {
		if got := normalizePath(tc.input); got != tc.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMetrics_Recording(t *testing.T) {
	recordHTTPMetrics("POST", "/api/v1/entries", 201, 15*time.Millisecond)
	recordWriteMetric("submit", "success")

	count := testutil.ToFloat64(registryWritesTotal.WithLabelValues("submit", "success"))
	if count < 1 {
		t.Errorf("expected registryWritesTotal >= 1, got %v", count)
	}
}
