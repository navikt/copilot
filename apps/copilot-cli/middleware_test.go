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
