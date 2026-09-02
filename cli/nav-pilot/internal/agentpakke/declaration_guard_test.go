package agentpakke

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDeclarationRewritePreservesUnknownKeys is #588 on the consumer side: the
// file teams are asked to commit must survive a bump by a binary that does not
// know every key in it.
func TestDeclarationRewritePreservesUnknownKeys(t *testing.T) {
	root := t.TempDir()
	body := `{
  "contractVersion": "1",
  "source": "navikt/grillmester",
  "sha": "0000000000000000000000000000000000000000",
  "policy": {"review": "required"},
  "zzzTrailing": 3
}
`
	if err := os.MkdirAll(filepath.Join(root, ManifestDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(DeclarationFilePath(root), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := LoadDeclaration(root)
	if err != nil {
		t.Fatalf("LoadDeclaration: %v", err)
	}
	d.SHA = "1111111111111111111111111111111111111111" // the bump `sync --apply` makes
	if err := WriteDeclaration(root, d); err != nil {
		t.Fatalf("WriteDeclaration: %v", err)
	}

	out, err := os.ReadFile(DeclarationFilePath(root))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, key := range []string{`"policy"`, `"review"`, `"zzzTrailing"`} {
		if !strings.Contains(got, key) {
			t.Errorf("bump dropped %s:\n%s", key, got)
		}
	}
	if !strings.Contains(got, "1111111111111111111111111111111111111111") {
		t.Errorf("bump did not write the new SHA:\n%s", got)
	}

	// And the rewrite is stable: writing the same declaration twice is a no-op
	// diff, or every consumer picks up spurious churn.
	if err := WriteDeclaration(root, d); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(DeclarationFilePath(root))
	if string(again) != got {
		t.Errorf("a second write changed the bytes:\n%s\n---\n%s", got, again)
	}
}

// A pull request can add the symlink; without a guard the write lands wherever
// it points, outside the repository being installed into.
func TestWriteDeclarationRefusesSymlinkedDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ManifestDir)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	d := &Declaration{ContractVersion: "1", Source: "navikt/grillmester"}
	if err := WriteDeclaration(root, d); err == nil {
		t.Fatal("WriteDeclaration wrote through a symlinked .nav-pilot directory")
	}
	if _, err := os.Stat(filepath.Join(outside, DeclarationFile)); !os.IsNotExist(err) {
		t.Error("the declaration landed outside the repository")
	}
}

// The read path is guarded too: a declaration reached through a symlink is a
// file the pull request did not show.
func TestLoadDeclarationRefusesSymlinkedDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, DeclarationFile),
		[]byte(`{"contractVersion":"1","source":"navikt/annen"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ManifestDir)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := LoadDeclaration(root); err == nil {
		t.Fatal("LoadDeclaration read through a symlinked .nav-pilot directory")
	}
}

// A path source has no revision to fetch, so a SHA beside one is a pin nobody
// can resolve. Refusing it is the only coherent answer: writing "unknown" into
// a committed file is worse than saying so.
func TestPathSourceMayNotBePinned(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ManifestDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(DeclarationFilePath(root),
		[]byte(`{"contractVersion":"1","source":"/srv/grillmester","sha":"deadbeef"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadDeclaration(root)
	if err == nil {
		t.Fatal("a pinned path source was accepted")
	}
	if !strings.Contains(err.Error(), "sha") {
		t.Errorf("error %q does not point at the offending field", err)
	}

	// The same path source without a pin is fine — that is how a developer
	// points a repo at a local checkout.
	if err := os.WriteFile(DeclarationFilePath(root),
		[]byte(`{"contractVersion":"1","source":"/srv/grillmester"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDeclaration(root); err != nil {
		t.Errorf("an unpinned path source was refused: %v", err)
	}
}

// The contract gate is shared with the manifest, but its advice must not be:
// a consumer holding an unreadable lock file cannot ask anyone to publish a
// different manifest.
func TestContractVersionAdviceFitsTheDeclaration(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ManifestDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(DeclarationFilePath(root),
		[]byte(`{"contractVersion":"99","source":"navikt/grillmester"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadDeclaration(root)
	if err == nil {
		t.Fatal("an unsupported contract version was accepted")
	}
	if strings.Contains(err.Error(), "publish a manifest") {
		t.Errorf("the declaration's refusal sends the reader to the wrong artifact: %v", err)
	}
	if !strings.Contains(err.Error(), "nav-pilot update") {
		t.Errorf("the refusal does not say how to proceed: %v", err)
	}
}
