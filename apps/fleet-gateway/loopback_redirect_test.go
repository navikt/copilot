package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedirectURIMatches(t *testing.T) {
	tests := []struct {
		registered, requested string
		want                  bool
	}{
		{"http://127.0.0.1/callback", "http://127.0.0.1:51234/callback", true},
		{"http://127.0.0.1:62471/callback", "http://127.0.0.1:40000/callback", true},
		{"http://127.0.0.1/", "http://127.0.0.1:51234/", true},
		{"http://[::1]/callback", "http://[::1]:51234/callback", true},
		{"https://app.example/cb", "https://app.example/cb", true},
		// Path, query and host family must still match.
		{"http://127.0.0.1/callback", "http://127.0.0.1:51234/evil", false},
		{"http://127.0.0.1/callback", "http://127.0.0.1:51234/callback?x=1", false},
		{"http://127.0.0.1/callback", "http://[::1]:51234/callback", false},
		// localhost names never get port flexibility (RFC 8252 section 8.3).
		{"http://localhost/callback", "http://localhost:51234/callback", false},
		{"http://127.0.0.1/callback", "http://localhost:51234/callback", false},
		// Non-loopback and non-http schemes stay exact.
		{"https://127.0.0.1/callback", "https://127.0.0.1:51234/callback", false},
		{"http://app.example/cb", "http://app.example:8080/cb", false},
		{"http://127.0.0.2/callback", "http://127.0.0.2:51234/callback", false},
		{"http://127.0.0.1/callback", "http://127.0.0.1.evil.example:51234/callback", false},
		{"http://127.0.0.1/callback", "http://user@127.0.0.1:51234/callback", false},
		{"http://127.0.0.1/callback", "http://127.0.0.1:0/callback", false},
		{"http://127.0.0.1/callback", "http://127.0.0.1:51234/callback#frag", false},
	}
	for _, tt := range tests {
		if got := redirectURIMatches(tt.registered, tt.requested); got != tt.want {
			t.Errorf("redirectURIMatches(%q, %q) = %v, want %v", tt.registered, tt.requested, got, tt.want)
		}
	}
}

func TestAuthorize_LoopbackRedirectAnyPort(t *testing.T) {
	verifier := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	for _, tc := range []struct {
		redirect   string
		wantStatus int
	}{
		{"http://127.0.0.1:51234/callback", http.StatusOK},
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
			if tc.wantStatus == http.StatusOK && !strings.Contains(rec.Body.String(), "cplt test client") {
				t.Fatalf("expected consent page, got %s", rec.Body.String())
			}
		})
	}
}
