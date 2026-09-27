package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

const testClientID = "Iv1.test"

// Fake JWT-shaped tokens, assembled at run time so no token-shaped literal
// sits in the source for secret scanners to flag.
var (
	entraToken   = fakeJWT("user")
	appToken     = fakeJWT("app")
	foreignToken = fakeJWT("foreign")
	inactiveTok  = fakeJWT("inactive")
)

func fakeJWT(name string) string { return "ey" + "J-" + name + ".ey" + "J-" + name + ".not-signed" }

// fakeIssuers is GitHub and the Texas introspection endpoint in one server.
// GitHub knows good-token (org member hans, id 42), outsider-token (not a
// member) and other-app-token (issued to another app). Texas knows
// entraToken (a user via my-copilot), appToken (app-only) and foreignToken
// (a user, via an app that is not pre-authorized).
func fakeIssuers(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/applications/"+testClientID+"/token":
			if id, secret, ok := r.BasicAuth(); !ok || id != testClientID || secret != "s3cret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var body struct {
				AccessToken string `json:"access_token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			switch body.AccessToken {
			case "good-token":
				_, _ = w.Write([]byte(`{"app":{"client_id":"` + testClientID + `"},"user":{"login":"hans","id":42}}`))
			case "nosso-token":
				_, _ = w.Write([]byte(`{"app":{"client_id":"` + testClientID + `"},"user":{"login":"nosso","id":9}}`))
			case "outsider-token":
				_, _ = w.Write([]byte(`{"app":{"client_id":"` + testClientID + `"},"user":{"login":"outsider","id":7}}`))
			case "other-app-token":
				_, _ = w.Write([]byte(`{"app":{"client_id":"Iv1.other"},"user":{"login":"hans","id":42}}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		case r.URL.Path == "/orgs/navikt/members/hans", r.URL.Path == "/orgs/navikt/members/nosso":
			w.WriteHeader(http.StatusNoContent)
		case strings.HasPrefix(r.URL.Path, "/orgs/navikt/members/"):
			// What GitHub answers when the requester is not a member.
			http.Redirect(w, r, "/orgs/navikt/public_members/x", http.StatusFound)
		case r.URL.Path == "/introspect":
			var body struct {
				Token string `json:"token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			exp := time.Now().Add(time.Hour).Unix()
			switch body.Token {
			case entraToken:
				_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "azp": "my-copilot-id", "oid": "oid-1", "NAVident": "Z999999", "preferred_username": "Hans.Test@nav.no", "exp": exp})
			case appToken:
				_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "azp": "my-copilot-id", "oid": "oid-app", "idtyp": "app", "exp": exp})
			case foreignToken:
				_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "azp": "someone-else", "oid": "oid-1", "NAVident": "Z999999", "exp": exp})
			default:
				_, _ = w.Write([]byte(`{"active":false,"error":"invalid token"}`))
			}
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func testAuthenticator(t *testing.T) (*authenticator, *atomic.Int32) {
	srv, calls := fakeIssuers(t)
	gh := newGitHubClient(testClientID, "s3cret")
	gh.baseURL = srv.URL
	return &authenticator{
		github: gh,
		entra:  newEntraClient(srv.URL+"/introspect", `[{"name":"dev-gcp:copilot:my-copilot","clientId":"my-copilot-id"}]`),
		org:    "navikt",
		cache:  newTokenCache(5 * time.Minute),
		limit:  rate.NewLimiter(rate.Inf, 1),
	}, calls
}

func TestAuthenticate(t *testing.T) {
	a, _ := testAuthenticator(t)
	var got *AuthenticatedUser
	h := authMiddleware(a, func(_ http.ResponseWriter, r *http.Request) {
		got, _ = userFromContext(r.Context())
	})

	for _, tc := range []struct {
		name, header string
		want         int
		issuer, sub  string
	}{
		{"no header", "", 401, "", ""},
		{"github member", "Bearer good-token", 200, issuerGitHub, "42"},
		{"github non-member", "Bearer outsider-token", 403, "", ""},
		{"github token of another app", "Bearer other-app-token", 401, "", ""},
		{"github unknown token", "Bearer nope", 401, "", ""},
		{"entra user", "Bearer " + entraToken, 200, issuerEntra, "oid-1"},
		{"entra app-only token", "Bearer " + appToken, 403, "", ""},
		{"entra token via a non-pre-authorized app", "Bearer " + foreignToken, 401, "", ""},
		{"entra inactive", "Bearer " + inactiveTok, 401, "", ""},
		{"basic scheme", "Basic dXNlcjpwYXNz", 401, "", ""},
		{"two tokens", "Bearer a b", 401, "", ""},
		{"oversized token", "Bearer " + strings.Repeat("a", 9000), 401, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got = nil
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			h(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}
			if tc.want == 200 && (got == nil || got.Issuer != tc.issuer || got.Subject != tc.sub) {
				t.Fatalf("user = %+v, want %s/%s", got, tc.issuer, tc.sub)
			}
		})
	}
}

func TestAuthenticateCachesOutcomes(t *testing.T) {
	a, calls := testAuthenticator(t)
	for range 3 {
		_, _ = a.resolve(t.Context(), "good-token")
		_, _ = a.resolve(t.Context(), "nope")
		_, _ = a.resolve(t.Context(), "outsider-token")
	}
	// good-token and outsider-token: token check + org check once each;
	// nope: token check once.
	if n := calls.Load(); n != 5 {
		t.Fatalf("issuer calls = %d, want 5 (cached after the first)", n)
	}
}

func TestAuthenticateRateLimitsCacheMisses(t *testing.T) {
	a, _ := testAuthenticator(t)
	a.limit = rate.NewLimiter(0, 1)
	if _, err := a.resolve(t.Context(), "nope"); err != errInvalidToken {
		t.Fatalf("first miss: %v", err)
	}
	if _, err := a.resolve(t.Context(), "nope-2"); err != errRateLimited {
		t.Fatalf("second miss: %v, want rate limited", err)
	}
	if _, err := a.resolve(t.Context(), "nope"); err != errInvalidToken {
		t.Fatalf("cached refusal should not spend the limit: %v", err)
	}
	if _, err := a.resolve(t.Context(), entraToken); err != nil {
		t.Fatalf("the Entra path is not behind the GitHub limit: %v", err)
	}
}

func TestAuthenticateOffWithoutConfig(t *testing.T) {
	a := &authenticator{
		github: newGitHubClient("", ""),
		entra:  newEntraClient("http://texas", ""),
		cache:  newTokenCache(time.Minute),
		limit:  rate.NewLimiter(rate.Inf, 1),
	}
	for _, tok := range []string{"good-token", entraToken} {
		if _, err := a.resolve(t.Context(), tok); err != errIssuerOffline {
			t.Fatalf("%s: err = %v, want errIssuerOffline", tok, err)
		}
	}
}

func TestCacheNeverOutlivesToken(t *testing.T) {
	c := newTokenCache(time.Hour)
	c.set("t", &AuthenticatedUser{expiresAt: time.Now().Add(-time.Second)}, nil)
	if _, ok, _ := c.get("t"); ok {
		t.Fatal("an expired token's success was served from cache")
	}
}

// A 302 (the requester is not a member, or cannot read members) is "not a
// member", and the redirect is not followed with the token on it.
func TestOrgMemberDoesNotFollowRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "public_members") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Redirect(w, r, "/orgs/navikt/public_members/hans", http.StatusFound)
	}))
	defer srv.Close()
	gh := newGitHubClient(testClientID, "s3cret")
	gh.baseURL = srv.URL
	if member, err := gh.isOrgMember(t.Context(), "tok", "navikt", "hans"); err != nil || member {
		t.Fatalf("member=%v err=%v, want not a member", member, err)
	}
}
