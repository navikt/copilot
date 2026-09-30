package provider

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func mcpConfigEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Chdir(t.TempDir())
	return home
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var figma = MCPServerEntry{Name: "com.figma/figma-mcp", Remotes: []MCPRemote{{Type: "streamable-http", URL: "https://mcp.figma.com/mcp"}}}

// The user's other servers and settings survive, in their order, the old
// file is the backup, and a second enable writes nothing.
func TestSetMCPServerCopilotKeepsTheRestAndIsIdempotent(t *testing.T) {
	home := mcpConfigEnv(t)
	path := filepath.Join(home, ".copilot", "mcp-config.json")
	orig := `{"zeta": 1, "mcpServers": {"mine": {"type": "local", "command": "my-mcp", "args": ["--x"], "env": {"TOKEN": "s3cret"}}}, "alpha": [1, 2]}`
	writeFile(t, path, orig)

	entry, err := MCPClientEntry(MCPClientCopilot, figma)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := SetMCPServer(MCPClientCopilot, figma.Name, entry)
	if err != nil || !ch.Changed {
		t.Fatalf("SetMCPServer = %+v, %v; want a change", ch, err)
	}
	if got := readFile(t, path+".bak"); got != orig {
		t.Fatalf("backup = %q, want the original file", got)
	}
	got := readFile(t, path)
	var cfg struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
		Zeta       int                        `json:"zeta"`
		Alpha      []int                      `json:"alpha"`
	}
	if err := json.Unmarshal([]byte(got), &cfg); err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(cfg.MCPServers["mine"], json.RawMessage(`{"type": "local", "command": "my-mcp", "args": ["--x"], "env": {"TOKEN": "s3cret"}}`)) {
		t.Errorf("the user's server changed: %s", cfg.MCPServers["mine"])
	}
	if !jsonEqual(cfg.MCPServers[figma.Name], json.RawMessage(`{"type":"http","url":"https://mcp.figma.com/mcp","tools":["*"]}`)) {
		t.Errorf("figma entry = %s", cfg.MCPServers[figma.Name])
	}
	if cfg.Zeta != 1 || len(cfg.Alpha) != 2 {
		t.Errorf("top-level keys lost: %s", got)
	}
	if strings.Index(got, `"zeta"`) > strings.Index(got, `"alpha"`) || strings.Index(got, `"mine"`) > strings.Index(got, figma.Name) {
		t.Errorf("key order changed:\n%s", got)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want the file's own 0600", info.Mode().Perm())
	}

	ch, err = SetMCPServer(MCPClientCopilot, figma.Name, entry)
	if err != nil || ch.Changed || ch.Existing != nil {
		t.Fatalf("second enable = %+v, %v; want no change", ch, err)
	}
	if readFile(t, path) != got {
		t.Error("second enable rewrote the file")
	}
}

// A different entry under the same name is the user's: kept, and returned.
func TestSetMCPServerKeepsADifferentEntry(t *testing.T) {
	home := mcpConfigEnv(t)
	path := filepath.Join(home, ".copilot", "mcp-config.json")
	orig := `{"mcpServers": {"com.figma/figma-mcp": {"type": "http", "url": "https://mcp.figma.com/mcp", "headers": {"X": "y"}}}}`
	writeFile(t, path, orig)
	entry, _ := MCPClientEntry(MCPClientCopilot, figma)
	ch, err := SetMCPServer(MCPClientCopilot, figma.Name, entry)
	if err != nil || ch.Changed || ch.Existing == nil {
		t.Fatalf("SetMCPServer = %+v, %v; want the existing entry back, no change", ch, err)
	}
	if readFile(t, path) != orig {
		t.Error("the user's entry was rewritten")
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("a backup was written for no change")
	}
}

// OpenCode: a new file gets the schema; comments are refused, not lost.
func TestSetMCPServerOpenCode(t *testing.T) {
	home := mcpConfigEnv(t)
	entry, _ := MCPClientEntry(MCPClientOpenCode, figma)
	ch, err := SetMCPServer(MCPClientOpenCode, figma.Name, entry)
	if err != nil || !ch.Changed || ch.Backup != "" {
		t.Fatalf("SetMCPServer on no file = %+v, %v", ch, err)
	}
	var cfg struct {
		Schema string                     `json:"$schema"`
		MCP    map[string]json.RawMessage `json:"mcp"`
	}
	if err := json.Unmarshal([]byte(readFile(t, ch.Path)), &cfg); err != nil || cfg.Schema == "" {
		t.Fatalf("new opencode.json = %s (%v)", readFile(t, ch.Path), err)
	}
	if !jsonEqual(cfg.MCP[figma.Name], json.RawMessage(`{"type":"remote","url":"https://mcp.figma.com/mcp","enabled":true}`)) {
		t.Errorf("entry = %s", cfg.MCP[figma.Name])
	}

	jsonc := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	os.Remove(ch.Path)
	orig := "{\n  // mine\n  \"mcp\": {},\n}\n"
	writeFile(t, jsonc, orig)
	ch, err = SetMCPServer(MCPClientOpenCode, figma.Name, entry)
	if !errors.Is(err, ErrMCPConfigHasComments) || ch.Path != jsonc {
		t.Fatalf("SetMCPServer on JSONC = %+v, %v; want ErrMCPConfigHasComments for %s", ch, err, jsonc)
	}
	if readFile(t, jsonc) != orig {
		t.Error("the JSONC file was changed")
	}
}

func TestRemoveMCPServer(t *testing.T) {
	home := mcpConfigEnv(t)
	path := filepath.Join(home, ".copilot", "mcp-config.json")
	orig := `{"mcpServers": {"a": {"url": "https://a"}, "com.figma/figma-mcp": {"url": "https://mcp.figma.com/mcp"}, "b": {"url": "https://b"}}}`
	writeFile(t, path, orig)
	ch, err := RemoveMCPServer(MCPClientCopilot, figma.Name)
	if err != nil || !ch.Changed {
		t.Fatalf("RemoveMCPServer = %+v, %v", ch, err)
	}
	if got := MCPConfigKeys(MCPClientCopilot); strings.Join(got, ",") != "a,b" {
		t.Errorf("keys after remove = %v, want a,b", got)
	}
	if readFile(t, path+".bak") != orig {
		t.Error("no backup of the original")
	}
	ch, err = RemoveMCPServer(MCPClientCopilot, figma.Name)
	if err != nil || ch.Changed {
		t.Errorf("second remove = %+v, %v; want no change", ch, err)
	}
}

func TestMCPClientEntry(t *testing.T) {
	pw := MCPServerEntry{Name: "com.microsoft/playwright-mcp", Packages: []MCPPackage{{RegistryType: "npm", Identifier: "@playwright/mcp", Version: "0.0.80"}}}
	pw.Packages[0].Arguments = append(pw.Packages[0].Arguments, struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{"--browser", "chromium"})
	for _, tt := range []struct {
		client string
		e      MCPServerEntry
		want   string
	}{
		{MCPClientCopilot, pw, `{"type":"local","command":"pnpm","args":["dlx","@playwright/mcp@0.0.80","--browser","chromium"],"tools":["*"]}`},
		{MCPClientOpenCode, pw, `{"type":"local","command":["pnpm","dlx","@playwright/mcp@0.0.80","--browser","chromium"],"enabled":true}`},
		{MCPClientCopilot, MCPServerEntry{Remotes: []MCPRemote{{Type: "sse", URL: "http://127.0.0.1:64342/sse"}}}, `{"type":"sse","url":"http://127.0.0.1:64342/sse","tools":["*"]}`},
		// A remote wins over a package: nothing to run from a cache.
		{MCPClientCopilot, MCPServerEntry{Remotes: figma.Remotes, Packages: pw.Packages}, `{"type":"http","url":"https://mcp.figma.com/mcp","tools":["*"]}`},
	} {
		got, err := MCPClientEntry(tt.client, tt.e)
		if err != nil || !jsonEqual(got, json.RawMessage(tt.want)) {
			t.Errorf("MCPClientEntry(%s, %s) = %s, %v; want %s", tt.client, tt.e.Name, got, err, tt.want)
		}
	}
	if _, err := MCPClientEntry(MCPClientCopilot, MCPServerEntry{Packages: []MCPPackage{{RegistryType: "oci"}}}); err == nil {
		t.Error("an oci package has no recipe and must be refused")
	}
}

func TestConfiguredMCPServers(t *testing.T) {
	home := mcpConfigEnv(t)
	writeFile(t, filepath.Join(home, ".copilot", "mcp-config.json"),
		`{"mcpServers": {"com.figma/figma-mcp": {}, "playwright-mcp": {}}}`)
	writeFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), `{"mcp": {
		"figma": {"type": "remote", "url": "https://MCP.figma.com/mcp/"},
		"pw": {"type": "local", "command": ["npx", "@playwright/mcp@latest"]},
		"off": {"type": "remote", "url": "https://mcp.svelte.dev/mcp", "enabled": false},
		"other": {"type": "remote", "url": "https://elsewhere.example/mcp"}}}`)
	// A project's config is shown, never counted.
	wd, _ := os.Getwd()
	writeFile(t, filepath.Join(wd, "opencode.json"), `{"mcp": {"repo-pick": {"type": "remote", "url": "https://mcp.svelte.dev/mcp"}}}`)
	entries := []MCPServerEntry{figma,
		{Name: "com.microsoft/playwright-mcp", Packages: []MCPPackage{{RegistryType: "npm", Identifier: "@playwright/mcp"}}},
		{Name: "dev.svelte/svelte-mcp", Remotes: []MCPRemote{{URL: "https://mcp.svelte.dev/mcp"}}}}
	c := ConfiguredMCPServers(entries)
	if !c.Copilot[figma.Name] || c.Copilot["com.microsoft/playwright-mcp"] || strings.Join(c.CopilotOther, ",") != "playwright-mcp" {
		t.Errorf("copilot = %v, other %v; the short name must not count as the registry's", c.Copilot, c.CopilotOther)
	}
	if !c.OpenCode[figma.Name] || !c.OpenCode["com.microsoft/playwright-mcp"] || c.OpenCode["dev.svelte/svelte-mcp"] || strings.Join(c.OpenCodeOther, ",") != "other" {
		t.Errorf("opencode = %v, other %v", c.OpenCode, c.OpenCodeOther)
	}
	if strings.Join(c.OpenCodeProject, ",") != "repo-pick" {
		t.Errorf("project = %v", c.OpenCodeProject)
	}
}

// Status, setup steps and sandbox hosts come from _meta, where the registry serves them.
func TestAskMCPRegistryEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"servers":[{"server":{"name":"com.microsoft/playwright-mcp","description":"d",
			"packages":[{"registryType":"npm","identifier":"@playwright/mcp","version":"0.0.80"}]},
			"_meta":{"io.modelcontextprotocol.registry/official":{"status":"deprecated"},
			"io.github.navikt/registry":{"setupInstructions":[{"title":"t","commands":["cplt config set sandbox.allow_cache_exec ms-playwright"]}],"sandboxHosts":["api.example"]}}}]}`))
	}))
	defer srv.Close()
	reg, err := askMCPRegistry(srv.URL)
	if err != nil || len(reg.Entries) != 1 {
		t.Fatalf("askMCPRegistry = %+v, %v", reg, err)
	}
	e := reg.Entries[0]
	if e.Status != "deprecated" || e.Usable() || len(e.Setup) != 1 || e.Packages[0].Version != "0.0.80" || !slices.Equal(e.SandboxHosts, []string{"api.example"}) {
		t.Errorf("entry = %+v", e)
	}
}

// A server the user added to OpenCode under a name of their own is found by
// its URL, so enable does not add it twice.
func TestMCPConfigKeyFor(t *testing.T) {
	home := mcpConfigEnv(t)
	writeFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), `{"mcp": {"figma": {"type": "remote", "url": "https://mcp.figma.com/mcp"}}}`)
	writeFile(t, filepath.Join(home, ".copilot", "mcp-config.json"), `{"mcpServers": {"figma": {"type": "http", "url": "https://mcp.figma.com/mcp"}}}`)
	if got := MCPConfigKeyFor(MCPClientOpenCode, figma); got != "figma" {
		t.Errorf("opencode = %q, want figma", got)
	}
	// Copilot's policy matches the name only: "figma" is not the registry's.
	if got := MCPConfigKeyFor(MCPClientCopilot, figma); got != "" {
		t.Errorf("copilot = %q, want none", got)
	}
}

// nav-pilot mcp reads the launch's cache: a fresh one asks nothing, a stale
// one is read again, and when that fails the old answer is used and said to
// be old. No cache and no registry is an error.
func TestMCPRegistryServersUsesTheCache(t *testing.T) {
	reg := testRegistry()
	reg.URL = "https://registry.test"
	reg.Entries = []MCPServerEntry{{Name: "com.figma/figma-mcp", Setup: []MCPSetupStep{{Title: "t"}}}}
	mcpEnv(t, `{"mcpServers": {"com.figma/figma-mcp": {}}}`, reg, nil)
	fetchMCPRegistry = func(string) (mcpRegistry, error) {
		t.Error("a fresh cache asked the registry")
		return reg, nil
	}
	name, entries, stale, err := MCPRegistryServers()
	if err != nil || stale != nil || name != reg.URL || len(entries) != 1 || len(entries[0].Setup) != 1 {
		t.Fatalf("fresh: %q %+v %v %v", name, entries, stale, err)
	}

	c, _ := readMCPRegistryCache()
	c.At = c.At.Add(-2 * mcpRegistryTTL)
	data, _ := json.Marshal(c)
	writeFile(t, mcpRegistryCachePath(), string(data))
	fetchMCPRegistry = func(string) (mcpRegistry, error) { return mcpRegistry{}, errors.New("timeout") }
	if _, entries, stale, err = MCPRegistryServers(); err != nil || stale == nil || len(entries) != 1 {
		t.Errorf("stale, registry down: %+v %v %v", entries, stale, err)
	}

	reg.Entries = append(reg.Entries, MCPServerEntry{Name: "io.github.navikt/mcp-onboarding"})
	fetchMCPRegistry = func(string) (mcpRegistry, error) { return reg, nil }
	if _, entries, stale, err = MCPRegistryServers(); err != nil || stale != nil || len(entries) != 2 {
		t.Errorf("stale, registry up: %+v %v %v", entries, stale, err)
	}

	if err := os.Remove(mcpRegistryCachePath()); err != nil {
		t.Fatal(err)
	}
	fetchMCPRegistry = func(string) (mcpRegistry, error) { return mcpRegistry{}, errors.New("timeout") }
	if _, _, _, err = MCPRegistryServers(); err == nil {
		t.Error("no cache, registry down: want an error")
	}
}

// disable narrows the approval to the hosts either client's servers still
// need; a host only OpenCode needs stays even when Copilot is the default.
func TestNarrowMCPApproval(t *testing.T) {
	mcpEnv(t, `{"mcpServers": {"io.github.navikt/mcp-onboarding": {}}}`, testRegistry(), nil)
	home, _ := os.UserHomeDir()
	writeFile(t, filepath.Join(openCodeConfigDir(), "opencode.json"),
		`{"mcp": {"f": {"type": "remote", "url": "https://mcp.figma.com/mcp"}}}`)
	approvedMCP(t, []MCPHost{{Host: "mcp.figma.com"}, {Host: "mcp-onboarding.intern.nav.no", Private: true}, {Host: "gone.example"}})
	gone, err := NarrowMCPApproval()
	if err != nil || strings.Join(gone, ",") != "gone.example" {
		t.Fatalf("gone = %v, %v", gone, err)
	}
	rec, _ := readMCPRecord()
	kept := recordedMCPHosts(rec)
	if !rec.Approved || len(kept) != 2 || !slices.ContainsFunc(kept, func(h MCPHost) bool { return h.Private }) {
		t.Errorf("kept = %+v", kept)
	}

	if err := os.Remove(mcpRegistryCachePath()); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(home, ".copilot", "mcp-config.json"), `{"mcpServers": {}}`)
	if gone, err := NarrowMCPApproval(); err != nil || gone != nil {
		t.Errorf("no cache: gone = %v, %v; want the approval kept", gone, err)
	}
}
