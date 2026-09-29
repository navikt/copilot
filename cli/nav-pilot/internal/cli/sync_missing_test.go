package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// missingScope is a repo scope tracking two agents the source still ships,
// with one of them deleted from disk. It returns the target dir and its scope.
func missingScope(t *testing.T) (string, *InstallScope) {
	t.Helper()
	isolatedConfig(t)
	dir := repoTarget(t)
	sourceDir := t.TempDir()

	var files []InstalledFile
	for _, name := range []string{"kept", "gone"} {
		rel := filepath.Join(".github", "agents", name+".agent.md")
		mustWrite(t, filepath.Join(dir, rel), "# "+name+"\n")
		mustWrite(t, filepath.Join(sourceDir, "agents", name+".agent.md"), "# "+name+"\n")
		hash, err := rawArtifactHash(filepath.Join(dir, rel), false)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, InstalledFile{Path: filepath.ToSlash(rel), Hash: hash})
	}
	writeState(dir, &StateFile{
		Collection: "kotlin-backend",
		Version:    "2026.06",
		SourceRepo: "my-custom/repo",
		SourceSHA:  "sha",
		Files:      files,
	})
	if err := os.Remove(filepath.Join(dir, ".github", "agents", "gone.agent.md")); err != nil {
		t.Fatal(err)
	}

	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(ref, sourceRepo string) (*source.Source, error) {
		return &source.Source{Dir: sourceDir, SHA: "sha", Version: "2026.06", Repo: "my-custom/repo"}, nil
	}
	return dir, ScopeRepo(dir)
}

// TestSyncDryRunWritesNoIgnoredState: a sync that does not apply must not
// write. It marked a missing file ignored, so the file doctor told the user
// to restore with sync dropped out of tracking, and doctor then said all was
// well (#1334). --dry-run wins over --apply, so this says both.
func TestSyncDryRunWritesNoIgnoredState(t *testing.T) {
	dir, scope := missingScope(t)
	t.Chdir(dir)
	before, err := os.ReadFile(scope.StatePath())
	if err != nil {
		t.Fatal(err)
	}

	var runErr error
	stdout, _ := captureRun(t, func() { runErr = run([]string{"sync", "--repo", "--dry-run", "--apply"}) })
	if runErr != nil {
		t.Fatalf("sync --dry-run: %v\n%s", runErr, stdout)
	}

	after, err := os.ReadFile(scope.StatePath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("sync --dry-run wrote the state file:\nbefore: %s\nafter:  %s", before, after)
	}
	out := stripANSI(stdout)
	if want := "nav-pilot install gone --type agent --repo --source my-custom/repo"; !strings.Contains(out, want) {
		t.Errorf("sync check does not name the restore command %q:\n%s", want, out)
	}
	if strings.Contains(out, "marked ignored") {
		t.Errorf("sync check claims it marked the file ignored:\n%s", out)
	}
}

// TestSyncApplyStillMarksAMissingFileIgnored: --apply is where a deletion is
// confirmed. A user who removed a file on purpose and runs sync --apply keeps
// it removed, as before #1334.
func TestSyncApplyStillMarksAMissingFileIgnored(t *testing.T) {
	dir, scope := missingScope(t)

	var err error
	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply: %v\n%s", err, out)
	}
	state, err := readScopedState(scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range state.Files {
		if f.Path == ".github/agents/gone.agent.md" && f.Status != fileStatusIgnored {
			t.Errorf("sync --apply left the deleted file %s as %q, want ignored", f.Path, f.Status)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".github", "agents", "gone.agent.md")); !os.IsNotExist(err) {
		t.Errorf("sync --apply put back a file the user deleted: %v", err)
	}
}

// TestSyncApplyDeclinedWritesNoIgnoredState: --apply in a terminal asks
// before it removes a file the source dropped, and "no" means nothing
// changes. The missing file was marked ignored before the question, so
// declining still wrote state (#1334).
func TestSyncApplyDeclinedWritesNoIgnoredState(t *testing.T) {
	dir, scope := missingScope(t)
	// A tracked file the source no longer ships, so there is a deletion to ask about.
	dropped := filepath.Join(dir, ".github", "agents", "dropped.agent.md")
	mustWrite(t, dropped, "# dropped\n")
	hash, err := rawArtifactHash(dropped, false)
	if err != nil {
		t.Fatal(err)
	}
	state, err := readScopedState(scope)
	if err != nil {
		t.Fatal(err)
	}
	state.Files = append(state.Files, InstalledFile{Path: ".github/agents/dropped.agent.md", Hash: hash})
	if err := writeScopedState(scope, state); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(scope.StatePath())
	if err != nil {
		t.Fatal(err)
	}

	origAsk, origInteractive := askSyncDeletions, isInteractive
	t.Cleanup(func() { askSyncDeletions, isInteractive, syncAsk = origAsk, origInteractive, false })
	asked := false
	askSyncDeletions = func(string) (bool, error) { asked = true; return false, nil }
	isInteractive = func() bool { return true }
	syncAsk = true

	out := captureStdoutFor(t, func() { err = cmdSync(scope, "", "", true, false) })
	if err != nil {
		t.Fatalf("sync --apply: %v\n%s", err, out)
	}
	if !asked {
		t.Fatalf("sync --apply did not ask about the deletion:\n%s", out)
	}
	after, err := os.ReadFile(scope.StatePath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("declining sync --apply wrote the state file:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestDoctorNamesTheCommandThatRestoresAMissingFile: doctor said "run
// nav-pilot sync to restore missing files", and sync does not restore them
// (#1334). It names the install command that does, and how to confirm the
// deletion instead.
func TestDoctorNamesTheCommandThatRestoresAMissingFile(t *testing.T) {
	dir, _ := missingScope(t)
	t.Chdir(dir)
	t.Setenv("PATH", t.TempDir()) // no cplt, no opencode: doctor stays offline

	out := stripANSI(captureStdoutFor(t, func() { _ = cmdDoctor() }))
	for _, want := range []string{
		"1 missing files",
		".github/agents/gone.agent.md",
		"nav-pilot install gone --type agent --repo --source my-custom/repo",
		"nav-pilot sync --repo --apply stops tracking them",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("doctor does not say %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "to restore missing files") {
		t.Errorf("doctor still sends the user to sync to restore:\n%s", out)
	}
}
