package source

import (
	"os"
	"path/filepath"
	"strings"
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

// A URL and an SSH form are what people try first, and both used to reach git,
// which failed with its own words about a repository name nobody typed (#801).
func TestValidateSourceValueSuggestsTheShorthand(t *testing.T) {
	for _, tc := range []struct{ in, wantSub string }{
		{"https://github.com/nais/pilot", `"nais/pilot"`},
		{"https://github.com/nais/pilot.git", `"nais/pilot"`},
		{"git@github.com:nais/pilot.git", `"nais/pilot"`},
		{"not-a-source", "owner/name"},
		{"too/many/slashes", "owner/name"},
	} {
		err := ValidateSourceValue(tc.in)
		if err == nil {
			t.Errorf("ValidateSourceValue(%q) = nil, want an error", tc.in)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantSub) {
			t.Errorf("ValidateSourceValue(%q) = %q, want it to contain %q", tc.in, err, tc.wantSub)
		}
	}
	for _, ok := range []string{"nais/pilot", "/abs/path"} {
		if err := ValidateSourceValue(ok); err != nil {
			t.Errorf("ValidateSourceValue(%q) = %v, want nil", ok, err)
		}
	}
}

// A browse URL is not a clone URL. Suggesting owner/name from its last two
// path segments pointed people at a repo that does not exist, and a query
// string ended up inside the suggested name.
func TestShorthandForOnlyAcceptsARepoRoot(t *testing.T) {
	for in, want := range map[string]string{
		"https://github.com/nais/pilot":                  "nais/pilot",
		"https://github.com/nais/pilot.git":              "nais/pilot",
		"https://github.com/nais/pilot/":                 "nais/pilot",
		"https://github.com/nais/pilot?tab=readme":       "nais/pilot",
		"https://github.com/nais/pilot#readme":           "nais/pilot",
		"git@github.com:nais/pilot.git":                  "nais/pilot",
		"https://github.com/nais/pilot/tree/main":        "",
		"https://github.com/nais/pilot/blob/main/go.mod": "",
		"https://github.com/nais":                        "",
		"https://github.com/":                            "",
	} {
		if got := shorthandFor(in); got != want {
			t.Errorf("shorthandFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSymlinkRejectedInFileHelpers(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}

	if _, err := DirHash(dir); err == nil {
		t.Error("DirHash accepted a symlink inside dir")
	}
	if _, err := DirHash(link); err == nil {
		t.Error("DirHash accepted a symlink root")
	}

	dstDir := t.TempDir()
	if err := CopyFile(link, filepath.Join(dstDir, "out.txt"), dstDir); err == nil {
		t.Error("CopyFile accepted a symlink source")
	}

	realSrc := filepath.Join(outside, "real.txt")
	if err := os.WriteFile(realSrc, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	dstLink := filepath.Join(dstDir, "dst-link.txt")
	if err := os.Symlink(secret, dstLink); err != nil {
		t.Fatal(err)
	}
	if err := CopyFile(realSrc, dstLink, dstDir); err == nil {
		t.Error("CopyFile overwrote a symlink destination")
	}
	if got, _ := os.ReadFile(secret); string(got) != "secret" {
		t.Errorf("secret file modified via symlink: %q", got)
	}

	copyRoot := t.TempDir()
	if err := CopyDir(dir, filepath.Join(copyRoot, "copied"), copyRoot); err == nil {
		t.Error("CopyDir accepted a directory containing a symlink")
	}
}

func TestReadGitHEAD(t *testing.T) {
	const (
		looseSHA  = "1111111111111111111111111111111111111111"
		packedSHA = "2222222222222222222222222222222222222222"
	)

	makeRepo := func(t *testing.T) (string, string) {
		t.Helper()
		root := t.TempDir()
		gitDir := filepath.Join(root, ".git")
		if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return root, gitDir
	}

	t.Run("loose ref and packed-refs fallback", func(t *testing.T) {
		root, gitDir := makeRepo(t)
		if err := os.WriteFile(filepath.Join(gitDir, "packed-refs"), []byte("# pack-refs with: peeled fully-peeled sorted\n"+packedSHA+" refs/heads/main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := readGitHEAD(root); got != packedSHA {
			t.Fatalf("packed-refs fallback = %q, want %q", got, packedSHA)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "refs", "heads", "main"), []byte(looseSHA+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := readGitHEAD(root); got != looseSHA {
			t.Fatalf("loose ref = %q, want %q", got, looseSHA)
		}
	})

	t.Run("chained symref in loose ref does not fall back to stale packed-refs", func(t *testing.T) {
		root, gitDir := makeRepo(t)
		if err := os.WriteFile(filepath.Join(gitDir, "packed-refs"), []byte(packedSHA+" refs/heads/main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "refs", "heads", "main"), []byte("ref: refs/heads/other\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := readGitHEAD(root); got != "" {
			t.Fatalf("chained symref returned stale packed SHA %q, want empty fallback", got)
		}
	})

	t.Run("rejects non-refs target and symlinks", func(t *testing.T) {
		root, gitDir := makeRepo(t)
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(looseSHA+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: config\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := readGitHEAD(root); got != "" {
			t.Fatalf("non-refs HEAD target = %q, want empty", got)
		}

		outside := filepath.Join(t.TempDir(), "sha")
		if err := os.WriteFile(outside, []byte(looseSHA+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(gitDir, "refs", "heads", "main")); err != nil {
			t.Fatal(err)
		}
		if got := readGitHEAD(root); got != "" {
			t.Fatalf("symlinked loose ref = %q, want empty", got)
		}
	})
}
