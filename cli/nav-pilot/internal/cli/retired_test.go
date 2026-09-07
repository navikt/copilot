package cli

import (
	"os"
	"path/filepath"
	"testing"
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
