package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func readSeedFiles(t *testing.T, env string) (routes, allowlist []byte) {
	t.Helper()
	routes, err := os.ReadFile("../fleet-gateway/pilot-routes." + env + ".yaml") //nolint:gosec // fixed repo paths
	if err != nil {
		t.Fatal(err)
	}
	allowlist, err = os.ReadFile("../mcp-registry/allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	return routes, allowlist
}

// TestSeedRealFiles seeds from the files in this repo, so a change there that
// the seed would reject fails here first.
func TestSeedRealFiles(t *testing.T) {
	for _, env := range []string{"dev", "prod"} {
		t.Run(env, func(t *testing.T) {
			routes, allowlist := readSeedFiles(t, env)
			servers, err := seedServers(routes, allowlist)
			if err != nil {
				t.Fatal(err)
			}
			byKey := map[string]Server{}
			for _, s := range servers {
				byKey[s.Key()] = s
			}

			var doc struct{ Servers []json.RawMessage }
			if err := json.Unmarshal(allowlist, &doc); err != nil {
				t.Fatal(err)
			}
			registry := 0
			for _, s := range servers {
				if s.Registry != nil {
					registry++
				}
			}
			if registry != len(doc.Servers) {
				t.Errorf("seeded %d registry entries, allowlist has %d", registry, len(doc.Servers))
			}

			onboarding := byKey["copilot/onboarding"]
			if onboarding.RegistryName != "io.github.navikt/mcp-onboarding" || onboarding.Route == nil {
				t.Fatalf("onboarding should join route and registry entry: %+v", onboarding)
			}
			if strings.Contains(string(onboarding.Registry), "https://mcp-onboarding.") {
				t.Error("direct onboarding remote must be dropped at seed")
			}
			if !strings.Contains(string(onboarding.Registry), gatewayRemotePrefix+"/mcp/copilot/onboarding") {
				t.Error("gateway remote must be kept")
			}
			if byKey["copilot/figma-mcp"].RegistryName != "com.figma/figma-mcp" {
				t.Error("non-gateway entries map to (copilot, last name segment)")
			}
			for _, s := range servers {
				if s.Status != "published" {
					t.Errorf("%s: seed marks everything published, got %q", s.Key(), s.Status)
				}
			}
		})
	}
	routes, allowlist := readSeedFiles(t, "dev")
	servers, _ := seedServers(routes, allowlist)
	for _, s := range servers {
		if s.Key() == "copilot/kodeverk" && s.Route.Enabled {
			t.Error("kodeverk is disabled in dev")
		}
	}
}

func TestSeedRejectsBadInput(t *testing.T) {
	route := func(body string) []byte { return []byte("version: 2\nroutes:\n" + body) }
	okRoute := route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: http://a.copilot/\n    tools: [x]\n")
	okList := []byte(`{"servers":[{"name":"io.github.navikt/b","remotes":[{"type":"streamable-http","url":"https://b.nav.no/mcp"}]}]}`)
	if _, err := seedServers(okRoute, okList); err != nil {
		t.Fatalf("baseline must pass: %v", err)
	}
	cases := map[string]struct{ routes, list []byte }{
		"public upstream":  {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: https://evil.example.com/\n    tools: [x]\n"), okList},
		"other namespace":  {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: http://a.other/\n    tools: [x]\n"), okList},
		"target mismatch":  {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:other:a\n    upstream_url: http://a.copilot/\n    tools: [x]\n"), okList},
		"no tools":         {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: http://a.copilot/\n"), okList},
		"pass all tools":   {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: http://a.copilot/\n    passAllTools: true\n"), okList},
		"bad tool":         {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: http://a.copilot/\n    tools: [\"a b\"]\n"), okList},
		"path traversal":   {route("  - route: /mcp/copilot/../a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: http://a.copilot/\n    tools: [x]\n"), okList},
		"unknown field":    {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: http://a.copilot/\n    tools: [x]\n    extra: 1\n"), okList},
		"empty routes":     {[]byte("version: 2\nroutes: []\n"), okList},
		"empty allowlist":  {okRoute, []byte(`{"servers":[]}`)},
		"bad name":         {okRoute, []byte(`{"servers":[{"name":"nope"}]}`)},
		"internal remote":  {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"https://c.intern.nav.no/mcp"}]}]}`)},
		"http remote":      {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"http://c.nav.no/mcp"}]}]}`)},
		"userinfo":         {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"https://u:p@c.nav.no/mcp"}]}]}`)},
		"other port":       {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"https://c.nav.no:8443/mcp"}]}]}`)},
		"punycode":         {okRoute, []byte(`{"servers":[{"name":"a.b/c","websiteUrl":"https://xn--nv-fia.no"}]}`)},
		"ip literal":       {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"https://10.0.0.1/mcp"}]}]}`)},
		"other template":   {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"https://c.{{domain_external}}/mcp"}]}]}`)},
		"bad gateway path": {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"https://fleet-gateway.{{domain_internal}}/mcp/x"}]}]}`)},
		"trailing dot":     {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"https://c.intern.nav.no./mcp"}]}]}`)},
		"uppercase intern": {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"https://C.INTERN.NAV.NO/mcp"}]}]}`)},
		"short ip":         {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"https://127.1/mcp"}]}]}`)},
		"hex ip":           {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"https://0x7f.1/mcp"}]}]}`)},
		"upstream dot":     {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: http://evil.example.com./\n    tools: [x]\n"), okList},
		"join by name":     {okRoute, []byte(`{"servers":[{"name":"io.github.navikt/a","remotes":[{"url":"https://a.nav.no/mcp"}]}]}`)},
		"name collision":   {okRoute, []byte(`{"servers":[{"name":"a.b/c"},{"name":"d.e/c"}]}`)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := seedServers(c.routes, c.list); err == nil {
				t.Error("want error")
			}
		})
	}
}
