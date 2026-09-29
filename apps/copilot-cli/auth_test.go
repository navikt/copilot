package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

const testClientID = "Iv1.test"

// fakeIssuers is GitHub, copilot-api's membership route and Texas. It knows good-token (org member hans, id 42),
// outsider-token (not a member) and other-app-token (issued to another app).
func fakeIssuers(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"access_token":"m2m","expires_in":3600}`))
			return
		}
		calls.Add(1)
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
			if r.Method == http.MethodDelete {
				if body.AccessToken == "good-token" {
					w.WriteHeader(http.StatusNoContent)
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
				return
			}
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
		case r.Method == http.MethodPost && r.URL.Path == "/internal/v1/github/org-membership":
			if r.Header.Get("Authorization") != "Bearer m2m" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var body struct {
				Login string `json:"login"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			active := body.Login == "hans" || body.Login == "nosso"
			_, _ = w.Write([]byte(`{"active":` + strconv.FormatBool(active) + `}`))
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
	api := newUpstream("copilot-api", srv.URL, newTexasClient(srv.URL+"/token", "api://x"))
	return &authenticator{
		github:   gh,
		org:      "navikt",
		isMember: api.isOrgMember,
		cache:    newTokenCache(5 * time.Minute),
		limit:    rate.NewLimiter(rate.Inf, 1),
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
		{"a JWT (the Entra path is gone)", "Bearer ey" + "J-x.ey" + "J-x.not-signed", 401, "", ""},
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
	// good-token and outsider-token: token check + copilot-api membership
	// once each; nope: token check once.
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
}

// One token spends at most its own burst, and what is left of the global
// bucket stays for everyone else. A token the global bucket refused gets no
// entry, so random tokens cannot grow the map.
func TestPerTokenLimit(t *testing.T) {
	a, _ := testAuthenticator(t)
	a.limit = rate.NewLimiter(0, perTokenBurst+2)
	for i := range perTokenBurst {
		if !a.allow("a") {
			t.Fatalf("a: check %d refused within its burst", i+1)
		}
	}
	if a.allow("a") {
		t.Fatal("a: allowed past its own burst")
	}
	if !a.allow("b") || !a.allow("c") {
		t.Fatal("others refused while the global bucket had room")
	}
	if a.allow("d") {
		t.Fatal("global ceiling not enforced")
	}
	if len(a.perToken) != 3 {
		t.Fatalf("buckets = %d, want 3 (none for a refused token)", len(a.perToken))
	}
}

func TestRevoke(t *testing.T) {
	a, calls := testAuthenticator(t)
	h := makeRouter(a, nil, nil).ServeHTTP
	if _, err := a.resolve(t.Context(), "good-token"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		header string
		want   int
	}{
		{"", 401},
		{"Bearer nope", 401},
		{"Bearer good-token", 204},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/revoke", nil)
		if tc.header != "" {
			req.Header.Set("Authorization", tc.header)
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("%q: status = %d, want %d (%s)", tc.header, rec.Code, tc.want, rec.Body)
		}
	}
	// The fake GitHub still accepts good-token: the refusal comes from the
	// cache, and a success from a check that raced the revoke cannot undo it.
	before := calls.Load()
	a.cache.set("good-token", &AuthenticatedUser{Login: "hans"}, nil)
	if _, err := a.resolve(t.Context(), "good-token"); err != errInvalidToken {
		t.Fatalf("revoked token: %v, want errInvalidToken", err)
	}
	if calls.Load() != before {
		t.Fatal("revoked token reached GitHub")
	}
}

func TestAuthenticateOffWithoutConfig(t *testing.T) {
	a := &authenticator{
		github: newGitHubClient("", ""),
		cache:  newTokenCache(time.Minute),
		limit:  rate.NewLimiter(rate.Inf, 1),
	}
	if _, err := a.resolve(t.Context(), "good-token"); err != errIssuerOffline {
		t.Fatalf("err = %v, want errIssuerOffline", err)
	}
}

func TestCacheNeverOutlivesToken(t *testing.T) {
	c := newTokenCache(time.Hour)
	c.set("t", &AuthenticatedUser{expiresAt: time.Now().Add(-time.Second)}, nil)
	if _, ok, _ := c.get("t"); ok {
		t.Fatal("an expired token's success was served from cache")
	}
}

// Any answer from copilot-api but a 200 with a boolean "active" refuses the
// sign-in, is not cached, and the user token never goes to copilot-api.
func TestMembershipFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"down", 503, `{"active":true}`},
		{"refused the gateway", 401, `{"active":true}`},
		{"forbidden", 403, `{"active":true}`},
		{"redirect", 302, `{"active":true}`},
		{"no active field", 200, `{"member":true}`},
		{"active not a bool", 200, `{"active":"true"}`},
		{"not json", 200, `yes`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sawUserToken atomic.Bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					_, _ = w.Write([]byte(`{"access_token":"m2m","expires_in":3600}`))
				case "/applications/" + testClientID + "/token":
					_, _ = w.Write([]byte(`{"app":{"client_id":"` + testClientID + `"},"user":{"login":"hans","id":42}}`))
				default:
					if strings.Contains(r.Header.Get("Authorization"), "good-token") {
						sawUserToken.Store(true)
					}
					if tc.status == 302 {
						w.Header().Set("Location", "/elsewhere")
					}
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.body))
				}
			}))
			defer srv.Close()
			a, _ := testAuthenticator(t)
			a.github.baseURL = srv.URL
			a.isMember = newUpstream("copilot-api", srv.URL, newTexasClient(srv.URL+"/token", "api://x")).isOrgMember
			h := authMiddleware(a, func(http.ResponseWriter, *http.Request) { t.Fatal("handler reached") })
			for range 2 {
				req := httptest.NewRequest(http.MethodGet, "/x", nil)
				req.Header.Set("Authorization", "Bearer good-token")
				rec := httptest.NewRecorder()
				h(rec, req)
				if rec.Code != http.StatusBadGateway {
					t.Fatalf("status = %d, want 502", rec.Code)
				}
			}
			if _, ok, _ := a.cache.get("good-token"); ok {
				t.Fatal("a failed membership check was cached")
			}
			if sawUserToken.Load() {
				t.Fatal("the user token was sent to copilot-api")
			}
		})
	}
}

// Texas down or copilot-api unreachable: refused too.
func TestMembershipUnreachable(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	for _, u := range []*upstream{
		newUpstream("copilot-api", dead.URL, newTexasClient(dead.URL+"/token", "api://x")),
		newUpstream("copilot-api", dead.URL, newTexasClient("", "api://x")),
	} {
		if ok, err := u.isOrgMember(t.Context(), "hans"); ok || err == nil {
			t.Fatalf("ok=%v err=%v, want an error", ok, err)
		}
	}
}
