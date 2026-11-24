package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const testGatewayClientID = "gw-client-id"

func newTestEntraAuth(t *testing.T, claims map[string]any) *EntraAuth {
	t.Helper()
	texas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["identity_provider"] != "entra_id" || body["token"] != "tok" {
			t.Errorf("unexpected introspection body")
		}
		_ = json.NewEncoder(w).Encode(claims)
	}))
	t.Cleanup(texas.Close)
	return &EntraAuth{introspectionEndpoint: texas.URL, gatewayClientID: testGatewayClientID, client: texas.Client()}
}

func TestEntraAuth(t *testing.T) {
	valid := func() map[string]any {
		return map[string]any{"active": true, "NAVident": "Z123456", "azp": testGatewayClientID, "preferred_username": "a@nav.no"}
	}
	cases := []struct {
		name   string
		mutate func(map[string]any)
		want   int
	}{
		{"valid user token", func(map[string]any) {}, http.StatusOK},
		{"inactive", func(c map[string]any) { c["active"] = false }, http.StatusUnauthorized},
		{"app token idtyp", func(c map[string]any) { c["idtyp"] = "app" }, http.StatusForbidden},
		{"app token role", func(c map[string]any) { c["roles"] = []string{"access_as_application"} }, http.StatusForbidden},
		{"missing NAVident", func(c map[string]any) { delete(c, "NAVident") }, http.StatusForbidden},
		{"wrong azp", func(c map[string]any) { c["azp"] = "other-app" }, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.mutate(c)
			var got *UserContext
			h := newTestEntraAuth(t, c).Authenticate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = GetUserFromContext(r.Context())
			}))
			req := httptest.NewRequest(http.MethodPost, "/mcp/gw", nil)
			req.Header.Set("Authorization", "Bearer tok")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if tc.want == http.StatusOK && (got == nil || got.NAVident != "Z123456" || got.GitHubAccessToken != "") {
				t.Fatalf("unexpected user context: %+v", got)
			}
		})
	}
}

func TestEntraAuth_MissingBearer(t *testing.T) {
	h := newTestEntraAuth(t, map[string]any{}).Authenticate(http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp/gw", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestGatewayClientID(t *testing.T) {
	raw := `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"dev-gw"},{"name":"prod-gcp:copilot:fleet-gateway","clientId":"prod-gw"},{"name":"dev-gcp:other:fleet-gateway","clientId":"x"}]`
	for cluster, want := range map[string]string{"dev-gcp": "dev-gw", "prod-gcp": "prod-gw", "": ""} {
		if id, err := appClientID(raw, cluster, "fleet-gateway"); err != nil || id != want {
			t.Fatalf("cluster %q: got %q, %v; want %q", cluster, id, err, want)
		}
	}
	devOnly := `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"dev-gw"}]`
	if id, _ := appClientID(devOnly, "prod-gcp", "fleet-gateway"); id != "" {
		t.Fatalf("dev gateway accepted in prod: %q", id)
	}
	if id, _ := appClientID(`[{"name":"dev-gcp:other:fleet-gateway","clientId":"x"}]`, "dev-gcp", "fleet-gateway"); id != "" {
		t.Fatalf("foreign namespace accepted: %q", id)
	}
	if _, err := appClientID("{", "dev-gcp", "fleet-gateway"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestEntraMode_GitHubToolsRefused(t *testing.T) {
	h := newSimpleMCPHandler()
	user := &UserContext{Login: "a@nav.no", NAVident: "Z123456"}
	for _, tool := range []string{"check_agent_readiness", "suggest_customizations", "generate_agents_md", "generate_setup_steps", "team_readiness", "some_future_tool"} {
		resp := callTool(h, tool, map[string]interface{}{"owner": "navikt", "repo": "copilot", "org": "navikt", "team": "x"}, user)
		result, ok := resp.Result.(CallToolResult)
		if !ok || !result.IsError || !strings.Contains(result.Content[0].Text, "krever GitHub-innlogging") {
			t.Fatalf("%s: expected GitHub login error, got %+v", tool, resp)
		}
	}
	resp := callTool(h, "whoami", nil, user)
	if result, ok := resp.Result.(CallToolResult); !ok || !strings.Contains(result.Content[0].Text, "Z123456") {
		t.Fatalf("whoami: %+v", resp)
	}
}

func TestGitHubLoginRequired_UsesBaseURL(t *testing.T) {
	const prod = "https://mcp-onboarding.intern.nav.no"
	for base, want := range map[string]string{
		prod: prod + "/mcp.",
		"https://mcp-onboarding.intern.dev.nav.no/": "https://mcp-onboarding.intern.dev.nav.no/mcp.",
	} {
		h := newSimpleMCPHandler()
		h.baseURL = base
		resp := callTool(h, "team_readiness", nil, &UserContext{Login: "Z123456", NAVident: "Z123456"})
		text := resp.Result.(CallToolResult).Content[0].Text
		if !strings.HasSuffix(text, want) {
			t.Fatalf("base %q: got %q, want suffix %q", base, text, want)
		}
		if base == prod && strings.Contains(text, ".dev.") {
			t.Fatalf("prod message names dev host: %q", text)
		}
	}
}

func TestProdManifest_NotDebug(t *testing.T) {
	b, err := os.ReadFile(".nais/prod-gcp.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\nlog_level: INFO\n") {
		t.Fatal("prod log_level must be INFO")
	}
}

func TestEntraAuth_Challenge(t *testing.T) {
	for name, c := range map[string]struct{ auth, want string }{
		"missing":  {"", `Bearer realm="mcp-onboarding"`},
		"basic":    {"Basic abc", `Bearer realm="mcp-onboarding"`},
		"inactive": {"Bearer tok", `Bearer error="invalid_token"`},
	} {
		t.Run(name, func(t *testing.T) {
			h := newTestEntraAuth(t, map[string]any{"active": false}).Authenticate(http.NotFoundHandler())
			req := httptest.NewRequest(http.MethodPost, "/mcp/gw", nil)
			if c.auth != "" {
				req.Header.Set("Authorization", c.auth)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") != c.want {
				t.Fatalf("status=%d challenge=%q, want 401 %q", rec.Code, rec.Header().Get("WWW-Authenticate"), c.want)
			}
		})
	}
}

// Temporary pilot (#2860): only a config with ENTRA_PILOT_APP accepts the pilot's azp.
func TestEntraPilotAzp_DevOnly(t *testing.T) {
	texas := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "NAVident": "Z123456", "azp": "pilot-id"})
	}))
	t.Cleanup(texas.Close)
	t.Setenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", texas.URL)
	t.Setenv("AZURE_APP_PRE_AUTHORIZED_APPS", `[{"name":"c:copilot:fleet-gateway","clientId":"gw"},{"name":"c:copilot:fleet-agentgateway-pilot","clientId":"pilot-id"}]`)
	t.Setenv("NAIS_CLUSTER_NAME", "c")
	for pilotEnv, wantOK := range map[string]bool{"": false, "fleet-agentgateway-pilot": true} {
		t.Setenv("ENTRA_PILOT_APP", pilotEnv)
		auth, err := NewEntraAuthFromEnv()
		if err != nil || auth == nil {
			t.Fatalf("setup: %v", err)
		}
		_, err = auth.validate(t.Context(), "tok")
		if (err == nil) != wantOK {
			t.Fatalf("ENTRA_PILOT_APP=%q: err=%v, want ok=%v", pilotEnv, err, wantOK)
		}
	}
	prod, _ := os.ReadFile(".nais/prod-gcp.yaml")
	dev, _ := os.ReadFile(".nais/dev-gcp.yaml")
	if strings.Contains(string(prod), "agentgateway_pilot") || !strings.Contains(string(dev), "\nagentgateway_pilot: true\n") {
		t.Fatal("pilot must be enabled in dev and absent from prod")
	}
}

func TestEntraMode_ToolsListHidesGitHubTools(t *testing.T) {
	h := newSimpleMCPHandler()
	list := func(u *UserContext) map[string]bool {
		resp := h.processRequest(&JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "tools/list"}, u)
		names := map[string]bool{}
		for _, tool := range resp.Result.(ListToolsResult).Tools {
			names[tool.Name] = true
		}
		return names
	}
	gw := list(&UserContext{Login: "Z123456", NAVident: "Z123456"})
	direct := list(&UserContext{Login: "testuser", GitHubAccessToken: "fake-token"})
	for name := range gw {
		if !entraTools[name] {
			t.Errorf("Entra mode lists %s, which needs a GitHub token", name)
		}
	}
	for name := range entraTools {
		if !gw[name] {
			t.Errorf("Entra mode does not list %s", name)
		}
	}
	for _, name := range []string{"check_agent_readiness", "suggest_customizations", "generate_agents_md", "generate_setup_steps", "team_readiness"} {
		if gw[name] || !direct[name] {
			t.Errorf("%s: gateway=%v direct=%v, want false/true", name, gw[name], direct[name])
		}
	}
}
