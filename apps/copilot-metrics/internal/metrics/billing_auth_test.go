package metrics

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/config"
)

// fakeGitHub records the Authorization header of every request it receives.
type fakeGitHub struct {
	mu    sync.Mutex
	auths []string
	srv   *httptest.Server
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	f := &fakeGitHub{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.auths = append(f.auths, r.URL.Path+" "+r.Header.Get("Authorization"))
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// to routes api.github.com to the fake server; wrap simulates ghinstallation setting the app token.
type to struct {
	base *url.URL
	app  bool
}

func (r to) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host = r.base.Scheme, r.base.Host
	if r.app {
		req.Header.Set("Authorization", "token app-installation-token")
	}
	return http.DefaultTransport.RoundTrip(req)
}

func callAllBilling(t *testing.T, f *fakeGitHub, hc *http.Client, token string) {
	t.Helper()
	ctx := context.Background()
	month := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	b := NewBillingClient(hc, token, "nav")
	_, _ = b.FetchEnterpriseAICreditUsage(ctx, month)
	_, _ = b.FetchUserBillingUsage(ctx, "someone", month)
	_, _ = b.FetchMonthlyUsage(ctx, 2026, 10)
	_, _ = b.FetchDailyUsage(ctx, month)
	_, _ = b.FetchEnterpriseUsage(ctx, month)
	bu := NewBudgetClient(hc, token, "nav")
	_, _ = bu.FetchAllBudgets(ctx)
	_, _ = bu.FetchUserEffectiveBudget(ctx, "someone")
}

func TestBillingUsesAppNotPAT(t *testing.T) {
	f := newFakeGitHub(t)
	base, _ := url.Parse(f.srv.URL)
	cfg := &config.Config{GitHubBillingToken: "classic-pat"}
	hc, token := billingAuth(cfg, &http.Client{Transport: to{base, true}})
	// Point the PAT path at the fake server too, so a PAT leak would be recorded.
	if token != "" {
		hc.Transport = to{base, false}
	}
	callAllBilling(t, f, hc, token)
	if len(f.auths) < 7 {
		t.Fatalf("expected a request per billing call, got %d", len(f.auths))
	}
	for _, a := range f.auths {
		if !strings.HasSuffix(a, " token app-installation-token") || strings.Contains(a, "classic-pat") {
			t.Errorf("billing call not on app token: %q", a)
		}
	}
}

func TestBillingWithoutAppIsUnavailable(t *testing.T) {
	cfg := &config.Config{GitHubBillingToken: "classic-pat"}
	hc, token := billingAuth(cfg, nil)
	if NewBillingClient(hc, token, "nav") != nil || NewBudgetClient(hc, token, "nav") != nil || token != "" {
		t.Fatal("billing must be unavailable without the app and without BILLING_PAT_FALLBACK")
	}
	if billingClientFor(cfg) != nil {
		t.Fatal("billingClientFor must return nil without app config")
	}
}

func TestBillingPATOnlyWithFlag(t *testing.T) {
	f := newFakeGitHub(t)
	base, _ := url.Parse(f.srv.URL)
	cfg := &config.Config{GitHubBillingToken: "classic-pat", BillingPATFallback: true}
	hc, token := billingAuth(cfg, nil)
	if token != "classic-pat" || hc == nil {
		t.Fatal("BILLING_PAT_FALLBACK=true must use the PAT")
	}
	hc.Transport = to{base, false}
	callAllBilling(t, f, hc, token)
	for _, a := range f.auths {
		if !strings.HasSuffix(a, " Bearer classic-pat") {
			t.Errorf("expected PAT on fallback, got %q", a)
		}
	}
}

func TestBillingConfiguredWithoutPAT(t *testing.T) {
	app := &config.Config{GitHubAppID: 1, GitHubAppInstallationID: 2, GitHubAppPrivateKey: "k"}
	if !billingConfigured(app) {
		t.Error("user billing sync must start with the app and no PAT")
	}
	if billingConfigured(&config.Config{GitHubBillingToken: "classic-pat"}) {
		t.Error("PAT without BILLING_PAT_FALLBACK must not count as billing credentials")
	}
	if !billingConfigured(&config.Config{GitHubBillingToken: "classic-pat", BillingPATFallback: true}) {
		t.Error("PAT with BILLING_PAT_FALLBACK=true counts")
	}
}

// TestBillingClientForUsesInstallationToken crosses the real app path:
// billingClientFor, NewGitHubClient, ghinstallation and the token exchange.
func TestBillingClientForUsesInstallationToken(t *testing.T) {
	var mu sync.Mutex
	var exchanged bool
	var auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPost && r.URL.Path == "/app/installations/115303467/access_tokens" {
			exchanged = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"token":"ghs_installation","expires_at":"` + time.Now().Add(time.Hour).Format(time.RFC3339) + `"}`))
			return
		}
		auths = append(auths, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	base, _ := url.Parse(srv.URL)
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = base.Scheme, base.Host
		return orig.RoundTrip(r)
	})
	defer func() { http.DefaultTransport = orig }()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cfg := &config.Config{EnterpriseSlug: "nav", GitHubAppID: 3054179, GitHubAppInstallationID: 115303467, GitHubAppPrivateKey: string(pemKey), GitHubBillingToken: "classic-pat"}
	b := billingClientFor(cfg)
	if b == nil {
		t.Fatal("billingClientFor returned nil with app config")
	}
	_, _ = b.FetchMonthlyUsage(context.Background(), 2026, 10)
	if !exchanged || len(auths) == 0 {
		t.Fatalf("token exchange %v, billing calls %d", exchanged, len(auths))
	}
	for _, a := range auths {
		if a != "token ghs_installation" {
			t.Errorf("billing call not on installation token: %q", a)
		}
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
