package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
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
			wantToken:  "token123",
			wantErr:    false,
		},
		{
			name:       "case-insensitive bearer scheme",
			authHeader: "bEaReR token123",
			wantToken:  "token123",
			wantErr:    false,
		},
		{
			name:        "invalid format with extra segments",
			authHeader:  "Bearer token123 extra",
			wantErr:     true,
			errContains: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			token, err := ExtractBearerToken(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractBearerToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ExtractBearerToken() error = %v, should contain %q", err, tt.errContains)
				}
			}

			if !tt.wantErr && token != tt.wantToken {
				t.Errorf("ExtractBearerToken() = %v, want %v", token, tt.wantToken)
			}
		})
	}
}

func TestValidateAudience(t *testing.T) {
	tests := []struct {
		name      string
		audience  interface{}
		expected  string
		wantValid bool
	}{
		{
			name:      "string audience matches",
			audience:  "api://copilot",
			expected:  "api://copilot",
			wantValid: true,
		},
		{
			name:      "string audience mismatch",
			audience:  "api://other",
			expected:  "api://copilot",
			wantValid: false,
		},
		{
			name:      "array audience matches",
			audience:  []interface{}{"api://other", "api://copilot"},
			expected:  "api://copilot",
			wantValid: true,
		},
		{
			name:      "array audience mismatch",
			audience:  []interface{}{"api://other"},
			expected:  "api://copilot",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateAudience(tt.audience, tt.expected)
			if got != tt.wantValid {
				t.Fatalf("validateAudience() = %v, want %v", got, tt.wantValid)
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

		ctx := context.WithValue(context.Background(), UserContextKey, expectedUser)
		user, ok := UserFromContext(ctx)

		if !ok {
			t.Error("UserFromContext() ok = false, want true")
		}

		if user != expectedUser {
			t.Errorf("UserFromContext() = %v, want %v", user, expectedUser)
		}
	})

	t.Run("no user in context", func(t *testing.T) {
		ctx := context.Background()
		user, ok := UserFromContext(ctx)

		if ok {
			t.Error("UserFromContext() ok = true, want false")
		}

		if user != nil {
			t.Errorf("UserFromContext() = %v, want nil", user)
		}
	})

	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), UserContextKey, "not a user")
		user, ok := UserFromContext(ctx)

		if ok {
			t.Error("UserFromContext() ok = true, want false")
		}

		if user != nil {
			t.Errorf("UserFromContext() = %v, want nil", user)
		}
	})
}

func TestMakeAuthMiddleware_DevMode(t *testing.T) {
	// The dev mock user's email is configurable via DEV_USER_EMAIL so local dev can
	// resolve a real GitHub identity (e.g. for the budget endpoint's SAML lookup).
	t.Setenv("DEV_USER_EMAIL", "tester@nav.no")

	config := Config{
		Environment: "local",
		AzureIssuer: "", // No Azure config = dev mode
	}

	middleware := Middleware(config)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			t.Error("Expected user in context in dev mode")
			return
		}

		if user.Email != "tester@nav.no" {
			t.Errorf("Expected dev user email tester@nav.no, got %s", user.Email)
		}
		if user.PreferredUsername != "tester@nav.no" {
			t.Errorf("Expected PreferredUsername tester@nav.no, got %s", user.PreferredUsername)
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

// The claims that decide IsAppToken must be read from a real token's shape:
// JSON arrays decode to []interface{}, and a role given as a string is not a
// list of roles.
func TestUserFromClaimsAppToken(t *testing.T) {
	for name, tc := range map[string]struct {
		claims jwt.MapClaims
		app    bool
	}{
		"M2M token":          {jwt.MapClaims{"idtyp": "app", "roles": []interface{}{"access_as_application"}}, true},
		"role only":          {jwt.MapClaims{"roles": []interface{}{"access_as_application"}}, true},
		"role as a string":   {jwt.MapClaims{"roles": "access_as_application"}, false},
		"user (OBO) token":   {jwt.MapClaims{"NAVident": "Z123456", "preferred_username": "ola@nav.no"}, false},
		"app with an e-mail": {jwt.MapClaims{"idtyp": "app", "email": "ola@nav.no"}, false},
	} {
		if got := userFromClaims("azp", tc.claims).IsAppToken(); got != tc.app {
			t.Errorf("%s: IsAppToken = %v, want %v", name, got, tc.app)
		}
	}
}

// copilot-survey's tokens pass only on the name-id route; Nais inbound access
// is pod-wide. Nothing on that route logs the caller's identity.
func TestBearerAuthSurveyScope(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	users := map[string]*User{
		"survey": {AZP: "survey-id", Idtyp: "app"},
		"user":   {AZP: "survey-id", NAVident: "Z999999"},
		"other":  {AZP: "my-copilot-id", NAVident: "Z123456"},
		// A second pre-authorized app named copilot-survey is fenced too.
		"survey2": {AZP: "survey-id-2", Idtyp: "app"},
	}
	validate := func(tok string) (*User, error) { return users[tok], nil }
	h := BearerAuth(validate, []string{"survey-id", "survey-id-2"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	for _, tc := range []struct {
		token, path string
		want        int
	}{
		{"survey", SAMLNameIDPath, 200},
		{"survey", "/api/v1/copilot/usage/metrics", 403},
		{"survey", "/api/v1/budget", 403},
		{"survey2", "/api/v1/budget", 403},
		{"user", SAMLNameIDPath, 200},
		{"other", "/api/v1/copilot/usage/metrics", 200},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s on %s = %d, want %d", tc.token, tc.path, rec.Code, tc.want)
		}
	}
	if strings.Contains(buf.String(), "Z999999") {
		t.Fatalf("an identity on the name-id route reached the log: %s", buf.String())
	}
	if strings.Contains(buf.String(), "Z123456") {
		t.Fatalf("a NAVident reached the log: %s", buf.String())
	}
}

// The survey fence fails closed: an ambiguous name yields every match.
func TestClientIDsForApp(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want []string
	}{
		{"", nil},
		{`[{"name":"dev-gcp:copilot:my-copilot","clientId":"a"}]`, nil},
		{`[{"name":"dev-gcp:copilot:copilot-survey","clientId":"a"}]`, []string{"a"}},
		{`[{"name":"dev-gcp:copilot:copilot-survey","clientId":"a"},{"name":"dev-gcp:other:copilot-survey","clientId":"b"}]`, []string{"a", "b"}},
	} {
		got, err := clientIDsForApp(tc.raw, "copilot-survey")
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("clientIDsForApp(%s) = %v, %v, want %v", tc.raw, got, err, tc.want)
		}
	}
}
