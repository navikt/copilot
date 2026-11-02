package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlers_NoDatabaseReturns503(t *testing.T) {
	mux := testMux(&Config{AdminGroupID: testAdminGroup}, nil, testFakeIntrospect())
	cases := []struct{ path, body string }{
		{"/api/v1/entries", `{"entry": ` + string(subJSON("io.github.navikt/no-db", "d", "1.0.0")) + `}`},
		{"/api/v1/entries/io.github.navikt/no-db/approve", `{"version": 1}`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer token-admin@nav.no")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: got %d, want 503: %s", tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestIntrospectorSendsEntraID(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"active":true,"preferred_username":"U@nav.no","NAVident":"A1","groups":["g1"]}`))
	}))
	defer srv.Close()
	res, err := newTexasIntrospector(srv.URL)(t.Context(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if got["identity_provider"] != "entra_id" {
		t.Errorf("identity_provider = %q, want entra_id", got["identity_provider"])
	}
	if len(res.Groups) != 1 || res.Groups[0] != "g1" {
		t.Errorf("groups = %v", res.Groups)
	}
}

func TestHandlers_Authentication(t *testing.T) {
	cfg := &Config{AdminGroupID: testAdminGroup}
	introspect := func(_ context.Context, token string) (*IntrospectionResult, error) {
		switch token {
		case "valid-user":
			return &IntrospectionResult{
				Active:   true,
				Username: "User@Nav.No",
				NAVident: "A123456",
			}, nil
		case "app-token":
			return &IntrospectionResult{
				Active:   true,
				Username: "app-id",
				IDType:   "app",
				NAVident: "A123456",
			}, nil
		case "no-navident":
			return &IntrospectionResult{
				Active:   true,
				Username: "user@nav.no",
				NAVident: "",
			}, nil
		case "inactive-token":
			return &IntrospectionResult{
				Active: false,
			}, nil
		default:
			return &IntrospectionResult{Active: false}, nil
		}
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

	t.Run("authorization header exceeds 8 KiB", func(t *testing.T) {
		hugeToken := strings.Repeat("x", 8193)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+hugeToken)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on header > 8KiB, got %d", rec.Code)
		}
	})

	t.Run("idtyp app token rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer app-token")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on app token, got %d", rec.Code)
		}
	})

	t.Run("missing NAVident rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer no-navident")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on missing NAVident, got %d", rec.Code)
		}
	})

	t.Run("authenticated user actor is lowercased", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", nil)
		req.Header.Set("Authorization", "Bearer valid-user")
		caller, err := authenticate(req, introspect)
		if err != nil {
			t.Fatalf("unexpected authenticate error: %v", err)
		}
		if caller.Actor != "user@nav.no" {
			t.Fatalf("expected actor 'user@nav.no', got %q", caller.Actor)
		}
	})
}

func TestHandlers_SubmitValidationFailures(t *testing.T) {
	cfg := &Config{AdminGroupID: testAdminGroup}
	introspect := func(_ context.Context, _ string) (*IntrospectionResult, error) {
		return &IntrospectionResult{
			Active:   true,
			Username: "user@nav.no",
			NAVident: "A123456",
		}, nil
	}

	mux := testMux(cfg, nil, introspect)

	t.Run("unknown fields in entry rejected with 400", func(t *testing.T) {
		body := `{
			"entry": {
				"name": "io.github.navikt/test-server",
				"description": "Valid description",
				"version": "1.0.0",
				"owner": "team-test",
				"status": "pending",
				"remotes": [{"type": "streamable-http", "url": "https://mcp.example.com/api"}]
			}
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on unknown fields, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid name in action path rejected with 400", func(t *testing.T) {
		body := `{"version": 1}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries/.invalid-name/approve", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 on invalid name, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
