package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// blobHash must agree with git, or the generated record is unreadable by the
// code that consumes it. git's empty-blob id is a fixed value, so this needs no
// git call to assert.
func TestBlobHashMatchesGit(t *testing.T) {
	if got, want := blobHash(nil), "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"; got != want {
		t.Errorf("blobHash(empty) = %s, want git's empty blob id %s", got, want)
	}
}

func writeRetired(t *testing.T, sourceDir, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(sourceDir, ".nav-pilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, retiredManifestPath), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func userScopeWithAgents(t *testing.T) (*InstallScope, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	scope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	dir := scope.DstPath("agents")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return scope, dir
}

func TestFindRetiredOrphans(t *testing.T) {
	scope, agentDir := userScopeWithAgents(t)
	published := []byte("---\nname: auth\n---\n\nretired agent\n")
	mine := []byte("---\nname: mine\n---\n\nmy own agent\n")

	for name, body := range map[string][]byte{
		"auth.agent.md": published,
		"mine.agent.md": mine,
	} {
		if err := os.WriteFile(filepath.Join(agentDir, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sourceDir := t.TempDir()
	writeRetired(t, sourceDir, `{"paths":{
		"agents/auth.agent.md":["`+blobHash(published)+`"],
		"agents/mine.agent.md":["`+blobHash(published)+`"],
		"agents/nevermind.agent.md":["`+blobHash(published)+`"]
	}}`)

	got := findRetiredOrphans(scope, sourceDir)
	if len(got) != 1 || got[0].Path != "agents/auth.agent.md" {
		t.Fatalf("findRetiredOrphans = %+v, want exactly agents/auth.agent.md: "+
			"mine.agent.md is retired upstream but holds different bytes, and nevermind is not installed", got)
	}
}

// The safety property, on its own because it is the whole argument for deleting
// anything: a file at a retired path whose content nav-pilot never published
// belongs to whoever wrote it.
func TestFindRetiredOrphansSparesEditedFiles(t *testing.T) {
	scope, agentDir := userScopeWithAgents(t)
	published := []byte("published body\n")
	edited := append(append([]byte{}, published...), []byte("plus my edit\n")...)
	if err := os.WriteFile(filepath.Join(agentDir, "auth.agent.md"), edited, 0o644); err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	writeRetired(t, sourceDir, `{"paths":{"agents/auth.agent.md":["`+blobHash(published)+`"]}}`)

	if got := findRetiredOrphans(scope, sourceDir); len(got) != 0 {
		t.Errorf("findRetiredOrphans = %+v, want none: an edited file is the user's", got)
	}
}

// A source without the record is the ordinary case for any agentpakke that is
// not this repo, and for older revisions of this one. Silent, not an error.
func TestFindRetiredOrphansWithoutManifest(t *testing.T) {
	scope, _ := userScopeWithAgents(t)
	if got := findRetiredOrphans(scope, t.TempDir()); got != nil {
		t.Errorf("findRetiredOrphans without a record = %+v, want nil", got)
	}
}

// TestSyncIsNotUpToDateWithRetiredOrphans pins the bug review caught in #722:
// the orphan scan ran after the up-to-date branch had already returned, so a
// scope whose only problem was a leftover printed "All N files up to date" and
// --apply removed nothing.
//
// That is not an edge case. It is the state the machine that found #716 was in:
// every tracked file current, three retired agents on disk, sync reporting
// green.
//
// It drives cmdSync rather than restating the boolean. A first version asserted
// len(orphans) != 0 one line after computing orphans, which is true by
// construction and would stay green with the term removed from sync.go.
func TestSyncIsNotUpToDateWithRetiredOrphans(t *testing.T) {
	scope, agentDir := userScopeWithAgents(t)
	published := []byte("---\nname: auth\n---\n\nretired\n")
	if err := os.WriteFile(filepath.Join(agentDir, "auth.agent.md"), published, 0o644); err != nil {
		t.Fatal(err)
	}

	// A source that ships nothing and has retired the agent: no updates, no
	// deletions, nothing but the orphan.
	sourceDir := t.TempDir()
	writeRetired(t, sourceDir, `{"paths":{"agents/auth.agent.md":["`+blobHash(published)+`"]}}`)
	if err := writeScopedState(scope, &StateFile{
		Collection: "pakke",
		Scope:      scope.Name,
		SourceRepo: sourceDir,
		SourceSHA:  "abc1234",
	}); err != nil {
		t.Fatal(err)
	}

	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(string, string) (*source.Source, error) {
		return &source.Source{Dir: sourceDir, SHA: "abc1234", Repo: sourceDir, Version: "dev"}, nil
	}

	err := cmdSync(scope, "", "", false, false)
	if err == nil {
		t.Fatal("cmdSync = nil over a scope holding a retired orphan, want updates available")
	}
	if !errors.Is(err, errUpdatesAvailable) {
		t.Errorf("cmdSync = %v, want errUpdatesAvailable", err)
	}

	// And --apply must actually remove it.
	if err := cmdSync(scope, "", "", true, false); err != nil {
		t.Fatalf("cmdSync --apply = %v, want nil", err)
	}
	if _, statErr := os.Stat(filepath.Join(agentDir, "auth.agent.md")); !os.IsNotExist(statErr) {
		t.Error("the retired orphan survived sync --apply")
	}
}

// TestSyncWithTrackedFilesIsNotUpToDateWithOrphans covers the other half. The
// test above drives a scope with no tracked files, which leaves through the
// "No customization files found" branch and never evaluates the UpToDate
// expression: removing the orphan term from that expression leaves it green.
//
// Here every tracked file is current, so the run reaches UpToDate with nothing
// else to report, which is precisely the shape #716 arrived in.
func TestSyncWithTrackedFilesIsNotUpToDateWithOrphans(t *testing.T) {
	scope, agentDir := userScopeWithAgents(t)

	current := []byte("---\nname: kept\n---\n\ncurrent\n")
	if err := os.WriteFile(filepath.Join(agentDir, "kept.agent.md"), current, 0o644); err != nil {
		t.Fatal(err)
	}
	published := []byte("---\nname: auth\n---\n\nretired\n")
	if err := os.WriteFile(filepath.Join(agentDir, "auth.agent.md"), published, 0o644); err != nil {
		t.Fatal(err)
	}

	// The source still ships kept.agent.md, byte-identical, and has retired auth.
	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "agents", "kept.agent.md"), current, 0o644); err != nil {
		t.Fatal(err)
	}
	writeRetired(t, sourceDir, `{"paths":{"agents/auth.agent.md":["`+blobHash(published)+`"]}}`)

	hash, err := rawArtifactHash(filepath.Join(agentDir, "kept.agent.md"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeScopedState(scope, &StateFile{
		Collection: "pakke",
		Scope:      scope.Name,
		SourceRepo: sourceDir,
		SourceSHA:  "abc1234",
		Files:      []InstalledFile{{Path: "agents/kept.agent.md", Hash: hash}},
	}); err != nil {
		t.Fatal(err)
	}

	orig := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = orig })
	resolveSourceForSync = func(string, string) (*source.Source, error) {
		return &source.Source{Dir: sourceDir, SHA: "abc1234", Repo: sourceDir, Version: "dev"}, nil
	}

	if err := cmdSync(scope, "", "", false, false); !errors.Is(err, errUpdatesAvailable) {
		t.Fatalf("cmdSync = %v over a current scope holding one orphan, want errUpdatesAvailable", err)
	}
	if err := cmdSync(scope, "", "", true, false); err != nil {
		t.Fatalf("cmdSync --apply = %v, want nil", err)
	}
	if _, statErr := os.Stat(filepath.Join(agentDir, "auth.agent.md")); !os.IsNotExist(statErr) {
		t.Error("the retired orphan survived sync --apply")
	}
	if _, statErr := os.Stat(filepath.Join(agentDir, "kept.agent.md")); statErr != nil {
		t.Errorf("sync removed a file the source still ships: %v", statErr)
	}
}

// TestIgnoredButInstalled covers #724: an artifact the state marks ignored
// while the file is on disk. Sync skips it by design, so it never updates, and
// nothing said so until doctor's model check tripped over one that had been
// pinned to a withdrawn model for weeks.
func TestIgnoredButInstalled(t *testing.T) {
	scope, agentDir := userScopeWithAgents(t)
	if err := os.WriteFile(filepath.Join(agentDir, "opus.agent.md"), []byte("---\nname: opus\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeScopedState(scope, &StateFile{
		Collection: "pakke",
		Scope:      scope.Name,
		SourceRepo: "navikt/copilot",
		SourceSHA:  "abc1234",
		Files: []InstalledFile{
			// On disk and ignored: the combination nobody chooses.
			{Path: "agents/opus.agent.md", Status: fileStatusIgnored},
			// Ignored and genuinely absent, which is the ordinary case and must
			// not be reported: nothing has stopped being maintained.
			{Path: "agents/finnes-ikke.agent.md", Status: fileStatusIgnored},
			// On disk and tracked, which sync updates normally.
			{Path: "agents/kept.agent.md", Hash: "abc"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	got := ignoredButInstalled(scope)
	if len(got) != 1 || got[0] != "agents/opus.agent.md" {
		t.Errorf("ignoredButInstalled = %v, want exactly agents/opus.agent.md", got)
	}
}

// TestFoldInSparesInstalledArtifacts covers the cause behind #724. The
// collection fold-in marked every artifact missing from the state file as
// ignored, on the assumption that missing from state means not installed. It
// does not: an artifact written by an older nav-pilot, or by a collection
// install that recorded less, is on disk and in use.
//
// Marking such a file ignored told sync to skip it forever. One then sat on a
// model GitHub had withdrawn until doctor's catalogue check found it.
func TestFoldInSparesInstalledArtifacts(t *testing.T) {
	scope, agentDir := userScopeWithAgents(t)

	// The source ships two agents. One is on disk but untracked, the other is
	// genuinely absent.
	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"onDisk", "absent"} {
		body := "---\nname: " + name + "\n---\n"
		if err := os.WriteFile(filepath.Join(sourceDir, "agents", name+".agent.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(agentDir, "onDisk.agent.md"), []byte("installed by an older nav-pilot\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := &StateFile{
		Collection: "fullstack",
		Scope:      scope.Name,
		SourceRepo: "navikt/copilot",
		SourceSHA:  "abc1234",
	}
	src := &Source{
		Dir:   sourceDir,
		SHA:   "abc1234",
		Repo:  "navikt/copilot",
		Pakke: &agentpakke.Manifest{Name: "nav-pilot"},
	}

	adoptPakkeIdentity(scope, src, state, NewSourceResolver(sourceDir), true)

	var ignored []string
	for _, f := range state.Files {
		if f.Status == fileStatusIgnored {
			ignored = append(ignored, f.Path)
		}
	}
	for _, p := range ignored {
		if strings.Contains(p, "onDisk") {
			t.Errorf("the fold-in marked an installed artifact as ignored: %v", ignored)
		}
	}
	if len(ignored) == 0 {
		t.Error("the fold-in marked nothing as ignored; the genuinely absent agent should be")
	}
}
