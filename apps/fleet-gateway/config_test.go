package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setBaseValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("BASE_URL", "https://fleet-gateway.intern.dev.nav.no")
	t.Setenv("ALLOWED_CLIENT_IDS", "https://my-copilot.ekstern.dev.nav.no/oauth-client.json")
	t.Setenv("ALLOWED_REDIRECT_URIS", "http://127.0.0.1:38249/callback")
	t.Setenv("AZURE_APP_CLIENT_ID", "entra-client-id")
	t.Setenv("AZURE_APP_CLIENT_SECRET", "entra-client-secret")
	t.Setenv("AZURE_OPENID_CONFIG_TOKEN_ENDPOINT", "https://login.microsoftonline.com/nav.no/oauth2/v2.0/token")
	t.Setenv("AZURE_APP_WELL_KNOWN_URL", "https://login.microsoftonline.com/nav.no/v2.0/.well-known/openid-configuration")
	t.Setenv("AZURE_OPENID_CONFIG_AUTHORIZATION_ENDPOINT", "")
	t.Setenv("COOKIE_SIGNING_SECRET", "")
	t.Setenv("NAIS_CLUSTER_NAME", "")
	t.Setenv("NAIS_TOKEN_EXCHANGE_ENDPOINT", "http://127.0.0.1:3000/api/v1/token/exchange")
	t.Setenv("TEAM_TOKEN_STRATEGY", "")
	t.Setenv("PILOT_ROUTES_FILE", "")
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
		{
			name:      "missing texas endpoint fails closed for default obo strategy",
			override:  map[string]string{"NAIS_TOKEN_EXCHANGE_ENDPOINT": ""},
			wantError: "invalid NAIS_TOKEN_EXCHANGE_ENDPOINT",
		},
		{
			name:      "invalid team token strategy fails closed",
			override:  map[string]string{"TEAM_TOKEN_STRATEGY": "magic"},
			wantError: "invalid TEAM_TOKEN_STRATEGY",
		},
		{
			name: "refresh strategy does not require texas endpoint",
			override: map[string]string{
				"TEAM_TOKEN_STRATEGY":          "refresh",
				"NAIS_TOKEN_EXCHANGE_ENDPOINT": "",
			},
			wantError: "",
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

func TestPilotRoutesValidation(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantError string
	}{
		{
			name: "valid YAML with internal http and https routes",
			content: `version: 1
routes:
  - route: /mcp/etterlevelse
    team: team-etterlevelse
    target: dev-gcp:team-etterlevelse:nav-etterlevelse-mcp
    upstream_url: http://nav-etterlevelse-mcp.team-etterlevelse
  - route: /mcp/docs
    team: copilot
    target: dev-gcp:copilot:docs-mcp
    upstream_url: https://docs-mcp.intern.dev.nav.no
    required_group: 11111111-2222-3333-4444-555555555555
`,
		},
		{
			name:      "valid JSON format succeeds",
			content:   `{"version":1,"routes":[{"route":"/mcp/json-app","team":"copilot","target":"dev-gcp:copilot:json-mcp","upstream_url":"http://json-mcp.copilot.svc.cluster.local"}]}`,
			wantError: "",
		},
		{
			name:      "empty routes fails closed",
			content:   "version: 1\nroutes: []\n",
			wantError: "must not be empty",
		},
		{
			name:      "unsupported version fails closed",
			content:   "version: 2\nroutes:\n  - route: /mcp/a\n    team: t\n    target: c:n:a\n    upstream_url: http://a.n\n",
			wantError: "unsupported pilot routes version",
		},
		{
			name: "duplicate routes fail closed",
			content: `version: 1
routes:
  - route: /mcp/dup
    team: t1
    target: dev-gcp:ns1:app1
    upstream_url: http://app1.ns1
  - route: /mcp/dup
    team: t2
    target: dev-gcp:ns2:app2
    upstream_url: http://app2.ns2
`,
			wantError: "duplicate route",
		},
		{
			name:      "invalid route prefix fails closed",
			content:   "version: 1\nroutes:\n  - route: /api/a\n    team: t\n    target: c:n:a\n    upstream_url: http://a.n\n",
			wantError: "must start with /mcp/",
		},
		{
			name:      "nested route path fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a/b\n    team: t\n    target: c:n:a\n    upstream_url: http://a.n\n",
			wantError: "single non-empty segment",
		},
		{
			name:      "invalid target format fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:only-two\n    upstream_url: http://a.only-two\n",
			wantError: "expected cluster:namespace:app",
		},
		{
			name:      "public domain upstream fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: https://example.com/mcp\n",
			wantError: "public or not recognized as internal",
		},
		{
			name:      "ekstern nav domain fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: https://app.ekstern.dev.nav.no\n",
			wantError: "must not be public",
		},
		{
			name:      "public IP upstream fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: https://8.8.8.8\n",
			wantError: "must not be a public IP",
		},
		{
			name:      "metadata IP upstream fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://169.254.169.254\n",
			wantError: "disallowed IP address",
		},
		{
			name:      "http intern nav host fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://app.intern.dev.nav.no\n",
			wantError: "must use https",
		},
		{
			name:      "cloud metadata internal DNS fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://metadata.google.internal\n",
			wantError: "disallowed internal host",
		},
		{
			name:      "loopback IPv4 fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://127.0.0.1:8080\n",
			wantError: "disallowed IP address",
		},
		{
			name:      "loopback IPv6 fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://[::1]:8080\n",
			wantError: "disallowed IP address",
		},
		{
			name:      "localhost fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://localhost:8080\n",
			wantError: "disallowed loopback host",
		},
		{
			name:      "link-local IPv6 fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://[fe80::1]\n",
			wantError: "disallowed IP address",
		},
		{
			name:      "private raw IP fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://10.0.0.1:8080\n",
			wantError: "raw IP addresses are not allowed",
		},
		{
			name:      "single-label host without namespace fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://app\n",
			wantError: "not recognized as internal",
		},
		{
			name:      "short svc suffix without cluster.local fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://app.ns.svc\n",
			wantError: "not recognized as internal",
		},
		{
			name:      "cross-namespace svc.cluster.local fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://app.other-ns.svc.cluster.local\n",
			wantError: "not recognized as internal",
		},
		{
			name:      "dot-local host fails closed",
			content:   "version: 1\nroutes:\n  - route: /mcp/a\n    team: t\n    target: dev-gcp:ns:app\n    upstream_url: http://app.local\n",
			wantError: "not recognized as internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "routes.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("write temp routes file: %v", err)
			}
			setBaseValidEnv(t)
			t.Setenv("PILOT_ROUTES_FILE", path)

			_, err := loadConfigFromEnv()
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
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

func TestDevPilotRoutesFileValid(t *testing.T) {
	setBaseValidEnv(t)
	t.Setenv("PILOT_ROUTES_FILE", "pilot-routes.dev.yaml")

	cfg, err := loadConfigFromEnv()
	if err != nil {
		t.Fatalf("expected pilot-routes.dev.yaml to load without error, got %v", err)
	}
	if len(cfg.Routes) != 1 {
		t.Fatalf("expected 1 dev route, got %d", len(cfg.Routes))
	}
	r := cfg.Routes[0]
	if r.Route != "/mcp/test" || r.Team != "copilot" || r.Target != "dev-gcp:copilot:fleet-echo" {
		t.Fatalf("unexpected dev route metadata: %+v", r)
	}
	if r.UpstreamURL != "http://fleet-echo.copilot/" {
		t.Fatalf("unexpected dev upstream URL: %q", r.UpstreamURL)
	}
}

func TestDefaultEntraScopeTargetsGateway(t *testing.T) {
	if got := defaultEntraScope("obo", "", "", "gw-client"); got != "openid api://gw-client/.default" {
		t.Fatalf("obo scope without RT_KEYS %q", got)
	}
	if got := defaultEntraScope("obo", "", "k1:secret", "gw-client"); got != "openid offline_access api://gw-client/.default" {
		t.Fatalf("obo scope with RT_KEYS and unset RT_STORAGE %q", got)
	}
	if got := defaultEntraScope("obo", rtStorageClientEncrypted, "k1:secret", "gw-client"); got != "openid offline_access api://gw-client/.default" {
		t.Fatalf("obo scope with client-encrypted %q", got)
	}
	if got := defaultEntraScope("obo", rtStorageServer, "k1:secret", "gw-client"); got != "openid api://gw-client/.default" {
		t.Fatalf("obo scope with server %q", got)
	}
	if got := defaultEntraScope(strategyRefresh, rtStorageServer, "k1:secret", "gw-client"); got != "openid offline_access api://gw-client/.default" {
		t.Fatalf("refresh scope %q", got)
	}
}
