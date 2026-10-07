package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		{mcpServer{Type: "remote", URL: "https://AKSEL-mcp.nav.no/mcp/"}, true},
		// A local server with a listed URL beside its command is still local.
		{mcpServer{Type: "local", URL: "https://aksel-mcp.nav.no/mcp", Command: []string{"node", "evil.js"}}, false},
		{mcpServer{URL: "https://aksel-mcp.nav.no/mcp"}, false},
		{mcpServer{Type: "remote", URL: "https://evil.example/mcp"}, false},
		{mcpServer{Type: "local", Command: []string{"npx", "-y", "@playwright/mcp@latest"}}, true},
		{mcpServer{Type: "local", Command: []string{"npx", "@playwright/mcp"}}, true},
		{mcpServer{Type: "local", Command: []string{"npx", "@playwright/mcp-evil"}}, false},
		{mcpServer{Type: "local", Command: []string{"./my-server"}}, false},
		// The package has to be what runs, not an argument to something else.
		{mcpServer{Type: "local", Command: []string{"node", "evil.js", "@playwright/mcp"}}, false},
		{mcpServer{Type: "local", Command: []string{"pnpm", "dlx", "@playwright/mcp"}}, true},
		{mcpServer{Type: "local", Command: []string{"npx", "--package", "@playwright/mcp", "mcp"}}, true},
	} {
		if got := reg.listed(tt.s); got != tt.want {
			t.Errorf("listed(%+v) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestOpenCodeMCPServersMergesJSONC(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
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
	os.MkdirAll(filepath.Join(proj, ".opencode"), 0o700)
	os.WriteFile(filepath.Join(proj, ".opencode", "opencode.json"), []byte(`{"mcp": {"e": {"type": "remote", "url": "{env:E_URL}"}}}`), 0o600)
	got := openCodeMCPServers(proj, []string{`OPENCODE_CONFIG_CONTENT={"mcp":{"d":{"type":"remote","url":"https://d"}}}`, "E_URL=https://e/mcp"})
	if got["e"].URL != "https://e/mcp" {
		t.Fatalf("{env:} not substituted: %+v", got["e"])
	}
	if len(got) != 5 || got["a"].URL != "https://a/mcp" || got["b"].Enabled == nil || *got["b"].Enabled || got["b"].Command[0] != "x" || got["d"].URL == "" {
		t.Fatalf("servers = %+v", got)
	}
}

func TestApplyOpenCodeMCPPolicy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
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
	if !strings.Contains(strings.Join(env, "\n"), `NAV_PILOT_MCP_BLOCKED={"blocked":["bad"],"listed":["ok"]}`) {
		t.Fatalf("blocked list not handed to the bridge: %v", env)
	}
	if !strings.Contains(env[0], `"bad":{"enabled":false}`) || strings.Contains(env[0], `"ok"`) || !strings.Contains(env[0], `"share":"disabled"`) {
		t.Fatalf("env = %v", env)
	}
	// No policy: nothing changes.
	fetchMCPPolicy = func() (string, error) { return "", nil }
	if env := applyOpenCodeMCPPolicy(nil, proj); len(env) != 0 {
		t.Fatalf("no policy changed env: %v", env)
	}
}

// doctor asks GitHub for the MCP policy once (#1072).
func TestOpenCodeMCPReportAsksForThePolicyOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	proj := t.TempDir()
	os.Mkdir(filepath.Join(proj, ".git"), 0o700)
	os.WriteFile(filepath.Join(proj, "opencode.json"), []byte(`{"mcp": {"ok": {"type": "remote", "url": "https://ok/mcp"}}}`), 0o600)
	origPolicy, origReg := fetchMCPPolicy, fetchMCPRegistry
	t.Cleanup(func() { fetchMCPPolicy, fetchMCPRegistry = origPolicy, origReg })
	calls := 0
	fetchMCPPolicy = func() (string, error) { calls++; return "https://registry/", nil }
	fetchMCPRegistry = func(string) (mcpRegistry, error) {
		return mcpRegistry{Remotes: map[string]bool{"https://ok/mcp": true}, Packages: map[string]bool{}}, nil
	}
	listed, unlisted, err := OpenCodeMCPReport(proj)
	if err != nil || len(listed) != 1 || len(unlisted) != 0 {
		t.Fatalf("report = %v %v %v", listed, unlisted, err)
	}
	if calls != 1 {
		t.Fatalf("the MCP policy was fetched %d times, want 1", calls)
	}
}

// opencode 2's own shape, mcp.servers, goes through the registry check like
// opencode 1's, and on opencode 2 a server turned off keeps its type and
// command or URL, without which opencode 2 drops the entry and runs it.
func TestApplyOpenCodeMCPPolicyV2(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	proj := t.TempDir()
	os.Mkdir(filepath.Join(proj, ".git"), 0o700)
	os.WriteFile(filepath.Join(proj, "opencode.json"), []byte(`{"mcp": {"timeout": {"catalog": 5}, "servers": {
		"ok": {"type": "remote", "url": "https://ok/mcp"},
		"bad": {"type": "remote", "url": "https://bad/mcp"},
		"evil": {"type": "local", "command": ["node", "evil.js"]},
		"off": {"type": "local", "command": ["node", "off.js"], "disabled": true}}}}`), 0o600)
	origPolicy, origReg := fetchMCPPolicy, fetchMCPRegistry
	t.Cleanup(func() { fetchMCPPolicy, fetchMCPRegistry = origPolicy, origReg; versionCache.Delete("opencode") })
	fetchMCPPolicy = func() (string, error) { return "https://registry/", nil }
	fetchMCPRegistry = func(string) (mcpRegistry, error) {
		return mcpRegistry{Remotes: map[string]bool{"https://ok/mcp": true}, Packages: map[string]bool{}}, nil
	}
	// opencode 1 knows no mcp.servers: what it reads stays as before.
	versionCache.Store("opencode", versionAnswer{"1.18.35\n", nil, time.Hour})
	if got := openCodeMCPServers(proj, nil); got["evil"].Command != nil || got["off"].Enabled != nil {
		t.Fatalf("opencode 1 read the v2 shape: %+v", got)
	}
	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24\n", nil, time.Hour})
	got := openCodeMCPServers(proj, nil)
	if len(got) != 4 || got["off"].Enabled == nil || *got["off"].Enabled || got["evil"].Command[1] != "evil.js" {
		t.Fatalf("servers = %+v", got)
	}

	env := applyOpenCodeMCPPolicy(nil, proj)
	all := strings.Join(env, "\n")
	for _, want := range []string{
		`NAV_PILOT_MCP_BLOCKED={"blocked":["bad","evil"],`,
		`"bad":{"enabled":false,"type":"remote","url":"https://bad/mcp"}`,
		`"evil":{"command":["node","evil.js"],"enabled":false,"type":"local"}`,
	} {
		if !strings.Contains(all, want) {
			t.Errorf("env lacks %s:\n%s", want, all)
		}
	}
	if strings.Contains(all, `"timeout":{"enabled"`) {
		t.Errorf("mcp.timeout read as a server: %s", all)
	}
}

// opencode 2 reads opencode.json in every directory up to "/", past the git
// root; opencode 1 stops at the git root.
func TestOpenCodeMCPServersV2WalksPastGitRoot(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { versionCache.Delete("opencode") })
	parent := t.TempDir()
	proj := filepath.Join(parent, "proj")
	os.MkdirAll(filepath.Join(proj, ".git"), 0o700)
	os.WriteFile(filepath.Join(parent, "opencode.json"), []byte(`{"mcp":{"servers":{"anc":{"type":"remote","url":"https://anc/mcp"}}}}`), 0o600)
	for ver, want := range map[string]bool{"1.18.35\n": false, "opencode v2.0.24\n": true} {
		versionCache.Store("opencode", versionAnswer{ver, nil, time.Hour})
		if _, got := openCodeMCPServers(proj, nil)["anc"]; got != want {
			t.Errorf("%q: server above the git root read = %v, want %v", ver, got, want)
		}
	}
}
