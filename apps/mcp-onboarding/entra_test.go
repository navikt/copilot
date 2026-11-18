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
		if id, err := gatewayClientID(raw, cluster); err != nil || id != want {
			t.Fatalf("cluster %q: got %q, %v; want %q", cluster, id, err, want)
		}
	}
	devOnly := `[{"name":"dev-gcp:copilot:fleet-gateway","clientId":"dev-gw"}]`
	if id, _ := gatewayClientID(devOnly, "prod-gcp"); id != "" {
		t.Fatalf("dev gateway accepted in prod: %q", id)
	}
	if id, _ := gatewayClientID(`[{"name":"dev-gcp:other:fleet-gateway","clientId":"x"}]`, "dev-gcp"); id != "" {
		t.Fatalf("foreign namespace accepted: %q", id)
	}
	if _, err := gatewayClientID("{", "dev-gcp"); err == nil {
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
