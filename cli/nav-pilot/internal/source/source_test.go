package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSource_UsesLocalRepoWhenCollectionsExist(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "collections"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	src, err := ResolveSource("", "", "dev")
	if err != nil {
		t.Fatal(err)
	}
	defer src.Cleanup()

	// Normalize symlinked temp paths on macOS (/var vs /private/var).
	got, _ := filepath.EvalSymlinks(src.Dir)
	want, _ := filepath.EvalSymlinks(repoDir)
	if got != want {
		t.Fatalf("resolveSource dir = %q (resolved: %q), want %q (resolved: %q)", src.Dir, got, repoDir, want)
	}
}

func TestResolveSourceForSync_SkipsLocalRepoAutoDetection(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "collections"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevWD) })
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}

	origClone := CloneRemoteFn
	t.Cleanup(func() { CloneRemoteFn = origClone })

	calls := 0
	CloneRemoteFn = func(ref, sourceRepo string) (*Source, error) {
		calls++
		if ref != "" || sourceRepo != "" {
			t.Fatalf("CloneRemoteFn called with unexpected args ref=%q sourceRepo=%q", ref, sourceRepo)
		}
		return &Source{Dir: "/tmp/remote", SHA: "abc123"}, nil
	}

	src, err := ResolveSourceForSync("", "", "dev")
	if err != nil {
		t.Fatal(err)
	}
	defer src.Cleanup()

	if calls != 1 {
		t.Fatalf("CloneRemoteFn calls = %d, want 1", calls)
	}
	if src.Dir != "/tmp/remote" {
		t.Fatalf("resolveSourceForSync dir = %q, want %q", src.Dir, "/tmp/remote")
	}
}
