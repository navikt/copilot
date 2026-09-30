package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

func openCodeMCPPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// copilotEntry is the server's entry in Copilot's config.
func copilotEntry(t *testing.T, name string) map[string]any {
	t.Helper()
	var cfg struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	b, _ := os.ReadFile(copilotMCPPath())
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("copilot config: %v\n%s", err, b)
	}
	return cfg.MCPServers[name]
}

// openCodeRules is OpenCode's permission map in file order.
func openCodeRulesInFile(t *testing.T) [][2]string {
	t.Helper()
	b, _ := os.ReadFile(openCodeMCPPath())
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatalf("opencode config: %v\n%s", err, b)
	}
	dec := json.NewDecoder(bytes.NewReader(top["permission"]))
	var out [][2]string
	if _, err := dec.Token(); err != nil {
		return nil
	}
	for dec.More() {
		k, _ := dec.Token()
		var v string
		_ = dec.Decode(&v)
		out = append(out, [2]string{k.(string), v})
	}
	return out
}

func tools(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func mcpEnable(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var err error
	out := captureStdout(func() { err = cmdMCP(append([]string{"enable"}, args...)) })
	return out, err
}

// The default is reads and writes: host-exec and external tools stay off in
// both clients, and GitHub gets its read-only endpoint.
func TestMCPEnableWritesTheDefaultTools(t *testing.T) {
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	for _, client := range []string{"copilot", "opencode"} {
		if _, err := mcpEnable(t, "intellij", "--client", client); err != nil {
			t.Fatal(err)
		}
		if _, err := mcpEnable(t, "github-mcp", "--client", client); err != nil {
			t.Fatal(err)
		}
	}
	ij := copilotEntry(t, "com.jetbrains/intellij")
	if got := tools(ij["tools"]); got != `["get_file_text_by_path","reformat_file"]` {
		t.Errorf("copilot intellij tools = %s", got)
	}
	gh := copilotEntry(t, "io.github.navikt/github-mcp")
	if gh["url"] != "https://api.githubcopilot.com/mcp/readonly" || tools(gh["tools"]) != `["*"]` {
		t.Errorf("copilot github = %v", gh)
	}
	want := [][2]string{
		{"com_jetbrains_intellij_*", "deny"},
		{"com_jetbrains_intellij_get_file_text_by_path", "allow"},
		{"com_jetbrains_intellij_reformat_file", "allow"},
	}
	if got := openCodeRulesInFile(t); !slices.Equal(got, want) {
		t.Errorf("opencode rules = %v, want %v", got, want)
	}
	b, _ := os.ReadFile(openCodeMCPPath())
	if !strings.Contains(string(b), `"url": "https://api.githubcopilot.com/mcp/readonly"`) {
		t.Errorf("opencode github is not read-only:\n%s", b)
	}
	if keys := strings.Index(string(b), `"mcp"`); keys < 0 || keys > strings.Index(string(b), `"permission"`) {
		t.Errorf("a new file has the servers before the rules:\n%s", b)
	}
}

// --tools is exactly those; a name the registry does not list is refused;
// --all-tools is every tool, and for GitHub the full endpoint.
func TestMCPEnableToolsFlags(t *testing.T) {
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	if _, err := mcpEnable(t, "intellij", "--client", "copilot", "--tools", "get_file_text_by_path"); err != nil {
		t.Fatal(err)
	}
	if got := tools(copilotEntry(t, "com.jetbrains/intellij")["tools"]); got != `["get_file_text_by_path"]` {
		t.Errorf("tools = %s", got)
	}
	if _, err := mcpEnable(t, "intellij", "--client", "copilot", "--tools", "nope"); err == nil {
		t.Error("an unknown tool was accepted")
	}
	out, err := mcpEnable(t, "github-mcp", "--client", "copilot", "--all-tools")
	if err != nil {
		t.Fatal(err)
	}
	gh := copilotEntry(t, "io.github.navikt/github-mcp")
	if gh["url"] != "https://api.githubcopilot.com/mcp/" || tools(gh["tools"]) != `["*"]` || !strings.Contains(out, "guard on gh") {
		t.Errorf("github = %v\n%s", gh, out)
	}
	// Only reads is the read-only endpoint again, the default form.
	if _, err := mcpEnable(t, "github-mcp", "--client", "copilot", "--tools", "get_file_contents"); err != nil {
		t.Fatal(err)
	}
	if gh = copilotEntry(t, "io.github.navikt/github-mcp"); gh["url"] != "https://api.githubcopilot.com/mcp/readonly" {
		t.Errorf("github = %v", gh)
	}
	for _, args := range [][]string{{"list", "--tools", "a"}, {"disable", "x", "--all-tools"}, {"enable", "x", "--tools", "a", "--all-tools"}} {
		if err := cmdMCP(args); err == nil {
			t.Errorf("%v: want a refusal", args)
		}
	}
}

// A host-exec tool is never on without a yes: off a terminal --tools alone
// is refused, --allow-host-exec turns it on, and in a terminal a no leaves
// it off whichever way it was picked.
func TestMCPEnableHostExecNeedsAYes(t *testing.T) {
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	ij := mcpTestEntries()[2]
	for _, o := range []mcpToolOpts{{tools: []string{"execute_terminal_command"}}, {all: true}} {
		if _, err := mcpChooseTools(ij, o); err == nil || !strings.Contains(err.Error(), "--allow-host-exec") {
			t.Errorf("%+v off a terminal: err = %v", o, err)
		}
		if _, err := mcpEnable(t, "intellij", "--client", "copilot", "--all-tools"); err == nil {
			t.Error("enable --all-tools off a terminal: want a refusal")
		}
	}
	if _, err := os.Stat(copilotMCPPath()); err == nil {
		t.Error("a refused enable wrote the config")
	}
	if _, err := mcpEnable(t, "intellij", "--client", "copilot", "--tools", "execute_terminal_command", "--allow-host-exec"); err != nil {
		t.Fatal(err)
	}
	if got := tools(copilotEntry(t, "com.jetbrains/intellij")["tools"]); got != `["execute_terminal_command"]` {
		t.Errorf("tools = %s", got)
	}

	isInteractive = func() bool { return true }
	prevPick, prevAsk := mcpPickTools, mcpAsk
	t.Cleanup(func() { mcpPickTools, mcpAsk = prevPick, prevAsk })
	mcpPickTools = func(e providerpkg.MCPServerEntry) ([]string, error) { return e.Tools, nil }
	for _, tc := range []struct {
		args []string
		yes  bool
		want string
	}{
		{nil, false, `["get_file_text_by_path","reformat_file"]`},
		{nil, true, `["get_file_text_by_path","reformat_file","execute_terminal_command"]`},
		{[]string{"--all-tools"}, false, `["get_file_text_by_path","reformat_file"]`},
		{[]string{"--all-tools"}, true, `["*"]`},
		{[]string{"--tools", "execute_terminal_command"}, false, `[]`},
	} {
		_ = os.Remove(copilotMCPPath())
		asked := ""
		mcpAsk = func(title string) bool { asked = title; return tc.yes }
		if _, err := mcpEnable(t, append([]string{"intellij", "--client", "copilot"}, tc.args...)...); err != nil {
			t.Fatal(err)
		}
		if got := tools(copilotEntry(t, "com.jetbrains/intellij")["tools"]); got != tc.want {
			t.Errorf("%v, yes=%v: tools = %s, want %s", tc.args, tc.yes, got, tc.want)
		}
		if !strings.Contains(asked, "outside the cplt sandbox") {
			t.Errorf("%v: asked %q", tc.args, asked)
		}
	}
}

// An entry already there is kept as it is unless tools are chosen; then
// only its tools change, the rest of it stays, and a backup is left.
func TestMCPEnableKeepsAnExistingEntry(t *testing.T) {
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	orig := `{"mcpServers": {"com.jetbrains/intellij": {"type": "sse", "url": "http://127.0.0.1:64342/sse", "headers": {"X": "y"}, "tools": ["*"]}}}`
	writeTestFile(t, copilotMCPPath(), orig)
	if _, err := mcpEnable(t, "intellij", "--client", "copilot"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(copilotMCPPath()); string(b) != orig {
		t.Errorf("enable without tool flags changed the entry:\n%s", b)
	}
	if _, err := mcpEnable(t, "intellij", "--client", "copilot", "--tools", "get_file_text_by_path"); err != nil {
		t.Fatal(err)
	}
	e := copilotEntry(t, "com.jetbrains/intellij")
	if tools(e["tools"]) != `["get_file_text_by_path"]` || tools(e["headers"]) != `{"X":"y"}` || e["url"] != "http://127.0.0.1:64342/sse" {
		t.Errorf("entry = %v", e)
	}
	if b, _ := os.ReadFile(copilotMCPPath() + ".bak"); string(b) != orig {
		t.Errorf("backup = %s", b)
	}

	// OpenCode: the user's own rules stay; nav-pilot's move to the end, deny
	// first, so the last match is the choice.
	writeTestFile(t, openCodeMCPPath(), `{"mcp": {"com.jetbrains/intellij": {"type": "remote", "url": "http://127.0.0.1:64342/sse"}},
		"permission": {"com_jetbrains_intellij_execute_terminal_command": "allow", "bash": "ask", "com_jetbrains_intellij_*": "deny"}}`)
	if _, err := mcpEnable(t, "intellij", "--client", "opencode", "--tools", "reformat_file"); err != nil {
		t.Fatal(err)
	}
	want := [][2]string{{"bash", "ask"}, {"com_jetbrains_intellij_*", "deny"}, {"com_jetbrains_intellij_reformat_file", "allow"}}
	if got := openCodeRulesInFile(t); !slices.Equal(got, want) {
		t.Errorf("rules = %v, want %v", got, want)
	}
	if _, err := mcpEnable(t, "intellij", "--client", "opencode", "--all-tools", "--allow-host-exec"); err != nil {
		t.Fatal(err)
	}
	if got := openCodeRulesInFile(t); !slices.Equal(got, [][2]string{{"bash", "ask"}}) {
		t.Errorf("--all-tools rules = %v", got)
	}
}

// list shows the tools that are on, flags host-exec and GitHub's full
// endpoint, and gives the command that narrows them; a localhost port whose
// server has host-exec tools on is a warning that narrows first.
func TestMCPListToolNudges(t *testing.T) {
	f := mcpCmdEnv(t, providerpkg.MCPHostState{})
	f.verdicts["127.0.0.1:64342"] = "BLOCKED-PORT"
	writeTestFile(t, copilotMCPPath(), `{"mcpServers": {
		"com.jetbrains/intellij": {"type": "sse", "url": "http://127.0.0.1:64342/sse", "tools": ["*"]},
		"io.github.navikt/github-mcp": {"type": "http", "url": "https://api.githubcopilot.com/mcp/", "tools": ["*"]}}}`)
	out := captureStdout(func() { _ = cmdMCP([]string{"list", "--json"}) })
	var rep mcpReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	narrow := "nav-pilot mcp enable com.jetbrains/intellij --client copilot --tools get_file_text_by_path,reformat_file"
	var ij, gh []mcpProblem
	for _, p := range rep.Problems {
		switch p.Server {
		case "com.jetbrains/intellij":
			ij = append(ij, p)
		case "io.github.navikt/github-mcp":
			gh = append(gh, p)
		}
	}
	if len(ij) != 1 || ij[0].Fix != narrow+" && cplt config set allow.localhost 64342" || !strings.Contains(ij[0].Problem, "execute_terminal_command, which runs on your machine outside the sandbox") {
		t.Errorf("intellij problems = %+v", ij)
	}
	if len(gh) != 1 || gh[0].Fix != "nav-pilot mcp enable io.github.navikt/github-mcp --client copilot --tools get_file_contents" {
		t.Errorf("github problems = %+v", gh)
	}
	for _, s := range rep.Servers {
		switch s.Name {
		case "com.jetbrains/intellij":
			if s.Tools != "all ⚠ host-exec" || !slices.Equal(s.HostExec, []string{"execute_terminal_command"}) {
				t.Errorf("intellij row = %+v", s)
			}
		case "io.github.navikt/github-mcp":
			if s.Tools != "all ⚠ full endpoint" {
				t.Errorf("github row = %+v", s)
			}
		}
	}

	// The port open: the nudge stands on its own. Narrowed: no nudge.
	f.verdicts["127.0.0.1:64342"] = "ALLOWED"
	rep = diagnoseMCP("r", mcpTestEntries(), mcpConfigured(mcpTestEntries()), "com.jetbrains/intellij")
	if len(rep.Problems) != 1 || rep.Problems[0].Fix != narrow {
		t.Errorf("problems = %+v", rep.Problems)
	}
	if _, err := mcpEnable(t, "intellij", "--client", "copilot", "--tools", "get_file_text_by_path,reformat_file"); err != nil {
		t.Fatal(err)
	}
	rep = diagnoseMCP("r", mcpTestEntries(), mcpConfigured(mcpTestEntries()), "com.jetbrains/intellij")
	if len(rep.Problems) != 0 || rep.Servers[0].Tools != "2 of 3" {
		t.Errorf("narrowed: %+v", rep)
	}
}

// doctor gives the same nudge, and its localhost hint warns while a
// host-exec tool is on.
func TestDoctorToolNudges(t *testing.T) {
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	prev := mcpCachedEntries
	mcpCachedEntries = mcpTestEntries
	t.Cleanup(func() { mcpCachedEntries = prev })
	writeTestFile(t, copilotMCPPath(), `{"mcpServers": {"com.jetbrains/intellij": {"type": "sse", "url": "http://127.0.0.1:64342/sse"}}}`)
	var out bytes.Buffer
	reportMCPTools(&out)
	if !strings.Contains(out.String(), "has every tool on, including execute_terminal_command") ||
		!strings.Contains(out.String(), "nav-pilot mcp enable com.jetbrains/intellij --client copilot --tools get_file_text_by_path,reformat_file") {
		t.Errorf("doctor:\n%s", out.String())
	}
	if w := mcpLoopbackNote("com.jetbrains/intellij", "64342"); !strings.Contains(w, "opening localhost:64342 lets the agent reach execute_terminal_command") {
		t.Errorf("loopback note = %q", w)
	}
	writeTestFile(t, copilotMCPPath(), `{"mcpServers": {"com.jetbrains/intellij": {"type": "sse", "url": "http://127.0.0.1:64342/sse", "tools": ["reformat_file"]}}}`)
	out.Reset()
	reportMCPTools(&out)
	if out.Len() != 0 || mcpLoopbackNote("com.jetbrains/intellij", "64342") != "" {
		t.Errorf("narrowed, doctor still nudges:\n%s", out.String())
	}
}

// Host-exec tools on in both clients: the localhost fix narrows both before
// it opens the port.
func TestMCPListLoopbackNarrowsEveryClient(t *testing.T) {
	f := mcpCmdEnv(t, providerpkg.MCPHostState{})
	f.verdicts["127.0.0.1:64342"] = "BLOCKED-PORT"
	writeTestFile(t, copilotMCPPath(), `{"mcpServers": {"com.jetbrains/intellij": {"type": "sse", "url": "http://127.0.0.1:64342/sse", "tools": ["*"]}}}`)
	writeTestFile(t, openCodeMCPPath(), `{"mcp": {"com.jetbrains/intellij": {"type": "remote", "url": "http://127.0.0.1:64342/sse"}}}`)
	rep := diagnoseMCP("r", mcpTestEntries(), mcpConfigured(mcpTestEntries()), "com.jetbrains/intellij")
	tools := " --tools get_file_text_by_path,reformat_file"
	want := "nav-pilot mcp enable com.jetbrains/intellij --client copilot" + tools +
		" && nav-pilot mcp enable com.jetbrains/intellij --client opencode" + tools +
		" && cplt config set allow.localhost 64342"
	if len(rep.Problems) != 1 || rep.Problems[0].Fix != want {
		t.Errorf("problems = %+v", rep.Problems)
	}
}

// enable without a tool flag keeps an existing entry, so it must not say the
// risky tools are off.
func TestMCPEnableExistingEntryNoToolsHint(t *testing.T) {
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	writeTestFile(t, copilotMCPPath(), `{"mcpServers": {"com.jetbrains/intellij": {"type": "sse", "url": "http://127.0.0.1:64342/sse", "tools": ["*"]}}}`)
	out, err := mcpEnable(t, "intellij", "--client", "copilot")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "are off") {
		t.Errorf("hint for a kept entry:\n%s", out)
	}
}
