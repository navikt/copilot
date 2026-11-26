package budget

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-api/internal/config"
)

// fakeBillingGitHub mints app installation tokens and records the
// Authorization header on every billing call. It replaces http.DefaultTransport
// for the test, so the app's own token request is routed here as well.
func fakeBillingGitHub(t *testing.T) *[]string {
	var mu sync.Mutex
	var auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/app/installations/") {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"token":"app-installation-token","expires_at":"` + time.Now().Add(time.Hour).Format(time.RFC3339) + `"}`))
			return
		}
		mu.Lock()
		auths = append(auths, r.Header.Get("Authorization"))
		mu.Unlock()
		_, _ = w.Write([]byte(`{"budgets":[],"has_next_page":false}`))
	}))
	t.Cleanup(srv.Close)
	orig := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme, r.URL.Host = "http", strings.TrimPrefix(srv.URL, "http://")
		return orig.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = orig })
	return &auths
}

func budgetAppConfig(t *testing.T) *config.Config {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return &config.Config{GitHubBillingToken: "classic-pat", BudgetAppID: "1", BudgetAppPrivateKey: string(pemKey), BudgetAppInstallationID: "2"}
}

func callBudgetReads(t *testing.T, tok func(context.Context) (string, error)) {
	t.Helper()
	c := NewBudgetClient(tok, "nav")
	if _, err := c.getEnterpriseBudgets(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, _ = c.getUserBudget(context.Background(), "someone")
	w := &budgetWriter{httpClient: &http.Client{}, baseURL: "https://api.github.com", enterprise: "nav", token: tok}
	if _, _, err := w.listUser(context.Background(), "someone"); err != nil {
		t.Fatal(err)
	}
}

func TestBillingReadsUseBudgetApp(t *testing.T) {
	auths := fakeBillingGitHub(t)
	tok, err := NewBillingTokenSource(budgetAppConfig(t))
	if err != nil || tok == nil {
		t.Fatalf("token source: %v", err)
	}
	callBudgetReads(t, tok)
	if len(*auths) != 3 {
		t.Fatalf("expected 3 billing calls, got %d", len(*auths))
	}
	for _, a := range *auths {
		if a != "Bearer app-installation-token" {
			t.Errorf("billing read not on app token: %q", a)
		}
	}
}

func TestBillingWithoutAppIsUnavailable(t *testing.T) {
	tok, err := NewBillingTokenSource(&config.Config{GitHubBillingToken: "classic-pat"})
	if err != nil || tok != nil {
		t.Fatalf("no app and no BILLING_PAT_FALLBACK must leave billing unavailable, got tok=%v err=%v", tok != nil, err)
	}
}

func TestBillingPATOnlyWithFlag(t *testing.T) {
	auths := fakeBillingGitHub(t)
	cfg := budgetAppConfig(t)
	cfg.BillingPATFallback = true
	tok, err := NewBillingTokenSource(cfg)
	if err != nil || tok == nil {
		t.Fatalf("token source: %v", err)
	}
	callBudgetReads(t, tok)
	for _, a := range *auths {
		if a != "Bearer classic-pat" {
			t.Errorf("BILLING_PAT_FALLBACK=true must use the PAT, got %q", a)
		}
	}
}

func TestBudgetWriteAPIVersions(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.Header.Get("X-GitHub-Api-Version"))
		_, _ = w.Write([]byte(`{"id":"b1","budgets":[]}`))
	}))
	defer srv.Close()
	w := &budgetWriter{httpClient: srv.Client(), baseURL: srv.URL, enterprise: "nav", token: staticToken("t")}
	_, _, _ = w.listUser(context.Background(), "someone")
	_, _, _ = w.createUserBudget(context.Background(), "someone", 10, time.Now())
	want := []string{"GET 2022-11-28", "POST 2026-03-10"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("API versions = %v, want %v", got, want)
	}
}
