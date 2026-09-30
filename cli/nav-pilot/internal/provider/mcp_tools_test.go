package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var intellij = MCPServerEntry{Name: "com.jetbrains/intellij", Remotes: []MCPRemote{{Type: "sse", URL: "http://127.0.0.1:64342/sse"}},
	Tools:    []string{"read_file", "reformat_file", "execute_terminal_command", "open_in_browser"},
	ToolRisk: map[string]string{"reformat_file": "write", "execute_terminal_command": "host-exec", "open_in_browser": "external"}}

// Reads and writes are the default; host-exec is never in it.
func TestMCPDefaultTools(t *testing.T) {
	if got := intellij.DefaultTools(); !slices.Equal(got, []string{"read_file", "reformat_file"}) {
		t.Errorf("default = %v", got)
	}
	if got := intellij.HostExecTools(); !slices.Equal(got, []string{"execute_terminal_command"}) {
		t.Errorf("host-exec = %v", got)
	}
	if !intellij.Loopback() || intellij.GitHubReadonlyURL() != "" {
		t.Error("intellij is loopback and not GitHub")
	}
	gh := MCPServerEntry{Remotes: []MCPRemote{{URL: "https://api.githubcopilot.com/mcp/"}}}
	if gh.GitHubReadonlyURL() != "https://api.githubcopilot.com/mcp/readonly" {
		t.Errorf("github read-only = %q", gh.GitHubReadonlyURL())
	}
}

// OpenCode takes the last rule that matches: deny-all first, then the
// allows, is exactly the choice; a later user rule would win over it.
func TestOpenCodeRulesLastMatchWins(t *testing.T) {
	rules := OpenCodeRules("com.jetbrains/intellij", MCPToolChoice{Tools: []string{"reformat_file"}})
	want := [][2]string{{"com_jetbrains_intellij_*", "deny"}, {"com_jetbrains_intellij_reformat_file", "allow"}}
	if !slices.Equal(rules, want) {
		t.Fatalf("rules = %v", rules)
	}
	for id, on := range map[string]bool{
		"com_jetbrains_intellij_reformat_file":            true,
		"com_jetbrains_intellij_execute_terminal_command": false,
		"com_jetbrains_intellij_added_later":              false,
		"other_server_tool":                               true,
	} {
		if openCodeAllows(rules, id) != on {
			t.Errorf("%s: on = %v", id, !on)
		}
	}
	reversed := [][2]string{rules[1], rules[0]}
	if openCodeAllows(reversed, "com_jetbrains_intellij_reformat_file") {
		t.Error("allow before deny-all: the deny must win, or the order means nothing")
	}
	if !openCodeMatch("a_?b*", "a_xbyz") || openCodeMatch("a.b", "axb") {
		t.Error("wildcard: * and ? only, the rest literal")
	}
}

// What an entry turns on, read back from each client's file.
func TestMCPToolsOn(t *testing.T) {
	home := mcpConfigEnv(t)
	writeFile(t, filepath.Join(home, ".copilot", "mcp-config.json"), `{"mcpServers": {
		"a": {"url": "u", "tools": ["reformat_file"]}, "b": {"url": "u"}, "c": {"tools": ["*"]}}}`)
	for key, want := range map[string]MCPToolState{
		"a": {On: []string{"reformat_file"}, URL: "u"},
		"b": {All: true, On: intellij.Tools, URL: "u"},
		"c": {All: true, On: intellij.Tools},
	} {
		got, ok := MCPToolsOn(MCPClientCopilot, key, intellij)
		if !ok || got.All != want.All || !slices.Equal(got.On, want.On) || got.URL != want.URL {
			t.Errorf("copilot %s = %+v, want %+v", key, got, want)
		}
	}
	writeFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), `{"mcp": {"com.jetbrains/intellij": {"type": "remote"}},
		"permission": {"bash": {"*": "ask"}, "com_jetbrains_intellij_*": "deny", "com_jetbrains_intellij_reformat_file": "ask"}}`)
	got, ok := MCPToolsOn(MCPClientOpenCode, "com.jetbrains/intellij", intellij)
	if !ok || got.All || !slices.Equal(got.On, []string{"reformat_file"}) {
		t.Errorf("opencode = %+v", got)
	}
}

// A cache from before tool lists is read again: an old answer would let
// enable turn on every tool of a server it cannot class.
func TestMCPRegistryCacheSchema(t *testing.T) {
	reg := testRegistry()
	reg.Entries = []MCPServerEntry{intellij}
	mcpEnv(t, `{}`, reg, nil)
	c, _ := readMCPRegistryCache()
	c.Schema = 0
	data, _ := json.Marshal(c)
	writeFile(t, mcpRegistryCachePath(), string(data))
	asked := false
	fetchMCPRegistry = func(string) (mcpRegistry, error) { asked = true; return reg, nil }
	if _, entries, _, err := MCPRegistryServers(); err != nil || !asked || len(entries[0].ToolRisk) != 3 {
		t.Errorf("old cache: asked=%v entries=%+v err=%v", asked, entries, err)
	}
}

// A choice that names no URL never writes GitHub's full endpoint.
func TestMCPClientEntryGitHubDefaultsToReadonly(t *testing.T) {
	gh := MCPServerEntry{Name: "io.github.navikt/github-mcp", Remotes: []MCPRemote{{Type: "streamable-http", URL: githubMCPURL}}}
	for _, client := range []string{MCPClientCopilot, MCPClientOpenCode} {
		b, err := MCPClientEntry(client, gh, MCPToolChoice{All: true})
		if err != nil || !strings.Contains(string(b), githubMCPReadonlyURL) {
			t.Errorf("%s: %s, %v", client, b, err)
		}
		if b, _ := MCPClientEntry(client, gh, MCPToolChoice{All: true, URL: githubMCPURL}); strings.Contains(string(b), "readonly") {
			t.Errorf("%s: an explicit full URL was not kept: %s", client, b)
		}
	}
}

// permRules is the permission map of the OpenCode config, in file order.
func permRules(t *testing.T, home string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatal(err)
	}
	return string(compactJSON(top["permission"]))
}

// A user's ask stays an ask: nav-pilot's rules come last, and OpenCode
// takes the last match, so an allow would loosen it.
func TestSetMCPServerToolsKeepsAsk(t *testing.T) {
	p := openCodeToolPrefix(intellij.Name)
	for name, tc := range map[string]struct {
		perm string
		c    MCPToolChoice
		want string
	}{
		"every tool asks": {`{"*": "ask"}`, MCPToolChoice{Tools: []string{"read_file"}},
			`{"*":"ask","` + p + `*":"deny","` + p + `read_file":"ask"}`},
		"server asks": {`{"` + p + `*": "ask"}`, MCPToolChoice{Tools: []string{"read_file"}},
			`{"` + p + `*":"deny","` + p + `read_file":"ask"}`},
		"one tool asks": {`{"` + p + `reformat_file": "ask"}`, MCPToolChoice{Tools: []string{"read_file", "reformat_file"}},
			`{"` + p + `*":"deny","` + p + `read_file":"allow","` + p + `reformat_file":"ask"}`},
		"all tools, server asks": {`{"` + p + `*": "ask"}`, MCPToolChoice{All: true},
			`{"` + p + `*":"ask"}`},
		"a glob of the user's asks": {`{"` + p + `ref*": "ask"}`, MCPToolChoice{Tools: []string{"read_file", "reformat_file"}},
			`{"` + p + `ref*":"ask","` + p + `*":"deny","` + p + `read_file":"allow","` + p + `reformat_file":"ask"}`},
		"a later allow wins over *": {`{"*": "ask", "` + p + `*": "allow"}`, MCPToolChoice{Tools: []string{"read_file"}},
			`{"*":"ask","` + p + `*":"deny","` + p + `read_file":"allow"}`},
		"no ask": {`{"bash": "ask"}`, MCPToolChoice{Tools: []string{"read_file"}},
			`{"bash":"ask","` + p + `*":"deny","` + p + `read_file":"allow"}`},
	} {
		t.Run(name, func(t *testing.T) {
			home := mcpConfigEnv(t)
			writeFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"),
				`{"mcp": {"`+intellij.Name+`": {"type": "remote", "url": "http://127.0.0.1:64342/sse"}}, "permission": `+tc.perm+`}`)
			if _, err := SetMCPServerTools(MCPClientOpenCode, intellij.Name, intellij, tc.c); err != nil {
				t.Fatal(err)
			}
			if got := permRules(t, home); got != tc.want {
				t.Errorf("permission = %s\nwant       %s", got, tc.want)
			}
		})
	}
}

// The same choice twice writes once: the backup keeps the file from before
// the real change.
func TestSetMCPServerToolsUnchangedIsNoWrite(t *testing.T) {
	home := mcpConfigEnv(t)
	path := filepath.Join(home, ".copilot", "mcp-config.json")
	orig := `{"mcpServers": {"` + intellij.Name + `": {"type": "sse", "url": "http://127.0.0.1:64342/sse", "tools": ["*"]}}}`
	writeFile(t, path, orig)
	c := MCPToolChoice{Tools: []string{"read_file"}}
	for i, want := range []bool{true, false} {
		ch, err := SetMCPServerTools(MCPClientCopilot, intellij.Name, intellij, c)
		if err != nil || ch.Changed != want {
			t.Fatalf("call %d: changed = %v, %v; want %v", i, ch.Changed, err, want)
		}
	}
	if b, _ := os.ReadFile(path + ".bak"); string(b) != orig {
		t.Errorf("backup = %s", b)
	}
}

// disable takes the server's permission rules with it, and only its own:
// a_b's rules share a's prefix.
func TestRemoveMCPServerDropsItsRules(t *testing.T) {
	home := mcpConfigEnv(t)
	writeFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"),
		`{"mcp": {"a": {"type": "remote", "url": "https://a.test"}, "a_b": {"type": "remote", "url": "https://ab.test"}},
		"permission": {"bash": "ask", "a_*": "deny", "a_x": "allow", "a_b_*": "deny", "a_b_y": "allow"}}`)
	if _, err := RemoveMCPServer(MCPClientOpenCode, "a", nil); err != nil {
		t.Fatal(err)
	}
	if got := permRules(t, home); got != `{"bash":"ask","a_b_*":"deny","a_b_y":"allow"}` {
		t.Errorf("permission = %s", got)
	}
	if _, err := RemoveMCPServer(MCPClientOpenCode, "a_b", nil); err != nil {
		t.Fatal(err)
	}
	if got := permRules(t, home); got != `{"bash":"ask"}` {
		t.Errorf("permission = %s", got)
	}
}

// OpenCode's own keys that look like a server's rules stay: "external_" is
// the prefix of external_directory. With the tool list known, only the
// server's deny-all and tool keys go.
func TestRemoveMCPServerKeepsOpenCodeKeys(t *testing.T) {
	home := mcpConfigEnv(t)
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeFile(t, path, `{"mcp": {"external": {"type": "remote", "url": "https://x.test"}, "doom": {"type": "remote", "url": "https://d.test"}},
		"permission": {"external_directory": "ask", "external_*": "deny", "external_x": "allow", "doom_loop": "ask", "doom_*": "deny", "doom_a": "allow", "doom_custom": "allow"}}`)
	if _, err := RemoveMCPServer(MCPClientOpenCode, "external", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := RemoveMCPServer(MCPClientOpenCode, "doom", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if got := permRules(t, home); got != `{"external_directory":"ask","doom_loop":"ask","doom_custom":"allow"}` {
		t.Errorf("permission = %s", got)
	}
}
