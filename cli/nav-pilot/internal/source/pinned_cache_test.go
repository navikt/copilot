package source

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
)

// pinnedFixture is a git repository with one commit, and a fake clone that
// copies it the way cloneRemote would: into cloneTempParent, as a TempDir.
func pinnedFixture(t *testing.T) (sha string, clones *int) {
	t.Helper()
	origin := t.TempDir()
	gitRun(t, origin, "init", "--quiet", "-b", "main", ".")
	writeFile(t, filepath.Join(origin, "skills", "base-skill", "SKILL.md"), "# base\n")
	gitRun(t, origin, "add", "-A")
	gitRun(t, origin, "commit", "--quiet", "-m", "base")
	sha = gitRun(t, origin, "rev-parse", "HEAD")

	prevCache := CacheDir
	CacheDir = t.TempDir()
	t.Cleanup(func() { CacheDir = prevCache })
	orig := CloneRemoteFn
	t.Cleanup(func() { CloneRemoteFn = orig })
	n := 0
	CloneRemoteFn = func(ref, repo string) (*Source, error) {
		n++
		tmp, err := os.MkdirTemp(cloneTempParent, cloneTempPattern)
		if err != nil {
			return nil, err
		}
		if err := os.CopyFS(tmp, os.DirFS(origin)); err != nil {
			return nil, err
		}
		return &Source{Dir: tmp, TempDir: tmp, SHA: getGitSHA(tmp), Repo: repo}, nil
	}
	return sha, &n
}

func pinnedDir(repo, sha string) string {
	return filepath.Join(CacheDir, "pinned", cacheKey(repo), sha)
}

// The first resolve fetches into the cache; every later one reads it there,
// offline, and cleaning up the source leaves the cache in place.
func TestResolvePinnedCachesTheRevision(t *testing.T) {
	sha, clones := pinnedFixture(t)

	src, err := resolvePinned(sha, "navikt/base")
	if err != nil {
		t.Fatal(err)
	}
	if *clones != 1 || src.Dir != pinnedDir("navikt/base", sha) || src.TempDir != "" || src.Repo != "navikt/base" {
		t.Fatalf("first resolve: %d clones, %+v", *clones, src)
	}
	src.Cleanup()
	if _, err := os.Stat(filepath.Join(src.Dir, "skills", "base-skill", "SKILL.md")); err != nil {
		t.Fatalf("Cleanup removed the cached checkout: %v", err)
	}

	CloneRemoteFn = func(string, string) (*Source, error) { return nil, errors.New("offline") }
	again, err := resolvePinned(sha, "navikt/base")
	if err != nil {
		t.Fatalf("a cached pinned base needed the network: %v", err)
	}
	if again.Dir != src.Dir || again.SHA != sha {
		t.Fatalf("second resolve: %+v", again)
	}
}

// A cache entry that is not a checkout of the pinned commit is rebuilt.
func TestResolvePinnedRebuildsABrokenEntry(t *testing.T) {
	sha, clones := pinnedFixture(t)
	co := pinnedDir("navikt/base", sha)
	writeFile(t, filepath.Join(co, "half-written"), "x")

	src, err := resolvePinned(sha, "navikt/base")
	if err != nil {
		t.Fatal(err)
	}
	if *clones != 1 || src.Dir != co || getGitSHA(co) != sha {
		t.Fatalf("broken entry not rebuilt: %d clones, %+v", *clones, src)
	}
	if _, err := os.Stat(filepath.Join(co, "half-written")); !os.IsNotExist(err) {
		t.Error("the broken entry's content survived the rebuild")
	}
}

// Local paths, short shas and a missing cache directory resolve as before,
// with nothing written to the cache.
func TestResolvePinnedBypassesTheCache(t *testing.T) {
	sha, clones := pinnedFixture(t)

	local := t.TempDir()
	if src, err := resolvePinned("", local); err != nil || src.Dir != local {
		t.Fatalf("local path: %+v, %v", src, err)
	}
	if src, err := resolvePinned(sha[:7], "navikt/base"); err != nil || src.TempDir == "" {
		t.Fatalf("short sha should be an uncached clone: %+v, %v", src, err)
	} else {
		src.Cleanup()
	}
	if _, err := os.Stat(filepath.Join(CacheDir, "pinned", cacheKey("navikt/base"), sha[:7])); !os.IsNotExist(err) {
		t.Error("a short sha was cached")
	}

	CacheDir = ""
	src, err := resolvePinned(sha, "navikt/base")
	if err != nil || src.TempDir == "" {
		t.Fatalf("no cache dir should be an uncached clone: %+v, %v", src, err)
	}
	src.Cleanup()
	if *clones != 2 {
		t.Errorf("clones = %d, want 2", *clones)
	}
}

// An upper-case pin and a differently cased repo find the entry a lower-case
// pin fetched, offline: declarations accept either case.
func TestResolvePinnedIgnoresCase(t *testing.T) {
	sha, clones := pinnedFixture(t)
	if _, err := resolvePinned(sha, "navikt/base"); err != nil {
		t.Fatal(err)
	}
	CloneRemoteFn = func(string, string) (*Source, error) { return nil, errors.New("offline") }
	src, err := resolvePinned(strings.ToUpper(sha), "Navikt/Base")
	if err != nil {
		t.Fatalf("a differently cased pin missed the cache: %v", err)
	}
	if src.Dir != pinnedDir("navikt/base", sha) || src.SHA != sha || src.Repo != "Navikt/Base" {
		t.Fatalf("got %+v", src)
	}
	if *clones != 1 {
		t.Errorf("clones = %d, want 1", *clones)
	}
}

// While another process holds the publish lock, a miss clones on its own,
// uncached, and neither waits nor touches the cache.
func TestResolvePinnedLeavesALockedRepoAlone(t *testing.T) {
	sha, clones := pinnedFixture(t)
	dir := filepath.Join(CacheDir, "pinned", cacheKey("navikt/base"))
	writeFile(t, filepath.Join(dir, ".publish.lock"), "")
	broken := filepath.Join(dir, sha, "being-published")
	writeFile(t, broken, "x")

	src, err := resolvePinned(sha, "navikt/base")
	if err != nil {
		t.Fatal(err)
	}
	if *clones != 1 || src.TempDir == "" {
		t.Fatalf("want one uncached clone, got %d clones, %+v", *clones, src)
	}
	src.Cleanup()
	if _, err := os.Stat(broken); err != nil {
		t.Error("an entry under another process's lock was removed")
	}
}

// A repo that is not owner/name never becomes a cache path.
func TestResolvePinnedRefusesOddRepoNames(t *testing.T) {
	sha, _ := pinnedFixture(t)
	if src, err := resolvePinned(sha, ".."); err == nil {
		src.Cleanup()
	}
	if _, err := os.Stat(filepath.Join(CacheDir, "pinned")); !os.IsNotExist(err) {
		t.Error("a repo of \"..\" reached the cache")
	}
}

// A publish prunes revisions of the same repo nobody has read for
// pinnedUnusedFor; recently read ones stay, and reading one keeps it.
func TestResolvePinnedPrunesUnusedRevisions(t *testing.T) {
	sha, _ := pinnedFixture(t)
	old := pinnedDir("navikt/base", strings.Repeat("1", 40))
	recent := pinnedDir("navikt/base", strings.Repeat("2", 40))
	writeFile(t, filepath.Join(old, "f"), "x")
	writeFile(t, filepath.Join(recent, "f"), "x")
	long := time.Now().Add(-pinnedUnusedFor - time.Hour)
	for _, d := range []string{old, recent} {
		if err := os.Chtimes(d, long, long); err != nil {
			t.Fatal(err)
		}
	}
	touchPinned(recent) // read since: kept

	if _, err := resolvePinned(sha, "navikt/base"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("an unused revision was not pruned")
	}
	if _, err := os.Stat(recent); err != nil {
		t.Error("a recently read revision was pruned")
	}
	if _, err := os.Stat(pinnedDir("navikt/base", sha)); err != nil {
		t.Error("the revision just published was pruned")
	}
}

// Composition goes through the cache: a second compose of the same pinned base
// fetches nothing and its bases' Cleanup leaves the cache intact.
func TestComposeResolverReadsPinnedBaseFromCache(t *testing.T) {
	sha, clones := pinnedFixture(t)
	top := t.TempDir()
	writeCompositionFile(t, top, agentpakke.DeclarationPath, `{"contractVersion":"1","source":"navikt/base","sha":"`+sha+`"}`)
	src := compositionSource(t, top)

	for i := 0; i < 2; i++ {
		resolver, bases, err := ComposeResolver(NewSourceResolverForLayout(top, src.Pakke.Layout), src)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := resolver.SourceRootFor("skills/base-skill/SKILL.md"); !ok {
			t.Fatal("base skill not composed")
		}
		bases.Cleanup()
	}
	if *clones != 1 {
		t.Errorf("clones = %d, want 1: the second compose should read the cache", *clones)
	}
}
