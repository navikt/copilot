package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The same declaration, whichever way the install comes in.
//
// installPaths are the three ways an install reaches a source. The item list
// hung on the first of them alone: `--all` and the picker built the composed
// resolver and never read the declaration, so a repo that had committed a
// selection got every artifact anyway (#869).
var installPaths = []struct {
	name string
	run  func(scope *InstallScope, src *Source) error
}{
	{"install <navn>", func(scope *InstallScope, src *Source) error {
		return cmdInstallFromSource("grillmester", src, scope, false, false, false)
	}},
	{"install --all", func(scope *InstallScope, src *Source) error {
		return installAllFromSource(scope, src, nil, false, false, false)
	}},
	{"plukkeren", func(scope *InstallScope, src *Source) error {
		return interactiveUserInstallFromSource(scope, src, "")
	}},
}

// declaringScope is a repo scope that has committed a selection against a
// source. Repo scope because that is the only scope a declaration lives in:
// user scope has no repository to commit one to.
func declaringScope(t *testing.T, items string) *InstallScope {
	t.Helper()
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"01d1234000000000000000000000000000000000","items":`+items+`}`)
	return scope
}

// A declared selection means the same thing on every path in. The fixture ships
// an agent and a skill; the declaration takes the agent, so a path that ignores
// the list installs the skill too.
func TestEveryInstallPathHonoursDeclaredItems(t *testing.T) {
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })

	for _, path := range installPaths {
		t.Run(path.name, func(t *testing.T) {
			isolatedConfig(t)
			scope := declaringScope(t, `{"grillmester":"agent"}`)

			var err error
			captureStdoutFor(t, func() { err = path.run(scope, pakkeSource(t, "navikt/grillmester")) })
			if err != nil {
				t.Fatalf("install: %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(scope.RootDir, ".github", "agents", "grillmester.agent.md")); statErr != nil {
				t.Errorf("the declared agent did not land: %v", statErr)
			}
			if _, statErr := os.Stat(filepath.Join(scope.RootDir, ".github", "skills", "grilling")); !os.IsNotExist(statErr) {
				t.Error("an artifact the declaration does not name was installed anyway")
			}
		})
	}
}

// And a selection the source cannot honour is refused the same way on every
// path. A payload tree is staged and digest-verified as a whole, so there is no
// per-artifact selection to make in it. The paths that skipped this guard went
// on to fail for an unrelated reason, which told the reader nothing about the
// list they had committed.
func TestEveryInstallPathRefusesDeclaredItemsForTier2(t *testing.T) {
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })

	for _, path := range installPaths {
		t.Run(path.name, func(t *testing.T) {
			isolatedConfig(t)
			scope := declaringScope(t, `{"grillmester":"agent"}`)
			src := &Source{Dir: tier2SourceTree(t), SHA: "abc1234", Version: "dev", Repo: "navikt/grillmester"}
			if err := attachPakke(src); err != nil {
				t.Fatal(err)
			}

			var err error
			captureStdoutFor(t, func() { err = path.run(scope, src) })
			if err == nil {
				t.Fatal("a per-item selection against a Tier 2 agentpakke was accepted")
			}
			if !strings.Contains(err.Error(), "no per-item selection") {
				t.Errorf("the refusal says nothing about the item list: %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(scope.RootDir, ".github")); !os.IsNotExist(statErr) {
				t.Error("the refused install left files behind")
			}
		})
	}
}
