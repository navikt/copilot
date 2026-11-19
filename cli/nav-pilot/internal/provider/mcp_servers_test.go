package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
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

	entry, err := MCPClientEntry(MCPClientCopilot, figma, MCPToolChoice{All: true})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := SetMCPServer(MCPClientCopilot, figma.Name, entry, figma, MCPToolChoice{All: true})
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

	ch, err = SetMCPServer(MCPClientCopilot, figma.Name, entry, figma, MCPToolChoice{All: true})
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
	entry, _ := MCPClientEntry(MCPClientCopilot, figma, MCPToolChoice{All: true})
	ch, err := SetMCPServer(MCPClientCopilot, figma.Name, entry, figma, MCPToolChoice{All: true})
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
	entry, _ := MCPClientEntry(MCPClientOpenCode, figma, MCPToolChoice{All: true})
	ch, err := SetMCPServer(MCPClientOpenCode, figma.Name, entry, figma, MCPToolChoice{All: true})
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
	ch, err = SetMCPServer(MCPClientOpenCode, figma.Name, entry, figma, MCPToolChoice{All: true})
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
	ch, err := RemoveMCPServer(MCPClientCopilot, figma.Name, nil)
	if err != nil || !ch.Changed {
		t.Fatalf("RemoveMCPServer = %+v, %v", ch, err)
	}
	if got := MCPConfigKeys(MCPClientCopilot); strings.Join(got, ",") != "a,b" {
		t.Errorf("keys after remove = %v, want a,b", got)
	}
	if readFile(t, path+".bak") != orig {
		t.Error("no backup of the original")
	}
	ch, err = RemoveMCPServer(MCPClientCopilot, figma.Name, nil)
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
		// The registry's fixed headers go into both clients' entries.
		{MCPClientOpenCode, func() MCPServerEntry {
			e := MCPServerEntry{Remotes: []MCPRemote{{Type: "streamable-http", URL: "https://x.example/mcp"}}}
			e.Remotes[0].Headers = append(e.Remotes[0].Headers, struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			}{"X-MCP-Readonly", "true"})
			return e
		}(), `{"type":"remote","url":"https://x.example/mcp","enabled":true,"headers":{"X-MCP-Readonly":"true"}}`},
		// A remote wins over a package: nothing to run from a cache.
		{MCPClientCopilot, MCPServerEntry{Remotes: figma.Remotes, Packages: pw.Packages}, `{"type":"http","url":"https://mcp.figma.com/mcp","tools":["*"]}`},
	} {
		got, err := MCPClientEntry(tt.client, tt.e, MCPToolChoice{All: true})
		if err != nil || !jsonEqual(got, json.RawMessage(tt.want)) {
			t.Errorf("MCPClientEntry(%s, %s) = %s, %v; want %s", tt.client, tt.e.Name, got, err, tt.want)
		}
	}
	if _, err := MCPClientEntry(MCPClientCopilot, MCPServerEntry{Packages: []MCPPackage{{RegistryType: "oci"}}}, MCPToolChoice{All: true}); err == nil {
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
		"pwenv": {"type": "local", "command": ["npx", "@playwright/mcp"], "environment": {"X": "1"}},
		"dev.svelte/svelte-mcp": {"type": "remote", "url": "https://svelte.example/mcp"},
		"off": {"type": "remote", "url": "https://mcp.svelte.dev/mcp", "enabled": false},
		"other": {"type": "remote", "url": "https://elsewhere.example/mcp"}}}`)
	// A project's config is shown, never counted.
	wd, _ := os.Getwd()
	writeFile(t, filepath.Join(wd, "opencode.json"), `{"mcp": {"repo-pick": {"type": "remote", "url": "https://mcp.svelte.dev/mcp"}}}`)
	entries := []MCPServerEntry{figma,
		{Name: "com.microsoft/playwright-mcp", Packages: []MCPPackage{{RegistryType: "npm", Identifier: "@playwright/mcp"}}},
		{Name: "dev.svelte/svelte-mcp", Remotes: []MCPRemote{{URL: "https://mcp.svelte.dev/mcp"}}}}
	prevPolicy := fetchMCPPolicy
	t.Cleanup(func() { fetchMCPPolicy = prevPolicy })
	// allow_all: launch turns nothing off, so nothing is refused.
	fetchMCPPolicy = func() (string, error) { return "", nil }
	if c := ConfiguredMCPServers(entries); len(c.OpenCodeRefused) != 0 {
		t.Errorf("allow_all refused = %v", c.OpenCodeRefused)
	}
	fetchMCPPolicy = func() (string, error) { return "https://registry.test", nil }
	c := ConfiguredMCPServers(entries)
	if !c.Copilot[figma.Name] || c.Copilot["com.microsoft/playwright-mcp"] || strings.Join(c.CopilotOther, ",") != "playwright-mcp" {
		t.Errorf("copilot = %v, other %v; the short name must not count as the registry's", c.Copilot, c.CopilotOther)
	}
	// Matched by package or name, yet listed() turns them off at launch (#2195).
	if fmt.Sprint(c.OpenCodeRefused) != "[[dev.svelte/svelte-mcp dev.svelte/svelte-mcp] [pwenv com.microsoft/playwright-mcp]]" {
		t.Errorf("refused = %v", c.OpenCodeRefused)
	}
	if !c.OpenCode[figma.Name] || !c.OpenCode["com.microsoft/playwright-mcp"] || strings.Join(c.OpenCodeOther, ",") != "other" {
		t.Errorf("opencode = %v, other %v", c.OpenCode, c.OpenCodeOther)
	}
	if strings.Join(c.OpenCodeProject, ",") != "repo-pick" {
		t.Errorf("project = %v", c.OpenCodeProject)
	}
}

// Status, setup steps, sandbox hosts and protocol versions come from _meta, where the registry serves them.
func TestAskMCPRegistryEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"servers":[{"server":{"name":"com.microsoft/playwright-mcp","description":"d",
			"packages":[{"registryType":"npm","identifier":"@playwright/mcp","version":"0.0.80"}]},
			"_meta":{"io.modelcontextprotocol.registry/official":{"status":"deprecated"},
			"io.github.navikt/registry":{"setupInstructions":[{"title":"t","commands":["cplt config set sandbox.allow_cache_exec ms-playwright"]}],"sandboxHosts":["api.example"],"protocolVersions":["2026-07-28"]}}}]}`))
	}))
	defer srv.Close()
	reg, err := askMCPRegistry(srv.URL)
	if err != nil || len(reg.Entries) != 1 {
		t.Fatalf("askMCPRegistry = %+v, %v", reg, err)
	}
	e := reg.Entries[0]
	if e.Status != "deprecated" || e.Usable() || len(e.Setup) != 1 || e.Packages[0].Version != "0.0.80" || !slices.Equal(e.SandboxHosts, []string{"api.example"}) || !slices.Equal(e.ProtocolVersions, []string{"2026-07-28"}) {
		t.Errorf("entry = %+v", e)
	}
}

func TestGitHubReadonlyRegistryAllowlist(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "apps", "mcp-registry", "allowlist.json"))
	if err != nil {
		t.Fatal(err)
	}
	var allowlist struct {
		Servers []MCPServerEntry `json:"servers"`
	}
	if err := json.Unmarshal(data, &allowlist); err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(allowlist.Servers, func(e MCPServerEntry) bool { return e.Name == "io.github.navikt/github-mcp" })
	if i < 0 {
		t.Fatal("GitHub MCP is missing from the allowlist")
	}
	github := allowlist.Servers[i]
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0.1/servers" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"servers": []any{map[string]any{"server": github}},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()
	reg, err := askMCPRegistry(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Entries) != 1 {
		t.Fatalf("registry entries = %d, want 1", len(reg.Entries))
	}
	e := reg.Entries[0]
	wantHeaders := map[string]string{"X-MCP-Readonly": "true", "X-MCP-Lockdown": "true"}
	for _, remote := range e.Remotes {
		headers := map[string]string{}
		for _, h := range remote.Headers {
			headers[h.Name] = h.Value
		}
		if remote.Type != "streamable-http" || !maps.Equal(headers, wantHeaders) {
			t.Errorf("remote = %+v, want streamable-http with headers %v", remote, wantHeaders)
		}
	}
	entry, err := MCPClientEntry(MCPClientOpenCode, e, MCPToolChoice{All: true})
	if err != nil {
		t.Fatal(err)
	}
	var generated struct {
		mcpServer
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(entry, &generated); err != nil {
		t.Fatal(err)
	}
	if generated.URL != githubMCPReadonlyURL || !maps.Equal(generated.Headers, wantHeaders) {
		t.Errorf("generated entry = %s, want readonly URL and registry headers", entry)
	}
	if !reg.listed(generated.mcpServer) {
		t.Errorf("generated readonly entry is not listed: %s", entry)
	}
	if !reg.listed(mcpServer{Type: "remote", URL: githubMCPURL}) {
		t.Error("existing full endpoint is not listed")
	}
	for _, u := range []string{
		"https://api.githubcopilot.com/mcp/other",
		"https://api.githubcopilot.com/mcp/readonly/other",
		"https://api.githubcopilot.com.evil.example/mcp/readonly",
		githubMCPReadonlyURL + "?toolsets=all",
		githubMCPURL + "?toolsets=all",
	} {
		if reg.listed(mcpServer{Type: "remote", URL: u}) {
			t.Errorf("unlisted URL accepted: %s", u)
		}
	}
	if len(e.Remotes) != 2 || e.Remotes[0].URL != githubMCPURL || e.Remotes[1].URL != githubMCPReadonlyURL {
		t.Fatalf("remotes = %+v, want full then readonly", e.Remotes)
	}
	github.Remotes = github.Remotes[:1]
	reg, err = askMCPRegistry(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if reg.listed(generated.mcpServer) {
		t.Error("readonly entry accepted after the registry removed it")
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

// enable writes GitHub's read-only endpoint, so a custom OpenCode key with
// that URL is still GitHub's.
func TestMCPConfigKeyForGitHubReadonly(t *testing.T) {
	home := mcpConfigEnv(t)
	writeFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), `{"mcp": {"gh": {"type": "remote", "url": "https://api.githubcopilot.com/mcp/readonly"}}}`)
	gh := MCPServerEntry{Name: "io.github.navikt/github-mcp", Remotes: []MCPRemote{{Type: "streamable-http", URL: "https://api.githubcopilot.com/mcp/"}}}
	if got := MCPConfigKeyFor(MCPClientOpenCode, gh); got != "gh" {
		t.Errorf("opencode = %q, want gh", got)
	}
	if got := MCPConfigKeyFor(MCPClientOpenCode, figma); got != "" {
		t.Errorf("figma = %q, want none", got)
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

// disable narrows each client's approval to the hosts its servers still
// need, migrates a legacy combined record, and keeps hosts the other client
// still needs.
func TestNarrowMCPApproval(t *testing.T) {
	mcpEnv(t, `{"mcpServers": {"io.github.navikt/mcp-onboarding": {}}}`, testRegistry(), nil)
	prevProbe := probeCpltVersion
	t.Cleanup(func() { probeCpltVersion = prevProbe })
	probeCpltVersion = func() (string, error) { return "cplt " + cpltDenyEnv + "-7f0fd16\n", nil }
	home, _ := os.UserHomeDir()
	writeFile(t, filepath.Join(openCodeConfigDir(), "opencode.json"),
		`{"mcp": {"f": {"type": "remote", "url": "https://mcp.figma.com/mcp"}}}`)
	user, err := domain.ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	block, _ := json.Marshal([]MCPHost{{Host: "mcp.figma.com"}, {Host: "mcp-onboarding.intern.nav.no", Private: true}, {Host: "gone.example"}})
	if err := artifacts.WriteProposalConsent(artifacts.ProposalConsent{
		Scope: user.Name, Root: user.RootDir, Pakke: MCPConsentSource,
		Block: string(block), CpltStamp: CpltStamp(), Approved: true,
		Hosts: []string{"mcp-onboarding.intern.nav.no"},
	}); err != nil {
		t.Fatal(err)
	}
	gone, err := NarrowMCPApproval()
	if err != nil || strings.Join(gone, ",") != "gone.example" {
		t.Fatalf("gone = %v, %v", gone, err)
	}
	rec, _ := readMCPRecordFor(MCPClientCopilot)
	kept := recordedMCPHosts(rec)
	if !rec.Approved || len(kept) != 1 || kept[0].Host != "mcp-onboarding.intern.nav.no" || !kept[0].Private {
		t.Errorf("copilot kept = %+v", kept)
	}
	ocRec, _ := readMCPRecordFor(MCPClientOpenCode)
	ocKept := recordedMCPHosts(ocRec)
	if !ocRec.Approved || len(ocKept) != 1 || ocKept[0].Host != "mcp.figma.com" {
		t.Errorf("opencode kept = %+v", ocKept)
	}
	if got := MCPAllowlistHosts(); !slices.Equal(got, []string{"mcp-onboarding.intern.nav.no", "mcp.figma.com"}) {
		t.Errorf("allowlist hosts = %v", got)
	}

	if err := os.Remove(mcpRegistryCachePath()); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(home, ".copilot", "mcp-config.json"), `{"mcpServers": {}}`)
	if gone, err := NarrowMCPApproval(); err != nil || gone != nil {
		t.Errorf("no cache: gone = %v, %v; want the approval kept", gone, err)
	}
}

// The broker extension rides in server.json's own _meta, as the registry
// serves the nais entry from its allowlist, and lands in Meta.Broker.
func TestAskMCPRegistryBrokerMeta(t *testing.T) {
	data, err := os.ReadFile("../../../../apps/mcp-registry/allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	var allow struct {
		Servers []map[string]json.RawMessage `json:"servers"`
	}
	if err := json.Unmarshal(data, &allow); err != nil {
		t.Fatal(err)
	}
	var meta json.RawMessage
	for _, s := range allow.Servers {
		if string(s["name"]) == `"io.github.nais/nais-mcp"` {
			meta = s["_meta"]
		}
	}
	if meta == nil {
		t.Fatal("no _meta on nais-mcp in the allowlist")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"servers":[{"server":{"name":"io.github.nais/nais-mcp","description":"d","version":"6.6.1","_meta":` + string(meta) + `},"_meta":{}},
			{"server":{"name":"com.figma/figma-mcp","description":"d","remotes":[{"type":"streamable-http","url":"https://mcp.figma.com/mcp"}]},"_meta":{}}]}`))
	}))
	defer srv.Close()
	reg, err := askMCPRegistry(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	b := reg.Entries[0].Meta.Broker
	if b == nil || b.Credential != "nais-keychain" || !slices.Equal(b.Command, []string{"nais", "alpha", "mcp", "serve"}) ||
		b.Login.Status != "" || b.Tools.Classes["execute_graphql"] != "read" || b.ArgChecks["execute_graphql"] != "graphql-read-only" {
		t.Errorf("broker = %+v", b)
	}
	if reg.Entries[1].Meta.Broker != nil {
		t.Error("a server without the extension got one")
	}
}

// opencode 2's own layout keeps servers under mcp.servers; nav-pilot finds
// and removes them there instead of reporting them missing.
func TestMCPConfigOpenCodeNativeServers(t *testing.T) {
	home := mcpConfigEnv(t)
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeFile(t, path, `{"mcp": {"timeout": 5000, "servers": {"figma": {"type": "remote", "url": "https://mcp.figma.com/mcp"}, "b": {"type": "remote", "url": "https://b"}}}}`)
	if got := MCPConfigKeyFor(MCPClientOpenCode, figma); got != "figma" {
		t.Errorf("MCPConfigKeyFor = %q, want figma", got)
	}
	if ch, err := RemoveMCPServer(MCPClientOpenCode, "figma", nil); err != nil || !ch.Changed {
		t.Fatalf("RemoveMCPServer = %+v, %v", ch, err)
	}
	if got := MCPConfigKeys(MCPClientOpenCode); strings.Join(got, ",") != "b" {
		t.Errorf("keys after remove = %v, want b", got)
	}
	if got := readFile(t, path); !strings.Contains(got, `"timeout": 5000`) {
		t.Errorf("config lost its other keys: %s", got)
	}
}

// A flat server beside mcp.servers still runs (openCodeMCPServersIn loads
// both), so disable must find it and remove it where it is.
func TestMCPConfigOpenCodeFlatBesideNative(t *testing.T) {
	home := mcpConfigEnv(t)
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeFile(t, path, `{"mcp": {"figma": {"type": "remote", "url": "https://mcp.figma.com/mcp"}, "servers": {"b": {"type": "remote", "url": "https://b"}}}}`)
	if got := MCPConfigKeys(MCPClientOpenCode); strings.Join(got, ",") != "figma,b" {
		t.Errorf("keys = %v, want figma,b", got)
	}
	if ch, err := RemoveMCPServer(MCPClientOpenCode, "figma", nil); err != nil || !ch.Changed {
		t.Fatalf("RemoveMCPServer = %+v, %v", ch, err)
	}
	if got := MCPConfigKeys(MCPClientOpenCode); strings.Join(got, ",") != "b" {
		t.Errorf("keys after remove = %v, want b", got)
	}
}

// Same server present both flat and under mcp.servers: one remove drops both.
func TestMCPConfigOpenCodeDuplicateFlatAndNativeRemovedTogether(t *testing.T) {
	home := mcpConfigEnv(t)
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeFile(t, path, `{"mcp": {"timeout": 5000, "figma": {"type": "remote", "url": "https://old"}, "servers": {"figma": {"type": "remote", "url": "https://mcp.figma.com/mcp"}, "b": {"type": "remote", "url": "https://b"}}}}`)
	if ch, err := RemoveMCPServer(MCPClientOpenCode, "figma", nil); err != nil || !ch.Changed {
		t.Fatalf("RemoveMCPServer = %+v, %v", ch, err)
	}
	if got := MCPConfigKeys(MCPClientOpenCode); strings.Join(got, ",") != "b" {
		t.Errorf("keys after remove = %v, want b", got)
	}
	if got := readFile(t, path); !strings.Contains(got, `"timeout": 5000`) {
		t.Errorf("config lost timeout: %s", got)
	}
}

func TestOpenCodeProtocol(t *testing.T) {
	cases := []struct {
		name, url string
		versions  []string
		want      string // "" means no protocol key
	}{
		{"only 2026-07-28 pins", "https://mcp.example.com/mcp", []string{"2026-07-28"}, "2026-07-28"},
		{"only 2026-07-28 on nav", "https://x.nav.no/mcp", []string{"2026-07-28"}, "2026-07-28"},
		{"mixed gets auto", "https://mcp.example.com/mcp", []string{"2026-07-28", "2025-11-25"}, "auto"},
		{"older only on nav", "https://x.nav.no/mcp", []string{"2025-11-25"}, ""},
		{"older only elsewhere", "https://mcp.example.com/mcp", []string{"2025-11-25"}, ""},
		{"absent on nav", "https://aksel-mcp.nav.no/mcp", nil, "auto"},
		{"absent on nav apex", "https://nav.no/mcp", nil, "auto"},
		{"absent on intern nav", "https://x.intern.dev.nav.no/mcp", nil, "auto"},
		{"absent on http nav", "http://x.nav.no/mcp", nil, ""},
		{"absent on evilnav", "https://evilnav.no/mcp", nil, ""},
		{"absent on nav.no.example", "https://nav.no.example.com/mcp", nil, ""},
		{"absent elsewhere", "https://mcp.figma.com/mcp", nil, ""},
	}
	for _, tc := range cases {
		e := MCPServerEntry{Remotes: []MCPRemote{{Type: "streamable-http", URL: tc.url}}, ProtocolVersions: tc.versions}
		raw, err := MCPClientEntry(MCPClientOpenCode, e, MCPToolChoice{All: true})
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Protocol *string `json:"protocol"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if g := ptrOr(got.Protocol); g != tc.want || (tc.want == "") != (got.Protocol == nil) {
			t.Errorf("%s: protocol %q, want %q (%s)", tc.name, g, tc.want, raw)
		}
		// Copilot's config never gets the key.
		c, _ := MCPClientEntry(MCPClientCopilot, e, MCPToolChoice{All: true})
		if strings.Contains(string(c), "protocol") {
			t.Errorf("%s: copilot entry has protocol: %s", tc.name, c)
		}
	}
}

func ptrOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// An existing entry, with the user's own protocol, is kept as it is.
func TestSetMCPServerKeepsUserProtocol(t *testing.T) {
	mcpConfigEnv(t)
	e := MCPServerEntry{Name: "no.nav/x", Remotes: []MCPRemote{{URL: "https://x.nav.no/mcp"}}}
	user := json.RawMessage(`{"type":"remote","url":"https://x.nav.no/mcp","enabled":true,"protocol":"legacy"}`)
	if _, err := SetMCPServer(MCPClientOpenCode, e.Name, user, e, MCPToolChoice{All: true}); err != nil {
		t.Fatal(err)
	}
	entry, _ := MCPClientEntry(MCPClientOpenCode, e, MCPToolChoice{All: true})
	ch, err := SetMCPServer(MCPClientOpenCode, e.Name, entry, e, MCPToolChoice{All: true})
	if err != nil || ch.Changed || !strings.Contains(string(ch.Existing), `"legacy"`) {
		t.Fatalf("overwrote user entry: %+v, %v", ch, err)
	}
}

func TestBackfillOpenCodeProtocol(t *testing.T) {
	cases := []struct {
		name     string
		entry    string
		versions []string
		url      string
		wantRet  string
		wantVal  string
		wantChg  bool
	}{
		{
			name:     "only 2026-07-28",
			entry:    `{"type":"remote","url":"https://x.example/mcp"}`,
			versions: []string{"2026-07-28"},
			url:      "https://x.example/mcp",
			wantRet:  "2026-07-28",
			wantVal:  "2026-07-28",
			wantChg:  true,
		},
		{
			name:     "2026-07-28 with older",
			entry:    `{"type":"remote","url":"https://x.example/mcp"}`,
			versions: []string{"2024-11-05", "2026-07-28"},
			url:      "https://x.example/mcp",
			wantRet:  "auto",
			wantVal:  "auto",
			wantChg:  true,
		},
		{
			name:     "older only",
			entry:    `{"type":"remote","url":"https://x.example/mcp"}`,
			versions: []string{"2024-11-05"},
			url:      "https://x.example/mcp",
		},
		{
			name:     "older only on nav",
			entry:    `{"type":"remote","url":"https://x.nav.no/mcp"}`,
			versions: []string{"2025-11-25"},
			url:      "https://x.nav.no/mcp",
		},
		{
			name:    "nil versions with nav.no url",
			entry:   `{"type":"remote","url":"https://x.nav.no/mcp"}`,
			url:     "https://x.nav.no/mcp",
			wantRet: "auto",
			wantVal: "auto",
			wantChg: true,
		},
		{
			name:  "nil versions with non-nav https url",
			entry: `{"type":"remote","url":"https://example.com/mcp"}`,
			url:   "https://example.com/mcp",
		},
		{
			name:     "existing user protocol kept",
			entry:    `{"type":"remote","url":"https://x.example/mcp","protocol":"legacy"}`,
			versions: []string{"2026-07-28"},
			url:      "https://x.example/mcp",
			wantVal:  "legacy",
		},
		{
			name:     "local entry ignored",
			entry:    `{"type":"local","command":["npx","x"]}`,
			versions: []string{"2026-07-28"},
		},
		{
			name:     "registry name with different url",
			entry:    `{"type":"remote","url":"https://other.example/mcp"}`,
			versions: []string{"2026-07-28"},
			url:      "https://x.example/mcp",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := mcpConfigEnv(t)
			wantPath := filepath.Join(home, ".config", "opencode", "opencode.json")
			gotPath, err := MCPConfigPath(MCPClientOpenCode)
			if err != nil || gotPath != wantPath {
				t.Fatalf("MCPConfigPath = %q, %v; want %q", gotPath, err, wantPath)
			}
			writeFile(t, gotPath, fmt.Sprintf(`{"mcp":{"no.nav/x":%s}}`, tc.entry))
			e := MCPServerEntry{
				Name:             "no.nav/x",
				ProtocolVersions: tc.versions,
			}
			if tc.url != "" {
				e.Remotes = []MCPRemote{{URL: tc.url}}
			}
			p, ch, err := BackfillOpenCodeProtocol("no.nav/x", e)
			if err != nil {
				t.Fatalf("BackfillOpenCodeProtocol: %v", err)
			}
			if p != tc.wantRet || ch.Changed != tc.wantChg || ch.Path != wantPath {
				t.Fatalf("got p=%q ch=%+v; want p=%q changed=%v path=%q", p, ch, tc.wantRet, tc.wantChg, wantPath)
			}
			var cfg struct {
				MCP map[string]map[string]any `json:"mcp"`
			}
			if err := json.Unmarshal([]byte(readFile(t, wantPath)), &cfg); err != nil {
				t.Fatal(err)
			}
			gotVal, _ := cfg.MCP["no.nav/x"]["protocol"].(string)
			if gotVal != tc.wantVal {
				t.Errorf("protocol in file = %q, want %q", gotVal, tc.wantVal)
			}
			p2, ch2, err2 := BackfillOpenCodeProtocol("no.nav/x", e)
			if err2 != nil || p2 != "" || ch2.Changed {
				t.Fatalf("second call = p=%q ch=%+v err=%v; want no-op", p2, ch2, err2)
			}
		})
	}
}

func TestMCPClientEntryUsesChosenRemoteHeaders(t *testing.T) {
	type hdr = struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	multi := MCPServerEntry{
		Name: "io.example/multi",
		Remotes: []MCPRemote{
			{Type: "streamable-http", URL: "https://a.example/mcp", Headers: []hdr{{Name: "X-First", Value: "1"}}},
			{Type: "sse", URL: "https://b.example/sse", Headers: []hdr{{Name: "X-Second", Value: "2"}}},
		},
	}
	raw, err := MCPClientEntry(MCPClientCopilot, multi, MCPToolChoice{All: true, URL: "https://b.example/sse"})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Type    string            `json:"type"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Type != "sse" || got.URL != "https://b.example/sse" || !maps.Equal(got.Headers, map[string]string{"X-Second": "2"}) {
		t.Errorf("multi remote entry = %s", raw)
	}

	gh := MCPServerEntry{
		Name: "io.github.navikt/github-mcp",
		Remotes: []MCPRemote{
			{Type: "streamable-http", URL: githubMCPURL},
			{Type: "streamable-http", URL: githubMCPReadonlyURL, Headers: []hdr{{Name: "X-MCP-Readonly", Value: "true"}}},
		},
	}
	raw, err = MCPClientEntry(MCPClientOpenCode, gh, MCPToolChoice{All: true})
	if err != nil {
		t.Fatal(err)
	}
	got = struct {
		Type    string            `json:"type"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}{}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.URL != githubMCPReadonlyURL || !maps.Equal(got.Headers, map[string]string{"X-MCP-Readonly": "true"}) {
		t.Errorf("github readonly entry = %s", raw)
	}
}

// RefreshMCPRegistry rewrites a cache that is still inside its day.
func TestRefreshMCPRegistryRewritesFreshCache(t *testing.T) {
	reg := testRegistry()
	reg.URL = "https://registry.test"
	reg.Entries = []MCPServerEntry{{Name: "com.figma/figma-mcp"}}
	mcpEnv(t, `{"mcpServers": {}}`, reg, nil)
	reg.Entries = append(reg.Entries, MCPServerEntry{Name: "io.github.navikt/newcomer"})
	fetchMCPRegistry = func(string) (mcpRegistry, error) { return reg, nil }
	if _, entries, _, err := MCPRegistryServers(); err != nil || len(entries) != 1 {
		t.Fatalf("fresh cache: %+v %v", entries, err)
	}
	if err := RefreshMCPRegistry(); err != nil {
		t.Fatal(err)
	}
	if _, entries, _, err := MCPRegistryServers(); err != nil || len(entries) != 2 {
		t.Errorf("after refresh: %+v %v", entries, err)
	}
}
