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

	got := findRetiredOrphans(scope, sourceDir, nil)
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

	if got := findRetiredOrphans(scope, sourceDir, nil); len(got) != 0 {
		t.Errorf("findRetiredOrphans = %+v, want none: an edited file is the user's", got)
	}
}

// A source without the record is the ordinary case for any agentpakke that is
// not this repo, and for older revisions of this one. Silent, not an error.
func TestFindRetiredOrphansWithoutManifest(t *testing.T) {
	scope, _ := userScopeWithAgents(t)
	if got := findRetiredOrphans(scope, t.TempDir(), nil); got != nil {
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

// TestRetiredHonoursDeclaredLayout covers the half of #722 that only worked for
// this repo (#728): a Tier 1 agentpakke may declare where its content lives, so
// a retired path can read "content/agents/gammel.agent.md". Matching the
// canonical name alone meant such a pakke could never retire anything.
func TestRetiredHonoursDeclaredLayout(t *testing.T) {
	scope, agentDir := userScopeWithAgents(t)
	published := []byte("---\nname: gammel\n---\n\nretired\n")
	if err := os.WriteFile(filepath.Join(agentDir, "gammel.agent.md"), published, 0o644); err != nil {
		t.Fatal(err)
	}
	sourceDir := t.TempDir()
	writeRetired(t, sourceDir, `{"paths":{"content/agents/gammel.agent.md":["`+blobHash(published)+`"]}}`)

	// Without the manifest the directory is unknown, so nothing is found: the
	// control that keeps the layout lookup from being a no-op.
	if got := findRetiredOrphans(scope, sourceDir, nil); len(got) != 0 {
		t.Fatalf("findRetiredOrphans without a layout = %+v, want none", got)
	}

	pakke := &agentpakke.Manifest{
		Name:   "annet-team",
		Layout: &agentpakke.Layout{Agents: "content/agents", Skills: "content/skills"},
	}
	got := findRetiredOrphans(scope, sourceDir, pakke)
	if len(got) != 1 || got[0].Path != "content/agents/gammel.agent.md" {
		t.Errorf("findRetiredOrphans with a declared layout = %+v, want the one orphan", got)
	}
}

// TestRevisionIsRecordedAndSurfaced covers #729: the state file recorded a hash
// and nothing about where the bytes came from, so "differs from what nav-pilot
// installed" could not tell an edit from a file installed by an older revision.
// Both look identical to a hash comparison.
func TestRevisionIsRecordedAndSurfaced(t *testing.T) {
	t.Run("an install stamps the revision on every file it wrote", func(t *testing.T) {
		files := []InstalledFile{
			{Path: "agents/a.agent.md", Hash: "abc"},
			// Ignored entries have no bytes, so they have no provenance either.
			// Stamping one would claim a revision put something on disk.
			{Path: "agents/b.agent.md", Status: fileStatusIgnored},
		}
		got := stampRevision(files, "def5678")
		if got[0].Revision != "def5678" {
			t.Errorf("written file revision = %q, want def5678", got[0].Revision)
		}
		if got[1].Revision != "" {
			t.Errorf("ignored entry got revision %q, want none", got[1].Revision)
		}
	})

	t.Run("an unknown revision is absent, not empty", func(t *testing.T) {
		scope, _ := userScopeWithAgents(t)
		if err := writeScopedState(scope, &StateFile{
			Collection: "pakke",
			Scope:      scope.Name,
			SourceRepo: "navikt/copilot",
			SourceSHA:  "def5678",
			Files: []InstalledFile{
				{Path: "agents/ny.agent.md", Hash: "h1", Revision: "abc1234"},
				{Path: "agents/gammel.agent.md", Hash: "h2"},
			},
		}); err != nil {
			t.Fatal(err)
		}
		got := installedRevisions(scope)
		if got["agents/ny.agent.md"] != "abc1234" {
			t.Errorf("revision = %q, want abc1234", got["agents/ny.agent.md"])
		}
		// The distinction that matters: a state file written before provenance
		// must not be reported as "installed from ''".
		if _, present := got["agents/gammel.agent.md"]; present {
			t.Error("a file with no recorded revision is present in the map; the caller cannot then tell it from a known one")
		}
	})
}

// TestSameRevision covers the false positive review found in #732: a state file
// may carry a short sha while the source resolves to the full 40 characters, so
// comparing them for equality labelled a file "installed from <rev>" even when
// it came from exactly the revision the scope is on.
func TestSameRevision(t *testing.T) {
	full := "def5678901234567890123456789012345678901"
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", full, full, true},
		{"abbreviated state, full source", "def5678", full, true},
		{"full state, abbreviated source", full, "def5678", true},
		{"case differs", "DEF5678", full, true},
		{"different commits", "abc1234", full, false},
		// Absence is not agreement: a file with no recorded revision must not
		// read as "same as the source".
		{"empty either side", "", full, false},
		{"empty both sides", "", "", false},
		// Four hex characters agree by accident often enough to be worthless,
		// which is why git has a minimum too.
		{"prefix shorter than git's minimum", "def5", full, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameRevision(tt.a, tt.b); got != tt.want {
				t.Errorf("sameRevision(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestRetiredRecordIsValidatedAgainstTheSchema covers the contract half of
// #729. The record drives deletion, and any agentpakke may now publish one, so
// a malformed record must not reach the code that removes files.
//
// A record that does not conform is treated as absent rather than as a sync
// failure: a third party's broken file must not block an update that has
// nothing to do with it.
func TestRetiredRecordIsValidatedAgainstTheSchema(t *testing.T) {
	published := []byte("---\nname: gammel\n---\n")
	good := blobHash(published)

	tests := []struct {
		name   string
		record string
		want   int
	}{
		{"conforming record", `{"paths":{"agents/gammel.agent.md":["` + good + `"]}}`, 1},
		// A hash that is not a blob id could never match a real file, but it
		// says the producer is not emitting what the contract asks for.
		{"hash is not a blob id", `{"paths":{"agents/gammel.agent.md":["deadbeef"]}}`, 0},
		// The path rules matter most: this is the field that decides which file
		// gets removed.
		{"path escapes the repo", `{"paths":{"../../etc/passwd":["` + good + `"]}}`, 0},
		{"absolute path", `{"paths":{"/etc/passwd":["` + good + `"]}}`, 0},
		{"paths is missing", `{"_comment":"nothing here"}`, 0},
		{"empty hash list", `{"paths":{"agents/gammel.agent.md":[]}}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope, agentDir := userScopeWithAgents(t)
			if err := os.WriteFile(filepath.Join(agentDir, "gammel.agent.md"), published, 0o644); err != nil {
				t.Fatal(err)
			}
			sourceDir := t.TempDir()
			writeRetired(t, sourceDir, tt.record)

			if got := len(findRetiredOrphans(scope, sourceDir, nil)); got != tt.want {
				t.Errorf("findRetiredOrphans found %d orphan(s), want %d", got, tt.want)
			}
		})
	}
}
