package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func setBaseValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BASE_URL", "https://fleet-gateway.intern.dev.nav.no")
	t.Setenv("ALLOWED_CLIENT_IDS", "https://my-copilot.ekstern.dev.nav.no/oauth-client.json")
	t.Setenv("ALLOWED_REDIRECT_URIS", "http://127.0.0.1:38249/callback")
	t.Setenv("ALLOWED_RESOURCES", "https://fleet-gateway.intern.dev.nav.no/mcp")
	t.Setenv("AZURE_APP_CLIENT_ID", "entra-client-id")
	t.Setenv("AZURE_APP_CLIENT_SECRET", "entra-client-secret")
	t.Setenv("AZURE_OPENID_CONFIG_TOKEN_ENDPOINT", "https://login.microsoftonline.com/nav.no/oauth2/v2.0/token")
	t.Setenv("AZURE_APP_WELL_KNOWN_URL", "https://login.microsoftonline.com/nav.no/v2.0/.well-known/openid-configuration")
	t.Setenv("AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT", "")
	t.Setenv("COOKIE_SIGNING_SECRET", "")
}

func TestLoadConfig_FailClosed(t *testing.T) {
	tests := []struct {
		name      string
		override  map[string]string
		wantError string
	}{
		{
			name:      "valid config succeeds",
			override:  map[string]string{},
			wantError: "",
		},
		{
			name:      "empty client allowlist fails closed",
			override:  map[string]string{"ALLOWED_CLIENT_IDS": ""},
			wantError: "ALLOWED_CLIENT_IDS must not be empty",
		},
		{
			name:      "whitespace-only client allowlist fails closed",
			override:  map[string]string{"ALLOWED_CLIENT_IDS": " , , "},
			wantError: "ALLOWED_CLIENT_IDS must not be empty",
		},
		{
			name:      "empty redirect list fails closed",
			override:  map[string]string{"ALLOWED_REDIRECT_URIS": ""},
			wantError: "ALLOWED_REDIRECT_URIS must not be empty",
		},
		{
			name:      "invalid client_id URL fails closed",
			override:  map[string]string{"ALLOWED_CLIENT_IDS": "ftp://example.com/client.json"},
			wantError: "invalid ALLOWED_CLIENT_IDS",
		},
		{
			name:      "redirect_uri with fragment fails closed",
			override:  map[string]string{"ALLOWED_REDIRECT_URIS": "http://127.0.0.1:38249/callback#frag"},
			wantError: "fragment not allowed",
		},
		{
			name:      "missing base URL fails closed",
			override:  map[string]string{"BASE_URL": ""},
			wantError: "invalid BASE_URL",
		},
		{
			name:      "missing azure client id fails closed",
			override:  map[string]string{"AZURE_APP_CLIENT_ID": ""},
			wantError: "AZURE_APP_CLIENT_ID is required",
		},
		{
			name:      "missing azure client secret fails closed",
			override:  map[string]string{"AZURE_APP_CLIENT_SECRET": ""},
			wantError: "AZURE_APP_CLIENT_SECRET is required",
		},
		{
			name:      "missing azure token endpoint fails closed",
			override:  map[string]string{"AZURE_OPENID_CONFIG_TOKEN_ENDPOINT": ""},
			wantError: "invalid AZURE_OPENID_CONFIG_TOKEN_ENDPOINT",
		},
		{
			name: "missing both well-known and authorize endpoint fails closed",
			override: map[string]string{
				"AZURE_APP_WELL_KNOWN_URL":                   "",
				"AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT": "",
			},
			wantError: "AZURE_APP_WELL_KNOWN_URL or AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT is required",
		},
		{
			name:      "short cookie secret fails closed",
			override:  map[string]string{"COOKIE_SIGNING_SECRET": "too-short"},
			wantError: "COOKIE_SIGNING_SECRET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBaseValidEnv(t)
			for k, v := range tt.override {
				t.Setenv(k, v)
			}

			cfg, err := loadConfigFromEnv()
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if cfg == nil {
					t.Fatal("expected non-nil config")
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestHardenedHTTPClient_RefusesRedirectsAndTimesOut(t *testing.T) {
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/other", http.StatusFound)
	}))
	defer redirectServer.Close()

	client := newHardenedHTTPClient(100 * time.Millisecond)
	resp1, err := client.Get(redirectServer.URL)
	if resp1 != nil {
		_ = resp1.Body.Close()
	}
	if err == nil {
		t.Fatal("expected redirect to be refused")
	}

	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	resp2, err := client.Get(slowServer.URL)
	if resp2 != nil {
		_ = resp2.Body.Close()
	}
	if err == nil {
		t.Fatal("expected slow request to time out")
	}
}

func TestTTLMap_CapExpiryAndSweep(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	m := newTTLMap[string](2, func() time.Time { return now })

	if !m.Set("k1", "v1", time.Minute) {
		t.Fatal("expected first insert to succeed")
	}
	if !m.Set("k2", "v2", time.Minute) {
		t.Fatal("expected second insert to succeed")
	}
	if m.Set("k3", "v3", time.Minute) {
		t.Fatal("expected third insert to fail at cap")
	}
	if !m.Set("k1", "v1-updated", time.Minute) {
		t.Fatal("expected updating existing key at cap to succeed")
	}

	now = now.Add(61 * time.Second)
	if !m.Set("k3", "v3", time.Minute) {
		t.Fatal("expected insert after TTL expiry to sweep and succeed")
	}
	if got, ok := m.Get("k2"); ok || got != "" {
		t.Fatalf("expected expired k2 to be absent, got %q", got)
	}
	if removed := m.Sweep(); removed != 0 {
		t.Fatalf("expected 0 remaining expired entries, swept %d", removed)
	}

	for i := 0; i < 2; i++ {
		m.Set(fmt.Sprintf("s%d", i), "x", 5*time.Second)
	}
	now = now.Add(61 * time.Second)
	if removed := m.Sweep(); removed != 2 {
		t.Fatalf("expected Sweep to remove 2 expired entries, got %d", removed)
	}
	if m.Len() != 0 {
		t.Fatalf("expected empty map after Sweep, got %d", m.Len())
	}
}
