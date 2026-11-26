package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
	"github.com/navikt/copilot/apps/copilot-api/internal/github"
)

func TestSAMLNameIDHandler(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	survey := &auth.User{AZP: "survey-id", Idtyp: "app"}
	lookup := func(_ context.Context, login string) (string, error) {
		switch login {
		case "hans":
			return "hans.test@nav.no", nil
		case "nosso":
			return "", github.ErrNoSAMLIdentity
		}
		return "", errors.New("GitHub API returned 502")
	}

	for _, tc := range []struct {
		name     string
		clientID string
		user     *auth.User
		lookup   func(context.Context, string) (string, error)
		body     string
		want     int
	}{
		{"copilot-survey", "survey-id", survey, lookup, `{"login":"hans"}`, 200},
		{"no SAML identity", "survey-id", survey, lookup, `{"login":"nosso"}`, 404},
		{"GitHub fails", "survey-id", survey, lookup, `{"login":"broken"}`, 503},
		{"GitHub not configured", "survey-id", survey, nil, `{"login":"hans"}`, 503},
		{"another app", "survey-id", &auth.User{AZP: "copilot-cli-id", Idtyp: "app"}, lookup, `{"login":"hans"}`, 403},
		{"a user token via copilot-survey", "survey-id", &auth.User{AZP: "survey-id", NAVident: "Z123456", Email: "ola@nav.no"}, lookup, `{"login":"hans"}`, 403},
		{"copilot-survey not pre-authorized", "", &auth.User{Idtyp: "app"}, lookup, `{"login":"hans"}`, 403},
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
				req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, tc.user))
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
			if _, err := auth.ExtractBearerToken(r); err != nil {
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

// A copilot-survey token through the real mux: only POST on the exact
// name-id path reaches a handler.
func TestSurveyTokenThroughMux(t *testing.T) {
	validate := func(string) (*auth.User, error) { return &auth.User{AZP: "survey-id", Idtyp: "app"}, nil }
	auth := auth.BearerAuth(validate, []string{"survey-id"})
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

	cli := &auth.User{AZP: "cli-id", Idtyp: "app"}
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
		user     *auth.User
		check    func(context.Context, string) (bool, error)
		body     string
		want     int
		wantBody string
	}{
		{"member", "cli-id", cli, check, `{"login":"hans"}`, 200, `{"active":true}`},
		{"not a member", "cli-id", cli, check, `{"login":"outsider"}`, 200, `{"active":false}`},
		{"GitHub fails", "cli-id", cli, check, `{"login":"broken"}`, 503, ""},
		{"GitHub not configured", "cli-id", cli, nil, `{"login":"hans"}`, 503, ""},
		{"copilot-survey", "cli-id", &auth.User{AZP: "survey-id", Idtyp: "app"}, check, `{"login":"hans"}`, 403, ""},
		{"a user token via copilot-cli", "cli-id", &auth.User{AZP: "cli-id", NAVident: "Z123456", Email: "ola@nav.no"}, check, `{"login":"hans"}`, 403, ""},
		{"copilot-cli not pre-authorized", "", &auth.User{Idtyp: "app"}, check, `{"login":"hans"}`, 403, ""},
		{"no user", "cli-id", nil, check, `{"login":"hans"}`, 403, ""},
		{"malformed login", "cli-id", cli, check, `{"login":"inv@lid"}`, 400, ""},
		{"unknown field", "cli-id", cli, check, `{"login":"hans","org":"x"}`, 400, ""},
		{"two objects", "cli-id", cli, check, `{"login":"hans"}{"login":"hans"}`, 400, ""},
		{"oversized body", "cli-id", cli, check, `{"login":"hans"` + strings.Repeat(" ", 1100) + `}`, 400, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, orgMembershipPath, strings.NewReader(tc.body))
			if tc.user != nil {
				req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, tc.user))
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
		validate := func(string) (*auth.User, error) { return &auth.User{AZP: tc.azp, Idtyp: "app"}, nil }
		mux := http.NewServeMux()
		registerInternalRoutes(mux, auth.BearerAuth(validate, []string{"survey-id"}), "survey-id", nil, "cli-id",
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
