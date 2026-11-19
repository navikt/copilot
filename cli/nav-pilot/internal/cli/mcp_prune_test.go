package cli

import (
	"errors"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// mcpPruneEnv is mcpCmdEnv under isolatedConfig: HOME, XDG_CONFIG_HOME and
// NAV_PILOT_CONFIG all point at temporary directories. It writes a Copilot
// and an OpenCode config, each with one entry the registry dropped next to
// a listed one and servers the user added, and returns their paths.
func mcpPruneEnv(t *testing.T, opencode string) (copilot, oc string) {
	t.Helper()
	mcpCmdEnv(t, providerpkg.MCPHostState{})
	isolatedConfig(t)
	copilot = copilotMCPPath()
	writeTestFile(t, copilot, `{"mcpServers": {
		"com.figma/figma-mcp": {},
		"io.github.navikt/fleet-gateway-kodeverk": {},
		"homemade": {},
		"io.github.someone/tool": {}}}`)
	oc, err := providerpkg.MCPConfigPath(providerpkg.MCPClientOpenCode)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, oc, opencode)
	return copilot, oc
}

const mcpPruneOpenCode = `{"mcp": {
	"io.example/gone": {"type": "remote", "url": "https://gone.example/mcp"},
	"mine": {"type": "remote", "url": "https://mine.example/mcp"},
	"figma": {"type": "remote", "url": "https://mcp.figma.com/mcp"}}}`

// --dry-run names every entry it would remove and changes nothing.
func TestMCPPruneDryRun(t *testing.T) {
	copilot, oc := mcpPruneEnv(t, mcpPruneOpenCode)
	beforeC, beforeO := readFile(t, copilot), readFile(t, oc)
	out := captureStdout(func() {
		if err := cmdMCP([]string{"prune", "--dry-run"}); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{
		"Would remove io.github.navikt/fleet-gateway-kodeverk for copilot",
		"Would remove io.example/gone for opencode",
		"Dry run: would remove 2 entries.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if readFile(t, copilot) != beforeC || readFile(t, oc) != beforeO {
		t.Error("--dry-run changed a config")
	}
}

// --yes removes only the entries nav-pilot wrote that the registry no longer
// lists, in both clients, one line each and a summary. The user's own
// servers stay, also one in reverse-DNS form under a namespace the registry
// has no servers in.
func TestMCPPruneYesRemovesOnlyStaleRegistryEntries(t *testing.T) {
	_, _ = mcpPruneEnv(t, mcpPruneOpenCode)
	narrowed := 0
	narrowMCPApproval = func() ([]string, error) { narrowed++; return nil, nil }
	out := captureStdout(func() {
		if err := cmdMCP([]string{"prune", "--yes"}); err != nil {
			t.Fatal(err)
		}
	})
	if got := strings.Join(providerpkg.MCPConfigKeys("copilot"), ","); got != "com.figma/figma-mcp,homemade,io.github.someone/tool" {
		t.Errorf("copilot keys = %q", got)
	}
	if got := strings.Join(providerpkg.MCPConfigKeys("opencode"), ","); got != "figma,mine" && got != "mine,figma" {
		t.Errorf("opencode keys = %q", got)
	}
	if n := strings.Count(out, "Removed io."); n != 2 {
		t.Errorf("want one line per removed entry, got %d:\n%s", n, out)
	}
	if !strings.Contains(out, "Removed 2 entries") {
		t.Errorf("no summary:\n%s", out)
	}
	if narrowed != 1 {
		t.Errorf("host approval narrowed %d times, want 1", narrowed)
	}

	// A second run finds nothing.
	out = captureStdout(func() { _ = cmdMCP([]string{"prune", "--yes"}) })
	if !strings.Contains(out, "Nothing to prune") {
		t.Errorf("second run:\n%s", out)
	}
}

// A file with comments is not changed and is named, as in disable; the
// other client is still pruned.
func TestMCPPruneSkipsCommentedFile(t *testing.T) {
	commented := `{
	// my servers
	"mcp": {
		"io.example/gone": {"type": "remote", "url": "https://gone.example/mcp"},
		"mine": {"type": "remote", "url": "https://mine.example/mcp"},
	}
}`
	copilot, oc := mcpPruneEnv(t, commented)
	out := captureStdout(func() {
		if err := cmdMCP([]string{"prune", "--yes"}); err != nil {
			t.Fatal(err)
		}
	})
	if readFile(t, oc) != commented {
		t.Error("a file with comments was rewritten")
	}
	if !strings.Contains(out, oc+" has comments or trailing commas") || !strings.Contains(out, `"io.example/gone"`) {
		t.Errorf("commented file not named:\n%s", out)
	}
	if strings.Contains(readFile(t, copilot), "fleet-gateway-kodeverk") {
		t.Error("copilot entry not pruned")
	}
	if !strings.Contains(out, "Removed 1 entry; skipped 1 in commented file(s)") {
		t.Errorf("summary:\n%s", out)
	}
}

// Without --yes it asks; no terminal is a refusal and a no is a no.
func TestMCPPruneAsksFirst(t *testing.T) {
	copilot, oc := mcpPruneEnv(t, mcpPruneOpenCode)
	beforeC, beforeO := readFile(t, copilot), readFile(t, oc)
	var err error
	_ = captureStdout(func() { err = cmdMCP([]string{"prune"}) })
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Errorf("without a terminal: err = %v", err)
	}

	isInteractive = func() bool { return true }
	prev := mcpAsk
	t.Cleanup(func() { mcpAsk = prev })
	asked := ""
	mcpAsk = func(q string) bool { asked = q; return false }
	out := captureStdout(func() { err = cmdMCP([]string{"prune"}) })
	if err != nil || !strings.Contains(asked, "Remove 2 MCP entries") || !strings.Contains(out, "Cancelled") {
		t.Errorf("declined: err %v, asked %q, output:\n%s", err, asked, out)
	}
	if readFile(t, copilot) != beforeC || readFile(t, oc) != beforeO {
		t.Error("a refused or declined prune changed a config")
	}

	if err := cmdMCP([]string{"list", "--dry-run"}); err == nil {
		t.Error("--dry-run outside prune: want an error")
	}
}

func TestMCPPrunable(t *testing.T) {
	ns := map[string]bool{"io.github.navikt": true}
	for name, want := range map[string]bool{
		"io.github.navikt/old":     true,
		"io.github.someone/old":    false,
		"homemade":                 false,
		"io.github.navikt/":        false,
		"navikt/old":               false,
		"io.github.navikt/a b":     false,
		"io.github.navikt/x/y":     false,
		"io.github.navikt/ok.v2_x": true,
	} {
		if got := mcpPrunable(ns, name); got != want {
			t.Errorf("mcpPrunable(%q) = %v, want %v", name, got, want)
		}
	}
}

// A registry answer from the cache may predate a server the user enabled,
// so a stale answer prunes nothing.
func TestMCPPruneRefusesStaleRegistry(t *testing.T) {
	copilot, oc := mcpPruneEnv(t, mcpPruneOpenCode)
	prev := mcpRegistryServers
	t.Cleanup(func() { mcpRegistryServers = prev })
	mcpRegistryServers = func() (string, []providerpkg.MCPServerEntry, error, error) {
		reg, entries, _, err := prev()
		return reg, entries, errors.New("offline"), err
	}
	beforeC, beforeO := readFile(t, copilot), readFile(t, oc)
	if err := cmdMCP([]string{"prune", "--yes"}); err == nil {
		t.Fatal("prune ran on a stale registry")
	}
	if readFile(t, copilot) != beforeC || readFile(t, oc) != beforeO {
		t.Error("stale prune changed a config")
	}
}
