package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAuthorize_LoopbackRedirectAnyPort(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	for _, tc := range []struct {
		redirect   string
		wantStatus int
	}{
		{"http://127.0.0.1:51234/callback", http.StatusFound}, // validated, then off to Entra
		{"http://localhost:38249/callback", http.StatusBadRequest},
		{"http://127.0.0.1:51234/other", http.StatusBadRequest},
		{"http://evil.example:38249/callback", http.StatusBadRequest},
	} {
		t.Run(tc.redirect, func(t *testing.T) {
			te := newTestEnv(t)
			req := httptest.NewRequest(http.MethodGet, te.authorizeURL(map[string]string{"redirect_uri": tc.redirect}, verifier), nil)
			rec := httptest.NewRecorder()
			te.handler.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status %d, want %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

// A loopback redirect on a port the client metadata never listed goes through
// /authorize, the Entra callback, consent and the token exchange.
func TestLoopbackRandomPort_EndToEnd(t *testing.T) {
	te := newTestEnv(t)
	redirect := "http://127.0.0.1:61987/callback" // registered: 127.0.0.1:38249
	req := httptest.NewRequest(http.MethodGet, te.authorizeURL(map[string]string{"redirect_uri": redirect}, consentVerifier), nil)

	id, csrf, cookie, h := te.parseConsentPage(te.callback(te.followToEntra(req))) //nolint:bodyclose // parseConsentPage closes it
	if csp := h.Get("Content-Security-Policy"); !strings.Contains(csp, "form-action 'self' http://127.0.0.1:61987;") {
		t.Fatalf("form-action must allow the requested port: %q", csp)
	}
	resp := te.postConsent(id, csrf, cookie, "approve")
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, redirect+"?") {
		t.Fatalf("redirected to %q, want %s", loc, redirect)
	}
	code := te.clientCode(resp)

	tokenReq := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {te.clientID},
		"redirect_uri":  {redirect},
		"resource":      {te.resource},
		"code_verifier": {consentVerifier},
	}
	if status, body := te.postToken(tokenReq); status != http.StatusOK || body["access_token"] == nil {
		t.Fatalf("token exchange: %d %v", status, body)
	}
}
