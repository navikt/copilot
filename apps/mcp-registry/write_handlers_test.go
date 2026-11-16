package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlers_NoDatabaseReturns503(t *testing.T) {
	mux := testMux(&Config{AdminGroupID: testAdminGroup, AllowedWriteAZP: []string{testWriterAZP}}, nil, testFakeIntrospect())
	cases := []struct{ path, body string }{
		{"/api/v1/entries", `{"entry": ` + string(subJSON("io.github.navikt/no-db", "d", "1.0.0")) + `}`},
		{"/api/v1/entries/io.github.navikt/no-db/approve", `{"version": 1}`},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer token-admin")
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
		_, _ = w.Write([]byte(`{"active":true,"azp":"c1","NAVident":"A1","groups":["g1"]}`))
	}))
	defer srv.Close()
	res, err := newTexasIntrospector(srv.URL)(t.Context(), "tok")
	if err != nil {
		t.Fatal(err)
	}
	if got["identity_provider"] != "entra_id" {
		t.Errorf("identity_provider = %q, want entra_id", got["identity_provider"])
	}
	if res.Azp != "c1" {
		t.Errorf("azp = %q", res.Azp)
	}
	if len(res.Groups) != 1 || res.Groups[0] != "g1" {
		t.Errorf("groups = %v", res.Groups)
	}
}

func TestHandlers_Authentication(t *testing.T) {
	cfg := &Config{AdminGroupID: testAdminGroup, AllowedWriteAZP: []string{testWriterAZP}}
	introspect := func(_ context.Context, token string) (*IntrospectionResult, error) {
		switch token {
		case "valid-user":
			return &IntrospectionResult{
				Active:   true,
				NAVident: " a123456 ",
			}, nil
		case "app-token":
			return &IntrospectionResult{
				Active:   true,
				IDType:   "app",
				NAVident: "A123456",
			}, nil
		case "no-navident":
			return &IntrospectionResult{
				Active:   true,
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

	t.Run("actor is the uppercased NAVident", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", nil)
		req.Header.Set("Authorization", "Bearer valid-user")
		caller, err := authenticate(req, introspect)
		if err != nil {
			t.Fatalf("unexpected authenticate error: %v", err)
		}
		if caller.Actor != "A123456" {
			t.Fatalf("expected actor 'A123456', got %q", caller.Actor)
		}
	})
}

func TestHandlers_SubmitValidationFailures(t *testing.T) {
	cfg := &Config{AdminGroupID: testAdminGroup, AllowedWriteAZP: []string{testWriterAZP}}
	introspect := func(_ context.Context, _ string) (*IntrospectionResult, error) {
		return &IntrospectionResult{
			Active:   true,
			NAVident: "A123456",
			Azp:      testWriterAZP,
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

func TestHandlers_WriteRequiresAllowedAZP(t *testing.T) {
	for _, tc := range []struct {
		name    string
		allowed []string
	}{
		{"azp not in list", []string{"other-client"}},
		{"empty list fails closed", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Database "connected" so the 403 cannot be the 503 path.
			mux := testMux(&Config{AdminGroupID: testAdminGroup, AllowedWriteAZP: tc.allowed}, &DB{}, testFakeIntrospect())
			for _, path := range []string{"/api/v1/entries", "/api/v1/entries/io.github.navikt/x/approve", "/api/v1/entries/io.github.navikt/x/reject"} {
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
				req.Header.Set("Authorization", "Bearer token-admin")
				rec := httptest.NewRecorder()
				mux.ServeHTTP(rec, req)
				if rec.Code != http.StatusForbidden {
					t.Errorf("%s: expected 403, got %d", path, rec.Code)
				}
			}
		})
	}
}

func TestLogSafe(t *testing.T) {
	if got := logSafe("normal\nforged=true\r\nline"); got != "normalforged=trueline" {
		t.Errorf("logSafe() = %q, want %q", got, "normalforged=trueline")
	}
}

func TestHandlers_BodyLimitsAndTrailingJSON(t *testing.T) {
	cfg := &Config{AdminGroupID: testAdminGroup, AllowedWriteAZP: []string{testWriterAZP}}
	mux := testMux(cfg, &DB{}, testFakeIntrospect())
	validEntry := string(subJSON("io.github.navikt/x", "d", "1.0.0"))

	t.Run("oversized body returns 413", func(t *testing.T) {
		huge := `{"entry":"` + strings.Repeat("a", maxWriteBodyBytes+1) + `"}`
		for _, path := range []string{"/api/v1/entries", "/api/v1/entries/io.github.navikt/x/approve"} {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(huge))
			req.Header.Set("Authorization", "Bearer token-admin")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Errorf("%s: got %d, want 413: %s", path, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("trailing garbage after JSON returns 400", func(t *testing.T) {
		cases := []struct {
			path string
			body string
		}{
			{"/api/v1/entries", `{"entry": ` + validEntry + `} trailing`},
			{"/api/v1/entries", `{"entry": ` + validEntry + `}{"extra": 1}`},
			{"/api/v1/entries/io.github.navikt/x/approve", `{"version": 1} trailing`},
			{"/api/v1/entries/io.github.navikt/x/reject", `{"version": 1, "reason": "nope"} {"extra": 1}`},
		}
		for _, tc := range cases {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer token-admin")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s (%s): got %d, want 400: %s", tc.path, tc.body, rec.Code, rec.Body.String())
			}
		}
	})
}

func TestHandlers_ValidationErrorOmitsRejectedInput(t *testing.T) {
	var logBuf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cfg := &Config{AdminGroupID: testAdminGroup, AllowedWriteAZP: []string{testWriterAZP}}
	mux := testMux(cfg, nil, testFakeIntrospect())

	cases := []struct {
		name      string
		body      string
		wantErr   string
		forbidden string
	}{
		{
			name:      "invalid owner omits email value",
			body:      `{"entry":{"name":"io.github.navikt/test-server","description":"Valid description","version":"1.0.0","owner":"secret.person@nav.no","remotes":[{"type":"streamable-http","url":"https://mcp.example.com/api"}]}}`,
			wantErr:   "invalid owner",
			forbidden: "secret.person@nav.no",
		},
		{
			name:      "invalid websiteUrl omits url value",
			body:      `{"entry":{"name":"io.github.navikt/test-server","description":"Valid description","version":"1.0.0","owner":"team-test","websiteUrl":"http://secret-host.example.com/p","remotes":[{"type":"streamable-http","url":"https://mcp.example.com/api"}]}}`,
			wantErr:   "invalid websiteUrl",
			forbidden: "secret-host.example.com",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logBuf.Reset()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer token-user")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("got %d, want 400: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if resp["error"] != tc.wantErr {
				t.Errorf("error = %q, want %q", resp["error"], tc.wantErr)
			}
			if strings.Contains(rec.Body.String(), tc.forbidden) {
				t.Errorf("response leaked rejected input %q: %s", tc.forbidden, rec.Body.String())
			}
			if strings.Contains(logBuf.String(), tc.forbidden) {
				t.Errorf("log leaked rejected input %q: %s", tc.forbidden, logBuf.String())
			}
		})
	}
}
