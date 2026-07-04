package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestUsageRouteMethodNotAllowed verifies /api/v1/usage rejects non-GET
// methods with 405 instead of proxying them upstream as GET.
func TestUsageRouteMethodNotAllowed(t *testing.T) {
	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"login":"hans","name":"Hans"}`))
		case "/orgs/navikt/members/hans":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ghSrv.Close()

	gh := newGitHubClient()
	gh.baseURL = ghSrv.URL
	cache := newOrgMembershipCache(5 * time.Minute)
	proxy := newCopilotAPIProxy("http://unused.invalid", newTexasClient("", ""))
	cfg := &Config{GitHubOrg: "navikt"}

	router := makeRouter(cfg, gh, cache, proxy)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/usage", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST, got %d", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, got)
	}
}

// TestUsageRouteMissingContextUser verifies the /api/v1/usage handler fails
// closed (no panic) if userFromContext ever returns !ok, e.g. if a future
// refactor forgets to wrap the route in authMiddleware.
func TestUsageRouteMissingContextUser(t *testing.T) {
	proxy := newCopilotAPIProxy("http://unused.invalid", newTexasClient("", ""))

	mux := http.NewServeMux()
	// Deliberately bypass authMiddleware to simulate a missing context user.
	mux.HandleFunc("/api/v1/usage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		user, ok := userFromContext(r.Context())
		if !ok {
			writeAuthError(w, http.StatusInternalServerError, "missing authenticated user in context")
			return
		}
		proxy.forward(usagePath(user.Login))(w, r)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil).WithContext(context.Background())
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panicked instead of failing closed: %v", r)
		}
	}()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when context user is missing, got %d", rec.Code)
	}
}
