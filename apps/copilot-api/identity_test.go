package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockSamlLookup implements samlUsernameLookup for isolated resolver testing.
type mockSamlLookup struct {
	username string
	err      error
}

func (m *mockSamlLookup) getUsernameBySamlIdentity(_ context.Context, _ string) (string, error) {
	return m.username, m.err
}

func TestSAMLIdentityResolverCanResolve(t *testing.T) {
	r := NewSAMLIdentityResolver(&mockSamlLookup{})

	if r.CanResolve(&User{Email: "hans@nav.no"}, nil) != true {
		t.Error("expected CanResolve to be true for user with email")
	}
	if r.CanResolve(&User{}, nil) != false {
		t.Error("expected CanResolve to be false for user without email")
	}
	if r.CanResolve(nil, nil) != false {
		t.Error("expected CanResolve to be false for nil user")
	}
}

func TestSAMLIdentityResolverResolve(t *testing.T) {
	tests := []struct {
		name       string
		lookup     *mockSamlLookup
		wantUser   string
		wantErr    error
		wantErrMsg bool
	}{
		{
			name:     "success",
			lookup:   &mockSamlLookup{username: "hans"},
			wantUser: "hans",
		},
		{
			name:    "empty username maps to ErrNoGitHubAccount",
			lookup:  &mockSamlLookup{username: ""},
			wantErr: ErrNoGitHubAccount,
		},
		{
			name:       "lookup error is wrapped",
			lookup:     &mockSamlLookup{err: errors.New("github api down")},
			wantErrMsg: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := NewSAMLIdentityResolver(tc.lookup)
			identity, err := r.Resolve(context.Background(), &User{Email: "hans@nav.no"}, nil)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if tc.wantErrMsg {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if identity.GitHubUsername != tc.wantUser {
				t.Errorf("GitHubUsername = %q, want %q", identity.GitHubUsername, tc.wantUser)
			}
			if identity.Source != "saml" {
				t.Errorf("Source = %q, want %q", identity.Source, "saml")
			}
		})
	}
}

func TestOnBehalfOfIdentityResolverCanResolve(t *testing.T) {
	r := NewOnBehalfOfIdentityResolver(map[string]bool{"copilot-cli-client-id": true})
	get := httptest.NewRequest(http.MethodGet, "/", nil)

	if r.CanResolve(&User{AZP: "copilot-cli-client-id"}, get) != true {
		t.Error("expected CanResolve true for trusted azp on GET")
	}
	if r.CanResolve(&User{AZP: "some-other-client"}, get) != false {
		t.Error("expected CanResolve false for untrusted azp")
	}
	if r.CanResolve(nil, get) != false {
		t.Error("expected CanResolve false for nil user")
	}
	if r.CanResolve(&User{AZP: "copilot-cli-client-id"}, nil) != false {
		t.Error("expected CanResolve false for nil request")
	}

	// Trust is scoped to read-only routes: write methods must never match,
	// even for a trusted azp, so a compromised intermediary can't mutate
	// arbitrary users' resources (e.g. seats) via X-On-Behalf-Of.
	for _, method := range []string{http.MethodPost, http.MethodDelete, http.MethodPut, http.MethodPatch} {
		req := httptest.NewRequest(method, "/", nil)
		if r.CanResolve(&User{AZP: "copilot-cli-client-id"}, req) != false {
			t.Errorf("expected CanResolve false for trusted azp on %s (read-only trust)", method)
		}
	}

	empty := NewOnBehalfOfIdentityResolver(nil)
	if empty.CanResolve(&User{AZP: "copilot-cli-client-id"}, get) != false {
		t.Error("expected CanResolve false when trustedClientIDs is empty/nil")
	}
}

func TestOnBehalfOfIdentityResolverResolve(t *testing.T) {
	r := NewOnBehalfOfIdentityResolver(map[string]bool{"copilot-cli-client-id": true})

	t.Run("header present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-On-Behalf-Of", "hans")

		identity, err := r.Resolve(context.Background(), &User{AZP: "copilot-cli-client-id"}, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if identity.GitHubUsername != "hans" {
			t.Errorf("GitHubUsername = %q, want %q", identity.GitHubUsername, "hans")
		}
		if identity.Source != "on-behalf-of" {
			t.Errorf("Source = %q, want %q", identity.Source, "on-behalf-of")
		}
	})

	t.Run("header missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		_, err := r.Resolve(context.Background(), &User{AZP: "copilot-cli-client-id"}, req)
		if !errors.Is(err, ErrIdentityHeaderMissing) {
			t.Fatalf("expected ErrIdentityHeaderMissing, got %v", err)
		}
	})

	t.Run("header blank/whitespace-only is treated as missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-On-Behalf-Of", "   ")
		_, err := r.Resolve(context.Background(), &User{AZP: "copilot-cli-client-id"}, req)
		if !errors.Is(err, ErrIdentityHeaderMissing) {
			t.Fatalf("expected ErrIdentityHeaderMissing, got %v", err)
		}
	})

	t.Run("malformed header value is rejected", func(t *testing.T) {
		malformed := []string{
			"has/slash",
			"has spaces",
			"-leading-hyphen",
			"trailing-hyphen-",
			"inv@lid",
			strings.Repeat("a", 40),
		}
		for _, v := range malformed {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-On-Behalf-Of", v)
			_, err := r.Resolve(context.Background(), &User{AZP: "copilot-cli-client-id"}, req)
			if !errors.Is(err, ErrInvalidIdentityHeader) {
				t.Errorf("value %q: expected ErrInvalidIdentityHeader, got %v", v, err)
			}
		}
	})
}

func TestIdentityResolverChain(t *testing.T) {
	onBehalfOf := NewOnBehalfOfIdentityResolver(map[string]bool{"copilot-cli-client-id": true})
	saml := NewSAMLIdentityResolver(&mockSamlLookup{username: "hans"})
	chain := NewIdentityResolverChain(onBehalfOf, saml)

	t.Run("trusted intermediary takes priority", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-On-Behalf-Of", "hans")

		identity, err := chain.Resolve(context.Background(), &User{AZP: "copilot-cli-client-id", Email: "hans@nav.no"}, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if identity.Source != "on-behalf-of" {
			t.Errorf("expected on-behalf-of resolver to win, got source %q", identity.Source)
		}
	})

	t.Run("falls back to SAML for non-trusted azp", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		identity, err := chain.Resolve(context.Background(), &User{AZP: "my-copilot-client-id", Email: "hans@nav.no"}, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if identity.Source != "saml" {
			t.Errorf("expected saml resolver to be used, got source %q", identity.Source)
		}
	})

	t.Run("no applicable resolver", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		_, err := chain.Resolve(context.Background(), &User{}, req)
		if !errors.Is(err, ErrNoApplicableResolver) {
			t.Fatalf("expected ErrNoApplicableResolver, got %v", err)
		}
	})

	// A trusted intermediary with a valid X-On-Behalf-Of header must NOT be
	// able to resolve identity on write routes: the M2M token has no email
	// for SAML to fall back to, so the chain fails closed (401) for POST and
	// DELETE. This bounds the blast radius of a compromised copilot-cli to the
	// single read-only route it legitimately proxies.
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		t.Run("write method with trusted azp is not resolved: "+method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/copilot/seats", nil)
			req.Header.Set("X-On-Behalf-Of", "hans")
			_, err := chain.Resolve(context.Background(), &User{AZP: "copilot-cli-client-id"}, req)
			if !errors.Is(err, ErrNoApplicableResolver) {
				t.Fatalf("expected ErrNoApplicableResolver for %s, got %v", method, err)
			}
		})
	}

	t.Run("empty chain always fails", func(t *testing.T) {
		empty := NewIdentityResolverChain()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		_, err := empty.Resolve(context.Background(), &User{Email: "hans@nav.no"}, req)
		if !errors.Is(err, ErrNoApplicableResolver) {
			t.Fatalf("expected ErrNoApplicableResolver, got %v", err)
		}
	})
}

func TestIdentityMiddleware(t *testing.T) {
	onBehalfOf := NewOnBehalfOfIdentityResolver(map[string]bool{"copilot-cli-client-id": true})
	saml := NewSAMLIdentityResolver(&mockSamlLookup{username: "hans"})
	chain := NewIdentityResolverChain(onBehalfOf, saml)

	var gotIdentity *ResolvedIdentity
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIdentity, _ = GetResolvedIdentity(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	t.Run("required=true, no user in context", func(t *testing.T) {
		gotIdentity = nil
		handler := IdentityMiddleware(chain, true)(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("required=true, resolves successfully", func(t *testing.T) {
		gotIdentity = nil
		handler := IdentityMiddleware(chain, true)(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Email: "hans@nav.no"}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if gotIdentity == nil || gotIdentity.GitHubUsername != "hans" {
			t.Fatalf("expected identity for hans in context, got %+v", gotIdentity)
		}
	})

	t.Run("required=true, resolution fails", func(t *testing.T) {
		gotIdentity = nil
		emptyChain := NewIdentityResolverChain()
		handler := IdentityMiddleware(emptyChain, true)(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Email: "hans@nav.no"}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("required=true, rejects malformed X-On-Behalf-Of header", func(t *testing.T) {
		gotIdentity = nil
		handler := IdentityMiddleware(chain, true)(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-On-Behalf-Of", "inv@lid")
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{AZP: "copilot-cli-client-id"}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("required=false, proceeds without identity on failure", func(t *testing.T) {
		gotIdentity = nil
		emptyChain := NewIdentityResolverChain()
		handler := IdentityMiddleware(emptyChain, false)(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Email: "hans@nav.no"}))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (should proceed)", rec.Code, http.StatusOK)
		}
		if gotIdentity != nil {
			t.Errorf("expected nil identity, got %+v", gotIdentity)
		}
	})

	t.Run("required=false, no user proceeds without identity", func(t *testing.T) {
		gotIdentity = nil
		handler := IdentityMiddleware(chain, false)(next)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestGetResolvedIdentity(t *testing.T) {
	t.Run("not present", func(t *testing.T) {
		_, ok := GetResolvedIdentity(context.Background())
		if ok {
			t.Error("expected ok=false for empty context")
		}
	})

	t.Run("present", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), resolvedIdentityContextKey, &ResolvedIdentity{GitHubUsername: "hans"})
		identity, ok := GetResolvedIdentity(ctx)
		if !ok || identity.GitHubUsername != "hans" {
			t.Fatalf("expected identity for hans, got %+v, ok=%v", identity, ok)
		}
	})
}

func TestRequireOwnership(t *testing.T) {
	newReqWithIdentity := func(identity *ResolvedIdentity) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if identity != nil {
			req = req.WithContext(context.WithValue(req.Context(), resolvedIdentityContextKey, identity))
		}
		return req
	}

	t.Run("no identity in context", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newReqWithIdentity(nil)
		if requireOwnership(rec, req, "hans") {
			t.Fatal("expected requireOwnership to fail without identity")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("matching username", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newReqWithIdentity(&ResolvedIdentity{GitHubUsername: "hans", Source: "saml"})
		if !requireOwnership(rec, req, "hans") {
			t.Fatalf("expected requireOwnership to succeed, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("case-insensitive match", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newReqWithIdentity(&ResolvedIdentity{GitHubUsername: "Hans", Source: "saml"})
		if !requireOwnership(rec, req, "hans") {
			t.Fatalf("expected case-insensitive match to succeed, got %d", rec.Code)
		}
	})

	t.Run("mismatched username", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := newReqWithIdentity(&ResolvedIdentity{GitHubUsername: "attacker", Source: "on-behalf-of"})
		if requireOwnership(rec, req, "hans") {
			t.Fatal("expected requireOwnership to fail on mismatch")
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})
}

func TestWriteIdentityResolutionError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantType   string
	}{
		{"no applicable resolver", ErrNoApplicableResolver, 401, "unauthorized"},
		{"no github account", ErrNoGitHubAccount, 403, "no_github_account"},
		{"identity header missing", ErrIdentityHeaderMissing, 401, "unauthorized"},
		{"invalid identity header", ErrInvalidIdentityHeader, 401, "unauthorized"},
		{"unexpected error", errors.New("database timeout"), 500, "identity_check_failed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeIdentityResolutionError(rec, tc.err)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			body := rec.Body.String()
			if !strings.Contains(body, tc.wantType) {
				t.Errorf("body %q does not contain expected type %q", body, tc.wantType)
			}
		})
	}
}

func TestTrustedClientIDForApp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid entry matched on final name segment",
			input: `[{"name":"dev-gcp:copilot:copilot-cli","clientId":"cli-uuid"}]`,
			want:  "cli-uuid",
		},
		{
			name:  "no matching app name",
			input: `[{"name":"dev-gcp:copilot:my-copilot","clientId":"other-uuid"}]`,
			want:  "",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:    "malformed JSON fails closed with error",
			input:   `[{"name":`,
			want:    "",
			wantErr: true,
		},
		{
			name:  "multiple matches fail closed (ambiguous)",
			input: `[{"name":"dev-gcp:copilot:copilot-cli","clientId":"a"},{"name":"prod-gcp:copilot:copilot-cli","clientId":"b"}]`,
			want:  "",
		},
		{
			name:  "entry with empty clientId is ignored",
			input: `[{"name":"dev-gcp:copilot:copilot-cli","clientId":""}]`,
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := trustedClientIDForApp(tc.input, "copilot-cli")
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("trustedClientIDForApp() = %q, want %q", got, tc.want)
			}
		})
	}
}
