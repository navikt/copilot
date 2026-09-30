package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	prevRead, prevRec, prevClass, prevNoted, prevRefresh := readMCPHostState, recordMCPHosts, classifyMCPHosts, notedMCPHosts, refreshMCPRegistry
	readMCPHostState = func() (providerpkg.MCPHostState, error) { return st, nil }
	refreshMCPRegistry = func() error { return nil }
	prevReclassify, prevProtects := reclassifyMCPHosts, cpltProtectsNavPilotState
	reclassifyMCPHosts = func() {}
	cpltProtectsNavPilotState = func() bool { return true }
	t.Cleanup(func() { reclassifyMCPHosts, cpltProtectsNavPilotState = prevReclassify, prevProtects })
	classifyMCPHosts = func(h []providerpkg.MCPHost) []providerpkg.MCPHost { return h }
	var answers []bool
	recordMCPHosts = func(_ []providerpkg.MCPHost, approve bool) error {
		answers = append(answers, approve)
		return nil
	}
	notedMCPHosts = false
	t.Cleanup(func() {
		readMCPHostState, recordMCPHosts, classifyMCPHosts, notedMCPHosts, refreshMCPRegistry = prevRead, prevRec, prevClass, prevNoted, prevRefresh
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

	noteMCPHostConsent("copilot", false)
	for _, want := range []string{"mcp-onboarding.intern.nav.no", "io.github.navikt/mcp-onboarding", "mcp.figma.com", "com.figma/figma-mcp", "private address"} {
		if !strings.Contains(desc, want) {
			t.Errorf("the question does not show %q:\n%s", want, desc)
		}
	}
	if len(*answers) != 1 || !(*answers)[0] {
		t.Errorf("recorded %v, want one approval", *answers)
	}
}

// A launch never asks again about a set the user declined; `mcp enable` does,
// and a second decline names every host the server needs, the registry's
// sandboxHosts too, in the commands to allow it by hand.
func TestMCPEnableReasksADecline(t *testing.T) {
	figma := []providerpkg.MCPHost{
		{Host: "api.figma.com", Servers: []string{"com.figma/figma-mcp"}},
		{Host: "mcp.figma.com", Servers: []string{"com.figma/figma-mcp"}},
	}
	answers := mcpConsentEnv(t, providerpkg.MCPHostState{
		Current:  providerpkg.MCPHosts{Hosts: figma},
		Record:   &artifacts.ProposalConsent{Approved: false},
		Previous: figma,
	}, true)
	asked := 0
	prev := askProposalConsent
	askProposalConsent = func(string, string, *bool) error { asked++; return nil } // Enter: Decline
	t.Cleanup(func() { askProposalConsent = prev })

	noteMCPHostConsent("copilot", false)
	if asked != 0 {
		t.Fatalf("a launch asked again about a declined set")
	}
	stderr := captureStderr(func() { noteMCPHostConsent("copilot", true) })
	if asked != 1 || len(*answers) != 1 || (*answers)[0] {
		t.Fatalf("mcp enable: asked %d, recorded %v; want one question, one decline", asked, *answers)
	}
	for _, want := range []string{"cplt config set allow.domains mcp.figma.com", "cplt config set allow.domains api.figma.com"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the decline does not print %q:\n%s", want, stderr)
		}
	}
}

// No terminal: nothing is granted or recorded, and one line says so.
func TestMCPConsentNonInteractiveGrantsNothing(t *testing.T) {
	answers := mcpConsentEnv(t, pendingState(), false)
	prev := askProposalConsent
	askProposalConsent = func(string, string, *bool) error { t.Error("asked without a terminal"); return nil }
	t.Cleanup(func() { askProposalConsent = prev })

	stderr := captureStderr(func() { noteMCPHostConsent("copilot", false); noteMCPHostConsent("copilot", false) })
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

	// A later change rewrites the section only: the bytes above it stay as
	// they were, and no cplt is asked for its hosts.
	edited := string(before) + "custom.example\n"
	if err := os.WriteFile(path, []byte(edited+mcpAllowlistMarker+"\nmcp.figma.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	prevHosts := cpltBuiltinDomains
	cpltBuiltinDomains = func() ([]string, bool) { t.Error("cplt asked for its hosts"); return nil, false }
	t.Cleanup(func() { cpltBuiltinDomains = prevHosts })
	mcpAllowlistHosts = func() []string { return []string{"custom.example", "mcp.example.org"} }
	syncMCPAllowlist()
	after, _ = os.ReadFile(path)
	if want := edited + mcpAllowlistMarker + "\nmcp.example.org\n"; string(after) != want {
		t.Errorf("after an MCP change:\n%s\nwant:\n%s", after, want)
	}
	mcpAllowlistHosts = func() []string { return nil }
	syncMCPAllowlist()
	if after, _ = os.ReadFile(path); string(after) != edited {
		t.Errorf("after the MCP hosts went away:\n%s\nwant:\n%s", after, edited)
	}
}

// Outside a launch, the allowlist writers count the configured client's
// servers: an OpenCode user's MCP section is not computed for Copilot.
func TestAllowlistWritersUseTheConfiguredClient(t *testing.T) {
	isolatedConfig(t)
	if _, err := writeConfigKey("client", "opencode"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { providerpkg.MCPClient, providerpkg.MCPHostsOff = "", false })
	prev := mcpAllowlistHosts
	var seen []string
	mcpAllowlistHosts = func() []string { seen = append(seen, providerpkg.MCPClient); return nil }
	t.Cleanup(func() { mcpAllowlistHosts = prev })

	_, _, _ = seedCpltAllowlist(filepath.Join(t.TempDir(), "no-cplt"))
	providerpkg.MCPClient = ""
	dropMCPHosts() // the file now exists, so it reads the hosts again
	if len(seen) != 2 || slices.ContainsFunc(seen, func(c string) bool { return c != "opencode" }) {
		t.Errorf("clients seen = %q, want opencode each time", seen)
	}
}

// Enter declines: a prompt that is not moved off its default records a No.
func TestMCPConsentEnterDeclines(t *testing.T) {
	answers := mcpConsentEnv(t, pendingState(), true)
	prev := askProposalConsent
	askProposalConsent = func(string, string, *bool) error { return nil }
	t.Cleanup(func() { askProposalConsent = prev })

	noteMCPHostConsent("copilot", false)
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

	noteMCPHostConsent("copilot", false)
	if !strings.Contains(desc, "Changed since you last answered: + mcp-onboarding.intern.nav.no") {
		t.Errorf("no diff in:\n%s", desc)
	}
	// Not "the sandbox blocks": under standard a public host is reachable.
	if !strings.Contains(title, "2 MCP servers") || strings.Contains(title, "block") {
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

	noteMCPHostConsent("copilot", false)
	if len(*answers) != 0 || !providerpkg.MCPHostsOff {
		t.Errorf("off: recorded %v, provider off = %v", *answers, providerpkg.MCPHostsOff)
	}
	var out bytes.Buffer
	reportMCPHosts(&out, "")
	if strings.Count(out.String(), "\n") != 1 || !strings.Contains(out.String(), "mcp_hosts = off") {
		t.Errorf("doctor under off:\n%s", out.String())
	}
}

// Under a cplt that cannot protect the answer nothing is asked or recorded,
// and the launch says why once.
func TestMCPConsentWaitsForAProtectingCplt(t *testing.T) {
	answers := mcpConsentEnv(t, pendingState(), true)
	cpltProtectsNavPilotState = func() bool { return false }
	stderr := captureStderr(func() { noteMCPHostConsent("copilot", false); noteMCPHostConsent("copilot", false) })
	if len(*answers) != 0 {
		t.Errorf("recorded %v under a cplt that cannot protect it", *answers)
	}
	if n := strings.Count(stderr, "Upgrade cplt"); n != 2 { // one line per server, once per run
		t.Errorf("stderr:\n%s", stderr)
	}
}

// A strict user's file from an older release (#663: no registry hosts) gets
// the Nav hosts it lacks, appended above the MCP section with every byte it
// had kept, and without asking cplt. A file at that path nav-pilot did not
// write (no header) is left alone.
func TestSyncMCPAllowlistTopsUpNavHosts(t *testing.T) {
	isolatedConfig(t)
	prevHosts, prevMCP := cpltBuiltinDomains, mcpAllowlistHosts
	cpltBuiltinDomains = func() ([]string, bool) { t.Error("cplt asked for its hosts"); return nil, false }
	mcpAllowlistHosts = func() []string { return []string{"mcp.figma.com"} }
	t.Cleanup(func() { cpltBuiltinDomains, mcpAllowlistHosts = prevHosts, prevMCP })
	path := navAllowedDomainsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	old := navAllowlistHeader + " Edits are overwritten on the next run.\ngithub.com\naksel.nav.no\n"
	writeTestFile(t, path, old+mcpAllowlistMarker+"\nmcp.figma.com\n")
	syncMCPAllowlist()
	got, _ := os.ReadFile(path)
	above, section, _ := strings.Cut(string(got), mcpAllowlistMarker+"\n")
	if !strings.HasPrefix(above, old) || section != "mcp.figma.com\n" {
		t.Fatalf("want the old bytes, the added hosts, then the MCP section:\n%s", got)
	}
	for _, d := range navOwnDomains {
		if n := strings.Count(above, "\n"+d+"\n"); n != 1 {
			t.Errorf("%s appears %d times above the MCP section, want 1", d, n)
		}
	}
	syncMCPAllowlist()
	if again, _ := os.ReadFile(path); string(again) != string(got) {
		t.Errorf("a second sync changed the file:\n%s", again)
	}

	mine := "# my own list\ngithub.com\n"
	writeTestFile(t, path, mine)
	syncMCPAllowlist()
	if got, _ := os.ReadFile(path); string(got) != mine {
		t.Errorf("changed a file nav-pilot did not write:\n%s", got)
	}
}

// An unreadable consent record still tops up the Nav hosts, which need no
// consent, and leaves the MCP section exactly as it was: nothing added or
// removed on a record nav-pilot could not read.
func TestMCPConsentUnreadableRecordTopsUpNavHosts(t *testing.T) {
	mcpConsentEnv(t, providerpkg.MCPHostState{}, false)
	readMCPHostState = func() (providerpkg.MCPHostState, error) { return providerpkg.MCPHostState{}, errors.New("corrupt") }
	prev := mcpAllowlistHosts
	mcpAllowlistHosts = func() []string { t.Error("the MCP hosts were read"); return []string{"new.example"} }
	t.Cleanup(func() { mcpAllowlistHosts = prev })
	path := navAllowedDomainsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	old := navAllowlistHeader + "\ngithub.com\n"
	section := mcpAllowlistMarker + "\nstale.example\nmcp.figma.com\n"
	writeTestFile(t, path, old+section)

	_ = captureStderr(func() { noteMCPHostConsent("copilot", false) })
	got, _ := os.ReadFile(path)
	above, rest, _ := strings.Cut(string(got), mcpAllowlistMarker+"\n")
	if mcpAllowlistMarker+"\n"+rest != section || !strings.HasPrefix(above, old) {
		t.Fatalf("want the old bytes, the Nav hosts, then the MCP section unchanged:\n%s", got)
	}
	for _, d := range navOwnDomains {
		if !strings.Contains(above, "\n"+d+"\n") {
			t.Errorf("%s not topped up", d)
		}
	}
}
