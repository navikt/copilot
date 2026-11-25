package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRegisterDevRoutes verifies the double-lock on the unauthenticated
// raw-SQL /dev/query endpoint: it must only be registered when Environment
// is "local" AND ENABLE_DEV_QUERY=true is set explicitly. In particular,
// Environment defaulting to "local" (NAIS_CLUSTER_NAME unset) must NOT be
// enough on its own.
func TestRegisterDevRoutes(t *testing.T) {
	tests := []struct {
		name           string
		environment    string
		enableDevQuery bool
		client         *BigQueryClient
		wantRegistered bool
	}{
		{
			name:           "local without ENABLE_DEV_QUERY is not registered",
			environment:    "local",
			enableDevQuery: false,
			client:         &BigQueryClient{},
			wantRegistered: false,
		},
		{
			name:           "local with ENABLE_DEV_QUERY is registered",
			environment:    "local",
			enableDevQuery: true,
			client:         &BigQueryClient{},
			wantRegistered: true,
		},
		{
			name:           "non-local environment is never registered even with ENABLE_DEV_QUERY",
			environment:    "prod-gcp",
			enableDevQuery: true,
			client:         &BigQueryClient{},
			wantRegistered: false,
		},
		{
			name:           "nil BigQuery client is not registered",
			environment:    "local",
			enableDevQuery: true,
			client:         nil,
			wantRegistered: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			config := &Config{Environment: tt.environment, EnableDevQuery: tt.enableDevQuery}
			registerDevRoutes(mux, config, tt.client)

			req := httptest.NewRequest(http.MethodPost, "/dev/query", nil)
			_, pattern := mux.Handler(req)
			registered := pattern == "/dev/query"
			if registered != tt.wantRegistered {
				t.Errorf("route registered = %v, want %v (matched pattern %q)", registered, tt.wantRegistered, pattern)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string // We'll compare string representations
	}{
		{"DEBUG", "DEBUG"},
		{"debug", "DEBUG"},
		{"INFO", "INFO"},
		{"info", "INFO"},
		{"WARN", "WARN"},
		{"WARNING", "WARN"},
		{"ERROR", "ERROR"},
		{"invalid", "INFO"}, // Default
		{"", "INFO"},        // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLogLevel(tt.input)
			if got.String() != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]bool
	}{
		{
			name:  "single endpoint",
			input: "/api/v1/",
			want:  map[string]bool{"/api/v1/": true},
		},
		{
			name:  "multiple endpoints",
			input: "/api/v1/,/health,/ready",
			want:  map[string]bool{"/api/v1/": true, "/health": true, "/ready": true},
		},
		{
			name:  "with spaces",
			input: "/api/v1/ , /health , /ready",
			want:  map[string]bool{"/api/v1/": true, "/health": true, "/ready": true},
		},
		{
			name:  "empty string",
			input: "",
			want:  map[string]bool{},
		},
		{
			name:  "empty items",
			input: "/api/v1/,,/health",
			want:  map[string]bool{"/api/v1/": true, "/health": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEndpoints(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseEndpoints() length = %v, want %v", len(got), len(tt.want))
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("parseEndpoints() missing key %q", k)
				}
			}
		})
	}
}
