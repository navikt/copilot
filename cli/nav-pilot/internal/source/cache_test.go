package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A launch fetches a remote source once, then reads the cached checkout
// without the network; refresh brings in the next commit for the launch after.
func TestResolveForLaunchCachesAndRefreshes(t *testing.T) {
	repo := t.TempDir()
	gitRun(t, repo, "init", "--quiet", "-b", "main", ".")
	commit := func(text string) string {
		if err := os.WriteFile(filepath.Join(repo, "marker.txt"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		gitRun(t, repo, "add", "-A")
		gitRun(t, repo, "commit", "--quiet", "-m", text)
		return gitRun(t, repo, "rev-parse", "HEAD")
	}
	first := commit("one")
	localRemote(t, repo)
	CacheDir = t.TempDir()
	t.Cleanup(func() { CacheDir = "" })

	src, refresh, err := ResolveForLaunch("navikt/x", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if src.SHA != first || src.TempDir != "" || !strings.HasPrefix(src.Dir, CacheDir) {
		t.Fatalf("first launch: got %+v, want sha %s cached under %s", src, first, CacheDir)
	}
	refresh(context.Background()) // fresh: nothing to do

	// Cached: the network is not asked.
	orig := CloneRemoteFn
	t.Cleanup(func() { CloneRemoteFn = orig })
	CloneRemoteFn = func(string, string) (*Source, error) { return nil, errors.New("offline") }
	second := commit("two")
	src, refresh, err = ResolveForLaunch("navikt/x", "v1")
	if err != nil || src.SHA != first {
		t.Fatalf("cached launch: got %+v, %v; want %s", src, err, first)
	}
	refresh(context.Background())
	if src, _, _ := ResolveForLaunch("navikt/x", "v1"); src.SHA != first {
		t.Fatalf("refreshed a fresh cache: got %s", src.SHA)
	}

	// An hour on, the launch still starts from the cache, and its refresh
	// fetches the next commit for the launch after.
	dir := filepath.Join(CacheDir, cacheKey("navikt/x"))
	writeCacheMeta(dir, cacheMeta{SHA: first, FetchedAt: time.Now().Add(-2 * launchRefreshEvery)})
	src, refresh, _ = ResolveForLaunch("navikt/x", "v1")
	if src.SHA != first {
		t.Fatalf("stale cache: launch got %s, want the cached %s", src.SHA, first)
	}
	refresh(context.Background())
	src, _, _ = ResolveForLaunch("navikt/x", "v1")
	if src.SHA != second {
		t.Fatalf("after refresh: got %s, want %s", src.SHA, second)
	}
	if b, _ := os.ReadFile(filepath.Join(src.Dir, "marker.txt")); string(b) != "two" {
		t.Fatalf("checkout has %q", b)
	}
	// The previous checkout stays for a launch still reading it.
	if _, err := os.Stat(filepath.Join(dir, first)); err != nil {
		t.Fatalf("previous checkout removed: %v", err)
	}

	// A cancelled refresh leaves the cache as it was.
	writeCacheMeta(dir, cacheMeta{SHA: second, FetchedAt: time.Now().Add(-2 * launchRefreshEvery)})
	commit("three")
	_, refresh, _ = ResolveForLaunch("navikt/x", "v1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	refresh(ctx)
	if src, _, _ := ResolveForLaunch("navikt/x", "v1"); src.SHA != second {
		t.Fatalf("cancelled refresh changed the cache: %s", src.SHA)
	}
}
