package provider

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

func stderrOf(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	fn()
	os.Stderr = orig
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func setVerbose(t *testing.T, v bool) {
	prev := Verbose
	Verbose = v
	t.Cleanup(func() { Verbose = prev })
}

// An unsupported setting is said once per (client, setting, value).
func TestUnsupportedWarningOncePerValue(t *testing.T) {
	proposeEnv(t)
	setVerbose(t, false)
	low := domain.ResolvedConfig{AskUser: true, ReasoningEffort: "low"}
	say := func(client string, r domain.ResolvedConfig) string {
		return stderrOf(t, func() { warnUnsupported(client, OpenCodeUnsupportedConfigWarnings(r)) })
	}
	if out := say("opencode", low); !strings.Contains(out, `"low"`) {
		t.Fatalf("first launch said nothing: %q", out)
	}
	if out := say("opencode", low); out != "" {
		t.Errorf("repeat launch spoke: %q", out)
	}
	high := low
	high.ReasoningEffort = "high"
	if out := say("opencode", high); !strings.Contains(out, `"high"`) {
		t.Errorf("changed value was silent: %q", out)
	}
	if out := say("pi", high); !strings.Contains(out, `"high"`) {
		t.Errorf("changed client was silent: %q", out)
	}
	setVerbose(t, true)
	if out := say("opencode", high); !strings.Contains(out, `"high"`) {
		t.Errorf("--verbose was silent: %q", out)
	}
}

// onceThenChange runs say(v) as a launch would: shown for a new value, silent
// on repeat, shown again on a change and always with --verbose.
func onceThenChange(t *testing.T, say func(v string) string, a, b string) {
	t.Helper()
	proposeEnv(t)
	setVerbose(t, false)
	if out := say(a); !strings.Contains(out, a) {
		t.Fatalf("first launch was silent: %q", out)
	}
	if out := say(a); out != "" {
		t.Errorf("repeat launch spoke: %q", out)
	}
	if out := say(b); !strings.Contains(out, b) {
		t.Errorf("changed value was silent: %q", out)
	}
	setVerbose(t, true)
	defer setVerbose(t, false)
	if out := say(b); !strings.Contains(out, b) {
		t.Errorf("--verbose was silent: %q", out)
	}
}

func TestSandboxSubfolderLineOncePerScope(t *testing.T) {
	onceThenChange(t, func(v string) string {
		return stderrOf(t, func() { printSandboxScope("/repo/"+v, "/repo", []string{"/repo/AGENTS.md"}) })
	}, "sub-a", "sub-b")
}

// sayConflicts on a launch: once per set, silent on repeat, again on a
// change, and again after the set was empty in between.
func TestSayConflictsOncePerSetAndResets(t *testing.T) {
	proposeEnv(t)
	setVerbose(t, false)
	prev := artifacts.LaunchNotices
	artifacts.LaunchNotices = true
	t.Cleanup(func() { artifacts.LaunchNotices = prev })
	say := func(c ...string) string {
		return stderrOf(t, func() { sayConflicts("k", "modified: %s", c) })
	}
	if out := say("a.md"); !strings.Contains(out, "a.md") {
		t.Fatalf("first launch was silent: %q", out)
	}
	if out := say("a.md"); out != "" {
		t.Errorf("repeat spoke: %q", out)
	}
	if out := say("a.md", "b.md"); !strings.Contains(out, "b.md") {
		t.Errorf("changed set was silent: %q", out)
	}
	say()
	if out := say("a.md", "b.md"); !strings.Contains(out, "b.md") {
		t.Errorf("set back after empty was silent: %q", out)
	}
	artifacts.LaunchNotices = false
	if out := say("a.md", "b.md"); !strings.Contains(out, "b.md") {
		t.Errorf("an explicit command was silent: %q", out)
	}
}

// "MCP servers turned off": blocked, then allowed, then blocked again is
// said again; a repeat is not.
func TestMCPTurnedOffResetsWhenAllowed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	setVerbose(t, false)
	proj := t.TempDir()
	os.Mkdir(filepath.Join(proj, ".git"), 0o700)
	os.WriteFile(filepath.Join(proj, "opencode.json"), []byte(`{"mcp": {"bad": {"type": "remote", "url": "https://bad/mcp"}}}`), 0o600)
	origPolicy, origReg := fetchMCPPolicy, fetchMCPRegistry
	t.Cleanup(func() { fetchMCPPolicy, fetchMCPRegistry = origPolicy, origReg })
	fetchMCPPolicy = func() (string, error) { return "https://registry/", nil }
	allowed := false
	fetchMCPRegistry = func(string) (mcpRegistry, error) {
		return mcpRegistry{Remotes: map[string]bool{"https://bad/mcp": allowed}, Packages: map[string]bool{}}, nil
	}
	launch := func() string { return stderrOf(t, func() { applyOpenCodeMCPPolicy(nil, proj) }) }
	if out := launch(); !strings.Contains(out, "turned off") {
		t.Fatalf("first block was silent: %q", out)
	}
	if out := launch(); out != "" {
		t.Errorf("repeat spoke: %q", out)
	}
	allowed = true
	launch()
	allowed = false
	if out := launch(); !strings.Contains(out, "turned off") {
		t.Errorf("blocked again after allowed was silent: %q", out)
	}
}

// An unsupported setting that stops warning and comes back is said again.
func TestUnsupportedWarningResetsWhenDisabled(t *testing.T) {
	proposeEnv(t)
	setVerbose(t, false)
	say := func(r domain.ResolvedConfig) string {
		return stderrOf(t, func() { warnUnsupported("opencode", OpenCodeUnsupportedConfigWarnings(r)) })
	}
	on := domain.ResolvedConfig{AskUser: false}
	off := domain.ResolvedConfig{AskUser: true}
	if out := say(on); !strings.Contains(out, "ask_user") {
		t.Fatalf("first launch was silent: %q", out)
	}
	if out := say(on); out != "" {
		t.Errorf("repeat spoke: %q", out)
	}
	say(off)
	if out := say(on); !strings.Contains(out, "ask_user") {
		t.Errorf("re-enabled setting was silent: %q", out)
	}
}

func TestLaunchSyncProgressIsVerboseOnly(t *testing.T) {
	prev := artifacts.LaunchNotices
	t.Cleanup(func() { artifacts.LaunchNotices = prev })
	setVerbose(t, false)
	artifacts.LaunchNotices = true
	if !QuietLaunchSync() {
		t.Error("a launch's --sync progress is not quiet")
	}
	setVerbose(t, true)
	if QuietLaunchSync() {
		t.Error("--verbose did not show the --sync progress")
	}
	setVerbose(t, false)
	artifacts.LaunchNotices = false
	if QuietLaunchSync() {
		t.Error("an explicit nav-pilot sync lost its progress lines")
	}
}

// An approval the user just answered (✓ at answer time) is not repeated as an
// ℹ line by the launch that follows; the control, an approval nobody marked,
// still is.
func TestJustAnsweredApprovalIsNotRepeated(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)
	setVerbose(t, false)
	launch := func() string { return stderrOf(t, func() { cpltProposalFlags() }) }

	approve(t, scope, proposal.Hash(), "cloud.nais.io")
	if out := launch(); !strings.Contains(out, "approved for this scope") {
		t.Fatalf("control: an unmarked approval was silent: %q", out)
	}

	record := artifacts.ProposalConsent{
		Scope: scope.Name, Root: scope.RootDir, Pakke: "nais-pilot", Hash: proposal.Hash(), Approved: true,
		Hosts: []string{"cloud.nais.io", "example.nais.io"}, CpltStamp: minCpltStampProtectingNavPilotState, At: time.Now(),
	}
	if err := artifacts.WriteProposalConsent(record); err != nil {
		t.Fatal(err)
	}
	MarkApprovalSaid(record)
	if out := launch(); out != "" {
		t.Errorf("the launch repeated an approval just answered: %q", out)
	}
	if got := cpltProposalFlags(); len(got) == 0 {
		t.Error("the quiet launch dropped the approved flags")
	}
}

// A proposal nobody answered (a teammate who got the agentpakke with the
// repo) is named once per hash at launch; an answered one, the control, never.
func TestUnansweredProposalNamedOnce(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)
	setVerbose(t, false)
	launch := func() string { return stderrOf(t, func() { cpltProposalFlags() }) }

	out := launch()
	if !strings.Contains(out, "The agentpakke asks to let the agent ") || !strings.Contains(out, "Answer: nav-pilot pakke access") {
		t.Fatalf("an unanswered proposal was silent: %q", out)
	}
	if out := launch(); out != "" {
		t.Errorf("repeat launch spoke: %q", out)
	}
	if err := artifacts.WriteProposalConsent(artifacts.ProposalConsent{
		Scope: scope.Name, Root: scope.RootDir, Pakke: "nais-pilot", Hash: proposal.Hash(), Approved: false,
	}); err != nil {
		t.Fatal(err)
	}
	if out := launch(); strings.Contains(out, "asks to let the agent") {
		t.Errorf("control: a declined proposal was named as unanswered: %q", out)
	}
}

// A launch running the user-scope pin points the unanswered hint at
// `pakke access --user`, even with a repo install beside it (which plain
// `pakke access` would pick). Control: a repo-only launch gets no --user.
func TestUnansweredHintNamesTheActiveScope(t *testing.T) {
	for _, tc := range []struct {
		name    string
		userPin bool
		want    string
	}{
		{"user pin beside a repo install", true, "Answer: nav-pilot pakke access --user"},
		{"repo only", false, "Answer: nav-pilot pakke access\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proposeEnv(t)
			activeProposal(t)
			protectingCplt(t)
			setVerbose(t, false)
			activePakkeUser = tc.userPin
			t.Cleanup(func() { activePakkeUser = false })
			out := stderrOf(t, func() { cpltProposalFlags() })
			if !strings.Contains(out, tc.want) {
				t.Errorf("hint %q, want %q", out, tc.want)
			}
		})
	}
}

// Approved-scope lines show on a new or changed approval, not on a repeat.
func TestApprovedScopeLinesOnlyOnChange(t *testing.T) {
	scope := proposeEnv(t)
	proposal := activeProposal(t)
	protectingCplt(t)
	setVerbose(t, false)
	approve(t, scope, proposal.Hash(), "cloud.nais.io")

	launch := func() string { return stderrOf(t, func() { cpltProposalFlags() }) }
	if out := launch(); !strings.Contains(out, "approved for this scope") {
		t.Fatalf("new approval was silent: %q", out)
	}
	if out := launch(); out != "" {
		t.Errorf("repeat launch spoke: %q", out)
	}
	if got := cpltProposalFlags(); len(got) == 0 {
		t.Error("a silent launch dropped the approved flags")
	}
	approve(t, scope, proposal.Hash(), "cloud.nais.io", "example.nais.io")
	if out := launch(); !strings.Contains(out, "example.nais.io") {
		t.Errorf("changed approval was silent: %q", out)
	}
	// A re-approval with the same grants is a new approval: said once.
	if err := artifacts.WriteProposalConsent(artifacts.ProposalConsent{
		Scope: scope.Name, Root: scope.RootDir, Pakke: "nais-pilot", Hash: proposal.Hash(), Approved: true,
		Hosts: []string{"cloud.nais.io", "example.nais.io"}, CpltStamp: minCpltStampProtectingNavPilotState, At: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if out := launch(); !strings.Contains(out, "approved for this scope") {
		t.Errorf("re-approval with the same grants was silent: %q", out)
	}
	setVerbose(t, true)
	if out := launch(); !strings.Contains(out, "approved for this scope") {
		t.Errorf("--verbose was silent: %q", out)
	}
}
