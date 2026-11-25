package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/mcp-onboarding/internal/store"
)

func TestAuthMiddleware_NoAuthHeader(t *testing.T) {
	ts := store.NewTokenStore()
	mw := NewAuthMiddleware(ts, "https://mcp.example.com")

	handler := mw.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/mcp", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate header")
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	ts := store.NewTokenStore()
	mw := NewAuthMiddleware(ts, "https://mcp.example.com")

	handler := mw.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // not Bearer
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	ts := store.NewTokenStore()
	mw := NewAuthMiddleware(ts, "https://mcp.example.com")

	handler := mw.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	ts := store.NewTokenStore()
	ts.SaveToken("expired-token", &store.TokenData{
		GitHubAccessToken: "gh-token",
		UserLogin:         "testuser",
		UserID:            12345,
		ExpiresAt:         time.Now().Add(-1 * time.Hour),
	})
	mw := NewAuthMiddleware(ts, "https://mcp.example.com")

	handler := mw.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called for expired token")
	}))

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	ts := store.NewTokenStore()
	ts.SaveToken("valid-token", &store.TokenData{
		GitHubAccessToken: "gh-access-token",
		UserLogin:         "octocat",
		UserID:            42,
		ExpiresAt:         time.Now().Add(1 * time.Hour),
	})
	mw := NewAuthMiddleware(ts, "https://mcp.example.com")

	var gotUser *UserContext
	handler := mw.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser == nil {
		t.Fatal("expected user context to be set")
	}
	if gotUser.Login != "octocat" {
		t.Errorf("expected Login 'octocat', got %q", gotUser.Login)
	}
	if gotUser.ID != 42 {
		t.Errorf("expected ID 42, got %d", gotUser.ID)
	}
	if gotUser.GitHubAccessToken != "gh-access-token" {
		t.Errorf("expected GitHubAccessToken 'gh-access-token', got %q", gotUser.GitHubAccessToken)
	}
}

func TestAuthMiddleware_UnauthorizedResponse(t *testing.T) {
	ts := store.NewTokenStore()
	mw := NewAuthMiddleware(ts, "https://mcp.example.com/")

	handler := mw.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.Host = "attacker.example.com"
	req.Header.Set("X-Forwarded-Proto", "http")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	wantHeader := `Bearer resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource"`
	if got := w.Header().Get("WWW-Authenticate"); got != wantHeader {
		t.Errorf("expected WWW-Authenticate %q, got %q", wantHeader, got)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("expected error 'unauthorized', got %q", body["error"])
	}
}

// contextKey is needed to test GetUserFromContext
func TestGetUserFromContext_NoUser(t *testing.T) {
	ctx := context.Background()
	user := GetUserFromContext(ctx)
	if user == nil {
		t.Fatal("expected non-nil user from empty context (should return anonymous)")
	}
	if user.Login != "anonymous" {
		t.Errorf("expected Login 'anonymous', got %q", user.Login)
	}
}

func TestAuthMiddleware_InvalidTokenChallenge(t *testing.T) {
	mw := NewAuthMiddleware(store.NewTokenStore(), "https://mcp.example.com")
	h := mw.Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer unknown")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	want := `Bearer resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource", error="invalid_token"`
	if got := w.Header().Get("WWW-Authenticate"); w.Code != http.StatusUnauthorized || got != want {
		t.Fatalf("status=%d challenge=%q, want 401 %q", w.Code, got, want)
	}

	// A missing token carries no error code (RFC 6750 section 3.1).
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/mcp", nil))
	if got := w.Header().Get("WWW-Authenticate"); strings.Contains(got, "invalid_token") {
		t.Fatalf("missing token challenge has error code: %q", got)
	}
}

func TestAuthMiddleware_NonBearerSchemeHasNoErrorCode(t *testing.T) {
	h := NewAuthMiddleware(store.NewTokenStore(), "https://mcp.example.com").Authenticate(http.NotFoundHandler())
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwdw==")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	got := w.Header().Get("WWW-Authenticate")
	if w.Code != http.StatusUnauthorized || got == "" || strings.Contains(got, "invalid_token") {
		t.Fatalf("status=%d challenge=%q, want 401 challenge without invalid_token", w.Code, got)
	}
}
