package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
)

func TestMCPNativeLocalRegistry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"servers":[{"server":{"name":"io.github.navikt/nais"},
			"_meta":{"io.github.navikt/registry":{"local":{"command":"nais","args":["mcp","--readonly"]},
			"sandboxHosts":["console.nav.cloud"]}}}]}`))
	}))
	defer srv.Close()
	reg, err := askMCPRegistry(srv.URL)
	if err != nil || len(reg.Entries) != 1 {
		t.Fatalf("native-only registry = %+v, %v", reg, err)
	}
	e := reg.Entries[0]
	if e.Local == nil || e.Local.Command != "nais" || !slices.Equal(e.Local.Args, []string{"mcp", "--readonly"}) {
		t.Fatalf("local metadata = %+v", e.Local)
	}
	if len(reg.Packages) != 0 || len(e.Packages) != 0 {
		t.Fatal("a native recipe became a package")
	}

	mcpEnv(t, "", reg, nil)
	cached, ok := readMCPRegistryCache()
	if !ok || cached.Schema != mcpRegistryCacheSchema || cached.Registry.Entries[0].Local == nil {
		t.Fatalf("cached native recipe = %+v, %v", cached, ok)
	}
	reg = cached.Registry
	e = reg.Entries[0]
	for _, tt := range []struct {
		name    string
		command []string
		want    bool
	}{
		{"exact", []string{"nais", "mcp", "--readonly"}, true},
		{"reordered", []string{"nais", "--readonly", "mcp"}, false},
		{"extra", []string{"nais", "mcp", "--readonly", "--write"}, false},
		{"missing", []string{"nais", "mcp"}, false},
		{"bare", []string{"nais"}, false},
		{"other executable", []string{"other", "mcp", "--readonly"}, false},
		{"absolute path", []string{"/usr/local/bin/nais", "mcp", "--readonly"}, false},
		{"relative path", []string{"./nais", "mcp", "--readonly"}, false},
		{"case", []string{"NAIS", "mcp", "--readonly"}, false},
		{"package runner", []string{"npx", "nais", "mcp", "--readonly"}, false},
		{"name only", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := mcpServer{Type: "local", Command: tt.command}
			if got := reg.listed(s); got != tt.want {
				t.Errorf("listed(%v) = %v, want %v", tt.command, got, tt.want)
			}
			for _, key := range []string{e.Name, "my-nais"} {
				if got := openCodeRegistryName(reg.Entries, key, s); (got == e.Name) != tt.want {
					t.Errorf("recognition(%s, %v) = %q", key, tt.command, got)
				}
				hosts := matchMCPHosts(reg, nil, map[string]mcpServer{key: s})
				if got := slices.Equal(hosts.Names(), []string{"console.nav.cloud"}); got != tt.want {
					t.Errorf("hosts(%s, %v) = %v", key, tt.command, hosts)
				}
			}
		})
	}
	if reg.listed(mcpServer{Command: []string{"nais", "mcp", "--readonly"}}) {
		t.Error("a missing type authorized a native recipe")
	}
	retired := reg
	retired.Entries = []MCPServerEntry{e}
	retired.Entries[0].Status = "deprecated"
	exact := mcpServer{Type: "local", Command: []string{"nais", "mcp", "--readonly"}}
	if retired.listed(exact) || openCodeRegistryName(retired.Entries, e.Name, exact) != "" {
		t.Fatal("a retired native entry must not authorize or recognize the command")
	}
	if len(matchMCPHosts(retired, []string{e.Name}, nil).Hosts) != 0 || len(matchMCPHosts(retired, nil, map[string]mcpServer{e.Name: exact}).Hosts) != 0 {
		t.Fatal("a retired native entry must not grant hosts")
	}
	if got := openCodeRegistryName(reg.Entries, e.Name, mcpServer{}); got != "" {
		t.Errorf("name alone recognized %q", got)
	}
	if got := matchMCPHosts(reg, []string{e.Name}, nil).Names(); !slices.Equal(got, []string{"console.nav.cloud"}) {
		t.Errorf("Copilot native hosts = %v", got)
	}
	if got := matchMCPHosts(reg, nil, map[string]mcpServer{e.Name: {}}).Names(); len(got) != 0 {
		t.Errorf("name alone granted hosts: %v", got)
	}
	servers := map[string]mcpServer{
		"exact": {Type: "local", Command: []string{"nais", "mcp", "--readonly"}},
		e.Name:  {Type: "local", Command: []string{"nais", "mcp", "--readonly", "--write"}},
	}
	if off, err := unlistedIn(reg.URL, servers, []string{"exact", e.Name}); err != nil || !slices.Equal(off, []string{e.Name}) {
		t.Errorf("policy verdict = %v, %v", off, err)
	}
	if off := false; len(matchMCPHosts(reg, nil, map[string]mcpServer{
		"disabled": {Type: "local", Command: servers["exact"].Command, Enabled: &off},
	}).Hosts) != 0 {
		t.Error("a disabled native server granted hosts")
	}
}

func TestMCPNativeLocalClientEntry(t *testing.T) {
	local := &MCPLocal{Command: "nais", Args: []string{"mcp", "--readonly"}}
	for _, tt := range []struct {
		client string
		want   string
	}{
		{MCPClientCopilot, `{"type":"local","command":"nais","args":["mcp","--readonly"],"tools":["*"]}`},
		{MCPClientOpenCode, `{"type":"local","command":["nais","mcp","--readonly"],"enabled":true}`},
	} {
		got, err := MCPClientEntry(tt.client, MCPServerEntry{Local: local}, MCPToolChoice{All: true})
		if err != nil || !jsonEqual(got, json.RawMessage(tt.want)) {
			t.Errorf("%s entry = %s, %v; want %s", tt.client, got, err, tt.want)
		}
	}
	for _, e := range []MCPServerEntry{{}, {Local: &MCPLocal{}}} {
		if _, err := MCPClientEntry(MCPClientOpenCode, e, MCPToolChoice{}); err == nil {
			t.Error("an empty local command must be refused")
		}
	}
	for _, e := range []MCPServerEntry{
		{Remotes: figma.Remotes},
		{Packages: []MCPPackage{{RegistryType: "npm", Identifier: "@playwright/mcp"}}},
		{Packages: []MCPPackage{{RegistryType: "oci"}}},
	} {
		want, wantErr := MCPClientEntry(MCPClientOpenCode, e, MCPToolChoice{})
		e.Local = local
		got, err := MCPClientEntry(MCPClientOpenCode, e, MCPToolChoice{})
		if (err == nil) != (wantErr == nil) || err == nil && !jsonEqual(got, want) {
			t.Errorf("local changed remote/package precedence: %s, %v; want %s, %v", got, err, want, wantErr)
		}
	}
}

func TestMCPNativeLocalConfigRecognition(t *testing.T) {
	home := mcpConfigEnv(t)
	e := MCPServerEntry{Name: "io.github.navikt/nais", Local: &MCPLocal{Command: "nais", Args: []string{"mcp"}}}
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeFile(t, path, `{"mcp":{"my-nais":{"type":"local","command":["nais","mcp"]}}}`)
	if got := MCPConfigKeyFor(MCPClientOpenCode, e); got != "my-nais" {
		t.Errorf("config key = %q", got)
	}
	if got := OpenCodeRegistryNameFor("my-nais", []MCPServerEntry{e}); got != e.Name {
		t.Errorf("registry name = %q", got)
	}
	if got := ConfiguredMCPServers([]MCPServerEntry{e}); !got.OpenCode[e.Name] {
		t.Errorf("configured native server missing: %+v", got)
	}
	writeFile(t, path, `{"mcp":{"io.github.navikt/nais":{"type":"local","command":["nais","other"]}}}`)
	if got := MCPConfigKeyFor(MCPClientOpenCode, e); got != "" {
		t.Errorf("name alone matched config key %q", got)
	}
	if got := ConfiguredMCPServers([]MCPServerEntry{e}); got.OpenCode[e.Name] {
		t.Errorf("name alone counted as configured: %+v", got)
	}
}

func TestMCPNativeLocalRefreshesOlderCache(t *testing.T) {
	reg := testRegistry()
	reg.Entries = []MCPServerEntry{{Name: "io.github.navikt/nais", Local: &MCPLocal{Command: "nais", Args: []string{"mcp"}}}}
	mcpEnv(t, `{"mcpServers":{"io.github.navikt/nais":{}}}`, reg, nil)
	c, _ := readMCPRegistryCache()
	c.Schema = mcpRegistryCacheSchema - 1
	c.Registry.Entries[0].Local = nil
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, mcpRegistryCachePath(), string(data))
	if _, entries, stale, err := MCPRegistryServers(); err != nil || stale != nil || len(entries) != 1 || entries[0].Local == nil {
		t.Fatalf("upgraded entries = %+v, %v, %v", entries, stale, err)
	}
}
