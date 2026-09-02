package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// frozenMode turns on --frozen for one test, the way run() does for one
// invocation.
func frozenMode(t *testing.T) {
	t.Helper()
	installFrozen = true
	t.Cleanup(func() { installFrozen = false })
}

func declarationBytes(t *testing.T, scope *InstallScope) []byte {
	t.Helper()
	data, err := os.ReadFile(agentpakke.DeclarationFilePath(scope.RootDir))
	if err != nil {
		t.Fatalf("reading the declaration: %v", err)
	}
	return data
}

// ─── the real git path ───────────────────────────────────────────────────────

// TestFrozenInstallsPinnedRevisionWithoutTouchingTheFile is the flag end to
// end against a real repository: a frozen install gets the declared commit's
// content, and leaves the declaration byte-identical. The bytes matter — a
// plain install rewrites the file (indented, with minNavPilotVersion filled
// in), and a CI job must not produce a diff in a file it was only meant to
// obey.
func TestFrozenInstallsPinnedRevisionWithoutTouchingTheFile(t *testing.T) {
	isolatedConfig(t)
	frozenMode(t)
	repo, first, _ := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)
	before := declarationBytes(t, scope)

	captureStdoutFor(t, func() {
		if err := cmdInstallAuto("grillmester", "", scope, "", "", false, false, false); err != nil {
			t.Fatalf("frozen install of the declared pin: %v", err)
		}
	})

	if body := installedAgentBody(t, scope); !strings.Contains(body, "REVISION ONE") {
		t.Errorf("the frozen install did not get the pinned revision:\n%s", body)
	}
	if after := declarationBytes(t, scope); string(after) != string(before) {
		t.Errorf("the frozen install rewrote %s:\nbefore: %s\nafter:  %s",
			agentpakke.DeclarationPath, before, after)
	}
}

// A conflict is a warning to a developer and a lie to CI: the pin then says
// which revision was attempted, not what is on disk. Frozen refuses it, and
// --force is the way through.
func TestFrozenRefusesPartialInstallAndForceGetsThrough(t *testing.T) {
	isolatedConfig(t)
	frozenMode(t)
	repo, first, _ := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)
	mustWrite(t, filepath.Join(scope.RootDir, ".github", "agents", "grillmester.agent.md"),
		"---\nname: grillmester\ndescription: Chef\n---\nHAND EDITED\n")

	var err error
	captureStdoutFor(t, func() {
		err = cmdInstallAuto("grillmester", "", scope, "", "", false, false, false)
	})
	if err == nil {
		t.Fatal("a frozen install reported success while a conflict kept the tree off the pin")
	}
	if code := exitCodeFor(err); code != ExitFrozen {
		t.Errorf("the partial install exited %d, want ExitFrozen (%d)", code, ExitFrozen)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("the refusal does not say how to get through:\n%v", err)
	}

	captureStdoutFor(t, func() {
		err = cmdInstallAuto("grillmester", "", scope, "", "", false, true, false)
	})
	if err != nil {
		t.Fatalf("frozen --force: %v", err)
	}
	if body := installedAgentBody(t, scope); !strings.Contains(body, "REVISION ONE") {
		t.Errorf("--force did not make the tree match the pin:\n%s", body)
	}
}

// The JSON path returns before the state write, so it is the one that would
// print a success document over an incomplete install.
func TestFrozenRefusesPartialInstallBeforeJSON(t *testing.T) {
	isolatedConfig(t)
	frozenMode(t)
	repo, first, _ := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)
	mustWrite(t, filepath.Join(scope.RootDir, ".github", "agents", "grillmester.agent.md"), "different\n")

	var err error
	out := captureStdoutFor(t, func() {
		err = cmdInstallAuto("grillmester", "", scope, "", "", false, false, true)
	})
	if err == nil || exitCodeFor(err) != ExitFrozen {
		t.Fatalf("--json --frozen did not refuse the partial install: err=%v", err)
	}
	if strings.Contains(out, `"installed"`) {
		t.Errorf("--json printed a success document for an incomplete install:\n%s", out)
	}
}

// An item the scope cannot hold is the other half of a partial install.
func TestFrozenRefusesUnsupportedKinds(t *testing.T) {
	isolatedConfig(t)
	frozenMode(t)
	repo, first, _ := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	scope := ScopeRepo(repoTarget(t))
	scope.SupportedTypes = []string{"agent"} // a scope that cannot hold the skills
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)

	var err error
	captureStdoutFor(t, func() {
		err = cmdInstallAuto("grillmester", "", scope, "", "", false, false, false)
	})
	if err == nil {
		t.Fatal("a frozen install reported success while skipping kinds the scope cannot hold")
	}
	if code := exitCodeFor(err); code != ExitFrozen {
		t.Errorf("the incomplete install exited %d, want ExitFrozen (%d)", code, ExitFrozen)
	}
}

// ─── what it refuses before fetching anything ────────────────────────────────

func TestFrozenPrecheckRefusals(t *testing.T) {
	cases := []struct {
		name string
		body string // "" means no declaration at all
		want string
	}{
		{"no declaration", "", "no " + agentpakke.DeclarationPath},
		{"no pin", `{"contractVersion":"1","source":"navikt/grillmester"}`, "pins no revision"},
		{"short pin", `{"contractVersion":"1","source":"navikt/grillmester","sha":"9f1c0a7"}`, "40-character"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolatedConfig(t)
			frozenMode(t)
			scope := ScopeRepo(repoTarget(t))
			if tc.body != "" {
				writeDeclaration(t, scope, tc.body)
			}
			// The resolver must never be reached: the refusal comes first.
			orig := resolveSource
			t.Cleanup(func() { resolveSource = orig })
			resolveSource = func(ref, sourceRepo string) (*Source, error) {
				t.Error("a frozen install fetched a source it should have refused")
				return nil, nil
			}

			var err error
			captureStdoutFor(t, func() {
				err = cmdInstallAuto("grillmester", "", scope, "", "", false, false, false)
			})
			if err == nil {
				t.Fatal("the frozen install was not refused")
			}
			if code := exitCodeFor(err); code != ExitFrozen {
				t.Errorf("exited %d, want ExitFrozen (%d): %v", code, ExitFrozen, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the refusal does not say %q:\n%v", tc.want, err)
			}
		})
	}
}

// User scope never reads a declaration, so there is nothing to freeze against.
func TestFrozenRefusesUserScope(t *testing.T) {
	isolatedConfig(t)
	frozenMode(t)
	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	if err := frozenPrecheck(scope); err == nil || exitCodeFor(err) != ExitFrozen {
		t.Fatalf("a frozen install into user scope was not refused: %v", err)
	}
}

// ─── the flags that contradict it ────────────────────────────────────────────

// These are usage errors, not pin failures: exit 1, because the invocation is
// wrong and the pin has proven nothing either way.
func TestFrozenContradictoryFlags(t *testing.T) {
	isolatedConfig(t)
	for _, args := range [][]string{
		{"install", "grillmester", "--frozen", "--source", "navikt/other"},
		{"install", "grillmester", "--frozen", "--ref", "main"},
		{"install", "grillmester", "--frozen", "--user"},
		{"install", "grillmester", "--frozen", "--type", "agent"},
		{"sync", "--frozen"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			err := run(args)
			if err == nil {
				t.Fatal("the contradiction was accepted")
			}
			if code := exitCodeFor(err); code != ExitError {
				t.Errorf("exited %d, want ExitError (%d): %v", code, ExitError, err)
			}
			if !strings.Contains(err.Error(), "--frozen") {
				t.Errorf("the error does not name the flag it is about:\n%v", err)
			}
		})
	}
}

// --frozen must never open a picker: a CI job has nobody to answer it.
func TestFrozenNeverPrompts(t *testing.T) {
	isolatedConfig(t)
	orig := promptInstallScopeFn
	t.Cleanup(func() { promptInstallScopeFn = orig })
	promptInstallScopeFn = func(dir string) (*InstallScope, error) {
		t.Error("--frozen asked a question")
		return ScopeRepo(dir), nil
	}
	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origInteractive })

	// A repository with no declaration, so the run ends in a frozen refusal —
	// the point is which door it goes out of, not that it fails.
	t.Chdir(repoTarget(t))
	err := run([]string{"install", "grillmester", "--frozen"})
	if err == nil || exitCodeFor(err) != ExitFrozen {
		t.Fatalf("the frozen install did not refuse: %v", err)
	}

	// And a bare `install --frozen` says so instead of opening a picker.
	err = run([]string{"install", "--frozen"})
	if err == nil || !strings.Contains(err.Error(), "will not open a picker") {
		t.Fatalf("a bare frozen install did not refuse to pick for you: %v", err)
	}
}

// A single artifact is an a-la-carte install: it writes no declaration and
// ignores the items list, so there is nothing for --frozen to hold it to.
func TestFrozenRefusesSingleArtifact(t *testing.T) {
	isolatedConfig(t)
	frozenMode(t)
	repo, first, _ := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)

	var err error
	captureStdoutFor(t, func() {
		err = cmdInstallAuto("grilling", "", scope, "", "", false, false, false)
	})
	if err == nil {
		t.Fatal("a frozen install took one item out of the agentpakke")
	}
	if code := exitCodeFor(err); code != ExitFrozen {
		t.Errorf("exited %d, want ExitFrozen (%d): %v", code, ExitFrozen, err)
	}
}

// The assertion that keeps every future route to another revision out: a
// resolved source that is not the declared one is refused as a pin failure,
// not installed and not reported as an ordinary error.
func TestFrozenRefusesResolvedRevisionMismatch(t *testing.T) {
	isolatedConfig(t)
	frozenMode(t)
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}`)

	orig := resolveSource
	t.Cleanup(func() { resolveSource = orig })
	resolveSource = func(ref, sourceRepo string) (*Source, error) {
		return pakkeSource(t, "navikt/grillmester"), nil // SHA def5678, not the pin
	}

	var err error
	captureStdoutFor(t, func() {
		err = cmdInstallAuto("grillmester", "", scope, "", "", false, false, false)
	})
	if err == nil {
		t.Fatal("a frozen install accepted a revision the declaration does not name")
	}
	if code := exitCodeFor(err); code != ExitFrozen {
		t.Errorf("exited %d, want ExitFrozen (%d): %v", code, ExitFrozen, err)
	}
	if _, statErr := os.Stat(filepath.Join(scope.RootDir, ".github", "agents")); statErr == nil {
		t.Error("the refusal still installed content")
	}
}
