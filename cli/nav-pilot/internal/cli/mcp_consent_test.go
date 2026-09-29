package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// mcpConsentEnv answers the registry, the classification and the record with
// fakes, and returns what was recorded.
func mcpConsentEnv(t *testing.T, st providerpkg.MCPHostState, interactive bool) *[]bool {
	t.Helper()
	consentEnv(t)
	isInteractive = func() bool { return interactive }
	prevRead, prevRec, prevClass, prevNoted := readMCPHostState, recordMCPHosts, classifyMCPHosts, notedMCPHosts
	readMCPHostState = func() (providerpkg.MCPHostState, error) { return st, nil }
	classifyMCPHosts = func(h []providerpkg.MCPHost) []providerpkg.MCPHost { return h }
	var answers []bool
	recordMCPHosts = func(_ []providerpkg.MCPHost, approve bool) error {
		answers = append(answers, approve)
		return nil
	}
	notedMCPHosts = false
	t.Cleanup(func() {
		readMCPHostState, recordMCPHosts, classifyMCPHosts, notedMCPHosts = prevRead, prevRec, prevClass, prevNoted
	})
	return &answers
}

func pendingState() providerpkg.MCPHostState {
	return providerpkg.MCPHostState{Pending: []providerpkg.MCPHost{
		{Host: "mcp-onboarding.intern.nav.no", Servers: []string{"io.github.navikt/mcp-onboarding"}, Private: true},
		{Host: "mcp.figma.com", Servers: []string{"com.figma/figma-mcp"}},
	}}
}

// The question shows every host and the server that needs it, and the answer
// is recorded.
func TestMCPConsentAsksAndRecords(t *testing.T) {
	answers := mcpConsentEnv(t, pendingState(), true)
	var desc string
	prev := askProposalConsent
	askProposalConsent = func(_, d string, approve *bool) error { desc, *approve = d, true; return nil }
	t.Cleanup(func() { askProposalConsent = prev })

	noteMCPHostConsent()
	for _, want := range []string{"mcp-onboarding.intern.nav.no", "io.github.navikt/mcp-onboarding", "mcp.figma.com", "com.figma/figma-mcp", "private address"} {
		if !strings.Contains(desc, want) {
			t.Errorf("the question does not show %q:\n%s", want, desc)
		}
	}
	if len(*answers) != 1 || !(*answers)[0] {
		t.Errorf("recorded %v, want one approval", *answers)
	}
}

// No terminal: nothing is granted or recorded, and one line says so.
func TestMCPConsentNonInteractiveGrantsNothing(t *testing.T) {
	answers := mcpConsentEnv(t, pendingState(), false)
	prev := askProposalConsent
	askProposalConsent = func(string, string, *bool) error { t.Error("asked without a terminal"); return nil }
	t.Cleanup(func() { askProposalConsent = prev })

	stderr := captureStderr(func() { noteMCPHostConsent(); noteMCPHostConsent() })
	if len(*answers) != 0 {
		t.Errorf("recorded %v without a terminal", *answers)
	}
	if n := strings.Count(stderr, "\n"); n != 2 {
		t.Errorf("want one line per server, once, got %d:\n%s", n, stderr)
	}
}

// A loopback server gets the exact command, and no grant.
func TestDoctorShowsTheLoopbackHint(t *testing.T) {
	mcpConsentEnv(t, providerpkg.MCPHostState{Current: providerpkg.MCPHosts{
		Loopback: []providerpkg.MCPLoopback{{Server: "io.github.navikt/intellij", Port: "64342"}},
	}}, false)
	var out bytes.Buffer
	reportMCPHosts(&out, "")
	if !strings.Contains(out.String(), "cplt config set allow.localhost 64342") || !strings.Contains(out.String(), "io.github.navikt/intellij") {
		t.Errorf("doctor output:\n%s", out.String())
	}
}

// The allowlist file gets an MCP section when nav-pilot already wrote one, and
// no file is created for a user who never adopted it.
func TestSyncMCPAllowlist(t *testing.T) {
	isolatedConfig(t)
	prev := mcpAllowlistHosts
	mcpAllowlistHosts = func() []string { return []string{"mcp.figma.com"} }
	t.Cleanup(func() { mcpAllowlistHosts = prev })

	syncMCPAllowlist()
	if _, err := os.Stat(navAllowedDomainsPath()); err == nil {
		t.Fatal("syncMCPAllowlist created an allowlist file nobody adopted")
	}
	mcpAllowlistHosts = func() []string { return nil }
	path, err := writeNavAllowedDomains()
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	mcpAllowlistHosts = func() []string { return []string{"mcp.figma.com"} }
	syncMCPAllowlist()
	after, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(after), string(before)) || !strings.HasSuffix(string(after), mcpAllowlistMarker+"\nmcp.figma.com\n") {
		t.Errorf("want the old file untouched plus an MCP section:\n%s", after)
	}
}

// Enter declines: a prompt that is not moved off its default records a No.
func TestMCPConsentEnterDeclines(t *testing.T) {
	answers := mcpConsentEnv(t, pendingState(), true)
	prev := askProposalConsent
	askProposalConsent = func(string, string, *bool) error { return nil }
	t.Cleanup(func() { askProposalConsent = prev })

	noteMCPHostConsent()
	if len(*answers) != 1 || (*answers)[0] {
		t.Errorf("recorded %v, want one decline", *answers)
	}
}

// A grown set asks again and shows what was added.
func TestMCPConsentGrownSetShowsTheDiff(t *testing.T) {
	st := pendingState()
	st.Record = &artifacts.ProposalConsent{Approved: true}
	st.Previous = st.Pending[1:]
	mcpConsentEnv(t, st, true)
	var title, desc string
	prev := askProposalConsent
	askProposalConsent = func(tt, d string, _ *bool) error { title, desc = tt, d; return nil }
	t.Cleanup(func() { askProposalConsent = prev })

	noteMCPHostConsent()
	if !strings.Contains(desc, "Changed since you last answered: + mcp-onboarding.intern.nav.no") {
		t.Errorf("no diff in:\n%s", desc)
	}
	if !strings.Contains(title, "2 MCP servers") {
		t.Errorf("title = %q", title)
	}
}

// mcp_hosts = off: nothing asked, nothing granted, and doctor says so once.
func TestMCPHostsOff(t *testing.T) {
	answers := mcpConsentEnv(t, pendingState(), true)
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), []byte("version = 1\nmcp_hosts = \"off\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := askProposalConsent
	askProposalConsent = func(string, string, *bool) error { t.Error("asked under mcp_hosts = off"); return nil }
	t.Cleanup(func() { askProposalConsent = prev; providerpkg.MCPHostsOff = false })

	noteMCPHostConsent()
	if len(*answers) != 0 || !providerpkg.MCPHostsOff {
		t.Errorf("off: recorded %v, provider off = %v", *answers, providerpkg.MCPHostsOff)
	}
	var out bytes.Buffer
	reportMCPHosts(&out, "")
	if strings.Count(out.String(), "\n") != 1 || !strings.Contains(out.String(), "mcp_hosts = off") {
		t.Errorf("doctor under off:\n%s", out.String())
	}
}
