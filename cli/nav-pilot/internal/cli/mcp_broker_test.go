package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

func naisTestEntry() providerpkg.MCPServerEntry {
	e := providerpkg.MCPServerEntry{Name: "io.github.nais/nais-mcp", Version: "6.6.1",
		Tools: []string{"get_nais_context", "execute_graphql", "schema_search", "mutate_thing"}}
	b := &providerpkg.MCPBroker{V: 1, Credential: "nais-keychain", Command: []string{"nais", "alpha", "mcp", "serve"},
		EgressHosts: []string{"auth.nais.io", "console.nav.cloud.nais.io"}, ArgChecks: map[string]string{"execute_graphql": "graphql-read-only"}}
	// mutate_thing is a write; an unclassed tool would never be allowed.
	b.Tools.Classes = map[string]string{"get_nais_context": "read", "execute_graphql": "read", "schema_search": "read", "mutate_thing": "write"}
	b.Login.Run, b.Login.Status = "nais login -n", "nais auth print-access-token"
	e.Meta.Broker = b
	return e
}

type brokerFakes struct {
	dir, cplt, spec string
}

// brokerEnv isolates HOME, NAV_PILOT_CONFIG and XDG_CONFIG_HOME, and puts a
// fake cplt and a fake nais in a bin dir that is the whole PATH. cpltStatus
// is what `cplt mcp status --json` prints ("" exits 1, like an old cplt);
// nais is logged in when its token file exists, and `nais login -n` makes it.
func brokerEnv(t *testing.T, cpltStatus string) brokerFakes {
	t.Helper()
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(os.Getenv("HOME"), "xdg"))
	if err := os.MkdirAll(filepath.Dir(os.Getenv("NAV_PILOT_CONFIG")), 0o700); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	f := brokerFakes{dir: bin, cplt: filepath.Join(bin, "cplt"), spec: filepath.Join(os.Getenv("HOME"), "xdg", "cplt", "mcp", "io.github.nais__nais-mcp.json")}
	status := "echo \"error: unrecognized subcommand 'mcp'\" >&2; exit 2"
	switch cpltStatus {
	case "":
	case "home":
		status = "echo 'cplt: refusing to sandbox your home directory' >&2; exit 1"
	default:
		status = "echo '" + cpltStatus + "'"
	}
	writeTestFile(t, f.cplt, "#!/bin/sh\ncase \"$1 $2\" in\n\"mcp status\") "+status+";;\n\"--version \") echo 'cplt 2026.10.01-000000-abcdef0';;\nesac\n")
	writeTestFile(t, filepath.Join(bin, "nais"), "#!/bin/sh\ntok="+bin+"/token\ncase \"$1 $2\" in\n\"auth print-access-token\") [ -f $tok ];;\n\"login -n\") : > $tok;;\n*) exit 2;;\nesac\n")
	for _, p := range []string{f.cplt, filepath.Join(bin, "nais")} {
		if err := os.Chmod(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	mcpCpltPath = func() string { return f.cplt }
	prev := mcpAsk
	t.Cleanup(func() { mcpAsk = prev })
	mcpAsk = func(string) bool { t.Error("asked without a terminal"); return false }
	mcpRegistryServers = func() (string, []providerpkg.MCPServerEntry, error, error) {
		return "https://registry.test", append(mcpTestEntries(), naisTestEntry()), nil, nil
	}
	mcpClientInstalled = func(string) bool { t.Error("a brokered server needs no client"); return false }
	return f
}

func readBrokerSpec(t *testing.T, path string) map[string]any {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("no spec: %v", err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("spec mode %v, want 0600", st.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v["_meta"].(map[string]any)[mcpBrokerKey].(map[string]any)
}

const brokerStatusOK = `{"schema":[1],"servers":[{"id":"io.github.nais/nais-mcp","state":"brokered","credential_ok":true}],"rejected":[]}`

func TestMCPEnableBrokerWritesReadOnlySpec(t *testing.T) {
	f := brokerEnv(t, brokerStatusOK)
	out := captureStdout(func() {
		if err := cmdMCP([]string{"enable", "nais-mcp"}); err != nil {
			t.Fatal(err)
		}
	})
	ext := readBrokerSpec(t, f.spec)
	allow := ext["tools"].(map[string]any)["allow"].([]any)
	if !slices.Equal(allow, []any{"execute_graphql", "get_nais_context", "schema_search"}) {
		t.Errorf("allow %v, want the three reads", allow)
	}
	if ext["writeConsent"] != nil || ext["credential"] != "nais-keychain" || ext["argChecks"].(map[string]any)["execute_graphql"] != "graphql-read-only" {
		t.Errorf("spec %v", ext)
	}
	// Logged out and no terminal: the command, never a run.
	if !strings.Contains(out, "not logged in on this machine. Fix: nais login -n") {
		t.Errorf("output:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "token")); err == nil {
		t.Error("ran nais login without a yes")
	}
	// No client config written.
	for _, p := range []string{".copilot/mcp-config.json", "xdg/opencode/opencode.json"} {
		if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), p)); err == nil {
			t.Errorf("wrote %s", p)
		}
	}

	// list: LOGIN and PATH from the status command and cplt.
	out = captureStdout(func() {
		if err := cmdMCP([]string{"list"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "LOGIN") || !strings.Contains(out, "nais: logged out") || !strings.Contains(out, "cplt broker") || !strings.Contains(out, "3 of 4") {
		t.Errorf("list:\n%s", out)
	}

	// disable removes the spec, and needs no client either.
	captureStdout(func() {
		if err := cmdMCP([]string{"disable", "nais-mcp"}); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(f.spec); !os.IsNotExist(err) {
		t.Errorf("spec still there: %v", err)
	}
}

func TestMCPEnableBrokerLogsInAfterYes(t *testing.T) {
	f := brokerEnv(t, brokerStatusOK)
	isInteractive = func() bool { return true }
	asked := ""
	mcpAsk = func(q string) bool { asked = q; return true }
	captureStdout(func() {
		if err := cmdMCP([]string{"enable", "io.github.nais/nais-mcp"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(asked, "Log in now?") || !strings.Contains(asked, "nais login -n") {
		t.Errorf("asked %q", asked)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "token")); err != nil {
		t.Error("nais login -n did not run after a yes")
	}
	out := captureStdout(func() { _ = cmdMCP([]string{"list"}) })
	if !strings.Contains(out, "nais: ok") {
		t.Errorf("list:\n%s", out)
	}
}

func TestMCPEnableBrokerWritesNeedTools(t *testing.T) {
	f := brokerEnv(t, brokerStatusOK)
	captureStdout(func() {
		if err := cmdMCP([]string{"enable", "nais-mcp", "--tools", "schema_search,mutate_thing"}); err != nil {
			t.Fatal(err)
		}
	})
	if ext := readBrokerSpec(t, f.spec); ext["writeConsent"] != true {
		t.Errorf("a write picked with --tools needs writeConsent: %v", ext)
	}
	errOut := captureStderr(func() {
		if err := cmdMCP([]string{"enable", "nais-mcp", "--tools", "nope"}); err == nil {
			t.Error("an unknown tool was accepted")
		}
	})
	if !strings.Contains(errOut, "Fix:") {
		t.Errorf("no fix: %s", errOut)
	}
}

func TestMCPEnableBrokerJSONPrintsLoginCommand(t *testing.T) {
	brokerEnv(t, brokerStatusOK)
	isInteractive = func() bool { return true } // --json never asks
	out := captureStdout(func() {
		if err := cmdMCP([]string{"enable", "nais-mcp", "--json"}); err != nil {
			t.Fatal(err)
		}
	})
	var res mcpBrokerResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v:\n%s", err, out)
	}
	if res.Login != "logged out" || !slices.Contains(res.Fix, "nais login -n") || res.Path != "cplt broker" {
		t.Errorf("%+v", res)
	}
}

// An old cplt: a notice once per version, the spec still written, no failure.
func TestMCPEnableBrokerOldCplt(t *testing.T) {
	f := brokerEnv(t, "")
	run := func() string {
		return captureStderr(func() {
			captureStdout(func() {
				if err := cmdMCP([]string{"enable", "nais-mcp"}); err != nil {
					t.Fatal(err)
				}
			})
		})
	}
	if first := run(); !strings.Contains(first, "cannot run MCP servers on the host yet") || !strings.Contains(first, "Fix: brew upgrade cplt") {
		t.Errorf("first run:\n%s", first)
	}
	if second := run(); strings.Contains(second, "cannot run MCP servers") {
		t.Errorf("notice twice:\n%s", second)
	}
	readBrokerSpec(t, f.spec)
	out := captureStdout(func() { _ = cmdMCP([]string{"list"}) })
	if !strings.Contains(out, "Fix: brew upgrade cplt") {
		t.Errorf("list:\n%s", out)
	}
}

// cplt mcp status refuses to run in ~: list shows "-" and a hint, never the
// old-cplt notice.
func TestMCPListBrokerStatusFails(t *testing.T) {
	f := brokerEnv(t, "home")
	errOut := captureStderr(func() {
		captureStdout(func() {
			if err := cmdMCP([]string{"enable", "nais-mcp"}); err != nil {
				t.Fatal(err)
			}
		})
	})
	if strings.Contains(errOut, "brew upgrade") || !strings.Contains(errOut, "refusing to sandbox your home") {
		t.Errorf("stderr:\n%s", errOut)
	}
	readBrokerSpec(t, f.spec)
	out := captureStdout(func() {
		if err := cmdMCP([]string{"list"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Fix: cd <your repo> && nav-pilot mcp list") || strings.Contains(out, "cplt broker") {
		t.Errorf("list:\n%s", out)
	}
}

// Without cplt nothing enforces the spec: say so, and that execute_graphql
// is unchecked.
func TestMCPEnableBrokerWithoutCplt(t *testing.T) {
	f := brokerEnv(t, brokerStatusOK)
	mcpCpltPath = func() string { return "" }
	errOut := captureStderr(func() {
		captureStdout(func() {
			if err := cmdMCP([]string{"enable", "nais-mcp"}); err != nil {
				t.Fatal(err)
			}
		})
	})
	if !strings.Contains(errOut, "no read-only guarantee") || !strings.Contains(errOut, "execute_graphql") || !strings.Contains(errOut, "Fix: brew install navikt/tap/cplt") {
		t.Errorf("stderr:\n%s", errOut)
	}
	readBrokerSpec(t, f.spec)
	out := captureStdout(func() { _ = cmdMCP([]string{"list"}) })
	if !strings.Contains(out, "direct (no read-only guarantee)") {
		t.Errorf("list:\n%s", out)
	}
}
