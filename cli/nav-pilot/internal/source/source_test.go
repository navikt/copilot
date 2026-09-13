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
