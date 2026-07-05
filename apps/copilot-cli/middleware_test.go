package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthMiddleware(t *testing.T) {
	gh := newGitHubClient()

	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			if r.Header.Get("Authorization") != "Bearer good-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"login":"hans","name":"Hans"}`))
		case "/orgs/navikt/members/hans":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ghSrv.Close()
	gh.baseURL = ghSrv.URL

	cache := newOrgMembershipCache(5 * time.Minute)

	var gotUser *AuthenticatedUser
	next := func(w http.ResponseWriter, r *http.Request) {
		gotUser, _ = userFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}

	handler := authMiddleware(gh, cache, "navikt", next)

	t.Run("missing auth header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("valid token and org member", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
		req.Header.Set("Authorization", "Bearer good-token")
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if gotUser == nil || gotUser.Login != "hans" {
			t.Fatalf("expected user hans in context, got %+v", gotUser)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
		req.Header.Set("Authorization", "Bearer bad-token")
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		want    string
		wantErr bool
	}{
		{"canonical scheme", "Bearer tok123", "tok123", false},
		{"lowercase scheme", "bearer tok123", "tok123", false},
		{"uppercase scheme", "BEARER tok123", "tok123", false},
		{"missing header", "", "", true},
		{"scheme without token", "Bearer", "", true},
		{"scheme with only whitespace", "Bearer   ", "", true},
		{"wrong scheme", "Basic dXNlcjpwYXNz", "", true},
		{"extra fields", "Bearer tok123 extra", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/usage", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			got, err := bearerToken(req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for header %q, got token %q", tc.header, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("bearerToken(%q): %v", tc.header, err)
			}
			if got != tc.want {
				t.Fatalf("bearerToken(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

// TestOrgMembershipCacheEvictsExpired verifies that an expired entry is
// deleted from the backing map on lookup, not just treated as a miss.
func TestOrgMembershipCacheEvictsExpired(t *testing.T) {
	cache := newOrgMembershipCache(-1 * time.Second) // entries expire immediately
	cache.set("tok", &AuthenticatedUser{Login: "hans"})

	if user, ok := cache.get("tok"); ok {
		t.Fatalf("expected expired entry to be a miss, got %+v", user)
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.entries) != 0 {
		t.Fatalf("expected expired entry to be evicted, %d entries remain", len(cache.entries))
	}
}
