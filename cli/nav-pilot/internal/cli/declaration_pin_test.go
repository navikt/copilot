package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// ─── a pinned revision actually installs ─────────────────────────────────────

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// gitAgentpakke is a real git repository shipping a real agentpakke, whose one
// agent says "REVISION ONE" at the first commit and "REVISION TWO" at the
// second. It returns the two full commit SHAs.
func gitAgentpakke(t *testing.T) (dir, first, second string) {
	t.Helper()
	dir = t.TempDir()
	git(t, dir, "init", "--quiet", "-b", "main", ".")
	mustWrite(t, filepath.Join(dir, agentpakke.ManifestDir, agentpakke.ManifestFile), tier1ManifestJSON)
	mustWrite(t, filepath.Join(dir, "plugin", "skills", "grilling", "SKILL.md"), "# Grilling\n")
	agent := filepath.Join(dir, "plugin", "agents", "grillmester.agent.md")
	mustWrite(t, agent, "---\nname: grillmester\ndescription: Chef\n---\nREVISION ONE\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "--quiet", "-m", "one")
	first = git(t, dir, "rev-parse", "HEAD")

	mustWrite(t, agent, "---\nname: grillmester\ndescription: Chef\n---\nREVISION TWO\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "--quiet", "-m", "two")
	second = git(t, dir, "rev-parse", "HEAD")
	return dir, first, second
}

// localAgentpakkeRemote makes "navikt/grillmester" resolve to a repository on
// disk, so the install runs the real resolver and the real git plumbing rather
// than a stub that only records which ref string it was handed.
func localAgentpakkeRemote(t *testing.T, dir string) {
	t.Helper()
	orig := source.RemoteURLFn
	t.Cleanup(func() { source.RemoteURLFn = orig })
	source.RemoteURLFn = func(string) string { return "file://" + dir }
}

func installedAgentBody(t *testing.T, scope *InstallScope) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(scope.RootDir, ".github", "agents", "grillmester.agent.md"))
	if err != nil {
		t.Fatalf("the install wrote no agent: %v", err)
	}
	return string(data)
}

// TestDeclaredPinInstallsThatRevision is the feature, end to end and unstubbed:
// a repo that declares a source and a commit gets that commit's content. Every
// other test in this area stubs the resolver and asserts only which ref string
// was passed, which is exactly how a pin that could never be fetched shipped.
func TestDeclaredPinInstallsThatRevision(t *testing.T) {
	isolatedConfig(t)
	repo, first, second := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)

	captureStdoutFor(t, func() {
		if err := cmdInstallAuto("grillmester", "", scope, "", "", false, false, false); err != nil {
			t.Fatalf("install from the declared pin: %v", err)
		}
	})

	body := installedAgentBody(t, scope)
	if !strings.Contains(body, "REVISION ONE") {
		t.Errorf("the pinned install did not get the pinned revision:\n%s", body)
	}
	if strings.Contains(body, "REVISION TWO") {
		t.Error("the pinned install got the tip instead of the pin")
	}
	// And the declaration still names the revision it was pinned to.
	if got := readDeclaration(t, scope).SHA; got != first {
		t.Errorf("the install moved the pin to %q, want %q", got, first)
	}
	_ = second
}

// The pin an install writes must be one a later install can fetch: that
// round trip is the whole life of the file.
func TestRecordedPinIsInstallable(t *testing.T) {
	isolatedConfig(t)
	repo, _, second := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	scope := ScopeRepo(repoTarget(t))
	captureStdoutFor(t, func() {
		if err := cmdInstallAuto("grillmester", "", scope, "", "navikt/grillmester", false, false, false); err != nil {
			t.Fatalf("first install: %v", err)
		}
	})
	pinned := readDeclaration(t, scope).SHA
	if pinned != second {
		t.Fatalf("install recorded pin %q, want the tip %q", pinned, second)
	}

	// A colleague clones the repo and runs a bare `nav-pilot install`.
	fresh := ScopeRepo(repoTarget(t))
	writeDeclaration(t, fresh, `{"contractVersion":"1","source":"navikt/grillmester","sha":"`+pinned+`"}`)
	captureStdoutFor(t, func() {
		if err := cmdInstallAuto("grillmester", "", fresh, "", "", false, false, false); err != nil {
			t.Fatalf("installing the recorded pin: %v", err)
		}
	})
	if !strings.Contains(installedAgentBody(t, fresh), "REVISION TWO") {
		t.Error("the recorded pin did not install the revision it names")
	}
}

// ─── the no-argument install reads the declaration ───────────────────────────

// TestBareInstallHonoursDeclaration is the story the feature exists for: clone
// a Nav repo, run `nav-pilot install`, get what the repo says it uses. The
// no-argument path resolved its source before the scope existed, so it never
// consulted the declaration — and then overwrote it with what it had guessed.
func TestBareInstallHonoursDeclaration(t *testing.T) {
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })
	isolatedConfig(t)
	repo, first, _ := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	target := repoTarget(t)
	scope := ScopeRepo(target)
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)

	orig := promptInstallScopeFn
	t.Cleanup(func() { promptInstallScopeFn = orig })
	promptInstallScopeFn = func(dir string) (*InstallScope, error) { return ScopeRepo(dir), nil }

	captureStdoutFor(t, func() {
		if err := cmdInstallInteractive(target, "", ""); err != nil {
			t.Fatalf("bare install: %v", err)
		}
	})

	d := readDeclaration(t, scope)
	if d.Source != "navikt/grillmester" {
		t.Errorf("a bare install repointed the declaration to %q", d.Source)
	}
	if d.SHA != first {
		t.Errorf("a bare install moved the pin to %q, want %q", d.SHA, first)
	}
	if !strings.Contains(installedAgentBody(t, scope), "REVISION ONE") {
		t.Error("a bare install ignored the declared pin")
	}
}

// The root TUI's fresh-install path is the same story through a different door.
func TestInteractiveFreshInstallHonoursDeclaration(t *testing.T) {
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })
	isolatedConfig(t)
	repo, first, _ := gitAgentpakke(t)
	localAgentpakkeRemote(t, repo)

	target := repoTarget(t)
	scope := ScopeRepo(target)
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"`+first+`"}`)

	orig := promptInstallScopeFn
	t.Cleanup(func() { promptInstallScopeFn = orig })
	promptInstallScopeFn = func(dir string) (*InstallScope, error) { return ScopeRepo(dir), nil }

	captureStdoutFor(t, func() {
		_ = interactiveFreshInstall(target, ResolvedConfig{})
	})
	d := readDeclaration(t, scope)
	if d.Source != "navikt/grillmester" || d.SHA != first {
		t.Errorf("the root TUI overwrote the declaration: %+v", d)
	}
}

// ─── a path source is never pinned ───────────────────────────────────────────

// Installing from a local checkout must not commit a sha nobody can fetch. It
// used to write "unknown", which the loader now refuses — wedging the repo.
func TestPathSourceInstallWritesNoPin(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	dir := pakkeSourceTree(t, tier1ManifestJSON)
	src := &Source{Dir: dir, SHA: "unknown", Version: "dev", Repo: dir}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	captureStdoutFor(t, func() {
		if err := cmdInstallFromSource("grillmester", src, scope, false, false, false); err != nil {
			t.Fatalf("install from a path source: %v", err)
		}
	})
	d := readDeclaration(t, scope)
	if d.SHA != "" {
		t.Errorf("a path-source install pinned %q; a local checkout has no revision to pin", d.SHA)
	}
}

// ─── export and list read the declaration ────────────────────────────────────

func TestExportHonoursDeclaration(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}`)

	var gotRef, gotRepo string
	orig := exportFn
	t.Cleanup(func() { exportFn = orig })
	exportFn = func(format string, scope *InstallScope, ref, sourceRepo, version string, dryRun, force, jsonOutput bool) error {
		gotRef, gotRepo = ref, sourceRepo
		return nil
	}
	if err := cmdExport("opencode", scope, "", "", true, false, false); err != nil {
		t.Fatalf("cmdExport: %v", err)
	}
	if gotRepo != "navikt/grillmester" {
		t.Errorf("export resolved source %q, want the declared navikt/grillmester", gotRepo)
	}
	if gotRef != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
		t.Errorf("export resolved ref %q, want the declared pin", gotRef)
	}
}

func TestListHonoursDeclaration(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	writeDeclaration(t, scope,
		`{"contractVersion":"1","source":"navikt/grillmester","sha":"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}`)

	got := captureResolveSource(t, pakkeSource(t, "navikt/grillmester"))
	captureStdoutFor(t, func() {
		if err := cmdList(scope, "", "", false, false); err != nil {
			t.Fatalf("cmdList: %v", err)
		}
	})
	if got.repo != "navikt/grillmester" || got.ref != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
		t.Errorf("list resolved (ref=%q, source=%q), want the declared pin", got.ref, got.repo)
	}
}

// ─── the refusals say something the reader can act on ────────────────────────

// TestCrossSourceRefusalNamesTheDeclaration: when the conflicting value comes
// from the repo's declaration, telling the user to clear a config key they
// never set does nothing. Name the file that actually holds it.
func TestCrossSourceRefusalNamesTheDeclaration(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n") // no configured source at all
	scope := ScopeRepo(repoTarget(t))
	writeGuardState(t, scope, defaultSourceRepo)
	writeDeclaration(t, scope, `{"contractVersion":"1","source":"navikt/grillmester"}`)

	err := guardScopeSource(scope, "")
	if err == nil {
		t.Fatal("the cross-source conflict was not refused")
	}
	if !strings.Contains(err.Error(), agentpakke.DeclarationPath) {
		t.Errorf("the refusal never names the file holding the conflicting value:\n%v", err)
	}
	if strings.Contains(err.Error(), `config set source ""`) {
		t.Errorf("the refusal sends the user to a config key that does not hold the value:\n%v", err)
	}
}

// The configured-source half must keep pointing at the config key.
func TestCrossSourceRefusalNamesTheConfigKey(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\nsource = \"navikt/grillmester\"\n")
	scope := ScopeRepo(repoTarget(t))
	writeGuardState(t, scope, defaultSourceRepo)

	err := guardScopeSource(scope, "")
	if err == nil {
		t.Fatal("the cross-source conflict was not refused")
	}
	if !strings.Contains(err.Error(), `config set source ""`) {
		t.Errorf("the refusal does not offer clearing the config key:\n%v", err)
	}
}

// ─── sync says when the scope and the repo disagree ──────────────────────────

// The scope wins (B4) and the pin is correctly left alone — but silence lets
// the two drift apart indefinitely. Install refuses this state; sync must at
// least mention it.
func TestSyncReportsDeclarationDisagreement(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n")
	scope, srcDir := installedRepoScope(t, "navikt/grillmester")

	syncWithSource(t, srcDir, defaultSourceRepo, "new5678")
	out := captureStdoutFor(t, func() {
		_ = cmdSync(scope, "", "", true, false)
	})
	if !strings.Contains(out, agentpakke.DeclarationPath) {
		t.Errorf("sync never mentioned that the repo declares another agentpakke:\n%s", out)
	}
	if !strings.Contains(out, "navikt/grillmester") {
		t.Errorf("sync did not name the declared agentpakke:\n%s", out)
	}
}

// A declaration agreeing with the scope is not worth a word.
func TestSyncSilentWhenDeclarationAgrees(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\n")
	scope, srcDir := installedRepoScope(t, defaultSourceRepo)

	syncWithSource(t, srcDir, defaultSourceRepo, "new5678")
	out := captureStdoutFor(t, func() {
		_ = cmdSync(scope, "", "", true, false)
	})
	if strings.Contains(out, "declares") {
		t.Errorf("sync warned about a declaration that agrees:\n%s", out)
	}
}
