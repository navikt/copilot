package mcpkit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequireBearerToken(t *testing.T) {
	future, past := time.Now().Add(time.Hour), time.Now().Add(-time.Minute)
	ok := func(exp time.Time) TokenVerifier {
		return func(context.Context, string, *http.Request) (*TokenInfo, error) {
			return &TokenInfo{UserID: "Z1", Expiration: exp}, nil
		}
	}
	fail := func(err error) TokenVerifier {
		return func(context.Context, string, *http.Request) (*TokenInfo, error) { return nil, err }
	}
	cases := []struct {
		name      string
		header    string
		v         TokenVerifier
		status    int
		challenge string
	}{
		{"valid", "Bearer t", ok(future), 200, ""},
		{"no header", "", ok(future), 401, `Bearer realm="app"`},
		{"oversized", "Bearer " + strings.Repeat("a", MaxBearerLen), ok(future), 401, `Bearer realm="app"`},
		{"whitespace", "Bearer a b", ok(future), 401, `Bearer realm="app"`},
		{"wrong scheme", "Basic dXNlcg==", ok(future), 401, `Bearer realm="app"`},
		{"empty token", "Bearer ", ok(future), 401, `Bearer realm="app"`},
		{"missing exp", "Bearer t", ok(time.Time{}), 401, `Bearer error="invalid_token"`},
		{"expired", "Bearer t", ok(past), 401, `Bearer error="invalid_token"`},
		{"nil info", "Bearer t", fail(nil), 401, `Bearer error="invalid_token"`},
		{"invalid", "Bearer t", fail(ErrInvalidToken), 401, `Bearer error="invalid_token"`},
		{"forbidden", "Bearer t", fail(ErrForbidden), 403, ""},
		{"not configured", "Bearer t", fail(ErrNotConfigured), 503, ""},
		{"texas down", "Bearer t", fail(errors.New("boom")), 502, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got *TokenInfo
			calls := 0
			counted := func(ctx context.Context, tok string, r *http.Request) (*TokenInfo, error) {
				calls++
				return tc.v(ctx, tok, r)
			}
			h := RequireBearerToken(counted, &RequireBearerTokenOptions{Realm: "app"})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = TokenInfoFromContext(r.Context())
			}))
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.status || rec.Header().Get("WWW-Authenticate") != tc.challenge {
				t.Fatalf("got %d %q, want %d %q", rec.Code, rec.Header().Get("WWW-Authenticate"), tc.status, tc.challenge)
			}
			// A malformed header must never reach Texas.
			if wantCalls := map[bool]int{true: 0, false: 1}[tc.challenge == `Bearer realm="app"`]; calls != wantCalls {
				t.Fatalf("verifier called %d times, want %d", calls, wantCalls)
			}
			if (tc.status == 200) != (got != nil && got.UserID == "Z1") {
				t.Fatalf("token info in context = %v", got)
			}
		})
	}
}

func TestTexasVerifier(t *testing.T) {
	claims := map[string]any{"active": true, "NAVident": "Z123", "azp": "gw", "exp": 4102444800}
	ti, err := texasWith(t, 200, claims).Verifier()(context.Background(), "tok", nil)
	if err != nil || ti.UserID != "Z123" || ti.Expiration.Unix() != 4102444800 || ti.Extra["token"] != "tok" {
		t.Fatalf("got %+v, %v", ti, err)
	}
	delete(claims, "exp")
	if ti, err := texasWith(t, 200, claims).Verifier()(context.Background(), "tok", nil); err != nil || !ti.Expiration.IsZero() {
		t.Fatalf("missing exp: got %+v, %v", ti, err)
	}
	claims["azp"] = "other"
	if _, err := texasWith(t, 200, claims).Verifier()(context.Background(), "tok", nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("wrong azp: %v", err)
	}
}
