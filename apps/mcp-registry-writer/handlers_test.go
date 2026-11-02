package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlers_HealthAndReady(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("healthHandler returned %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal health response: %v", err)
	}
	if resp["status"] != "healthy" {
		t.Errorf("status = %q, want healthy", resp["status"])
	}

	// Ready without store
	readyReq := httptest.NewRequest(http.MethodGet, "/ready", nil)
	readyRec := httptest.NewRecorder()
	readyHandler(nil)(readyRec, readyReq)

	if readyRec.Code != http.StatusOK {
		t.Fatalf("readyHandler returned %d", readyRec.Code)
	}
}

func TestHandlers_AuthenticationFailures(t *testing.T) {
	cfg := &Config{AdminUsers: map[string]bool{"admin@nav.no": true}}
	introspect := func(_ context.Context, token string) (*IntrospectionResult, error) {
		if token == "active-user" {
			return &IntrospectionResult{Active: true, Username: "user@nav.no"}, nil
		}
		return &IntrospectionResult{Active: false}, nil
	}

	mux := testMux(cfg, nil, introspect)

	t.Run("missing authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on missing auth, got %d", rec.Code)
		}
	})

	t.Run("invalid token format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Basic user:pass")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on Basic auth, got %d", rec.Code)
		}
	})

	t.Run("inactive token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer inactive-token")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on inactive token, got %d", rec.Code)
		}
	})
}
