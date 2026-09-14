package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"
)

// A user with Nav's agentpakke installed user-wide ran
// `nav-pilot install nais-platform --source nais/pilot --user`, and watched 63
// lines say each of Nav's artifacts was no longer part of the install. None of
// them was: the scope had changed agentpakke, and nothing said so, what it
// cost, or how to undo it.
//
// These drive the real install path — two installs into one scope, the second
// from another source — so a refactor that stops routing through the prompt
// fails here rather than in a helper nobody calls.

// switchSource builds a resolved agentpakke over a temp tree, under the source
// repo id the test wants it to have.
func switchSource(t *testing.T, name, repo string, agents ...string) *Source {
	t.Helper()
	dir := t.TempDir()
	writePakke(t, dir, name, agents...)
	src := &Source{Dir: dir, SHA: "abc1234", Version: "dev", Repo: repo}
	if err := attachPakke(src); err != nil {
		t.Fatalf("attachPakke: %v", err)
	}
	return src
}

// installPakke runs the install the way the command does.
func installPakke(t *testing.T, src *Source, scope *InstallScope) {
	t.Helper()
	if err := cmdInstallFromSource(pakkeInstallName(src), src, scope, false, false, false); err != nil {
		t.Fatalf("install from %s: %v", src.Repo, err)
	}
}

// stubAskSwitch replaces the confirm prompt. It records the title it was given
// and the default it was handed, and answers with `answer`.
func stubAskSwitch(t *testing.T, answer bool) (asked *int, title *string, defaultedYes *bool) {
	t.Helper()
	orig := askSwitch
	t.Cleanup(func() { askSwitch = orig })
	asked, title, defaultedYes = new(int), new(string), new(bool)
	askSwitch = func(q string, proceed *bool) error {
		*asked++
		*title = q
		*defaultedYes = *proceed
		*proceed = answer
		return nil
	}
	return
}

func installed(t *testing.T, scope *InstallScope, agent string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(scope.RootDir, ".github", "agents", agent+".agent.md"))
	return err == nil
}

// A file that goes because the scope changed agentpakke is not retired
// upstream, and must not be reported as if it were.
func TestInstallFromAnotherSourceReportsTheSwitchNotARetirement(t *testing.T) {
	isolatedConfig(t)
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })

	scope := ScopeRepo(repoTarget(t))
	installPakke(t, switchSource(t, "navpakke", "navikt/copilot", "klarsprak"), scope)

	nais := switchSource(t, "naispakke", "nais/pilot", "nais-platform")
	out := captureStdoutFor(t, func() { installPakke(t, nais, scope) })

	if strings.Contains(out, "no longer part of the install") {
		t.Errorf("a switch was reported as an upstream retirement:\n%s", out)
	}
	for _, want := range []string{"navikt/copilot", "nais/pilot", "Removed 1 file(s) from"} {
		if !strings.Contains(out, want) {
			t.Errorf("the switch output never says %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "klarsprak.agent.md") > 0 {
		t.Errorf("the removed files were listed one by one instead of summarised:\n%s", out)
	}
	if installed(t, scope, "klarsprak") {
		t.Error("the previous agentpakke's agent survived the switch")
	}
}

// The reversal line is the whole point of naming the previous source: the user
// has just been told their content is gone.
func TestSwitchNamesTheCommandThatPutsThePreviousPakkeBack(t *testing.T) {
	isolatedConfig(t)
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })

	scope := ScopeRepo(repoTarget(t))
	installPakke(t, switchSource(t, "navpakke", "navikt/copilot", "klarsprak"), scope)
	out := captureStdoutFor(t, func() {
		installPakke(t, switchSource(t, "naispakke", "nais/pilot", "nais-platform"), scope)
	})

	want := "nav-pilot install navpakke --source navikt/copilot --repo"
	if !strings.Contains(out, want) {
		t.Errorf("no way back was offered; wanted %q in:\n%s", want, out)
	}
}

// An artifact the same agentpakke stopped shipping keeps the wording it has
// always had: that one really is gone from the install.
func TestRetirementWithinOneSourceKeepsItsWording(t *testing.T) {
	isolatedConfig(t)
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })

	scope := ScopeRepo(repoTarget(t))
	installPakke(t, switchSource(t, "navpakke", "navikt/copilot", "klarsprak", "gammel"), scope)

	trimmed := switchSource(t, "navpakke", "navikt/copilot", "klarsprak")
	out := captureStdoutFor(t, func() { installPakke(t, trimmed, scope) })

	if !strings.Contains(out, "no longer part of the install") {
		t.Errorf("a genuine retirement lost its wording:\n%s", out)
	}
	if strings.Contains(out, "Put navikt/copilot back") {
		t.Errorf("a retirement was reported as a source switch:\n%s", out)
	}
	if installed(t, scope, "gammel") {
		t.Error("the retired agent is still installed")
	}
}

// With a terminal the switch is a question, and Enter answers it yes: the user
// passed --source, which is the documented way to ask for exactly this.
func TestSwitchAsksFirstAndDefaultsToYes(t *testing.T) {
	isolatedConfig(t)
	forceInteractive(t)
	asked, title, defaultedYes := stubAskSwitch(t, true)

	scope := ScopeRepo(repoTarget(t))
	installPakke(t, switchSource(t, "navpakke", "navikt/copilot", "klarsprak"), scope)
	installPakke(t, switchSource(t, "naispakke", "nais/pilot", "nais-platform"), scope)

	if *asked != 1 {
		t.Fatalf("the switch prompt ran %d times, want 1", *asked)
	}
	if !*defaultedYes {
		t.Error("Enter would have cancelled the switch the user asked for")
	}
	for _, want := range []string{"nais/pilot", "repo"} {
		if !strings.Contains(*title, want) {
			t.Errorf("the question never mentions %q: %q", want, *title)
		}
	}
	if installed(t, scope, "klarsprak") || !installed(t, scope, "nais-platform") {
		t.Error("answering yes did not carry out the switch")
	}
}

// Answering no leaves the scope as it was, rather than holding both pakker.
func TestDecliningTheSwitchInstallsNothing(t *testing.T) {
	isolatedConfig(t)
	forceInteractive(t)
	stubAskSwitch(t, false)

	scope := ScopeRepo(repoTarget(t))
	installPakke(t, switchSource(t, "navpakke", "navikt/copilot", "klarsprak"), scope)

	nais := switchSource(t, "naispakke", "nais/pilot", "nais-platform")
	if err := cmdInstallFromSource(pakkeInstallName(nais), nais, scope, false, false, false); err != errInstallCancelled {
		t.Fatalf("declining returned %v, want errInstallCancelled", err)
	}
	if !installed(t, scope, "klarsprak") || installed(t, scope, "nais-platform") {
		t.Error("a declined switch changed the scope anyway")
	}
	if state, _ := readScopedState(scope); state == nil || state.SourceRepo != "navikt/copilot" {
		t.Errorf("a declined switch rewrote the state: %+v", state)
	}
}

// Ctrl-C at the question is a no, not a yes: nobody agreed to lose files.
func TestAbortingTheSwitchPromptCancels(t *testing.T) {
	isolatedConfig(t)
	forceInteractive(t)
	orig := askSwitch
	t.Cleanup(func() { askSwitch = orig })
	askSwitch = func(string, *bool) error { return huh.ErrUserAborted }

	scope := ScopeRepo(repoTarget(t))
	installPakke(t, switchSource(t, "navpakke", "navikt/copilot", "klarsprak"), scope)

	nais := switchSource(t, "naispakke", "nais/pilot", "nais-platform")
	if err := cmdInstallFromSource(pakkeInstallName(nais), nais, scope, false, false, false); err != errInstallCancelled {
		t.Fatalf("aborting returned %v, want errInstallCancelled", err)
	}
	if !installed(t, scope, "klarsprak") || installed(t, scope, "nais-platform") {
		t.Error("an aborted switch changed the scope anyway")
	}
}

// The user-scope picker sends a payload-only pakke straight to the pin path,
// which removes the outgoing install's files. That door asks too.
func TestPickerPinPathAsksBeforeSwitching(t *testing.T) {
	scope := pinEnv(t)
	forceInteractive(t)
	asked, _, _ := stubAskSwitch(t, false)

	legacy := &Source{Dir: legacySourceTree(t), SHA: "abc1234", Version: "dev", Repo: "navikt/other"}
	if err := attachPakke(legacy); err != nil {
		t.Fatal(err)
	}
	if err := cmdInstallFromSource("fullstack", legacy, scope, false, false, false); err != nil {
		t.Fatalf("Tier 1 install: %v", err)
	}
	kept := filepath.Join(scope.RootDir, "agents", "test-a.agent.md")

	if err := interactiveUserInstallFromSource(scope, tier2PinSource(t, "sha-one"), ""); err != errInstallCancelled {
		t.Fatalf("declining returned %v, want errInstallCancelled", err)
	}
	if *asked != 1 {
		t.Errorf("the pin path asked %d times, want 1", *asked)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("a declined switch removed %s: %v", kept, err)
	}
	if state, _ := readScopedState(scope); state == nil || state.SourceRepo != "navikt/other" {
		t.Errorf("a declined switch rewrote the state: %+v", state)
	}
}

// The picker and `install --all` force-refresh managed files on every
// re-install, which is exactly when a scope has another pakke to lose. That
// internal force must not pass for the user's --force and skip the question.
func TestRefreshForceDoesNotSkipTheSwitchPrompt(t *testing.T) {
	isolatedConfig(t)
	forceInteractive(t)
	asked, _, _ := stubAskSwitch(t, true)

	scope := ScopeRepo(repoTarget(t))
	installPakke(t, switchSource(t, "navpakke", "navikt/copilot", "klarsprak"), scope)

	nais := switchSource(t, "naispakke", "nais/pilot", "nais-platform")
	if err := installAllFromSource(scope, nais, nil, false, true, false); err != nil {
		t.Fatalf("install --all: %v", err)
	}
	if *asked != 1 {
		t.Errorf("the switch prompt ran %d times under the picker's force, want 1", *asked)
	}
}

// --force on the command line is the one thing that skips the question.
func TestForceFlagSkipsTheSwitchPrompt(t *testing.T) {
	isolatedConfig(t)
	forceInteractive(t)
	asked, _, _ := stubAskSwitch(t, false)
	installForce = true
	t.Cleanup(func() { installForce = false })

	scope := ScopeRepo(repoTarget(t))
	installPakke(t, switchSource(t, "navpakke", "navikt/copilot", "klarsprak"), scope)
	installPakke(t, switchSource(t, "naispakke", "nais/pilot", "nais-platform"), scope)

	if *asked != 0 {
		t.Errorf("--force still asked %d time(s)", *asked)
	}
	if installed(t, scope, "klarsprak") || !installed(t, scope, "nais-platform") {
		t.Error("--force did not carry out the switch")
	}
}

// A state entry with nothing behind it on disk (a picker-ignored item) is not
// a file about to be removed, and must not pad either number.
func TestSwitchCountsOnlyFilesOnDisk(t *testing.T) {
	isolatedConfig(t)
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })

	scope := ScopeRepo(repoTarget(t))
	installPakke(t, switchSource(t, "navpakke", "navikt/copilot", "klarsprak"), scope)
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("state after install: %v", err)
	}
	state.Files = append(state.Files, InstalledFile{Path: ".github/agents/ghost.agent.md", Status: fileStatusIgnored})
	if err := writeScopedState(scope, state); err != nil {
		t.Fatal(err)
	}

	out := captureStdoutFor(t, func() {
		installPakke(t, switchSource(t, "naispakke", "nais/pilot", "nais-platform"), scope)
	})
	for _, want := range []string{"Up to 1 file(s)", "Removed 1 file(s)"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q missing; an ignored entry was counted as a file:\n%s", want, out)
		}
	}
}

// No terminal means no prompt — scripts and CI install this way — but the
// switch is still announced before anything is removed.
func TestNonInteractiveSwitchProceedsAndSaysSo(t *testing.T) {
	isolatedConfig(t)
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })
	orig := askSwitch
	t.Cleanup(func() { askSwitch = orig })
	askSwitch = func(string, *bool) error {
		t.Error("a run with no terminal asked a question nothing could answer")
		return nil
	}

	scope := ScopeRepo(repoTarget(t))
	installPakke(t, switchSource(t, "navpakke", "navikt/copilot", "klarsprak"), scope)
	out := captureStdoutFor(t, func() {
		installPakke(t, switchSource(t, "naispakke", "nais/pilot", "nais-platform"), scope)
	})

	for _, want := range []string{"is installed from", "will be removed", "navikt/copilot", "nais/pilot"} {
		if !strings.Contains(out, want) {
			t.Errorf("the switch was not announced (%q missing):\n%s", want, out)
		}
	}
	if installed(t, scope, "klarsprak") {
		t.Error("the switch did not happen")
	}
}
