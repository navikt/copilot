package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPRegistryListed(t *testing.T) {
	reg := mcpRegistry{
		Remotes:  map[string]bool{normalizeMCPURL("https://aksel-mcp.nav.no/mcp"): true},
		Packages: map[string]bool{"@playwright/mcp": true},
	}
	for _, tt := range []struct {
		s    mcpServer
		want bool
	}{
		{mcpServer{URL: "https://AKSEL-mcp.nav.no/mcp/"}, true},
		{mcpServer{URL: "https://evil.example/mcp"}, false},
		{mcpServer{Command: []string{"npx", "-y", "@playwright/mcp@latest"}}, true},
		{mcpServer{Command: []string{"npx", "@playwright/mcp"}}, true},
		{mcpServer{Command: []string{"npx", "@playwright/mcp-evil"}}, false},
		{mcpServer{Command: []string{"./my-server"}}, false},
	} {
		if got := reg.listed(tt.s); got != tt.want {
			t.Errorf("listed(%+v) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestOpenCodeMCPServersMergesJSONC(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	global := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "opencode")
	os.MkdirAll(global, 0o700)
	os.WriteFile(filepath.Join(global, "opencode.jsonc"), []byte(`{
		// mine
		"mcp": {"a": {"type": "remote", "url": "https://a/mcp",}, /* x */ "b": {"type": "local", "command": ["x"]},},
	}`), 0o600)
	proj := t.TempDir()
	os.Mkdir(filepath.Join(proj, ".git"), 0o700)
	os.WriteFile(filepath.Join(proj, "opencode.json"), []byte(`{"mcp": {"b": {"enabled": false}, "c": {"type": "remote", "url": "https://c"}}}`), 0o600)
	got := openCodeMCPServers(proj, []string{`OPENCODE_CONFIG_CONTENT={"mcp":{"d":{"type":"remote","url":"https://d"}}}`})
	if len(got) != 4 || got["a"].URL != "https://a/mcp" || got["b"].Enabled == nil || *got["b"].Enabled || got["b"].Command[0] != "x" || got["d"].URL == "" {
		t.Fatalf("servers = %+v", got)
	}
}

func TestApplyOpenCodeMCPPolicy(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	proj := t.TempDir()
	os.Mkdir(filepath.Join(proj, ".git"), 0o700)
	os.WriteFile(filepath.Join(proj, "opencode.json"), []byte(`{"mcp": {"ok": {"type": "remote", "url": "https://ok/mcp"}, "bad": {"type": "remote", "url": "https://bad/mcp"}}}`), 0o600)
	origPolicy, origReg := fetchMCPPolicy, fetchMCPRegistry
	t.Cleanup(func() { fetchMCPPolicy, fetchMCPRegistry = origPolicy, origReg })
	fetchMCPPolicy = func() (string, error) { return "https://registry/", nil }
	fetchMCPRegistry = func(string) (mcpRegistry, error) {
		return mcpRegistry{Remotes: map[string]bool{"https://ok/mcp": true}, Packages: map[string]bool{}}, nil
	}
	env := applyOpenCodeMCPPolicy([]string{`OPENCODE_CONFIG_CONTENT={"share":"disabled"}`}, proj)
	if !strings.Contains(env[0], `"bad":{"enabled":false}`) || strings.Contains(env[0], `"ok"`) || !strings.Contains(env[0], `"share":"disabled"`) {
		t.Fatalf("env = %v", env)
	}
	// No policy: nothing changes.
	fetchMCPPolicy = func() (string, error) { return "", nil }
	if env := applyOpenCodeMCPPolicy(nil, proj); len(env) != 0 {
		t.Fatalf("no policy changed env: %v", env)
	}
}
