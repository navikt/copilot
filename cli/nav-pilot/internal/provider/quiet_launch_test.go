package provider

import (
	"io"
	"os"
	"strings"
	"testing"

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

func TestLaunchSyncProgressIsVerboseOnly(t *testing.T) {
	prev := artifacts.LaunchNotices
	t.Cleanup(func() { artifacts.LaunchNotices = prev })
	setVerbose(t, false)
	artifacts.LaunchNotices = true
	if !quietLaunchSync() {
		t.Error("a launch's --sync progress is not quiet")
	}
	setVerbose(t, true)
	if quietLaunchSync() {
		t.Error("--verbose did not show the --sync progress")
	}
	setVerbose(t, false)
	artifacts.LaunchNotices = false
	if quietLaunchSync() {
		t.Error("an explicit nav-pilot sync lost its progress lines")
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
	setVerbose(t, true)
	if out := launch(); !strings.Contains(out, "approved for this scope") {
		t.Errorf("--verbose was silent: %q", out)
	}
}
