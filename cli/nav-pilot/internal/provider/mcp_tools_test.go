package provider

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
)

var intellij = MCPServerEntry{Name: "com.jetbrains/intellij", Remotes: []MCPRemote{{Type: "sse", URL: "http://127.0.0.1:64342/sse"}},
	Tools:    []string{"get_file_text_by_path", "reformat_file", "execute_terminal_command", "open_in_browser"},
	ToolRisk: map[string]string{"reformat_file": "write", "execute_terminal_command": "host-exec", "open_in_browser": "external"}}

// Reads and writes are the default; host-exec is never in it.
func TestMCPDefaultTools(t *testing.T) {
	if got := intellij.DefaultTools(); !slices.Equal(got, []string{"get_file_text_by_path", "reformat_file"}) {
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
