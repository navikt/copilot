package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/navikt/copilot/fleet-control/internal/model"
)

func readSeedFiles(t *testing.T, env string) (routes, allowlist []byte) {
	t.Helper()
	routes, err := os.ReadFile("../../../fleet-gateway/pilot-routes." + env + ".yaml") //nolint:gosec // fixed repo paths
	if err != nil {
		t.Fatal(err)
	}
	allowlist, err = os.ReadFile("../../../mcp-registry/allowlist.json")
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
			servers, err := seedServers(env+"-gcp", routes, allowlist)
			if err != nil {
				t.Fatal(err)
			}
			byKey := map[string]model.Server{}
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
			gateway := 0
			for _, raw := range doc.Servers {
				if strings.Contains(string(raw), gatewayRemotePrefix) {
					gateway++
				}
			}
			if gateway == 0 || registry != gateway {
				t.Errorf("seeded %d registry entries, allowlist has %d gateway entries", registry, gateway)
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
			for _, s := range servers {
				if s.Registry != nil && !strings.Contains(string(s.Registry), gatewayRemotePrefix) {
					t.Errorf("%s: only fleet-gateway entries are seeded", s.Key())
				}
			}
			if _, ok := byKey["copilot/figma-mcp"]; ok {
				t.Error("non-gateway entry figma-mcp must not be seeded")
			}
			if _, ok := byKey["copilot/test"]; env == "dev" && !ok {
				t.Error("pilot route test must still be seeded")
			}
			for _, s := range servers {
				if s.Status != "published" {
					t.Errorf("%s: seed marks everything published, got %q", s.Key(), s.Status)
				}
			}
		})
	}
	routes, allowlist := readSeedFiles(t, "dev")
	servers, _ := seedServers("dev-gcp", routes, allowlist)
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
	servers, err := seedServers("dev-gcp", okRoute, okList)
	if err != nil {
		t.Fatalf("baseline must pass: %v", err)
	}
	if len(servers) != 1 || servers[0].Key() != "copilot/a" {
		t.Errorf("non-gateway allowlist entry must not be seeded: %+v", servers)
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
		"ingress upstream": {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: https://a.intern.nav.no/\n    tools: [x]\n"), okList},
		"other app":        {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: http://b.copilot/\n    tools: [x]\n"), okList},
		"upstream query":   {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: http://a.copilot/?x=1\n    tools: [x]\n"), okList},
		"other cluster":    {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: prod-gcp:copilot:a\n    upstream_url: http://a.copilot/\n    tools: [x]\n"), okList},
		"upstream dot":     {route("  - route: /mcp/copilot/a\n    team: copilot\n    target: dev-gcp:copilot:a\n    upstream_url: http://evil.example.com./\n    tools: [x]\n"), okList},
		"name collision":   {okRoute, []byte(`{"servers":[{"name":"a.b/c","remotes":[{"url":"https://fleet-gateway.{{domain_internal}}/mcp/copilot/c"}]},{"name":"d.e/c","remotes":[{"url":"https://fleet-gateway.{{domain_internal}}/mcp/copilot/c"}]}]}`)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := seedServers("dev-gcp", c.routes, c.list); err == nil {
				t.Error("want error")
			}
		})
	}
}
