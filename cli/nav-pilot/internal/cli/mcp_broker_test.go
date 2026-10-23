package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/huh"

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
	case "sleep":
		status = "exec /bin/sleep 5"
	default:
		status = "echo '" + cpltStatus + "'"
	}
	writeTestFile(t, f.cplt, "#!/bin/sh\ncase \"$1 $2\" in\n\"mcp status\") "+status+";;\n\"--version \") echo 'cplt 2026.10.01-000000-abcdef0';;\nesac\n")
	writeTestFile(t, filepath.Join(bin, "nais"), "#!/bin/sh\ntok="+bin+"/token\ncase \"$1 $2\" in\n\"auth print-access-token\") [ -f $tok ];;\n\"login -n\") : > $tok;;\n\"--version \") [ -f "+bin+"/version ] && read v < "+bin+"/version && echo \"nais version $v\";;\n*) exit 2;;\nesac\n")
	for _, p := range []string{f.cplt, filepath.Join(bin, "nais")} {
		if err := os.Chmod(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	mcpCpltPath = func() string { return f.cplt }
	prev, prevYes := mcpAsk, mcpAskYes
	t.Cleanup(func() { mcpAsk, mcpAskYes = prev, prevYes })
	mcpAsk = func(string) bool { t.Error("asked without a terminal"); return false }
	mcpAskYes = mcpAsk
	mcpRegistryServers = func() (string, []providerpkg.MCPServerEntry, error, error) {
		return "https://registry.test", append(mcpTestEntries(), naisTestEntry()), nil, nil
	}
	// No client installed: a brokered server needs none to enable or disable.
	mcpClientInstalled = func(string) bool { return false }
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

const brokerStatusOK = `{"schema":[1],"servers":[{"id":"io.github.nais/nais-mcp","state":"brokered"}],"rejected":[]}`

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
	mcpAskYes = func(q string) bool { asked = q; return true }
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

// An old cplt: enable refuses with the upgrade command and writes nothing;
// list, with a spec left from before, never fails and says the same.
func TestMCPEnableBrokerOldCplt(t *testing.T) {
	f := brokerEnv(t, "")
	var err error
	errOut := captureStderr(func() {
		captureStdout(func() { err = cmdMCP([]string{"enable", "nais-mcp"}) })
	})
	if err == nil {
		t.Error("enable succeeded with an old cplt")
	}
	if !strings.Contains(errOut, "nothing was written. Fix: brew upgrade cplt && nav-pilot mcp enable io.github.nais/nais-mcp") {
		t.Errorf("stderr:\n%s", errOut)
	}
	if _, err := os.Stat(f.spec); !os.IsNotExist(err) {
		t.Fatalf("spec written with an old cplt: %v", err)
	}
	writeTestFile(t, f.spec, `{"name":"io.github.nais/nais-mcp","_meta":{"io.github.navikt/broker":{"tools":{"allow":["schema_search"]}}}}`)
	out := captureStdout(func() {
		if err := cmdMCP([]string{"list"}); err != nil {
			t.Fatal(err)
		}
	})
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

// Without cplt nothing would enforce the spec: enable refuses like an old
// cplt, and list says the server is not configured.
func TestMCPEnableBrokerWithoutCplt(t *testing.T) {
	f := brokerEnv(t, brokerStatusOK)
	mcpCpltPath = func() string { return "" }
	var err error
	errOut := captureStderr(func() {
		captureStdout(func() { err = cmdMCP([]string{"enable", "nais-mcp"}) })
	})
	if err == nil || !strings.Contains(errOut, "nothing was written. Fix: brew install navikt/tap/cplt && nav-pilot mcp enable io.github.nais/nais-mcp") {
		t.Errorf("err %v, stderr:\n%s", err, errOut)
	}
	if _, err := os.Stat(f.spec); !os.IsNotExist(err) {
		t.Fatalf("spec written without cplt: %v", err)
	}
	writeTestFile(t, f.spec, `{}`)
	out := captureStdout(func() {
		if err := cmdMCP([]string{"list"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "not configured (no cplt)") || strings.Contains(out, "direct") {
		t.Errorf("list:\n%s", out)
	}
}

// A cplt that hangs is a failed status, not an old cplt: the spec is
// written, with a warning.
func TestMCPEnableBrokerStatusTimeout(t *testing.T) {
	f := brokerEnv(t, "sleep")
	prev := mcpCpltStatusTimeout
	t.Cleanup(func() { mcpCpltStatusTimeout = prev })
	mcpCpltStatusTimeout = 200 * time.Millisecond
	var err error
	errOut := captureStderr(func() {
		captureStdout(func() { err = cmdMCP([]string{"enable", "nais-mcp"}) })
	})
	if err != nil || !strings.Contains(errOut, "timed out") || strings.Contains(errOut, "brew upgrade") {
		t.Errorf("err %v, stderr:\n%s", err, errOut)
	}
	readBrokerSpec(t, f.spec)
}

func TestMCPEnableBrokerAllToolsAllowsWrites(t *testing.T) {
	f := brokerEnv(t, brokerStatusOK)
	captureStdout(func() {
		if err := cmdMCP([]string{"enable", "nais-mcp", "--all-tools"}); err != nil {
			t.Fatal(err)
		}
	})
	ext := readBrokerSpec(t, f.spec)
	if allow := ext["tools"].(map[string]any)["allow"].([]any); len(allow) != 4 || ext["writeConsent"] != true {
		t.Errorf("--all-tools: %v", ext)
	}
}

// cplt's credential_ok answers LOGIN, and the clients show cplt.
func TestMCPListBrokerUsesCredentialOK(t *testing.T) {
	brokerEnv(t, `{"schema":[1],"servers":[{"id":"io.github.nais/nais-mcp","state":"brokered","credential_ok":true}]}`)
	captureStdout(func() { _ = cmdMCP([]string{"enable", "nais-mcp"}) })
	// The fake nais says logged out; cplt's probe says ok and wins.
	out := captureStdout(func() { _ = cmdMCP([]string{"list"}) })
	if !strings.Contains(out, "nais: ok") || !regexp.MustCompile(`nais-mcp\s+cplt\s+cplt\s`).MatchString(out) {
		t.Errorf("list:\n%s", out)
	}
}

func TestMCPListBrokerOffHasFix(t *testing.T) {
	f := brokerEnv(t, `{"schema":[1],"servers":[{"id":"io.github.nais/nais-mcp","state":"off"}]}`)
	writeTestFile(t, f.spec, `{}`)
	out := captureStdout(func() { _ = cmdMCP([]string{"list"}) })
	if !strings.Contains(out, "cplt off") || !strings.Contains(out, "Fix: cplt config set sandbox.mcp_broker true") {
		t.Errorf("list:\n%s", out)
	}
}

// nav-pilot runs only its own commands: a registry entry with other login
// commands, another command, or an unknown or empty class is refused, and
// nothing runs.
func TestMCPBrokerCheckPinsCommands(t *testing.T) {
	for name, mut := range map[string]func(b *providerpkg.MCPBroker){
		"login":         func(b *providerpkg.MCPBroker) { b.Login.Run = "sh -c touch-pwned" },
		"status":        func(b *providerpkg.MCPBroker) { b.Login.Status = "nais whoami" },
		"command":       func(b *providerpkg.MCPBroker) { b.Command = []string{"nais", "x"} },
		"credential":    func(b *providerpkg.MCPBroker) { b.Credential = "gh-token" },
		"empty class":   func(b *providerpkg.MCPBroker) { b.Tools.Classes["schema_search"] = "" },
		"unknown class": func(b *providerpkg.MCPBroker) { b.Tools.Classes["schema_search"] = "host-exec" },
	} {
		e := naisTestEntry()
		mut(e.Meta.Broker)
		if _, err := mcpBrokerCheck(e); err == nil || !strings.Contains(err.Error(), "Fix:") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := mcpBrokerCheck(naisTestEntry()); err != nil {
		t.Errorf("control: %v", err)
	}
}

// The registry's own nais entry passes nav-pilot's checks.
func TestMCPBrokerCheckRegistryNais(t *testing.T) {
	data, err := os.ReadFile("../../../../apps/mcp-registry/allowlist.json")
	if err != nil {
		t.Fatal(err)
	}
	var reg struct {
		Servers []providerpkg.MCPServerEntry `json:"servers"`
	}
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	for _, e := range reg.Servers {
		if e.Name == "io.github.nais/nais-mcp" {
			if e.Meta.Broker == nil {
				t.Fatal("no broker extension")
			}
			if _, err := mcpBrokerCheck(e); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("nais-mcp not in the registry")
}

// nais older than the mechanism's minVersion gets a nudge on enable and in
// list, never a refusal; a newer one says nothing.
func TestMCPBrokerOldNaisNudge(t *testing.T) {
	for _, c := range []struct {
		version string
		nudge   bool
	}{{"7.0.1", true}, {"7.0.3", false}, {"v7.0.2", false}} {
		f := brokerEnv(t, brokerStatusOK)
		writeTestFile(t, filepath.Join(f.dir, "version"), c.version+"\n")
		var err error
		errOut := captureStderr(func() {
			captureStdout(func() { err = cmdMCP([]string{"enable", "nais-mcp"}) })
		})
		if err != nil {
			t.Errorf("%s: enable refused: %v", c.version, err)
		}
		if got := strings.Contains(errOut, "older than 7.0.2") && strings.Contains(errOut, "Fix: brew upgrade nais"); got != c.nudge {
			t.Errorf("%s: nudge %v, stderr:\n%s", c.version, got, errOut)
		}
		out := captureStdout(func() {
			if err := cmdMCP([]string{"list"}); err != nil {
				t.Fatal(err)
			}
		})
		if got := strings.Contains(out, "Fix: brew upgrade nais"); got != c.nudge {
			t.Errorf("%s: list nudge %v:\n%s", c.version, got, out)
		}
	}
}

// A registry answer without the broker extension (before the registry is
// deployed): both paths say so, with a fix, and write nothing.
func TestMCPEnableNoBrokerMeta(t *testing.T) {
	for _, args := range [][]string{{"enable", "nais-mcp"}, {"enable", "nais-mcp", "--json"}} {
		f := brokerEnv(t, brokerStatusOK)
		e := naisTestEntry()
		e.Meta.Broker = nil
		mcpRegistryServers = func() (string, []providerpkg.MCPServerEntry, error, error) {
			return "https://registry.test", append(mcpTestEntries(), e), nil, nil
		}
		var err error
		errOut := captureStderr(func() {
			captureStdout(func() { err = cmdMCP(args) })
		})
		if err == nil || !strings.Contains(errOut, "does not offer io.github.nais/nais-mcp through cplt yet") || !strings.Contains(errOut, "Fix: nav-pilot update") || strings.Contains(errOut, "--json is only") {
			t.Errorf("%v: err %v, stderr:\n%s", args, err, errOut)
		}
		if _, err := os.Stat(f.spec); !os.IsNotExist(err) {
			t.Errorf("%v: spec written", args)
		}
	}
}

// A symlinked cplt config dir is refused for the write and the remove, and
// nothing lands where it points.
func TestMCPBrokerRefusesSymlinkedDir(t *testing.T) {
	f := brokerEnv(t, brokerStatusOK)
	elsewhere := t.TempDir()
	xdg := filepath.Join(os.Getenv("HOME"), "xdg")
	if err := os.MkdirAll(xdg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(xdg, "cplt")); err != nil {
		t.Fatal(err)
	}
	var err error
	errOut := captureStderr(func() {
		captureStdout(func() { err = cmdMCP([]string{"enable", "nais-mcp"}) })
	})
	if err == nil || !strings.Contains(errOut, "symlink") || !strings.Contains(errOut, "Fix:") {
		t.Errorf("enable through a link: %v\n%s", err, errOut)
	}
	if ents, _ := os.ReadDir(elsewhere); len(ents) != 0 {
		t.Errorf("wrote through the link: %v", ents)
	}
	target := filepath.Join(elsewhere, "mcp", filepath.Base(f.spec))
	writeTestFile(t, target, `{}`)
	captureStderr(func() {
		captureStdout(func() { err = cmdMCP([]string{"disable", "nais-mcp"}) })
	})
	if _, statErr := os.Stat(target); err == nil || statErr != nil {
		t.Errorf("disable through a link: err %v, file %v", err, statErr)
	}
}

// enable reports what cplt makes of the spec after it is written, not
// before: a platform without the broker, or the broker off.
func TestMCPEnableBrokerReportsCpltState(t *testing.T) {
	for _, c := range []struct{ status, path, want string }{
		{`{"schema":[1],"platform_supported":false,"servers":[{"id":"io.github.nais/nais-mcp","state":"off"}]}`, "cplt (macOS only)", "macOS only"},
		{`{"schema":[1],"platform_supported":true,"servers":[{"id":"io.github.nais/nais-mcp","state":"off"}]}`, "cplt off", "Fix: cplt config set sandbox.mcp_broker true"},
		{`{"schema":[1],"platform_supported":true,"servers":[],"rejected":[{"file":"/x/io.github.nais__nais-mcp.json","reason":"bad","fix":"nav-pilot mcp enable io.github.nais/nais-mcp"}]}`, "rejected", "cplt rejected its spec: bad"},
	} {
		brokerEnv(t, c.status)
		var out string
		errOut := captureStderr(func() {
			out = captureStdout(func() {
				if err := cmdMCP([]string{"enable", "nais-mcp"}); err != nil {
					t.Fatal(err)
				}
			})
		})
		if strings.Contains(out, "cplt runs it on the host") || !strings.Contains(errOut, c.want) {
			t.Errorf("%s: stdout:\n%s\nstderr:\n%s", c.path, out, errOut)
		}
		jsonOut := captureStdout(func() { _ = cmdMCP([]string{"enable", "nais-mcp", "--json"}) })
		var res mcpBrokerResult
		if json.Unmarshal([]byte(jsonOut), &res) != nil || res.Path != c.path {
			t.Errorf("%s: json %s", c.path, jsonOut)
		}
	}
}

// A Nais entry written by hand in a client, with no spec: list shows LOGIN
// and PATH and says how to put it under the broker; disable removes both
// the spec and that entry.
func TestMCPBrokerManualClientEntry(t *testing.T) {
	f := brokerEnv(t, brokerStatusOK)
	manual := `{"mcpServers":{"io.github.nais/nais-mcp":{"type":"local","command":"nais","args":["alpha","mcp","serve"],"tools":["*"]}}}`
	writeTestFile(t, copilotMCPPath(), manual)
	out := captureStdout(func() {
		if err := cmdMCP([]string{"list"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "direct (outside the broker)") || !strings.Contains(out, "nais: logged out") || !strings.Contains(out, "Fix: nav-pilot mcp enable io.github.nais/nais-mcp") {
		t.Errorf("list:\n%s", out)
	}
	captureStdout(func() { _ = cmdMCP([]string{"enable", "nais-mcp"}) })
	readBrokerSpec(t, f.spec)
	captureStdout(func() {
		if err := cmdMCP([]string{"disable", "nais-mcp", "--client", "copilot"}); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(f.spec); !os.IsNotExist(err) {
		t.Error("spec left")
	}
	if data, _ := os.ReadFile(copilotMCPPath()); strings.Contains(string(data), "nais-mcp") {
		t.Errorf("manual entry left: %s", data)
	}
}

// A cancel in a later picker leaves the brokered server unwritten too.
func TestMCPEnableBrokerPickerCancelWritesNothing(t *testing.T) {
	f := brokerEnv(t, brokerStatusOK)
	isInteractive = func() bool { return true }
	prevPick := mcpPickTools
	t.Cleanup(func() { mcpPickTools = prevPick })
	mcpPickTools = func(providerpkg.MCPServerEntry) ([]string, error) { return nil, huh.ErrUserAborted }
	var err error
	captureStdout(func() { err = cmdMCP([]string{"enable", "nais-mcp", "intellij", "--client", "copilot"}) })
	if !errors.As(err, new(cancelledError)) {
		t.Fatalf("err = %v, want a cancel", err)
	}
	if _, statErr := os.Stat(f.spec); !os.IsNotExist(statErr) {
		t.Error("a cancelled picker left the Nais spec written")
	}
	if _, statErr := os.Stat(filepath.Join(f.dir, "token")); statErr == nil {
		t.Error("a cancelled picker ran the login")
	}
}
