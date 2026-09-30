package cli

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

func mcpTestEntries() []providerpkg.MCPServerEntry {
	pw := providerpkg.MCPServerEntry{Name: "com.microsoft/playwright-mcp", Packages: []providerpkg.MCPPackage{{RegistryType: "npm", Identifier: "@playwright/mcp", Version: "0.0.80"}},
		Setup: []providerpkg.MCPSetupStep{
			{Title: "Install the browser", Commands: []string{"pnpm dlx @playwright/mcp@0.0.80 install-browser chromium"}},
			{Title: "cache exec", Commands: []string{"cplt config set sandbox.allow_cache_exec ms-playwright", "cplt config set sandbox.allow_cache_exec pnpm/dlx"}},
		}}
	return []providerpkg.MCPServerEntry{
		{Name: "com.figma/figma-mcp", Description: "Figma", Remotes: []providerpkg.MCPRemote{{Type: "streamable-http", URL: "https://mcp.figma.com/mcp"}}, SandboxHosts: []string{"api.figma.com"}},
		{Name: "io.github.navikt/mcp-onboarding", Remotes: []providerpkg.MCPRemote{{URL: "https://mcp-onboarding.intern.nav.no/mcp"}}},
		{Name: "com.jetbrains/intellij", Remotes: []providerpkg.MCPRemote{{Type: "sse", URL: "http://127.0.0.1:64342/sse"}},
			Tools:    []string{"read_file", "reformat_file", "execute_terminal_command"},
			ToolRisk: map[string]string{"reformat_file": "write", "execute_terminal_command": "host-exec"}},
		{Name: "io.github.navikt/github-mcp", Remotes: []providerpkg.MCPRemote{{Type: "streamable-http", URL: "https://api.githubcopilot.com/mcp/"}},
			Tools:    []string{"get_file_contents", "issue_write"},
			ToolRisk: map[string]string{"issue_write": "external"}},
		{Name: "io.example/old", Status: "deprecated", Remotes: []providerpkg.MCPRemote{{URL: "https://old.example/mcp"}}},
		{Name: "io.example/unused", Remotes: []providerpkg.MCPRemote{{URL: "https://unused.example/mcp"}}},
		pw,
	}
}

type mcpFakes struct {
	verdicts map[string]string // cplt check net target -> probe name
	config   map[string]string // cplt config get key -> value
	probes   []string
	recorded [][]providerpkg.MCPHost
}

// mcpCmdEnv isolates HOME, answers the registry, cplt and the consent record
// with fakes, and starts without a terminal.
func mcpCmdEnv(t *testing.T, st providerpkg.MCPHostState) *mcpFakes {
	t.Helper()
	consentEnv(t)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Chdir(t.TempDir())
	isInteractive = func() bool { return false }
	f := &mcpFakes{verdicts: map[string]string{}, config: map[string]string{}}
	prev := []any{mcpRegistryServers, mcpCpltPath, mcpCpltConfigGet, mcpLookPath, mcpProbeRun, readMCPHostState, recordMCPHosts, notedMCPHosts, narrowMCPApproval}
	mcpRegistryServers = func() (string, []providerpkg.MCPServerEntry, error, error) {
		return "https://registry.test", mcpTestEntries(), nil, nil
	}
	narrowMCPApproval = func() ([]string, error) { return nil, nil }
	mcpCpltPath = func() string { return "/fake/cplt" }
	mcpCpltConfigGet = func(_, key string) string { return f.config[key] }
	mcpLookPath = func(string) (string, error) { return "", os.ErrNotExist }
	var mu = make(chan struct{}, 1)
	mcpProbeRun = func(_ string, args []string) (string, string) {
		mu <- struct{}{}
		f.probes = append(f.probes, strings.Join(args, " "))
		<-mu
		v, ok := f.verdicts[args[len(args)-1]]
		if !ok {
			v = "ALLOWED"
		}
		return v, "cplt's own fix"
	}
	readMCPHostState = func() (providerpkg.MCPHostState, error) { return st, nil }
	recordMCPHosts = func(h []providerpkg.MCPHost, approve bool) error {
		if !approve {
			t.Error("recorded a decline")
		}
		f.recorded = append(f.recorded, h)
		return nil
	}
	notedMCPHosts = false
	prevInstalled := mcpClientInstalled
	mcpClientInstalled = func(c string) bool { return c == "copilot" }
	t.Cleanup(func() {
		mcpClientInstalled = prevInstalled
		mcpRegistryServers = prev[0].(func() (string, []providerpkg.MCPServerEntry, error, error))
		narrowMCPApproval = prev[8].(func() ([]string, error))
		mcpCpltPath = prev[1].(func() string)
		mcpCpltConfigGet = prev[2].(func(string, string) string)
		mcpLookPath = prev[3].(func(string) (string, error))
		mcpProbeRun = prev[4].(func(string, []string) (string, string))
		readMCPHostState = prev[5].(func() (providerpkg.MCPHostState, error))
		recordMCPHosts = prev[6].(func([]providerpkg.MCPHost, bool) error)
		notedMCPHosts = prev[7].(bool)
	})
	return f
}

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func copilotMCPPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".copilot", "mcp-config.json")
}

// Every failure mode list can see gets its own line and the exact fix.
func TestMCPListDiagnosesEachFailureMode(t *testing.T) {
	f := mcpCmdEnv(t, providerpkg.MCPHostState{
		Pending: []providerpkg.MCPHost{{Host: "mcp-onboarding.intern.nav.no", Private: true}},
	})
	f.verdicts["mcp.figma.com:443"] = "BLOCKED-ALLOWLIST"
	f.verdicts["api.figma.com:443"] = "BLOCKED-ALLOWLIST"
	f.verdicts["mcp-onboarding.intern.nav.no:443"] = "BLOCKED-PRIVATE-RESOLVED"
	f.verdicts["127.0.0.1:64342"] = "BLOCKED-PORT"
	f.config["sandbox.allow_cache_exec"] = `["ms-playwright"]`
	writeTestFile(t, copilotMCPPath(), `{"mcpServers": {
		"com.figma/figma-mcp": {}, "io.github.navikt/mcp-onboarding": {}, "com.jetbrains/intellij": {"tools": ["read_file"]},
		"io.example/old": {}, "com.microsoft/playwright-mcp": {}, "playwright-mcp": {}, "homemade": {}}}`)

	out := captureStdout(func() {
		if err := cmdMCP([]string{"list", "--json"}); err != nil {
			t.Fatal(err)
		}
	})
	var rep mcpReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("--json is not JSON: %v\n%s", err, out)
	}
	fixes := map[string][]string{}
	for _, p := range rep.Problems {
		fixes[p.Server] = append(fixes[p.Server], p.Fix)
	}
	for server, want := range map[string]string{
		"com.figma/figma-mcp":             "cplt config set allow.domains mcp.figma.com",
		"io.github.navikt/mcp-onboarding": "start nav-pilot in a terminal; it asks whether to allow it",
		"com.jetbrains/intellij":          "cplt config set allow.localhost 64342",
		"io.example/old":                  "nav-pilot mcp disable io.example/old",
		"com.microsoft/playwright-mcp":    "cplt config set sandbox.allow_cache_exec pnpm/dlx",
		"playwright-mcp":                  "nav-pilot mcp disable playwright-mcp --client copilot && nav-pilot mcp enable com.microsoft/playwright-mcp --client copilot",
		"homemade":                        "nav-pilot mcp disable homemade --client copilot",
	} {
		if !slices.ContainsFunc(fixes[server], func(f string) bool { return strings.HasPrefix(f, want) }) {
			t.Errorf("%s: fixes %q, want one starting %q", server, fixes[server], want)
		}
	}
	// The registry's sandboxHosts get a fix line of their own, like a remote.
	if !slices.Contains(fixes["com.figma/figma-mcp"], "cplt config set allow.domains api.figma.com") {
		t.Errorf("figma: fixes %q, want one for api.figma.com too", fixes["com.figma/figma-mcp"])
	}
	if !slices.Contains(fixes["com.microsoft/playwright-mcp"], "npm install -g pnpm") {
		t.Errorf("missing pnpm not reported: %q", fixes["com.microsoft/playwright-mcp"])
	}
	// ms-playwright is set, pnpm/dlx is not: one line for it, not two.
	if n := slices.Index(fixes["com.microsoft/playwright-mcp"], "cplt config set sandbox.allow_cache_exec ms-playwright"); n >= 0 {
		t.Errorf("reported a cache exec the config already has: %q", fixes["com.microsoft/playwright-mcp"])
	}
	// Only configured servers are probed, never the rest of the registry.
	for _, p := range f.probes {
		if strings.Contains(p, "unused.example") {
			t.Errorf("probed a server nobody configured: %s", p)
		}
		if !strings.Contains(p, "check --json net --no-connect") {
			t.Errorf("probe is not static: %s", p)
		}
	}
	i := slices.IndexFunc(rep.Servers, func(s mcpServerRow) bool { return s.Name == "com.figma/figma-mcp" })
	if i < 0 || !rep.Servers[i].Copilot || rep.Servers[i].OpenCode || rep.Servers[i].Hosts[0].Cplt != "BLOCKED-ALLOWLIST" {
		t.Errorf("figma row = %+v", rep.Servers)
	}
}

// A granted private host is probed with the flag the launch adds, and a
// host the user turned off points at the key, not at cplt.
func TestMCPListGrantAndOff(t *testing.T) {
	f := mcpCmdEnv(t, providerpkg.MCPHostState{Grant: []providerpkg.MCPHost{{Host: "mcp-onboarding.intern.nav.no", Private: true}}})
	writeTestFile(t, copilotMCPPath(), `{"mcpServers": {"io.github.navikt/mcp-onboarding": {}, "com.figma/figma-mcp": {}}}`)
	_ = captureStdout(func() { _ = cmdMCP([]string{"list"}) })
	if !slices.ContainsFunc(f.probes, func(p string) bool {
		return strings.HasPrefix(p, "--allow-private-domain mcp-onboarding.intern.nav.no ") && strings.HasSuffix(p, "mcp-onboarding.intern.nav.no:443")
	}) {
		t.Errorf("granted host not probed with its launch flag: %q", f.probes)
	}

	writeTestFile(t, configPath(), "mcp_hosts = \"off\"\n")
	f.verdicts["mcp.figma.com:443"] = "BLOCKED-ALLOWLIST"
	rep := diagnoseMCP("r", mcpTestEntries(), mcpConfigured(mcpTestEntries()), "com.figma/figma-mcp")
	if len(rep.Problems) != 1 || rep.Problems[0].Fix != "nav-pilot config set mcp_hosts ask" {
		t.Errorf("mcp_hosts = off: %+v", rep.Problems)
	}
}

// enable writes the registry's entry next to the user's own servers, keeps a
// backup, asks nothing without a terminal, and a second run changes nothing.
func TestMCPEnable(t *testing.T) {
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	orig := `{"mcpServers": {"mine": {"type": "local", "command": "x"}}}`
	writeTestFile(t, copilotMCPPath(), orig)
	out := captureStdout(func() {
		if err := cmdMCP([]string{"enable", "figma-mcp", "--client", "copilot"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Enabled") {
		t.Errorf("output:\n%s", out)
	}
	if got := providerpkg.MCPConfigKeys("copilot"); strings.Join(got, ",") != "mine,com.figma/figma-mcp" {
		t.Errorf("keys = %v", got)
	}
	if b, _ := os.ReadFile(copilotMCPPath() + ".bak"); string(b) != orig {
		t.Errorf("backup = %q", b)
	}
	before, _ := os.ReadFile(copilotMCPPath())
	out = captureStdout(func() { _ = cmdMCP([]string{"enable", "com.figma/figma-mcp", "--client", "copilot"}) })
	if after, _ := os.ReadFile(copilotMCPPath()); string(after) != string(before) || !strings.Contains(out, "already enabled") {
		t.Errorf("second enable changed the file or said so:\n%s", out)
	}

	for _, name := range []string{"nope", "io.example/old"} {
		if err := cmdMCP([]string{"enable", name, "--client", "copilot"}); err == nil {
			t.Errorf("enable %s: want a refusal", name)
		}
	}
	if keys := providerpkg.MCPConfigKeys("copilot"); len(keys) != 2 {
		t.Errorf("a refused enable wrote: %v", keys)
	}
}

// disable removes the server, narrows the host approval and names the
// hosts dropped (which ones is NarrowMCPApproval's, tested in provider).
func TestMCPDisableNarrowsTheApproval(t *testing.T) {
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	narrowed := 0
	narrowMCPApproval = func() ([]string, error) { narrowed++; return []string{"mcp.figma.com"}, nil }
	writeTestFile(t, copilotMCPPath(), `{"mcpServers": {"com.figma/figma-mcp": {}, "io.github.navikt/mcp-onboarding": {}}}`)
	out := captureStdout(func() {
		if err := cmdMCP([]string{"disable", "figma-mcp"}); err != nil {
			t.Fatal(err)
		}
	})
	if keys := providerpkg.MCPConfigKeys("copilot"); strings.Join(keys, ",") != "io.github.navikt/mcp-onboarding" {
		t.Errorf("keys = %v", keys)
	}
	if narrowed != 1 || !strings.Contains(out, "mcp.figma.com") {
		t.Errorf("narrowed %d times; output:\n%s", narrowed, out)
	}
}

// An allowed host whose lookup timed out is not waived as private yet; list
// says so and how it gets fixed, and the JSON carries the classification.
func TestMCPListUnknownHost(t *testing.T) {
	f := mcpCmdEnv(t, providerpkg.MCPHostState{Grant: []providerpkg.MCPHost{{Host: "mcp-onboarding.intern.nav.no", Unknown: true}}})
	writeTestFile(t, copilotMCPPath(), `{"mcpServers": {"io.github.navikt/mcp-onboarding": {}}}`)
	f.verdicts["mcp-onboarding.intern.nav.no:443"] = "BLOCKED-PRIVATE-RESOLVED"
	rep := diagnoseMCP("r", mcpTestEntries(), mcpConfigured(mcpTestEntries()), "io.github.navikt/mcp-onboarding")
	if len(rep.Servers) != 1 || len(rep.Servers[0].Hosts) != 1 || rep.Servers[0].Hosts[0].DNS != "unknown" {
		t.Fatalf("servers = %+v", rep.Servers)
	}
	if len(rep.Problems) != 1 || !strings.Contains(rep.Problems[0].Problem, "DNS lookup failed") || !strings.Contains(rep.Problems[0].Fix, "naisdevice") {
		t.Errorf("problems = %+v", rep.Problems)
	}
	if slices.ContainsFunc(f.probes, func(p string) bool { return strings.HasPrefix(p, "--allow-private-domain") }) {
		t.Errorf("an unknown host was probed with the private waiver: %q", f.probes)
	}
}

// A stale registry answer is used and said to be old; a project's OpenCode
// server is mentioned and not counted.
func TestMCPListStaleAndProjectConfig(t *testing.T) {
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	mcpRegistryServers = func() (string, []providerpkg.MCPServerEntry, error, error) {
		return "https://registry.test", mcpTestEntries(), errors.New("timeout; showing its answer from 2026-09-01 10:00"), nil
	}
	wd, _ := os.Getwd()
	writeTestFile(t, filepath.Join(wd, "opencode.json"), `{"mcp": {"repo-pick": {"type": "remote", "url": "https://mcp.figma.com/mcp"}}}`)
	out := captureStdout(func() {
		if err := cmdMCP([]string{"list"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "could not be read again (timeout; showing its answer from 2026-09-01 10:00)") || !strings.Contains(out, "OpenCode also loads repo-pick") {
		t.Errorf("output:\n%s", out)
	}
	if c := mcpConfigured(mcpTestEntries()); len(c.OpenCode) != 0 {
		t.Errorf("the project's server counted as configured: %v", c.OpenCode)
	}
}

// The mcp command never runs outside `nav-pilot mcp`: nothing on the launch
// path or in another command references it or the client-config writers.
func TestMCPCommandIsOffTheHotPath(t *testing.T) {
	fset := token.NewFileSet()
	own := map[string]bool{}
	file, err := parser.ParseFile(fset, "mcp_cmd.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				own[d.Name.Name] = true
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				if v, ok := s.(*ast.ValueSpec); ok {
					for _, n := range v.Names {
						own[n.Name] = true
					}
				}
			}
		}
	}
	writers := map[string]bool{"MCPRegistryServers": true, "SetMCPServer": true, "RemoveMCPServer": true, "MCPConfigKeys": true, "ConfiguredMCPServers": true, "MCPClientEntry": true, "MCPConfigKeyFor": true, "NarrowMCPApproval": true, "SetMCPServerTools": true}
	var files []string
	for _, dir := range []string{".", "../provider"} {
		m, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		files = append(files, m...)
	}
	for _, path := range files {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || base == "mcp_cmd.go" || base == "mcp_servers.go" || base == "mcp_tools.go" {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if writers[n.Sel.Name] {
					t.Errorf("%s uses %s outside nav-pilot mcp", fset.Position(n.Pos()), n.Sel.Name)
				}
			case *ast.Ident:
				if writers[n.Name] || filepath.Dir(path) == "." && own[n.Name] && !(base == "cli.go" && n.Name == "cmdMCP") {
					t.Errorf("%s uses %s outside nav-pilot mcp", fset.Position(n.Pos()), n.Name)
				}
			}
			return true
		})
	}
}

// Several names: each on its own. A bad name fails without stopping the
// others, and the run exits non-zero. No --client means every installed one.
func TestMCPEnableDisableSeveralNamesAndClients(t *testing.T) {
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	mcpClientInstalled = func(string) bool { return true }
	var err error
	out := captureStdout(func() {
		err = cmdMCP([]string{"enable", "figma-mcp", "nope", "com.jetbrains/intellij"})
	})
	if err == nil || !strings.Contains(err.Error(), "1 of 3") {
		t.Errorf("err = %v, want 1 of 3 failed", err)
	}
	for _, c := range []string{"copilot", "opencode"} {
		if got := strings.Join(providerpkg.MCPConfigKeys(c), ","); got != "com.figma/figma-mcp,com.jetbrains/intellij" {
			t.Errorf("%s keys = %q", c, got)
		}
		if !strings.Contains(out, "for "+c+" in") {
			t.Errorf("output does not say %s changed:\n%s", c, out)
		}
	}

	_ = captureStdout(func() { err = cmdMCP([]string{"disable", "intellij", "--client", "opencode"}) })
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(providerpkg.MCPConfigKeys("opencode"), ","); got != "com.figma/figma-mcp" {
		t.Errorf("opencode keys after disable = %q", got)
	}
	if got := strings.Join(providerpkg.MCPConfigKeys("copilot"), ","); got != "com.figma/figma-mcp,com.jetbrains/intellij" {
		t.Errorf("--client opencode touched copilot: %q", got)
	}

	// A server the user named themselves in OpenCode is still that server.
	home, _ := os.UserHomeDir()
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), `{"mcp": {"figma": {"type": "remote", "url": "https://mcp.figma.com/mcp"}}}`)
	_ = captureStdout(func() { err = cmdMCP([]string{"disable", "figma-mcp", "never-there"}) })
	if err == nil || len(providerpkg.MCPConfigKeys("opencode")) != 0 {
		t.Errorf("disable figma-mcp never-there: err %v, opencode keys %v", err, providerpkg.MCPConfigKeys("opencode"))
	}
}

// No name is an error pointing at list, never "all servers".
func TestMCPEnableDisableNeedAName(t *testing.T) {
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	for _, verb := range []string{"enable", "disable"} {
		err := cmdMCP([]string{verb, "--client", "copilot"})
		if err == nil || !strings.Contains(err.Error(), "nav-pilot mcp list") {
			t.Errorf("%s with no name: %v", verb, err)
		}
	}
	if _, err := os.Stat(copilotMCPPath()); err == nil {
		t.Error("a nameless run wrote a config")
	}
	if err := cmdMCP([]string{"enable", "figma-mcp", "--client", "pi"}); err == nil {
		t.Error("--client pi: want an error")
	}
}

// A package server's sandboxHosts are probed too, not only a remote's: a
// blocked one gets a row and a fix line.
func TestMCPListPackageSandboxHosts(t *testing.T) {
	f := mcpCmdEnv(t, providerpkg.MCPHostState{})
	e := providerpkg.MCPServerEntry{Name: "io.example/pkg", Packages: []providerpkg.MCPPackage{{RegistryType: "npm", Identifier: "@example/pkg", Version: "1.0.0"}}, SandboxHosts: []string{"api.example.org"}}
	f.verdicts["api.example.org:443"] = "BLOCKED-ALLOWLIST"
	writeTestFile(t, copilotMCPPath(), `{"mcpServers": {"io.example/pkg": {}}}`)
	entries := []providerpkg.MCPServerEntry{e}
	rep := diagnoseMCP("r", entries, mcpConfigured(entries), "")
	if len(rep.Servers) != 1 || len(rep.Servers[0].Hosts) != 1 || rep.Servers[0].Hosts[0].Host != "api.example.org" {
		t.Fatalf("servers = %+v", rep.Servers)
	}
	if !slices.ContainsFunc(rep.Problems, func(p mcpProblem) bool { return p.Fix == "cplt config set allow.domains api.example.org" }) {
		t.Errorf("problems = %+v", rep.Problems)
	}
}
