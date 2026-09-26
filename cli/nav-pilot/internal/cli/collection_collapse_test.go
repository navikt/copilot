package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
)

// ─── the five collections collapse into the one agentpakke (#468) ────────────

// collapsedManifestJSON is the shape navikt/copilot ships after the collapse:
// one pakke, canonical layout, no collections/.
const collapsedManifestJSON = `{
  "contractVersion": "1",
  "name": "nav-pilot",
  "description": "Nav's default agents, skills, instructions, and prompts",
  "clients": {"copilot": {"primaryAgents": ["nav-pilot"]}},
  "layout": {"agents": "agents", "skills": "skills"}
}`

// collapseSource turns a legacy source tree into the post-collapse one: the
// manifest appears, and the pool holds artifacts no collection ever named.
func collapseSource(t *testing.T, dir string) {
	t.Helper()
	mustWrite(t, filepath.Join(dir, ".nav-pilot", "agentpakke.json"), collapsedManifestJSON)
	mustWrite(t, filepath.Join(dir, "agents", "test-b.agent.md"), "---\nname: test-b\ndescription: B\n---\nBody B\n")
	mustWrite(t, filepath.Join(dir, "skills", "test-t", "SKILL.md"), "# Skill T\n")
}

// syncFrom points sync at dir as the default source, manifest attached.
func syncFrom(t *testing.T, dir string) {
	t.Helper()
	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*Source, error) {
		src := &Source{Dir: dir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}
		return src, attachPakke(src)
	}
}

// fullstackScope installs the fullstack collection against the legacy source
// and returns the repo scope it landed in. The pool grows two artifacts the
// collection never named once the source collapses.
func fullstackScope(t *testing.T, srcDir string) *InstallScope {
	t.Helper()
	scope := ScopeRepo(repoTarget(t))
	src := &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	captureStdoutFor(t, func() {
		if err := cmdInstallFromSource("fullstack", src, scope, false, false, false); err != nil {
			t.Fatalf("legacy install: %v", err)
		}
	})
	return scope
}

// theRestOfThePakke is what a fullstack install does not have once the source
// collapses: the artifacts no collection ever named.
var theRestOfThePakke = []string{".github/agents/test-b.agent.md", ".github/skills/test-t/"}

// treeSnapshot is every file under dir with its bytes, so a test can say that
// a command changed nothing rather than that it changed nothing it looked for.
func treeSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(b)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

// ignoredPaths lists the entries a scope's state writes off.
func ignoredPaths(t *testing.T, scope *InstallScope) []string {
	t.Helper()
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("readScopedState = (%v, %v)", state, err)
	}
	var paths []string
	for _, f := range state.Files {
		if f.Status == fileStatusIgnored {
			paths = append(paths, f.Path)
		}
	}
	return paths
}

// TestAdoptionWritesNoIgnores: a collection-era scope meeting its source's
// manifest keeps the collection alive as an invisible ignore list if the
// adoption writes off everything the collection left out — and `ignored` then
// means two things in one file, the user's own deselection in the picker and a
// migration's leftovers. It means one thing (#878).
func TestAdoptionWritesNoIgnores(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	scope := fullstackScope(t, srcDir)

	collapseSource(t, srcDir)
	syncFrom(t, srcDir)

	out := captureStdoutFor(t, func() { _ = cmdSync(scope, "", "", true, false) })
	if got := ignoredPaths(t, scope); len(got) != 0 {
		t.Errorf("adoption wrote %v as ignored; the migration must leave the ignore list to the user:\n%s", got, out)
	}
	state, _ := readScopedState(scope)
	if state == nil || state.Collection != "nav-pilot" {
		t.Fatalf("state.Collection = %v, want the pakke name", state)
	}
	if !strings.Contains(out, "fullstack") || !strings.Contains(out, "nav-pilot") {
		t.Errorf("adoption is silent; output must name both identities:\n%s", out)
	}
}

// TestSyncReportsWhatTheCollectionLeftOut: the artifacts the scope does not
// have are pending work, reported next to the updates and counted by the exit
// code. Before this they were a note pointing at `nav-pilot install`, which is
// not what a user who just ran sync is going to type.
func TestSyncReportsWhatTheCollectionLeftOut(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	scope := fullstackScope(t, srcDir)

	collapseSource(t, srcDir)
	syncFrom(t, srcDir)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if !errors.Is(err, errUpdatesAvailable) {
		t.Fatalf("cmdSync = %v, want errUpdatesAvailable for a scope missing half its pakke:\n%s", err, out)
	}
	for _, want := range theRestOfThePakke {
		if !strings.Contains(out, want) {
			t.Errorf("sync does not report %q as pending:\n%s", want, out)
		}
	}
}

// TestSyncJSONReportsAdded pins the document `sync --json` writes: `added`
// arrives next to `updates`, and nothing already there changes shape.
func TestSyncJSONReportsAdded(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	scope := fullstackScope(t, srcDir)

	collapseSource(t, srcDir)
	syncFrom(t, srcDir)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, true) })
	if !errors.Is(err, errUpdatesAvailable) {
		t.Fatalf("cmdSync --json = %v, want errUpdatesAvailable:\n%s", err, out)
	}
	var doc struct {
		UpToDate bool     `json:"up_to_date"`
		Added    []string `json:"added"`
	}
	if jsonErr := json.Unmarshal([]byte(out), &doc); jsonErr != nil {
		t.Fatalf("sync --json wrote no document: %v\n%s", jsonErr, out)
	}
	if doc.UpToDate {
		t.Errorf("up_to_date is true with artifacts pending:\n%s", out)
	}
	sort.Strings(doc.Added)
	if strings.Join(doc.Added, " ") != strings.Join(theRestOfThePakke, " ") {
		t.Errorf("added = %v, want %v", doc.Added, theRestOfThePakke)
	}
}

// TestSyncApplyInstallsWhatTheCollectionLeftOut: --apply is what puts them
// there. Nothing is deleted or overwritten, so the artifacts the collection
// did include are still on disk afterwards.
func TestSyncApplyInstallsWhatTheCollectionLeftOut(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	scope := fullstackScope(t, srcDir)

	collapseSource(t, srcDir)
	syncFrom(t, srcDir)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("cmdSync --apply = %v:\n%s", err, out)
	}

	tracked := map[string]bool{}
	state, _ := readScopedState(scope)
	if state == nil {
		t.Fatal("no state after sync --apply")
	}
	for _, f := range state.Files {
		tracked[f.Path] = true
	}
	for _, want := range append([]string{".github/agents/test-a.agent.md", ".github/skills/test-s/"}, theRestOfThePakke...) {
		if !tracked[want] {
			t.Errorf("%q is not tracked after sync --apply; state = %+v", want, state.Files)
		}
		if _, statErr := os.Stat(filepath.Join(scope.RootDir, filepath.FromSlash(strings.TrimSuffix(want, "/")))); statErr != nil {
			t.Errorf("%q is not on disk after sync --apply: %v", want, statErr)
		}
	}

	// The migration is over: a second sync has nothing left to say.
	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err != nil {
		t.Fatalf("second cmdSync = %v:\n%s", err, out)
	}
	if strings.Contains(out, "fullstack") {
		t.Errorf("second sync still talks about the retired collection:\n%s", out)
	}
}

// TestSyncWithoutApplyChangesNothing: a check-only sync reports the migration
// and performs none of it. The state file is committed in a repo scope, so a
// scheduled check that rewrites it hands every consumer a dirty tree.
func TestSyncWithoutApplyChangesNothing(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	scope := fullstackScope(t, srcDir)

	collapseSource(t, srcDir)
	syncFrom(t, srcDir)

	before := treeSnapshot(t, scope.RootDir)
	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if !errors.Is(err, errUpdatesAvailable) {
		t.Fatalf("cmdSync = %v, want errUpdatesAvailable:\n%s", err, out)
	}
	if diff := changedPaths(before, treeSnapshot(t, scope.RootDir)); len(diff) != 0 {
		t.Errorf("a sync without --apply changed %v:\n%s", diff, out)
	}
}

// changedPaths names every path that differs between two tree snapshots.
func changedPaths(before, after map[string]string) []string {
	var changed []string
	for path, body := range after {
		if prior, ok := before[path]; !ok || prior != body {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

// TestSyncAdoptsAllInstallWithPoolGrowth: "(all)" meant everything, so its
// adoption is a rename — and everything the pool has grown since is pending
// work, on the same terms as a collection's remainder.
func TestSyncAdoptsAllInstallWithPoolGrowth(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(scope.RootDir, "agents", "test-a.agent.md"),
		"---\nname: test-a\ndescription: A\n---\nBody A\n")
	if err := writeScopedState(scope, &StateFile{
		Collection: CollectionAll,
		Version:    "dev",
		Scope:      scope.Name,
		SourceRepo: defaultSourceRepo,
		SourceSHA:  "abc1234",
		Files:      []InstalledFile{{Path: "agents/test-a.agent.md"}},
	}); err != nil {
		t.Fatal(err)
	}

	collapseSource(t, srcDir)
	syncFrom(t, srcDir)

	var syncErr error
	out := captureStdoutFor(t, func() { syncErr = cmdSync(scope, "", "", true, false) })
	if syncErr != nil {
		t.Fatalf("cmdSync --apply = %v:\n%s", syncErr, out)
	}

	state, _ := readScopedState(scope)
	if state == nil || state.Collection != "nav-pilot" {
		t.Fatalf("state.Collection = %v, want the pakke name", state)
	}
	if got := ignoredPaths(t, scope); len(got) != 0 {
		t.Errorf("an (all) adoption must not ignore anything, got %v", got)
	}
	for _, want := range []string{"agents/test-b.agent.md", "skills/test-t/"} {
		found := false
		for _, f := range state.Files {
			found = found || f.Path == want
		}
		if !found {
			t.Errorf("%q did not arrive in an (all) scope: %+v", want, state.Files)
		}
	}
}

// TestSyncLeavesALaCarteScopeAlone: an à-la-carte install never claimed a
// collection, so the collapse has nothing to migrate.
func TestSyncLeavesALaCarteScopeAlone(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	target := repoTarget(t)
	scope := ScopeRepo(target)
	mustWrite(t, filepath.Join(target, ".github", "agents", "test-a.agent.md"),
		"---\nname: test-a\ndescription: A\n---\nBody A\n")
	if err := writeScopedState(scope, &StateFile{
		Collection: "(à la carte)",
		Version:    "dev",
		Scope:      scope.Name,
		SourceRepo: defaultSourceRepo,
		SourceSHA:  "abc1234",
		Files:      []InstalledFile{{Path: ".github/agents/test-a.agent.md"}},
	}); err != nil {
		t.Fatal(err)
	}

	collapseSource(t, srcDir)
	syncFrom(t, srcDir)

	before := treeSnapshot(t, target)
	out := captureStdoutFor(t, func() {
		if err := cmdSync(scope, "", "", true, false); err != nil {
			t.Fatalf("cmdSync --apply = %v", err)
		}
	})

	state, _ := readScopedState(scope)
	if state == nil || state.Collection != "(à la carte)" {
		t.Fatalf("state.Collection = %v, want the à-la-carte label untouched", state)
	}
	if len(state.Files) != 1 || state.Files[0].Status != "" {
		t.Fatalf("state.Files = %+v, want the single active entry untouched", state.Files)
	}
	// Not even under --apply: a scope that picked its artifacts one by one is
	// not a scope that tracks the whole pakke, and the fold-in must not turn
	// it into one (#878).
	if diff := changedPaths(before, treeSnapshot(t, target)); len(diff) != 0 {
		t.Errorf("sync --apply added %v to an à-la-carte scope:\n%s", diff, out)
	}
}

// TestInstallNamesTheFoldForLegacyCollection: `nav-pilot install frontend`
// against the post-collapse source refuses with the mapping, not a bare "not
// found" — the one command every collection user has in muscle memory.
func TestInstallNamesTheFoldForLegacyCollection(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	collapseSource(t, srcDir)

	orig := resolveSource
	t.Cleanup(func() { resolveSource = orig })
	resolveSource = func(ref, sourceRepo string) (*Source, error) {
		src := &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}
		return src, attachPakke(src)
	}

	scope := ScopeRepo(repoTarget(t))
	var err error
	captureStdoutFor(t, func() {
		err = cmdInstallAuto("frontend", "", scope, "", "", false, false, false)
	})
	if err == nil {
		t.Fatal("install frontend succeeded against a source that no longer ships it")
	}
	for _, want := range []string{"frontend", "nav-pilot install nav-pilot"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err, want)
		}
	}
}

// ─── fase 0: the migration works for everyone (#876, #877) ───────────────────

// clearRecordedSource turns a scope's state back into what an install from
// before source tracking wrote: files and a collection, no source.
func clearRecordedSource(t *testing.T, scope *InstallScope) {
	t.Helper()
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("readScopedState = (%v, %v)", state, err)
	}
	state.SourceRepo = ""
	if err := writeScopedState(scope, state); err != nil {
		t.Fatal(err)
	}
}

// preTrackingScope installs a collection and then strips the recorded source,
// which is the state every install from before source tracking is in.
func preTrackingScope(t *testing.T, srcDir string) *InstallScope {
	t.Helper()
	scope := ScopeRepo(repoTarget(t))
	src := &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	captureStdoutFor(t, func() {
		if err := cmdInstallFromSource("fullstack", src, scope, false, false, false); err != nil {
			t.Fatalf("legacy install: %v", err)
		}
	})
	clearRecordedSource(t, scope)
	return scope
}

// TestSyncAdoptsPreTrackingScopeWithPendingUpdates is #877 driven end to end:
// a scope from before source tracking, one file behind its source, and a
// single plain `sync` with no --apply.
//
// errUpdatesAvailable is a successful check — the source was fetched and read
// — so the scope has to come out of it owning both its source and the pakke
// identity. It came out unchanged, every time, and nothing in the output said
// so.
func TestSyncAdoptsPreTrackingScopeWithPendingUpdates(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\nsource = \""+defaultSourceRepo+"\"\n")

	srcDir := legacySourceTree(t)
	scope := preTrackingScope(t, srcDir)

	collapseSource(t, srcDir)
	// The source moves on, so the sync has something to report.
	mustWrite(t, filepath.Join(srcDir, "agents", "test-a.agent.md"),
		"---\nname: test-a\ndescription: A\n---\nBody A, revised\n")
	syncFrom(t, srcDir)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if !errors.Is(err, errUpdatesAvailable) {
		t.Fatalf("cmdSync = %v, want errUpdatesAvailable:\n%s", err, out)
	}

	state, _ := readScopedState(scope)
	if state == nil {
		t.Fatal("no state after sync")
	}
	if state.SourceRepo == "" {
		t.Errorf("state.SourceRepo is still empty after a sync that read the source:\n%s", out)
	}
	// The identity itself waits for --apply, which is what the check run says
	// (#878), and one --apply is all it waits for.
	if !strings.Contains(out, "nav-pilot") {
		t.Errorf("the check run does not name the migration it is holding:\n%s", out)
	}
	out = captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("cmdSync --apply = %v:\n%s", err, out)
	}
	if state, _ = readScopedState(scope); state.Collection != "nav-pilot" {
		t.Errorf("state.Collection = %q, want the pakke identity:\n%s", state.Collection, out)
	}
}

// TestSyncAdoptsPreTrackingScopeInOneRun: the source was recorded only after
// the sync finished, and the adoption ran before it, so the identity waited
// for a second sync that nothing asked for. One `sync --apply` does both.
func TestSyncAdoptsPreTrackingScopeInOneRun(t *testing.T) {
	path := isolatedConfig(t)
	mustWrite(t, path, "version = 1\nsource = \""+defaultSourceRepo+"\"\n")

	srcDir := legacySourceTree(t)
	scope := preTrackingScope(t, srcDir)

	collapseSource(t, srcDir)
	syncFrom(t, srcDir)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("cmdSync --apply = %v:\n%s", err, out)
	}

	state, _ := readScopedState(scope)
	if state == nil || state.Collection != "nav-pilot" {
		t.Fatalf("state.Collection = %v after one sync, want the pakke identity:\n%s", state, out)
	}
	if state.SourceRepo == "" {
		t.Errorf("state.SourceRepo is still empty after the same run:\n%s", out)
	}
}

// TestSyncRefusesScopeWhoseRecordedSourceIsGone: a scope recorded against a
// source that no longer resolves was skipped in silence — sameSourceRepo found
// no match, so the adoption never ran and nothing said why. nav-pilot must not
// move the scope to another source on its own (#691), so it says what is wrong
// and names the reinstall.
func TestSyncRefusesScopeWhoseRecordedSourceIsGone(t *testing.T) {
	isolatedConfig(t)
	srcDir := legacySourceTree(t)
	collapseSource(t, srcDir)

	gone := filepath.Join(t.TempDir(), "agentpakke-checkout")
	scope := ScopeRepo(repoTarget(t))
	state := &StateFile{
		Collection: "fullstack",
		Version:    "dev",
		Scope:      scope.Name,
		SourceRepo: gone,
		SourceSHA:  "abc1234",
	}
	if err := writeScopedState(scope, state); err != nil {
		t.Fatal(err)
	}
	syncFrom(t, srcDir)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, false) })
	if err == nil {
		t.Fatalf("sync passed over a scope whose source is gone without a word:\n%s", out)
	}
	if want := reinstallCommand(scope, state); !strings.Contains(stripANSI(err.Error()), want) {
		t.Errorf("error %q does not name the reinstall command %q", err, want)
	}
}

// TestDoctorReportsGoneSource: doctor is where someone looks when a command
// refuses, so it reports the same problem with the same command.
func TestDoctorReportsGoneSource(t *testing.T) {
	isolatedConfig(t)
	t.Chdir(t.TempDir())
	t.Setenv("PATH", t.TempDir()) // no cplt, no opencode: doctor stays offline

	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(t.TempDir(), "agentpakke-checkout")
	state := &StateFile{
		Collection: "fullstack",
		Version:    "dev",
		Scope:      scope.Name,
		SourceRepo: gone,
		SourceSHA:  "abc1234",
	}
	if err := writeScopedState(scope, state); err != nil {
		t.Fatal(err)
	}

	out := stripANSI(captureStdoutFor(t, func() { _ = cmdDoctor() }))
	if !strings.Contains(out, gone) {
		t.Errorf("doctor does not mention the source that is gone:\n%s", out)
	}
	if want := reinstallCommand(scope, state); !strings.Contains(out, want) {
		t.Errorf("doctor does not name the reinstall command %q:\n%s", want, out)
	}
}

// TestScopeStalenessCountsPendingAdoption: a user who never syncs is never
// prompted, because staleness only ever meant "a newer release exists". A
// scope still holding a collection identity has a migration waiting for the
// sync the startup already offers.
func TestScopeStalenessCountsPendingAdoption(t *testing.T) {
	orig := assessStaleness
	t.Cleanup(func() { assessStaleness = orig })
	assessStaleness = func(string) artifacts.StalenessAssessment {
		return artifacts.StalenessAssessment{LatestVersion: "2026.01.01-000000-abc1234"}
	}

	scope := ScopeRepo(repoTarget(t))
	state := &StateFile{
		Collection: "fullstack",
		Version:    "2026.01.01-000000-abc1234",
		SourceRepo: defaultSourceRepo,
	}
	if got := scopeStaleness(scope, state); got == "" {
		t.Error("a scope still recording a collection is never offered the sync that migrates it")
	}

	state.Collection = "nav-pilot"
	if got := scopeStaleness(scope, state); got != "" {
		t.Errorf("scopeStaleness = %q for a migrated scope on the newest release, want %q", got, "")
	}
}

// TestFirstRunCommandsSuggestSomethingThatRuns is #876: the commands a user
// with nothing installed meets told her to run `nav-pilot install
// <collection>`, which fails — and it is the first instruction she gets.
//
// The suggestion is taken out of each command's own output and then run, since
// checking the string alone would have passed before this bug too.
func TestFirstRunCommandsSuggestSomethingThatRuns(t *testing.T) {
	srcDir := legacySourceTree(t)
	collapseSource(t, srcDir)
	// The manifest names nav-pilot as the primary agent, so the pakke has to
	// ship it or the install refuses before it gets anywhere near the picker.
	mustWrite(t, filepath.Join(srcDir, "agents", "nav-pilot.agent.md"),
		"---\nname: nav-pilot\ndescription: Pilot\n---\nBody\n")

	commands := map[string]func(){
		"list --installed": func() { _ = cmdListInstalledAuto(t.TempDir(), false) },
		"sync":             func() { _ = cmdSyncAuto(t.TempDir(), "", "", false, false) },
		"doctor":           func() { _ = cmdDoctor() },
		"init":             func() { _ = cmdInit(repoTarget(t), false, false) },
	}

	var suggestions []string
	for name, cmd := range commands {
		t.Run(name, func(t *testing.T) {
			isolatedConfig(t)
			t.Chdir(t.TempDir())
			t.Setenv("PATH", t.TempDir())

			out := stripANSI(captureStdoutFor(t, cmd))
			suggested := firstInstallSuggestion(out)
			if suggested == "" {
				t.Fatalf("%s suggests no install command at all:\n%s", name, out)
			}
			suggestions = append(suggestions, suggested)

			// The suggestion has to be a command that runs.
			isolatedConfig(t)
			forceNonInteractive = true
			t.Cleanup(func() { forceNonInteractive = false })
			stubResolveSource(t, mustAttachPakke(t, &Source{Dir: srcDir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}))
			// Without a terminal an install asks for consent it cannot get,
			// and says to pass --yes; in one, the picker is the consent.
			args := append(strings.Fields(suggested)[1:], "--yes")
			var runErr error
			captureStdoutFor(t, func() { runErr = run(args) })
			if runErr != nil {
				t.Errorf("%s suggests %q, which fails: %v", name, suggested, runErr)
			}
		})
	}

	for _, s := range suggestions {
		if s != suggestions[0] {
			t.Errorf("the first-run commands do not agree on one wording: %q vs %q", s, suggestions[0])
		}
	}
}

func mustAttachPakke(t *testing.T, src *Source) *Source {
	t.Helper()
	if err := attachPakke(src); err != nil {
		t.Fatal(err)
	}
	return src
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiPattern.ReplaceAllString(s, "") }

// firstInstallSuggestion pulls the `nav-pilot install ...` command out of a
// command's output, stopping at the end of the line or the first punctuation
// that is prose rather than argument.
func firstInstallSuggestion(out string) string {
	i := strings.Index(out, "nav-pilot install")
	if i < 0 {
		return ""
	}
	line := out[i:]
	if j := strings.IndexAny(line, "\n.,"); j >= 0 {
		line = line[:j]
	}
	return strings.TrimSpace(line)
}
