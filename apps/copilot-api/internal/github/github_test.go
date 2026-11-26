package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"time"

	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"github.com/navikt/copilot/apps/copilot-api/internal/cache"
	"strings"
	"testing"
)

func TestNewGitHubClient_AcceptsLiteralNewlineEscapesInPrivateKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	privateKeyEscaped := strings.ReplaceAll(string(privateKeyPEM), "\n", `\n`)

	client, err := New("navikt", "12345", privateKeyEscaped, "67890")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client == nil || client.privateKey == nil {
		t.Fatal("expected initialized github client with parsed private key")
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
	g := &Client{httpClient: &http.Client{Transport: rewrite{srv}}, org: "navikt",
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
		{"another user's identity", node("someone", "x@nav.no"), 200, "", ErrNoSAMLIdentity, false},
		{"no nodes", `{"data":{"organization":{"samlIdentityProvider":{"externalIdentities":{"nodes":[]}}}}}`, 200, "", ErrNoSAMLIdentity, false},
		{"no SAML access", `{"data":{"organization":{"samlIdentityProvider":null}}}`, 200, "", nil, true},
		{"GraphQL errors", `{"errors":[{"message":"x"}]}`, 200, "", nil, true},
		{"not 200", ``, 502, "", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answer, status = tc.answer, tc.status
			got, err := g.GetSamlNameIDByLogin(t.Context(), "hans")
			switch {
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr),
				tc.anyErr && (err == nil || errors.Is(err, ErrNoSAMLIdentity)),
				!tc.anyErr && tc.wantErr == nil && (err != nil || got != tc.want):
				t.Fatalf("got %q, %v", got, err)
			}
		})
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
	cache := cache.New(time.Minute)
	defer cache.Stop()
	g := &Client{httpClient: &http.Client{Transport: rewrite{srv}}, org: "navikt",
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
		got, err := g.IsActiveOrgMember(t.Context(), tc.login)
		if got != tc.want || (err != nil) != tc.anyErr {
			t.Errorf("%s: got %v, %v; want %v, err=%v", tc.login, got, err, tc.want, tc.anyErr)
		}
	}
	before := calls
	if ok, _ := g.IsActiveOrgMember(t.Context(), "HANS"); !ok || calls != before {
		t.Fatalf("cached answer not reused: ok=%v calls=%d->%d", ok, before, calls)
	}
	if _, err := g.IsActiveOrgMember(t.Context(), "down"); err == nil || calls != before+1 {
		t.Fatal("an error was cached")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := g.IsActiveOrgMember(ctx, "secretlogin"); err == nil || strings.Contains(err.Error(), "secretlogin") {
		t.Fatalf("err = %v, want an error without the login", err)
	}
}

func TestGetRepositoryContributorsPinsOwner(t *testing.T) {
	g := &Client{org: "navikt"}
	if _, err := g.GetRepositoryContributors(context.Background(), "other-org", "repo", []string{"a"}); !errors.Is(err, ErrOwnerNotAllowed) {
		t.Fatalf("err = %v, want ErrOwnerNotAllowed", err)
	}
}
