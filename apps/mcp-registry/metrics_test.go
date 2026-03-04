package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/v0.1/servers", "/v0.1/servers"},
		{"/v0.1/servers/", "/v0.1/servers/"},
		{"/v0.1/servers/io.github.navikt%2Fgithub-mcp/versions/latest", "/v0.1/servers/{name}"},
		{"/v0.1/servers/io.github.navikt%2Fgithub-mcp/versions/1.0.0", "/v0.1/servers/{name}"},
		{"/v0.1/servers/io.github.navikt%2Fgithub-mcp/latest", "/v0.1/servers/{name}"},
		{"/health", "/health"},
		{"/ready", "/ready"},
		{"/metrics", "/metrics"},
		{"/", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := normalizePath(tt.path)
			if got != tt.want {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestRecordServerLookup(t *testing.T) {
	registryServerLookupsTotal.Reset()

	recordServerLookup("io.github.navikt/github-mcp", "found")
	recordServerLookup("io.github.navikt/github-mcp", "found")
	recordServerLookup("io.github.nonexistent/server", "not_found")

	foundCount := testutil.ToFloat64(registryServerLookupsTotal.WithLabelValues("io.github.navikt/github-mcp", "found"))
	if foundCount != 2 {
		t.Errorf("expected 2 found lookups, got %v", foundCount)
	}

	notFoundCount := testutil.ToFloat64(registryServerLookupsTotal.WithLabelValues("io.github.nonexistent/server", "not_found"))
	if notFoundCount != 1 {
		t.Errorf("expected 1 not_found lookup, got %v", notFoundCount)
	}
}
