package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type seen struct{ method, path, auth, onBehalfOf, contentType, body string }

// fakeUpstream is copilot-api, copilot-survey and Texas in one server.
func fakeUpstream(t *testing.T, got *seen) (*upstream, *upstream) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"access_token":"m2m","expires_in":3600}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		*got = seen{r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("X-On-Behalf-Of"), r.Header.Get("Content-Type"), string(b)}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = w.Write([]byte(`{"from":"upstream"}`))
	}))
	t.Cleanup(srv.Close)
	texas := newTexasClient(srv.URL+"/token", "api://x")
	return newUpstream("copilot-api", srv.URL, texas), newUpstream("copilot-survey", srv.URL, texas)
}

// Each /api/v1/* route that shipped nav-pilot binaries call reaches its
// service unchanged; signed-in ones carry the M2M token and the login.
func TestRoutesForward(t *testing.T) {
	a, _ := testAuthenticator(t)
	var got seen
	api, surveys := fakeUpstream(t, &got)
	h := makeRouter(a, api, surveys)

	for _, tc := range []struct {
		name, method, path, token, body string
		want                            int
		upstream                        seen
	}{
		{"usage", "GET", "/api/v1/usage", "good-token", "", 200,
			seen{"GET", "/api/v1/copilot/usage/user/hans", "Bearer m2m", "hans", "", ""}},
		{"survey definitions, public", "GET", "/api/v1/surveys/active", "", "", 200,
			seen{"GET", "/api/v1/surveys/active", "", "", "", ""}},
		{"survey answer", "POST", "/api/v1/surveys/q4-2026/responses", "good-token", `{"answers":{}}`, 201,
			seen{"POST", "/api/v1/surveys/q4-2026/responses", "Bearer m2m", "hans", "application/json", `{"answers":{}}`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got = seen{}
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want || rec.Body.String() != `{"from":"upstream"}` || rec.Header().Get("Cache-Control") != "public, max-age=300" {
				t.Fatalf("response %d %q %q", rec.Code, rec.Body, rec.Header().Get("Cache-Control"))
			}
			if got != tc.upstream {
				t.Fatalf("upstream saw %+v, want %+v", got, tc.upstream)
			}
		})
	}
}

// Nothing reaches a service before sign-in, and a wrong method is 405
// before the token check.
func TestRoutesRefuseBeforeForwarding(t *testing.T) {
	a, _ := testAuthenticator(t)
	var got seen
	api, surveys := fakeUpstream(t, &got)
	h := makeRouter(a, api, surveys)
	for _, tc := range []struct {
		method, path, token string
		want                int
	}{
		{"POST", "/api/v1/surveys/q4-2026/responses", "", 401},
		{"POST", "/api/v1/surveys/q4-2026/responses", "outsider-token", 403},
		{"GET", "/api/v1/usage", "", 401},
		{"POST", "/api/v1/usage", "", 405},
		{"GET", "/api/v1/surveys/q4-2026/responses", "good-token", 405},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want || got != (seen{}) {
			t.Fatalf("%s %s: %d, upstream saw %+v; want %d and nothing", tc.method, tc.path, rec.Code, got, tc.want)
		}
	}
}

// A 401 from the service is copilot-cli's problem, not the caller's: it
// becomes 502, so nav-pilot keeps the answer and retries. An oversized body
// is 413, not a retryable 502.
func TestForwardErrorMapping(t *testing.T) {
	a, _ := testAuthenticator(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"access_token":"m2m","expires_in":3600}`))
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	texas := newTexasClient(srv.URL+"/token", "api://x")
	h := makeRouter(a, newUpstream("copilot-api", srv.URL, texas), newUpstream("copilot-survey", srv.URL, texas))

	for _, tc := range []struct {
		body string
		want int
	}{
		{`{"answers":{}}`, 502},
		{`{"answers":"` + strings.Repeat("a", 40<<10) + `"}`, 413},
	} {
		req := httptest.NewRequest("POST", "/api/v1/surveys/q4-2026/responses", strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer good-token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("body of %d bytes: %d, want %d", len(tc.body), rec.Code, tc.want)
		}
	}
}

// A downstream redirect comes back as it is (no second request), and a
// missing Content-Type stays missing.
func TestForwardPassesRedirectAsIs(t *testing.T) {
	a, _ := testAuthenticator(t)
	followed := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elsewhere" {
			followed = true
		}
		w.Header().Set("Location", "/elsewhere")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	texas := newTexasClient(srv.URL+"/token", "api://x")
	h := makeRouter(a, newUpstream("copilot-api", srv.URL, texas), newUpstream("copilot-survey", srv.URL, texas))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/surveys/active", nil))
	if rec.Code != http.StatusFound || followed || rec.Header().Get("Content-Type") != "" {
		t.Fatalf("got %d, followed=%v, Content-Type %q; want 302, no follow, none", rec.Code, followed, rec.Header().Get("Content-Type"))
	}
}
