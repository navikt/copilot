package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name        string
		authHeader  string
		wantToken   string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
			wantToken:  "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
			wantErr:    false,
		},
		{
			name:        "missing header",
			authHeader:  "",
			wantErr:     true,
			errContains: "missing",
		},
		{
			name:        "invalid format (no Bearer prefix)",
			authHeader:  "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:        "invalid format (wrong scheme)",
			authHeader:  "Basic dXNlcjpwYXNz",
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name:       "bearer with extra spaces",
			authHeader: "Bearer   token123",
			wantToken:  "  token123",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			token, err := extractBearerToken(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractBearerToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("extractBearerToken() error = %v, should contain %q", err, tt.errContains)
				}
			}

			if !tt.wantErr && token != tt.wantToken {
				t.Errorf("extractBearerToken() = %v, want %v", token, tt.wantToken)
			}
		})
	}
}

func TestGetUserFromContext(t *testing.T) {
	t.Run("user in context", func(t *testing.T) {
		expectedUser := &User{
			Email:    "test@nav.no",
			NAVident: "T123456",
		}

		ctx := context.WithValue(context.Background(), userContextKey, expectedUser)
		user, ok := getUserFromContext(ctx)

		if !ok {
			t.Error("getUserFromContext() ok = false, want true")
		}

		if user != expectedUser {
			t.Errorf("getUserFromContext() = %v, want %v", user, expectedUser)
		}
	})

	t.Run("no user in context", func(t *testing.T) {
		ctx := context.Background()
		user, ok := getUserFromContext(ctx)

		if ok {
			t.Error("getUserFromContext() ok = true, want false")
		}

		if user != nil {
			t.Errorf("getUserFromContext() = %v, want nil", user)
		}
	})

	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), userContextKey, "not a user")
		user, ok := getUserFromContext(ctx)

		if ok {
			t.Error("getUserFromContext() ok = true, want false")
		}

		if user != nil {
			t.Errorf("getUserFromContext() = %v, want nil", user)
		}
	})
}

func TestMakeAuthMiddleware_DevMode(t *testing.T) {
	config := &Config{
		Environment: "local",
		AzureIssuer: "", // No Azure config = dev mode
	}

	middleware := makeAuthMiddleware(config)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := getUserFromContext(r.Context())
		if !ok {
			t.Error("Expected user in context in dev mode")
			return
		}

		if user.Email != "dev@nav.no" {
			t.Errorf("Expected dev user email, got %s", user.Email)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(user)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
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

func TestJWKSCache(t *testing.T) {
	// Mock JWKS server with valid RSA key (test key, not for production)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwks := JWKS{
			Keys: []JWK{
				{
					Kid: "test-kid",
					Kty: "RSA",
					Use: "sig",
					// Valid base64url-encoded 2048-bit RSA modulus (test key)
					N: "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
					E: "AQAB",
				},
			},
		}
		json.NewEncoder(w).Encode(jwks)
	}))
	defer server.Close()

	cache := newJWKSCache(server.URL)

	// Test refresh
	err := cache.refresh()
	if err != nil {
		t.Fatalf("refresh() error = %v", err)
	}

	// Verify key exists
	cache.mu.RLock()
	if len(cache.keys) != 1 {
		t.Errorf("Expected 1 key after refresh, got %d", len(cache.keys))
	}
	cache.mu.RUnlock()

	// Test TTL - keys should be cached
	cache.mu.Lock()
	cache.lastUpdate = time.Now().Add(-2 * time.Hour) // Simulate old cache
	cache.mu.Unlock()

	// getKey should trigger refresh
	_, err = cache.getKey("test-kid")
	if err != nil {
		t.Errorf("getKey() error = %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
