package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestSAMLNameIDHandler(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	survey := &User{AZP: "survey-id", Idtyp: "app"}
	lookup := func(_ context.Context, login string) (string, error) {
		switch login {
		case "hans":
			return "hans.test@nav.no", nil
		case "nosso":
			return "", errNoSAMLIdentity
		}
		return "", errors.New("GitHub API returned 502")
	}

	for _, tc := range []struct {
		name     string
		clientID string
		user     *User
		lookup   func(context.Context, string) (string, error)
		body     string
		want     int
	}{
		{"copilot-survey", "survey-id", survey, lookup, `{"login":"hans"}`, 200},
		{"no SAML identity", "survey-id", survey, lookup, `{"login":"nosso"}`, 404},
		{"GitHub fails", "survey-id", survey, lookup, `{"login":"broken"}`, 503},
		{"GitHub not configured", "survey-id", survey, nil, `{"login":"hans"}`, 503},
		{"another app", "survey-id", &User{AZP: "copilot-cli-id", Idtyp: "app"}, lookup, `{"login":"hans"}`, 403},
		{"a user token via copilot-survey", "survey-id", &User{AZP: "survey-id", NAVident: "Z123456", Email: "ola@nav.no"}, lookup, `{"login":"hans"}`, 403},
		{"copilot-survey not pre-authorized", "", &User{Idtyp: "app"}, lookup, `{"login":"hans"}`, 403},
		{"no user", "survey-id", nil, lookup, `{"login":"hans"}`, 403},
		{"malformed login", "survey-id", survey, lookup, `{"login":"inv@lid"}`, 400},
		{"unknown field", "survey-id", survey, lookup, `{"login":"hans","x":1}`, 400},
		{"trailing garbage", "survey-id", survey, lookup, `{"login":"hans"} garbage`, 400},
		{"two objects", "survey-id", survey, lookup, `{"login":"hans"}{"login":"hans"}`, 400},
		{"oversized body", "survey-id", survey, lookup, `{"login":"hans"` + strings.Repeat(" ", 1100) + `}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/internal/v1/saml/name-id", strings.NewReader(tc.body))
			if tc.user != nil {
				req = req.WithContext(context.WithValue(req.Context(), userContextKey, tc.user))
			}
			rec := httptest.NewRecorder()
			samlNameIDHandler(tc.clientID, tc.lookup)(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
			if tc.want == 200 && !strings.Contains(rec.Body.String(), `"name_id":"hans.test@nav.no"`) {
				t.Fatalf("body = %s", rec.Body)
			}
		})
	}
	if strings.Contains(buf.String(), "broken") || strings.Contains(buf.String(), "hans") {
		t.Fatalf("a login reached the log: %s", buf.String())
	}
}

// The route sits on the root mux behind the token check, and nowhere under
// /api/v1/.
func TestRegisterInternalRoutes(t *testing.T) {
	mux := http.NewServeMux()
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := extractBearerToken(r); err != nil {
				respondError(w, "unauthorized", err.Error(), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	registerInternalRoutes(mux, auth, "survey-id", nil, "cli-id", nil)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/internal/v1/saml/name-id", 401},
		{http.MethodGet, "/internal/v1/saml/name-id", 405},
		{http.MethodPost, "/api/v1/internal/v1/saml/name-id", 404},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}
}

// rewrite sends every request to srv, whatever its host.
type rewrite struct{ srv *httptest.Server }

func (rw rewrite) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL.Scheme, r.URL.Host = "http", strings.TrimPrefix(rw.srv.URL, "http://")
	return http.DefaultTransport.RoundTrip(r)
}

func TestGetSamlNameIDByLogin(t *testing.T) {
	var answer string
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" || r.Header.Get("Authorization") != "Bearer inst" {
			w.WriteHeader(http.StatusTeapot)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	defer srv.Close()
	g := &GitHubClient{httpClient: &http.Client{Transport: rewrite{srv}}, org: "navikt",
		token: "inst", tokenExpiry: time.Now().Add(time.Hour)}

	node := func(login, nameID string) string {
		return `{"data":{"organization":{"samlIdentityProvider":{"externalIdentities":{"nodes":[{"samlIdentity":{"nameId":"` + nameID + `"},"user":{"login":"` + login + `"}}]}}}}}`
	}
	for _, tc := range []struct {
		name, answer string
		status       int
		want         string
		wantErr      error
		anyErr       bool
	}{
		{"found, login case differs", node("Hans", "hans.test@nav.no"), 200, "hans.test@nav.no", nil, false},
		{"another user's identity", node("someone", "x@nav.no"), 200, "", errNoSAMLIdentity, false},
		{"no nodes", `{"data":{"organization":{"samlIdentityProvider":{"externalIdentities":{"nodes":[]}}}}}`, 200, "", errNoSAMLIdentity, false},
		{"no SAML access", `{"data":{"organization":{"samlIdentityProvider":null}}}`, 200, "", nil, true},
		{"GraphQL errors", `{"errors":[{"message":"x"}]}`, 200, "", nil, true},
		{"not 200", ``, 502, "", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answer, status = tc.answer, tc.status
			got, err := g.getSamlNameIDByLogin(t.Context(), "hans")
			switch {
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr),
				tc.anyErr && (err == nil || errors.Is(err, errNoSAMLIdentity)),
				!tc.anyErr && tc.wantErr == nil && (err != nil || got != tc.want):
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

// copilot-survey's tokens pass only on the name-id route; Nais inbound access
// is pod-wide. Nothing on that route logs the caller's identity.
func TestBearerAuthSurveyScope(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	users := map[string]*User{
		"survey": {AZP: "survey-id", Idtyp: "app"},
		"user":   {AZP: "survey-id", NAVident: "Z999999"},
		"other":  {AZP: "my-copilot-id", NAVident: "Z123456"},
		// A second pre-authorized app named copilot-survey is fenced too.
		"survey2": {AZP: "survey-id-2", Idtyp: "app"},
	}
	validate := func(tok string) (*User, error) { return users[tok], nil }
	h := bearerAuth(validate, []string{"survey-id", "survey-id-2"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	for _, tc := range []struct {
		token, path string
		want        int
	}{
		{"survey", samlNameIDPath, 200},
		{"survey", "/api/v1/copilot/usage/metrics", 403},
		{"survey", "/api/v1/budget", 403},
		{"survey2", "/api/v1/budget", 403},
		{"user", samlNameIDPath, 200},
		{"other", "/api/v1/copilot/usage/metrics", 200},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s on %s = %d, want %d", tc.token, tc.path, rec.Code, tc.want)
		}
	}
	if strings.Contains(buf.String(), "Z999999") {
		t.Fatalf("an identity on the name-id route reached the log: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "Z123456") {
		t.Fatalf("other routes should still log at debug: %s", buf.String())
	}
}

// The survey fence fails closed: an ambiguous name yields every match.
func TestClientIDsForApp(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want []string
	}{
		{"", nil},
		{`[{"name":"dev-gcp:copilot:my-copilot","clientId":"a"}]`, nil},
		{`[{"name":"dev-gcp:copilot:copilot-survey","clientId":"a"}]`, []string{"a"}},
		{`[{"name":"dev-gcp:copilot:copilot-survey","clientId":"a"},{"name":"dev-gcp:other:copilot-survey","clientId":"b"}]`, []string{"a", "b"}},
	} {
		got, err := clientIDsForApp(tc.raw, "copilot-survey")
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("clientIDsForApp(%s) = %v, %v, want %v", tc.raw, got, err, tc.want)
		}
	}
}

// A copilot-survey token through the real mux: only POST on the exact
// name-id path reaches a handler.
func TestSurveyTokenThroughMux(t *testing.T) {
	validate := func(string) (*User, error) { return &User{AZP: "survey-id", Idtyp: "app"}, nil }
	auth := bearerAuth(validate, []string{"survey-id"})
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", auth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	registerInternalRoutes(mux, auth, "survey-id", nil, "cli-id", nil)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, samlNameIDPath, 503}, // reached the handler; GitHub not configured
		{http.MethodGet, samlNameIDPath, 405},
		{http.MethodPost, samlNameIDPath + "/", 404},
		{http.MethodGet, "/api/v1/x", 403},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"login":"hans"}`))
		req.Header.Set("Authorization", "Bearer t")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
	}
}

func TestOrgMembershipHandler(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cli := &User{AZP: "cli-id", Idtyp: "app"}
	check := func(_ context.Context, login string) (bool, error) {
		switch login {
		case "hans":
			return true, nil
		case "outsider":
			return false, nil
		}
		return false, errors.New("GitHub API returned 502")
	}
	for _, tc := range []struct {
		name     string
		clientID string
		user     *User
		check    func(context.Context, string) (bool, error)
		body     string
		want     int
		wantBody string
	}{
		{"member", "cli-id", cli, check, `{"login":"hans"}`, 200, `{"active":true}`},
		{"not a member", "cli-id", cli, check, `{"login":"outsider"}`, 200, `{"active":false}`},
		{"GitHub fails", "cli-id", cli, check, `{"login":"broken"}`, 503, ""},
		{"GitHub not configured", "cli-id", cli, nil, `{"login":"hans"}`, 503, ""},
		{"copilot-survey", "cli-id", &User{AZP: "survey-id", Idtyp: "app"}, check, `{"login":"hans"}`, 403, ""},
		{"a user token via copilot-cli", "cli-id", &User{AZP: "cli-id", NAVident: "Z123456", Email: "ola@nav.no"}, check, `{"login":"hans"}`, 403, ""},
		{"copilot-cli not pre-authorized", "", &User{Idtyp: "app"}, check, `{"login":"hans"}`, 403, ""},
		{"no user", "cli-id", nil, check, `{"login":"hans"}`, 403, ""},
		{"malformed login", "cli-id", cli, check, `{"login":"inv@lid"}`, 400, ""},
		{"unknown field", "cli-id", cli, check, `{"login":"hans","org":"x"}`, 400, ""},
		{"two objects", "cli-id", cli, check, `{"login":"hans"}{"login":"hans"}`, 400, ""},
		{"oversized body", "cli-id", cli, check, `{"login":"hans"` + strings.Repeat(" ", 1100) + `}`, 400, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, orgMembershipPath, strings.NewReader(tc.body))
			if tc.user != nil {
				req = req.WithContext(context.WithValue(req.Context(), userContextKey, tc.user))
			}
			rec := httptest.NewRecorder()
			orgMembershipHandler(tc.clientID, tc.check)(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body)
			}
			if tc.wantBody != "" && strings.TrimSpace(rec.Body.String()) != tc.wantBody {
				t.Fatalf("body = %s, want %s", rec.Body, tc.wantBody)
			}
		})
	}
	if strings.Contains(buf.String(), "broken") || strings.Contains(buf.String(), "hans") {
		t.Fatalf("a login reached the log: %s", buf.String())
	}
}

// copilot-survey's token is fenced off the membership route before the
// handler; copilot-cli's reaches it.
func TestOrgMembershipThroughMux(t *testing.T) {
	for _, tc := range []struct {
		azp  string
		want int
	}{{"cli-id", 200}, {"survey-id", 403}, {"my-copilot-id", 403}} {
		validate := func(string) (*User, error) { return &User{AZP: tc.azp, Idtyp: "app"}, nil }
		mux := http.NewServeMux()
		registerInternalRoutes(mux, bearerAuth(validate, []string{"survey-id"}), "survey-id", nil, "cli-id",
			func(context.Context, string) (bool, error) { return true, nil })
		req := httptest.NewRequest(http.MethodPost, orgMembershipPath, strings.NewReader(`{"login":"hans"}`))
		req.Header.Set("Authorization", "Bearer t")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("azp %s: status = %d, want %d", tc.azp, rec.Code, tc.want)
		}
	}
}

func TestIsActiveOrgMember(t *testing.T) {
	var calls int
	answers := map[string]struct {
		status int
		body   string
	}{
		"hans":    {200, `{"state":"active","role":"member","user":{"login":"Hans"}}`},
		"invited": {200, `{"state":"pending","role":"member","user":{"login":"invited"}}`},
		"swapped": {200, `{"state":"active","role":"member","user":{"login":"someone"}}`},
		"gone":    {404, `{"message":"Not Found"}`},
		"moved":   {302, ``},
		"down":    {502, ``},
		"garbage": {200, `not json`},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		login := strings.TrimPrefix(r.URL.Path, "/orgs/navikt/memberships/")
		a, ok := answers[login]
		if !ok || r.Header.Get("Authorization") != "Bearer inst" {
			w.WriteHeader(http.StatusTeapot)
			return
		}
		if a.status == 302 {
			w.Header().Set("Location", "/elsewhere")
		}
		w.WriteHeader(a.status)
		_, _ = w.Write([]byte(a.body))
	}))
	defer srv.Close()
	cache := NewCache(time.Minute)
	defer cache.Stop()
	g := &GitHubClient{httpClient: &http.Client{Transport: rewrite{srv}}, org: "navikt",
		token: "inst", tokenExpiry: time.Now().Add(time.Hour), memberCache: cache}

	for _, tc := range []struct {
		login  string
		want   bool
		anyErr bool
	}{
		{"hans", true, false},
		{"invited", false, false},
		{"swapped", false, false},
		{"gone", false, false},
		{"moved", false, true},
		{"down", false, true},
		{"garbage", false, true},
	} {
		got, err := g.isActiveOrgMember(t.Context(), tc.login)
		if got != tc.want || (err != nil) != tc.anyErr {
			t.Errorf("%s: got %v, %v; want %v, err=%v", tc.login, got, err, tc.want, tc.anyErr)
		}
	}
	before := calls
	if ok, _ := g.isActiveOrgMember(t.Context(), "HANS"); !ok || calls != before {
		t.Fatalf("cached answer not reused: ok=%v calls=%d->%d", ok, before, calls)
	}
	if _, err := g.isActiveOrgMember(t.Context(), "down"); err == nil || calls != before+1 {
		t.Fatal("an error was cached")
	}
}
