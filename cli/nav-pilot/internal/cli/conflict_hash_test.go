package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// driftSource builds a legacy-layout source with two collections over the same
// two agents, so a test can install wide and then narrow.
func driftSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "collections", "wide", "manifest.json"),
		`{"name":"wide","description":"Wide","agents":["test-a","test-b"]}`)
	mustWrite(t, filepath.Join(dir, "collections", "narrow", "manifest.json"),
		`{"name":"narrow","description":"Narrow","agents":["test-a"]}`)
	mustWrite(t, filepath.Join(dir, "agents", "test-a.agent.md"), "---\nname: test-a\ndescription: A\n---\nBody A\n")
	mustWrite(t, filepath.Join(dir, "agents", "test-b.agent.md"), "---\nname: test-b\ndescription: B\n---\nBody B\n")
	return dir
}

func localSource(dir string) *Source {
	return &Source{Dir: dir, SHA: "abc1234", Version: "dev", Repo: defaultSourceRepo}
}

func stateEntry(t *testing.T, scope *InstallScope, path string) *InstalledFile {
	t.Helper()
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("readScopedState: %v (state=%v)", err, state)
	}
	for i, f := range state.Files {
		if f.Path == path {
			return &state.Files[i]
		}
	}
	return nil
}

const agentA = ".github/agents/test-a.agent.md"
const agentB = ".github/agents/test-b.agent.md"

// TestInstallSourceDriftIsNotConflict: nav-pilot's own untouched file must be
// updated when the source moves, not reported as the user's conflict.
func TestInstallSourceDriftIsNotConflict(t *testing.T) {
	isolatedConfig(t)
	srcDir := driftSource(t)
	target := repoTarget(t)
	scope := ScopeRepo(target)

	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// Upstream moves. The user has touched nothing.
	mustWrite(t, filepath.Join(srcDir, "agents", "test-a.agent.md"), "---\nname: test-a\ndescription: A\n---\nBody A v2\n")

	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("second install: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(target, agentA))
	if want := "---\nname: test-a\ndescription: A\n---\nBody A v2\n"; string(got) != want {
		t.Errorf("untouched file was not updated:\ngot  %q\nwant %q", got, want)
	}
	if e := stateEntry(t, scope, agentA); e == nil || e.Status == fileStatusConflict {
		t.Errorf("source drift recorded as a conflict: %+v", e)
	}
}

// TestInstallUserEditIsConflict: the protection must survive the fix.
func TestInstallUserEditIsConflict(t *testing.T) {
	isolatedConfig(t)
	srcDir := driftSource(t)
	target := repoTarget(t)
	scope := ScopeRepo(target)

	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	mustWrite(t, filepath.Join(target, agentA), "my own edit\n")
	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("second install: %v", err)
	}

	if got, _ := os.ReadFile(filepath.Join(target, agentA)); string(got) != "my own edit\n" {
		t.Errorf("user edit was overwritten: %q", got)
	}
	if e := stateEntry(t, scope, agentA); e == nil || e.Status != fileStatusConflict {
		t.Errorf("user edit not recorded as a conflict: %+v", e)
	}
}

// TestInstallUntrackedHandPlacedConflicts: a foreign file nav-pilot never wrote
// is still a conflict.
func TestInstallUntrackedHandPlacedConflicts(t *testing.T) {
	isolatedConfig(t)
	srcDir := driftSource(t)
	target := repoTarget(t)
	scope := ScopeRepo(target)

	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	// test-b was never installed; someone put their own file there.
	mustWrite(t, filepath.Join(target, agentB), "hand placed\n")

	if err := cmdInstallFromSource("wide", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("wide install: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(target, agentB)); string(got) != "hand placed\n" {
		t.Errorf("hand-placed file was overwritten: %q", got)
	}
	if e := stateEntry(t, scope, agentB); e == nil || e.Status != fileStatusConflict {
		t.Errorf("hand-placed file not recorded as a conflict: %+v", e)
	}
}

// TestInstallPreexistingConflictNotOverwritten: once a path is in conflict, a
// later install must not adopt it silently.
func TestInstallPreexistingConflictNotOverwritten(t *testing.T) {
	isolatedConfig(t)
	srcDir := driftSource(t)
	target := repoTarget(t)
	scope := ScopeRepo(target)

	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	mustWrite(t, filepath.Join(target, agentA), "my own edit\n")
	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("second install: %v", err)
	}
	if e := stateEntry(t, scope, agentA); e == nil || e.Status != fileStatusConflict {
		t.Fatalf("precondition: expected a recorded conflict, got %+v", e)
	}

	// Now upstream also moves. The conflict must stand.
	mustWrite(t, filepath.Join(srcDir, "agents", "test-a.agent.md"), "---\nname: test-a\ndescription: A\n---\nBody A v2\n")
	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("third install: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(target, agentA)); string(got) != "my own edit\n" {
		t.Errorf("pre-existing conflict was silently overwritten: %q", got)
	}
	if e := stateEntry(t, scope, agentA); e == nil || e.Status != fileStatusConflict {
		t.Errorf("pre-existing conflict lost its status: %+v", e)
	}
}

// TestNarrowInstallDropsWhatItDeleted: the two halves of this fix meet here.
// removeOrphans (#615) deletes an artifact the narrower install no longer
// ships, so the merge must not put its state entry back — a committed state
// listing a file the tool just deleted has status report it missing and the
// next sync report a deletion nav-pilot performed itself.
func TestNarrowInstallDropsWhatItDeleted(t *testing.T) {
	isolatedConfig(t)
	srcDir := driftSource(t)
	target := repoTarget(t)
	scope := ScopeRepo(target)

	if err := cmdInstallFromSource("wide", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("wide install: %v", err)
	}
	// test-b is untouched, so it is nav-pilot's to retire.
	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("narrow install: %v", err)
	}

	if _, err := os.Stat(filepath.Join(target, agentB)); !os.IsNotExist(err) {
		t.Fatalf("precondition: %s should have been removed from disk: %v", agentB, err)
	}
	if e := stateEntry(t, scope, agentB); e != nil {
		t.Errorf("state still lists %s, which the same install deleted from disk: %+v", agentB, e)
	}
}

// TestNarrowInstallKeepsTheUserEditItDidNotDelete: the hole #615 leaves, and
// the only one the merge needs to fill. removeOrphans keeps an edited file on
// disk because nav-pilot does not own it; dropping its state entry would make
// that file invisible to every later sync.
func TestNarrowInstallKeepsTheUserEditItDidNotDelete(t *testing.T) {
	isolatedConfig(t)
	srcDir := driftSource(t)
	target := repoTarget(t)
	scope := ScopeRepo(target)

	if err := cmdInstallFromSource("wide", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("wide install: %v", err)
	}
	mustWrite(t, filepath.Join(target, agentB), "my own edit\n")

	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("narrow install: %v", err)
	}

	if got, _ := os.ReadFile(filepath.Join(target, agentB)); string(got) != "my own edit\n" {
		t.Fatalf("precondition: the edited file should still be on disk, got %q", got)
	}
	if e := stateEntry(t, scope, agentB); e == nil {
		t.Errorf("the narrower install dropped %s from state while leaving it on disk; nothing can manage it now", agentB)
	}
}

// TestInstallDoesNotUnIgnore: `ignore` keeps the recorded hash, so an untouched
// ignored file hashes equal to it. Counting that as "nav-pilot's own, update
// it" overwrites the file and clears the status — the user's ignore decision,
// silently undone.
func TestInstallDoesNotUnIgnore(t *testing.T) {
	isolatedConfig(t)
	srcDir := driftSource(t)
	scope, err := ScopeUser() // ignore is a user-scope gesture
	if err != nil {
		t.Fatal(err)
	}

	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if err := cmdIgnore("agent", "test-a", scope, false); err != nil {
		t.Fatalf("ignore: %v", err)
	}
	rel := scope.RelPath("agents", "test-a.agent.md")
	onDisk := filepath.Join(scope.RootDir, rel)
	installed, err := os.ReadFile(onDisk)
	if err != nil {
		t.Fatal(err)
	}

	// Upstream moves. The ignored file must not follow it.
	mustWrite(t, filepath.Join(srcDir, "agents", "test-a.agent.md"), "---\nname: test-a\ndescription: A\n---\nBody A v2\n")
	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("second install: %v", err)
	}

	if got, _ := os.ReadFile(onDisk); string(got) != string(installed) {
		t.Errorf("an ignored file was overwritten by the install: %q", got)
	}
	if e := stateEntry(t, scope, rel); e == nil || e.Status == "" {
		t.Errorf("an ignored file came back unmanaged: %+v", e)
	}
}

// TestMergeStateFilesCollapsesDuplicates: a prior list that already held a path
// twice must not leave the stale copy behind — conflictStatePaths and
// resolveSyncFiles would then disagree about the same path.
func TestMergeStateFilesCollapsesDuplicates(t *testing.T) {
	prior := []InstalledFile{
		{Path: agentA, Hash: "stale", Status: fileStatusConflict},
		{Path: agentA, Hash: "fresh"},
	}
	merged := mergeStateFiles(prior, nil)
	if len(merged) != 1 {
		t.Fatalf("duplicate path survived the merge: %+v", merged)
	}
	if merged[0].Hash != "fresh" || merged[0].Status == fileStatusConflict {
		t.Errorf("the stale duplicate won: %+v", merged[0])
	}
}

// TestMergeStateFilesKeepsForeignSource: a scope install writes entries with no
// Source, so a path that `add --source X` tagged would lose its marker (#571).
func TestMergeStateFilesKeepsForeignSource(t *testing.T) {
	prior := []InstalledFile{{Path: agentA, Hash: "old", Source: "other/repo"}}
	merged := mergeStateFiles(prior, []InstalledFile{{Path: agentA, Hash: "new"}})
	if len(merged) != 1 || merged[0].Source != "other/repo" {
		t.Errorf("foreign-source marker lost: %+v", merged)
	}
}

// TestStateFilesAreWrittenSorted: the repo-scope state file is committed, so
// its order must come from the content, not from the order the installs ran in.
func TestStateFilesAreWrittenSorted(t *testing.T) {
	isolatedConfig(t)
	scope := ScopeRepo(repoTarget(t))
	if err := writeScopedState(scope, &StateFile{Files: []InstalledFile{
		{Path: agentB}, {Path: agentA},
	}}); err != nil {
		t.Fatalf("writeScopedState: %v", err)
	}
	state, err := readScopedState(scope)
	if err != nil || state == nil {
		t.Fatalf("readScopedState: %v", err)
	}
	if state.Files[0].Path != agentA {
		t.Errorf("state file order follows install history, not the paths: %+v", state.Files)
	}
}

// TestDeselectedItemIsNotDeleted: the picker's deselected items are appended to
// state after result.Files is built, so feeding removeOrphans the narrower set
// makes a deselection look like an artifact the source stopped shipping — and
// deletes a file the user explicitly chose to keep.
func TestDeselectedItemIsNotDeleted(t *testing.T) {
	isolatedConfig(t)
	srcDir := driftSource(t)
	target := repoTarget(t)
	scope := ScopeRepo(target)

	// Everything installed first, the way a picker run over an existing scope starts.
	if err := cmdInstallFromSource("wide", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("wide install: %v", err)
	}

	// Now the user unticks test-b: the manifest carries only test-a, and test-b
	// arrives as a deselected extra that must stay on disk.
	narrow, err := loadManifest(srcDir, "narrow")
	if err != nil {
		t.Fatal(err)
	}
	deselected := InstalledFile{Path: agentB, Status: fileStatusIgnored}
	if err := installAllFromSource(scope, localSource(srcDir), narrow, false, false, false, deselected); err != nil {
		t.Fatalf("install with a deselected item: %v", err)
	}

	if _, err := os.Stat(filepath.Join(target, agentB)); err != nil {
		t.Errorf("deselecting %s in the picker deleted it from disk: %v", agentB, err)
	}
}
