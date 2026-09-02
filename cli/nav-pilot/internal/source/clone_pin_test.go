package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitRun runs git in dir and fails the test on error.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// twoCommitRepo builds a real git repository whose single tracked file says
// "one" at the first commit and "two" at the second, and returns its path plus
// both full SHAs.
func twoCommitRepo(t *testing.T) (dir, first, second string) {
	t.Helper()
	dir = t.TempDir()
	gitRun(t, dir, "init", "--quiet", "-b", "main", ".")
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "--quiet", "-m", "one")
	first = gitRun(t, dir, "rev-parse", "HEAD")
	gitRun(t, dir, "tag", "v1")
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "--quiet", "-m", "two")
	second = gitRun(t, dir, "rev-parse", "HEAD")
	return dir, first, second
}

// localRemote points the clone path at a repository on disk instead of GitHub,
// so these tests exercise the real git plumbing without a network.
func localRemote(t *testing.T, dir string) {
	t.Helper()
	orig := RemoteURLFn
	t.Cleanup(func() { RemoteURLFn = orig })
	RemoteURLFn = func(string) string { return "file://" + dir }
}

func marker(t *testing.T, src *Source) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(src.Dir, "marker.txt"))
	if err != nil {
		t.Fatalf("reading the checkout: %v", err)
	}
	return strings.TrimSpace(string(data))
}

// TestCloneRemoteResolvesPinnedCommit is the defect the declaration feature
// stands on: a pinned commit SHA must actually check out. `git clone --branch`
// never could — it takes a branch or tag name — so a repo that pinned a
// revision could not install it back.
func TestCloneRemoteResolvesPinnedCommit(t *testing.T) {
	dir, first, second := twoCommitRepo(t)
	localRemote(t, dir)

	src, err := cloneRemote(first, "navikt/grillmester")
	if err != nil {
		t.Fatalf("cloning the pinned commit: %v", err)
	}
	defer src.Cleanup()

	if got := marker(t, src); got != "one" {
		t.Errorf("pinned checkout has marker %q, want the first commit's %q", got, "one")
	}
	if src.SHA != first {
		t.Errorf("pinned checkout reports SHA %q, want %q", src.SHA, first)
	}
	if src.SHA == second {
		t.Error("pinning the first commit checked out the second")
	}
}

// A SHA is only re-fetchable at full length: git refuses an abbreviated object
// id in a fetch request, so recording a short one is a pin nobody can resolve.
func TestRecordedSHAIsFetchable(t *testing.T) {
	dir, first, _ := twoCommitRepo(t)
	localRemote(t, dir)

	src, err := cloneRemote("", "navikt/grillmester")
	if err != nil {
		t.Fatalf("cloning the default branch: %v", err)
	}
	defer src.Cleanup()

	if len(src.SHA) != 40 {
		t.Fatalf("recorded SHA %q is %d characters; a fetchable pin needs the full 40", src.SHA, len(src.SHA))
	}
	// And the value it records round-trips: fetching it back gets that commit.
	again, err := cloneRemote(src.SHA, "navikt/grillmester")
	if err != nil {
		t.Fatalf("re-fetching the recorded SHA %q: %v", src.SHA, err)
	}
	defer again.Cleanup()
	if again.SHA != src.SHA {
		t.Errorf("re-fetch landed on %q, want %q", again.SHA, src.SHA)
	}
	_ = first
}

// The branch and tag paths are what every non-pinned install uses; the pin fix
// must not disturb them.
func TestCloneRemoteBranchAndTag(t *testing.T) {
	dir, first, second := twoCommitRepo(t)
	localRemote(t, dir)

	for _, tc := range []struct{ ref, want, sha string }{
		{"", "two", second},
		{"main", "two", second},
		{"v1", "one", first},
	} {
		src, err := cloneRemote(tc.ref, "navikt/grillmester")
		if err != nil {
			t.Fatalf("cloning ref %q: %v", tc.ref, err)
		}
		if got := marker(t, src); got != tc.want {
			t.Errorf("ref %q checked out marker %q, want %q", tc.ref, got, tc.want)
		}
		if src.SHA != tc.sha {
			t.Errorf("ref %q reports SHA %q, want %q", tc.ref, src.SHA, tc.sha)
		}
		src.Cleanup()
	}
}

// A ref that exists nowhere still fails, and says so.
func TestCloneRemoteUnknownRef(t *testing.T) {
	dir, _, _ := twoCommitRepo(t)
	localRemote(t, dir)

	src, err := cloneRemote("no-such-ref", "navikt/grillmester")
	if err == nil {
		src.Cleanup()
		t.Fatal("an unknown ref cloned successfully")
	}
	if !strings.Contains(err.Error(), "no-such-ref") {
		t.Errorf("error %q does not name the ref that failed", err)
	}
}
