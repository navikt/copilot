package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// deletionScope installs two tracked agents with honest hashes and a source
// that ships neither, so every file in it is one sync away from being deleted.
// It returns the target dir and its scope.
func deletionScope(t *testing.T) (string, *InstallScope, string) {
	t.Helper()
	isolatedConfig(t)
	dir := repoTarget(t)
	sourceDir := t.TempDir()

	mustWrite(t, filepath.Join(dir, ".github", "agents", "kept.agent.md"), "# Kept\n")
	mustWrite(t, filepath.Join(dir, ".github", "agents", "gone.agent.md"), "# Gone\n")

	keptHash, err := rawArtifactHash(filepath.Join(dir, ".github", "agents", "kept.agent.md"), false)
	if err != nil {
		t.Fatal(err)
	}
	goneHash, err := rawArtifactHash(filepath.Join(dir, ".github", "agents", "gone.agent.md"), false)
	if err != nil {
		t.Fatal(err)
	}

	writeState(dir, &StateFile{
		Collection: "kotlin-backend",
		Version:    "2026.06",
		SourceRepo: "my-custom/repo",
		Files: []InstalledFile{
			{Path: ".github/agents/kept.agent.md", Hash: keptHash},
			{Path: ".github/agents/gone.agent.md", Hash: goneHash},
		},
	})

	// The source ships neither, so both paths are "deleted upstream". It
	// ships something else: a source that ships nothing is refused.
	mustWrite(t, filepath.Join(sourceDir, "agents", "other.agent.md"), "# Other\n")

	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*source.Source, error) {
		return &source.Source{Dir: sourceDir, SHA: "new-sha", Version: "2026.07", Repo: "my-custom/repo"}, nil
	}

	return dir, ScopeRepo(dir), sourceDir
}

// TestSyncSavesALocallyEditedFileDeletedUpstream: a file the source dropped
// goes, edited or not, because leaving an edited one in place kept a dropped
// always-on instruction loading in every session. The edit is not lost: it is
// saved as <file>.orig first, as an update does (#987). #729 kept the file in
// place instead; the .orig copy keeps its guarantee that nothing edited is
// ever deleted outright.
func TestSyncSavesALocallyEditedFileDeletedUpstream(t *testing.T) {
	dir, scope, _ := deletionScope(t)

	// The user edits one of the two after install.
	edited := "# Kept, and edited by me\n"
	mustWrite(t, filepath.Join(dir, ".github", "agents", "kept.agent.md"), edited)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply: %v\n%s", err, out)
	}

	if _, statErr := os.Stat(filepath.Join(dir, ".github", "agents", "kept.agent.md")); !os.IsNotExist(statErr) {
		t.Errorf("sync left the edited file the source deleted in place: %v", statErr)
	}
	if got, readErr := os.ReadFile(filepath.Join(dir, ".github", "agents", "kept.agent.md.orig")); readErr != nil || string(got) != edited {
		t.Errorf("the edited copy was not saved as .orig: %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".github", "agents", "gone.agent.md")); !os.IsNotExist(statErr) {
		t.Errorf("sync kept an untouched file the source deleted: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".github", "agents", "gone.agent.md.orig")); !os.IsNotExist(statErr) {
		t.Errorf("sync saved a copy of an untouched file: %v", statErr)
	}
	if !strings.Contains(out, "kept.agent.md (changed here; your copy is saved as .orig)") {
		t.Errorf("sync did not say it saved the edited file:\n%s", out)
	}
	// Both files were the only ones tracked, so the state may be gone too.
	if st, _ := readScopedState(scope); st != nil {
		for _, f := range st.Files {
			t.Errorf("state still tracks a file sync deleted: %+v", f)
		}
	}
}

// TestSyncJSONReportsTheEditedDeletion: --json is the workflow's eye, so a
// deletion that saves a local edit is listed with the deletions and named
// under removed_local_edits, the way an update is under replaced_local_edits.
func TestSyncJSONReportsTheEditedDeletion(t *testing.T) {
	dir, scope, _ := deletionScope(t)
	mustWrite(t, filepath.Join(dir, ".github", "agents", "kept.agent.md"), "# Kept, and edited by me\n")

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", false, true) })
	if err != nil && err != errUpdatesAvailable {
		t.Fatalf("sync --json: %v\n%s", err, out)
	}
	var doc struct {
		Deletions         []string `json:"deletions"`
		RemovedLocalEdits []string `json:"removed_local_edits"`
		Kept              []string `json:"kept"`
	}
	if jsonErr := json.Unmarshal([]byte(out), &doc); jsonErr != nil {
		t.Fatalf("sync --json: %v\n%s", jsonErr, out)
	}
	if !slices.Contains(doc.Deletions, ".github/agents/kept.agent.md") || !slices.Contains(doc.Deletions, ".github/agents/gone.agent.md") {
		t.Errorf("deletions = %v, want both files", doc.Deletions)
	}
	if !reflect.DeepEqual(doc.RemovedLocalEdits, []string{".github/agents/kept.agent.md"}) {
		t.Errorf("removed_local_edits = %v, want the edited file only", doc.RemovedLocalEdits)
	}
	if len(doc.Kept) != 0 {
		t.Errorf("kept = %v, want none: only a hook is kept", doc.Kept)
	}
	// A check writes nothing.
	if _, statErr := os.Stat(filepath.Join(dir, ".github", "agents", "kept.agent.md.orig")); !os.IsNotExist(statErr) {
		t.Errorf("a check without --apply saved a copy: %v", statErr)
	}
}

// TestUninstallKeepsALocallyEditedFile: uninstall removes everything the state
// names, which used to include files the user had since made their own.
func TestUninstallKeepsALocallyEditedFile(t *testing.T) {
	dir, scope, _ := deletionScope(t)
	mustWrite(t, filepath.Join(dir, ".github", "agents", "kept.agent.md"), "# Kept, and edited by me\n")

	var err error
	out := captureStdoutFor(t, func() { err = cmdUninstall(scope, false, false) })
	if err != nil {
		t.Fatalf("uninstall: %v\n%s", err, out)
	}

	if _, statErr := os.Stat(filepath.Join(dir, ".github", "agents", "kept.agent.md")); statErr != nil {
		t.Errorf("uninstall deleted a locally edited file: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".github", "agents", "gone.agent.md")); !os.IsNotExist(statErr) {
		t.Errorf("uninstall kept an untouched file: %v", statErr)
	}
	if !strings.Contains(out, "kept.agent.md") || !strings.Contains(out, "differ from what nav-pilot installed") {
		t.Errorf("uninstall did not say what it left behind:\n%s", out)
	}
}

// TestUninstallForceRemovesTheEditedFile: --force is the existing way past a
// file that differs from the source, and it has to reach the removal too.
func TestUninstallForceRemovesTheEditedFile(t *testing.T) {
	dir, scope, _ := deletionScope(t)
	mustWrite(t, filepath.Join(dir, ".github", "agents", "kept.agent.md"), "# Kept, and edited by me\n")

	var err error
	out := captureStdoutFor(t, func() { err = cmdUninstall(scope, false, true) })
	if err != nil {
		t.Fatalf("uninstall --force: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".github", "agents", "kept.agent.md")); !os.IsNotExist(statErr) {
		t.Errorf("uninstall --force left the edited file behind: %v", statErr)
	}
}

// TestUninstallDryRunReportsWhatItWouldKeep: a dry run that promises to remove
// a file the real run would keep is a dry run that lies.
func TestUninstallDryRunReportsWhatItWouldKeep(t *testing.T) {
	dir, scope, _ := deletionScope(t)
	mustWrite(t, filepath.Join(dir, ".github", "agents", "kept.agent.md"), "# Kept, and edited by me\n")

	var err error
	out := captureStdoutFor(t, func() { err = cmdUninstall(scope, true, false) })
	if err != nil {
		t.Fatalf("uninstall --dry-run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "differ from what nav-pilot installed") {
		t.Errorf("dry run did not report the file it would keep:\n%s", out)
	}
	// The one untouched file, and the state file the list names too.
	if !strings.Contains(out, "Would remove 2 items") {
		t.Errorf("dry run counted the kept file as a removal:\n%s", out)
	}
}

// TestUninstallRemovesAnEntryWithNoRecordedHash: a state from before hashes
// were recorded has nothing to compare, and its word that nav-pilot installed
// the file is all there is. Keeping those would make uninstall a no-op for
// every repo that has not reinstalled since.
func TestUninstallRemovesAnEntryWithNoRecordedHash(t *testing.T) {
	isolatedConfig(t)
	dir := repoTarget(t)
	mustWrite(t, filepath.Join(dir, ".github", "agents", "old.agent.md"), "# Old\n")
	writeState(dir, &StateFile{
		Collection: "kotlin-backend",
		Files:      []InstalledFile{{Path: ".github/agents/old.agent.md"}},
	})

	if err := cmdUninstall(ScopeRepo(dir), false, false); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".github", "agents", "old.agent.md")); !os.IsNotExist(err) {
		t.Errorf("uninstall kept a hashless entry it has always removed: %v", err)
	}
}

// TestInstallAdoptsAByteMatchingUntrackedFile is #651 point 1 from the install
// side: a file on disk that nav-pilot never recorded, whose bytes are what the
// source would write, is nav-pilot's after all and belongs in state — not in
// the conflict report.
func TestInstallAdoptsAByteMatchingUntrackedFile(t *testing.T) {
	isolatedConfig(t)
	srcDir := driftSource(t)
	target := repoTarget(t)
	scope := ScopeRepo(target)

	if err := cmdInstallFromSource("narrow", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	// test-b was never installed, but someone copied the source's own bytes in.
	body, err := os.ReadFile(filepath.Join(srcDir, "agents", "test-b.agent.md"))
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(target, agentB), string(body))

	if err := cmdInstallFromSource("wide", localSource(srcDir), scope, false, false, false); err != nil {
		t.Fatalf("wide install: %v", err)
	}
	e := stateEntry(t, scope, agentB)
	if e == nil {
		t.Fatalf("a byte-matching file was not adopted into state")
	}
	if e.Status == fileStatusConflict {
		t.Errorf("a byte-matching file was reported as a conflict: %+v", e)
	}
	if e.Hash == "" {
		t.Errorf("adopted entry has no hash, so nothing can ever prove it unedited: %+v", e)
	}
}

// TestSyncSavesAHashlessEntryBeforeRemoving: a state written before hashes
// were recorded cannot tell nav-pilot's copy from an edited one, so a file the
// source dropped is saved as .orig before it goes.
func TestSyncSavesAHashlessEntryBeforeRemoving(t *testing.T) {
	dir, scope, _ := deletionScope(t)
	writeState(dir, &StateFile{
		Collection: "kotlin-backend",
		Version:    "2026.06",
		SourceRepo: "my-custom/repo",
		Files:      []InstalledFile{{Path: ".github/agents/kept.agent.md"}},
	})

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".github", "agents", "kept.agent.md")); !os.IsNotExist(statErr) {
		t.Errorf("sync left the file the source deleted in place: %v", statErr)
	}
	if got, readErr := os.ReadFile(filepath.Join(dir, ".github", "agents", "kept.agent.md.orig")); readErr != nil || string(got) != "# Kept\n" {
		t.Errorf("the hashless entry's copy was not saved as .orig: %q, %v", got, readErr)
	}
}

// TestSyncRefusesASourceThatShipsNothing: an empty source is a broken one, and
// reading it as "everything was deleted upstream" would empty the scope.
func TestSyncRefusesASourceThatShipsNothing(t *testing.T) {
	dir, scope, sourceDir := deletionScope(t)
	if err := os.RemoveAll(filepath.Join(sourceDir, "agents")); err != nil {
		t.Fatal(err)
	}

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err == nil || !strings.Contains(err.Error(), "ships no agents") {
		t.Fatalf("sync --apply against an empty source: err = %v\n%s", err, out)
	}
	for _, name := range []string{"kept.agent.md", "gone.agent.md"} {
		if _, statErr := os.Stat(filepath.Join(dir, ".github", "agents", name)); statErr != nil {
			t.Errorf("%s was removed: %v", name, statErr)
		}
	}
}
