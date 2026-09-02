package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// writeDeclaration puts a declaration into a repo-scope target.
func writeDeclaration(t *testing.T, scope *InstallScope, body string) {
	t.Helper()
	mustWrite(t, agentpakke.DeclarationFilePath(scope.RootDir), body)
}

func readDeclaration(t *testing.T, scope *InstallScope) *agentpakke.Declaration {
	t.Helper()
	d, err := agentpakke.LoadDeclaration(scope.RootDir)
	if err != nil {
		t.Fatalf("LoadDeclaration: %v", err)
	}
	return d
}

// captureResolveSource records what the source funnel was asked for and hands
// back a fixed checkout, so the precedence ladder can be asserted without
// cloning anything.
func captureResolveSource(t *testing.T, src *Source) *struct{ ref, repo string } {
	t.Helper()
	got := &struct{ ref, repo string }{}
	orig := resolveSource
	t.Cleanup(func() { resolveSource = orig })
	resolveSource = func(ref, sourceRepo string) (*Source, error) {
		got.ref, got.repo = ref, sourceRepo
		return src, nil
	}
	return got
}

// ─── the declaration decides the source ──────────────────────────────────────

// TestDeclarationDrivesInstallSourceAndRevision is the point of the file: a
// developer who clones a Nav repo and runs `nav-pilot install` gets the
// agentpakke the repo names, at the revision the repo pins, without knowing to
// pass --source.
func TestDeclarationDrivesInstallSourceAndRevision(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"deadbee"}`)

	src := pakkeSource(t, "navikt/grillmester")
	got := captureResolveSource(t, src)

	captureStdoutFor(t, func() {
		if err := cmdInstallAuto("grillmester", "", scope, "", "", true, false, false); err != nil {
			t.Fatalf("cmdInstallAuto: %v", err)
		}
	})
	if got.repo != "navikt/grillmester" || got.ref != "deadbee" {
		t.Errorf("install resolved (ref=%q, source=%q), want (ref=%q, source=%q)",
			got.ref, got.repo, "deadbee", "navikt/grillmester")
	}
}

// An explicit --source is the developer reaching past the repo's declaration on
// purpose, so the declared revision goes with the declared source: installing
// another agentpakke at this one's SHA would be meaningless.
func TestFlagSourceOverridesDeclaration(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"deadbee"}`)

	src := pakkeSource(t, "navikt/annen")
	got := captureResolveSource(t, src)
	captureStdoutFor(t, func() {
		_ = cmdInstallAuto("grillmester", "", scope, "", "navikt/annen", true, false, false)
	})
	if got.repo != "navikt/annen" || got.ref != "" {
		t.Errorf("--source install resolved (ref=%q, source=%q), want (ref=\"\", source=%q)",
			got.ref, got.repo, "navikt/annen")
	}
}

// An explicit --ref keeps the declared source and overrides only the revision.
func TestFlagRefOverridesPinnedSHA(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"deadbee"}`)

	got := captureResolveSource(t, pakkeSource(t, "navikt/grillmester"))
	captureStdoutFor(t, func() {
		_ = cmdInstallAuto("grillmester", "", scope, "main", "", true, false, false)
	})
	if got.repo != "navikt/grillmester" || got.ref != "main" {
		t.Errorf("--ref install resolved (ref=%q, source=%q), want (ref=\"main\", source=%q)",
			got.ref, got.repo, "navikt/grillmester")
	}
}

// The declaration outranks the machine-wide config key: the repo's reviewed
// choice beats one developer's default.
func TestDeclarationOutranksConfigSource(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\nsource = \""+defaultSourceRepo+"\"\n")
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"deadbee"}`)

	got := captureResolveSource(t, pakkeSource(t, "navikt/grillmester"))
	captureStdoutFor(t, func() {
		_ = cmdInstallAuto("grillmester", "", scope, "", "", true, false, false)
	})
	if got.repo != "navikt/grillmester" {
		t.Errorf("install resolved source %q, want the declared navikt/grillmester", got.repo)
	}
}

// User scope has no repository to commit a declaration to, so it never reads
// one — not even when the process happens to run inside a repo that has one.
func TestUserScopeIgnoresDeclaration(t *testing.T) {
	isolatedConfig(t)
	userScope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	writeDeclaration(t, userScope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"deadbee"}`)
	if d, err := scopeDeclaration(userScope); err != nil || d != nil {
		t.Errorf("scopeDeclaration(user) = (%+v, %v), want (nil, nil)", d, err)
	}
}

// ─── B3: the cross-source guard reads the same ladder ────────────────────────

// TestGuardScopeSourceUsesDeclaration is the #571 sharp edge on the new rung: a
// repo whose declaration names one agentpakke while its scope state records
// another must be refused, exactly as a differing config key already was.
func TestGuardScopeSourceUsesDeclaration(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n") // no configured source at all
	scope := ScopeRepo(repoTarget(t))
	writeGuardState(t, scope, defaultSourceRepo)
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"deadbee"}`)

	err := guardScopeSource(scope, "")
	if err == nil {
		t.Fatal("a declaration naming another agentpakke than the scope's own was not guarded")
	}
	if !strings.Contains(err.Error(), "--source") {
		t.Errorf("guard error %q does not tell the user how to proceed", err)
	}

	// The same declaration agreeing with the scope is not a conflict.
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"`+defaultSourceRepo+`","sha":"deadbee"}`)
	if err := guardScopeSource(scope, ""); err != nil {
		t.Errorf("a declaration naming the scope's own source was guarded: %v", err)
	}
}

// A scope predating source tracking adopts what the repo declares, so the pin
// in the pull request is what the scope ends up recorded against.
func TestSyncAdoptsDeclaredSource(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n")
	scope := ScopeRepo(repoTarget(t))
	writeGuardState(t, scope, "")
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester","sha":"deadbee"}`)

	adopted, err := adoptSyncSource(scope, "")
	if err != nil {
		t.Fatalf("adoptSyncSource: %v", err)
	}
	if adopted != "navikt/grillmester" {
		t.Errorf("adoptSyncSource = %q, want the declared navikt/grillmester", adopted)
	}
}

// ─── install writes the pin ──────────────────────────────────────────────────

func TestInstallWritesDeclaration(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	src := pakkeSource(t, "navikt/grillmester")

	captureStdoutFor(t, func() {
		if err := cmdInstallFromSource("grillmester", src, scope, false, false, false); err != nil {
			t.Fatalf("cmdInstallFromSource: %v", err)
		}
	})
	d := readDeclaration(t, scope)
	if d.Source != "navikt/grillmester" || d.SHA != src.SHA {
		t.Errorf("declaration after install = %+v, want source navikt/grillmester at %s", d, src.SHA)
	}
}

// A dry run installs nothing, so it pins nothing.
func TestDryRunInstallWritesNoDeclaration(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	captureStdoutFor(t, func() {
		_ = cmdInstallFromSource("grillmester", pakkeSource(t, "navikt/grillmester"), scope, true, false, false)
	})
	if _, err := os.Stat(agentpakke.DeclarationFilePath(scope.RootDir)); !os.IsNotExist(err) {
		t.Error("a dry-run install wrote a declaration")
	}
}

// A user-scope install has no repository to pin in.
func TestUserInstallWritesNoDeclaration(t *testing.T) {
	isolatedConfig(t)
	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	captureStdoutFor(t, func() {
		_ = cmdInstallFromSource("grillmester", pakkeSource(t, "navikt/grillmester"), scope, false, false, false)
	})
	if _, err := os.Stat(agentpakke.DeclarationFilePath(scope.RootDir)); !os.IsNotExist(err) {
		t.Error("a user-scope install wrote a declaration")
	}
}

// The hand-written item list survives an install. Rewriting it from whatever
// happened to be installed would turn every upstream addition into a merge
// conflict for every consumer.
func TestInstallPreservesDeclaredItems(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"old1234","items":{"grillmester":"agent"}}`)

	src := pakkeSource(t, "navikt/grillmester")
	captureStdoutFor(t, func() {
		if err := cmdInstallFromSource("grillmester", src, scope, false, false, false); err != nil {
			t.Fatalf("cmdInstallFromSource: %v", err)
		}
	})
	d := readDeclaration(t, scope)
	if d.Items["grillmester"] != "agent" || len(d.Items) != 1 {
		t.Errorf("install rewrote the item list: %+v", d.Items)
	}
	if d.SHA != src.SHA {
		t.Errorf("install left the pin at %q, want %q", d.SHA, src.SHA)
	}
}

// ─── per-item selection ──────────────────────────────────────────────────────

// A team taking one of an agentpakke's items must not have to fork it. The
// fixture ships one agent and one skill; the declaration takes the agent.
func TestDeclaredItemsNarrowTheInstall(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"old1234","items":{"grillmester":"agent"}}`)

	captureStdoutFor(t, func() {
		if err := cmdInstallFromSource("grillmester", pakkeSource(t, "navikt/grillmester"), scope, false, false, false); err != nil {
			t.Fatalf("cmdInstallFromSource: %v", err)
		}
	})
	if _, err := os.Stat(filepath.Join(scope.RootDir, ".github", "agents", "grillmester.agent.md")); err != nil {
		t.Errorf("the declared agent was not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scope.RootDir, ".github", "skills", "grilling")); !os.IsNotExist(err) {
		t.Error("a skill the declaration does not name was installed anyway")
	}
}

// A typo that silently installs three of four agents is precisely what a
// committed, reviewed declaration exists to prevent, so an unknown item refuses
// the install rather than skipping the item.
func TestUnknownDeclaredItemRefusesInstall(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"old1234","items":{"grilmester":"agent"}}`)

	var err error
	captureStdoutFor(t, func() {
		err = cmdInstallFromSource("grillmester", pakkeSource(t, "navikt/grillmester"), scope, false, false, false)
	})
	if err == nil {
		t.Fatal("an item the agentpakke does not ship was accepted")
	}
	if !strings.Contains(err.Error(), "grilmester") {
		t.Errorf("error %q does not name the offending item", err)
	}
	if _, statErr := os.Stat(filepath.Join(scope.RootDir, ".github", "agents")); !os.IsNotExist(statErr) {
		t.Error("the refused install left files behind")
	}
}

// Selection is a Tier 1 concept: a Tier 2 payload tree is staged and
// digest-verified whole, so naming items against one is refused rather than
// quietly ignored.
func TestDeclaredItemsRefusedForTier2(t *testing.T) {
	isolatedConfig(t)
	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	// A Tier 2 pakke only installs into user scope, which has no declaration;
	// the refusal is asserted on the guard itself, which is where every install
	// path reaches it.
	src := &Source{Dir: tier2SourceTree(t), SHA: "abc1234", Version: "dev", Repo: "navikt/grillmester"}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	_ = scope
	if err := guardDeclaredItems(src, map[string]string{"grillmester": "agent"}); err == nil {
		t.Fatal("per-item selection against a Tier 2 agentpakke was accepted")
	}
	if err := guardDeclaredItems(src, nil); err != nil {
		t.Errorf("a Tier 2 install without an item list was refused: %v", err)
	}
}

// ─── sync bumps the pin ──────────────────────────────────────────────────────

func syncWithSource(t *testing.T, dir, repo, sha string) {
	t.Helper()
	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*Source, error) {
		return &Source{Dir: dir, SHA: sha, Version: "dev", Repo: repo}, nil
	}
}

// installedRepoScope is a repo that has actually installed a collection, which
// is what gives sync something to diff. It returns the scope and the source
// tree the install came from, so a test can move the source and sync onto it.
func installedRepoScope(t *testing.T, declaredSource string) (*InstallScope, string) {
	t.Helper()
	scope := ScopeRepo(repoTarget(t))
	srcDir := legacySourceTree(t)
	src := &Source{Dir: srcDir, SHA: "old1234", Version: "dev", Repo: defaultSourceRepo}
	captureStdoutFor(t, func() {
		if err := cmdInstallFromSource("fullstack", src, scope, false, false, false); err != nil {
			t.Fatalf("cmdInstallFromSource: %v", err)
		}
	})
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"`+declaredSource+`","sha":"old1234"}`)
	// Move the source so the sync has a real update to apply.
	mustWrite(t, filepath.Join(srcDir, "agents", "test-a.agent.md"),
		"---\nname: test-a\ndescription: A\n---\nBody A, revised\n")
	return scope, srcDir
}

// The whole reason the SHA lives in a committed file: `sync --apply` turns an
// agentpakke update into a one-line diff a reviewer can see.
func TestSyncApplyBumpsPinnedSHA(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n")
	scope, srcDir := installedRepoScope(t, defaultSourceRepo)

	syncWithSource(t, srcDir, defaultSourceRepo, "new5678")
	captureStdoutFor(t, func() {
		if err := cmdSync(scope, "", "", true, false); err != nil {
			t.Fatalf("cmdSync --apply: %v", err)
		}
	})
	if got := readDeclaration(t, scope).SHA; got != "new5678" {
		t.Errorf("pinned SHA after sync --apply = %q, want new5678", got)
	}
}

// A check-only sync must never dirty a file the developer would have to commit.
func TestSyncCheckDoesNotBumpPinnedSHA(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n")
	scope, srcDir := installedRepoScope(t, defaultSourceRepo)

	syncWithSource(t, srcDir, defaultSourceRepo, "new5678")
	captureStdoutFor(t, func() {
		_ = cmdSync(scope, "", "", false, false)
	})
	if got := readDeclaration(t, scope).SHA; got != "old1234" {
		t.Errorf("a check-only sync moved the pin to %q", got)
	}
}

// Sync bumps a revision; it never repoints a repo at another agentpakke. That
// is an install, where the source is actually chosen.
func TestSyncDoesNotRepointDeclaration(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n")
	scope, srcDir := installedRepoScope(t, "navikt/grillmester")

	syncWithSource(t, srcDir, defaultSourceRepo, "new5678")
	captureStdoutFor(t, func() {
		_ = cmdSync(scope, "", "", true, false)
	})
	d := readDeclaration(t, scope)
	if d.Source != "navikt/grillmester" || d.SHA != "old1234" {
		t.Errorf("sync from another source touched the declaration: %+v", d)
	}
}
