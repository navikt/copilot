package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestIsLoopbackRedirect_RejectsNonDigitPort(t *testing.T) {
	for _, rawHost := range []string{"127.0.0.1:+80", "[::1]:+80", "127.0.0.1:-80", "127.0.0.1:0"} {
		u := &url.URL{Scheme: "http", Host: rawHost, Path: "/callback"}
		if isLoopbackRedirect(u) {
			t.Errorf("isLoopbackRedirect(%q) = true, want false", rawHost)
		}
	}
	if !isLoopbackRedirect(&url.URL{Scheme: "http", Host: "127.0.0.1:8080", Path: "/callback"}) {
		t.Error("isLoopbackRedirect with valid numeric port = false, want true")
	}
}

func TestRedirectWithValues_PreservesRegisteredQuery(t *testing.T) {
	base, err := url.Parse("http://127.0.0.1:38249/callback?foo=bar&keep=1")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/authorize", nil)
	redirectWithValues(rec, req, base, url.Values{
		"code":  {"gw-code"},
		"state": {"st-1"},
		"iss":   {"https://gw.example"},
	})
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := loc.Query()
	if q.Get("foo") != "bar" || q.Get("keep") != "1" || q.Get("code") != "gw-code" || q.Get("state") != "st-1" || q.Get("iss") != "https://gw.example" {
		t.Fatalf("merged redirect query = %v", q)
	}
}
