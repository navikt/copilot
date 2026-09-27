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
		{"oversized body", "survey-id", survey, lookup, `{"login":"` + strings.Repeat("a", 2000) + `"}`, 400},
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
