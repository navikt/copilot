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
