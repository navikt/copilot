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

	// The first clone is made inside the cache, so moving it there is a
	// rename on one filesystem even when /tmp is a tmpfs.
	orig := CloneRemoteFn
	t.Cleanup(func() { CloneRemoteFn = orig })
	CloneRemoteFn = func(ref, repo string) (*Source, error) {
		if want := filepath.Join(CacheDir, cacheKey("navikt/x")); cloneTempParent != want {
			t.Errorf("first clone made in %q, want %q", cloneTempParent, want)
		}
		return cloneRemote(ref, repo)
	}
	src, refresh, err := ResolveForLaunch("navikt/x", "v1")
	if err != nil {
		t.Fatal(err)
	}
	if cloneTempParent != "" {
		t.Errorf("clone temp parent left at %q", cloneTempParent)
	}
	if src.SHA != first || src.TempDir != "" || !strings.HasPrefix(src.Dir, CacheDir) {
		t.Fatalf("first launch: got %+v, want sha %s cached under %s", src, first, CacheDir)
	}
	refresh(context.Background()) // fresh: nothing to do

	// Cached: the network is not asked.
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

	// A refresh already running elsewhere (its lock is fresh): this one
	// leaves the cache alone.
	if err := os.WriteFile(filepath.Join(dir, ".refresh.lock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	writeCacheMeta(dir, cacheMeta{SHA: second, FetchedAt: time.Now().Add(-2 * launchRefreshEvery)})
	commit("three")
	_, refresh, _ = ResolveForLaunch("navikt/x", "v1")
	refresh(context.Background())
	if src, _, _ := ResolveForLaunch("navikt/x", "v1"); src.SHA != second {
		t.Fatalf("refreshed while another refresh held the lock: %s", src.SHA)
	}
	os.Remove(filepath.Join(dir, ".refresh.lock"))
	if info, err := os.Stat(dir); err != nil {
		t.Error(err)
	} else if info.Mode().Perm() != 0o700 {
		t.Errorf("cache dir mode %v, want 0700 (a private source is checked out there)", info.Mode().Perm())
	}

	// A cancelled refresh leaves the cache as it was.
	_, refresh, _ = ResolveForLaunch("navikt/x", "v1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	refresh(ctx)
	if src, _, _ := ResolveForLaunch("navikt/x", "v1"); src.SHA != second {
		t.Fatalf("cancelled refresh changed the cache: %s", src.SHA)
	}
}

// Offline, the first launch of a source waits for the fetch once; the
// launches in the hour after go without, and fetch in the background.
func TestResolveForLaunchOfflineWaitsOnce(t *testing.T) {
	CacheDir = t.TempDir()
	t.Cleanup(func() { CacheDir = "" })
	orig := CloneRemoteFn
	t.Cleanup(func() { CloneRemoteFn = orig })
	clones := 0
	CloneRemoteFn = func(string, string) (*Source, error) {
		clones++
		return nil, errors.New("unreachable")
	}
	if _, _, err := ResolveForLaunch("navikt/x", "v1"); err == nil || clones != 1 {
		t.Fatalf("first launch: err %v, %d clones", err, clones)
	}
	_, refresh, err := ResolveForLaunch("navikt/x", "v1")
	if err == nil || clones != 1 || !strings.Contains(err.Error(), "tries again while this session runs") {
		t.Fatalf("second launch: err %v, %d clones; want no wait and a background fetch", err, clones)
	}

	// The background fetch lands the checkout for the launch after.
	repo := t.TempDir()
	gitRun(t, repo, "init", "--quiet", "-b", "main", ".")
	if err := os.WriteFile(filepath.Join(repo, "marker.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "--quiet", "-m", "one")
	localRemote(t, repo)
	refresh(context.Background())
	if src, _, err := ResolveForLaunch("navikt/x", "v1"); err != nil || src.SHA != gitRun(t, repo, "rev-parse", "HEAD") {
		t.Fatalf("after the background fetch: %+v, %v", src, err)
	}
	dir := filepath.Join(CacheDir, cacheKey("navikt/x"))
	if _, err := os.Stat(filepath.Join(dir, ".first-fetch-failed")); !os.IsNotExist(err) {
		t.Errorf("the failure marker outlived a successful refresh: %v", err)
	}
}

// A failure more than an hour old is tried again at launch; the reason is
// kept in the message; a successful first fetch clears the marker.
func TestResolveForLaunchOfflineMarker(t *testing.T) {
	CacheDir = t.TempDir()
	t.Cleanup(func() { CacheDir = "" })
	orig := CloneRemoteFn
	t.Cleanup(func() { CloneRemoteFn = orig })
	clones := 0
	CloneRemoteFn = func(string, string) (*Source, error) {
		clones++
		return nil, errors.New("authentication failed")
	}
	ResolveForLaunch("navikt/y", "v1")
	if _, _, err := ResolveForLaunch("navikt/y", "v1"); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("the reason is lost: %v", err)
	}
	failed := filepath.Join(CacheDir, cacheKey("navikt/y"), ".first-fetch-failed")
	old := time.Now().Add(-61 * time.Minute)
	if err := os.Chtimes(failed, old, old); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	gitRun(t, repo, "init", "--quiet", "-b", "main", ".")
	gitRun(t, repo, "commit", "--quiet", "--allow-empty", "-m", "one")
	localRemote(t, repo)
	CloneRemoteFn = func(ref, r string) (*Source, error) { clones++; return cloneRemote(ref, r) }
	if _, _, err := ResolveForLaunch("navikt/y", "v1"); err != nil || clones != 2 {
		t.Fatalf("an hour on: err %v, %d clones; want a fresh try", err, clones)
	}
	if _, err := os.Stat(failed); !os.IsNotExist(err) {
		t.Errorf("the failure marker outlived a successful first fetch: %v", err)
	}
}

// Past maxAge the launch does not start from the cached checkout: it waits for
// a new one, and a failed fetch is an error rather than the old copy.
func TestResolveForLaunchWithinMaxAge(t *testing.T) {
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
	const maxAge = 24 * time.Hour

	if src, _, err := ResolveForLaunchWithin("navikt/x", "v1", maxAge); err != nil || src.SHA != first {
		t.Fatalf("first launch: %+v, %v", src, err)
	}
	second := commit("two")
	dir := filepath.Join(CacheDir, cacheKey("navikt/x"))

	// Younger than maxAge: the cached checkout, no wait.
	writeCacheMeta(dir, cacheMeta{SHA: first, FetchedAt: time.Now().Add(-maxAge / 2)})
	if src, _, err := ResolveForLaunchWithin("navikt/x", "v1", maxAge); err != nil || src.SHA != first {
		t.Fatalf("cache inside maxAge: %+v, %v; want %s", src, err, first)
	}

	// Older: the launch fetches the next commit and starts from it.
	writeCacheMeta(dir, cacheMeta{SHA: first, FetchedAt: time.Now().Add(-2 * maxAge)})
	src, _, err := ResolveForLaunchWithin("navikt/x", "v1", maxAge)
	if err != nil || src.SHA != second {
		t.Fatalf("cache past maxAge: %+v, %v; want %s", src, err, second)
	}

	// Older, and another process holds the refresh lock but brings in
	// nothing: an error once the wait is over, not the old copy.
	if err := os.WriteFile(filepath.Join(dir, ".refresh.lock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	writeCacheMeta(dir, cacheMeta{SHA: second, FetchedAt: time.Now().Add(-2 * maxAge)})
	// The short wait is for this case only: a later case that fetches for
	// real must not race git against it (#1335).
	t.Cleanup(func() { FetchTimeout = 0 })
	FetchTimeout = 600 * time.Millisecond
	src, _, err = ResolveForLaunchWithin("navikt/x", "v1", maxAge)
	FetchTimeout = 0
	if err == nil {
		t.Fatalf("cache past maxAge, lock held: got %+v, want an error", src)
	}
	os.Remove(filepath.Join(dir, ".refresh.lock"))

	// A copy dated in the future (the clock was set back) is not fresh.
	third := commit("three")
	writeCacheMeta(dir, cacheMeta{SHA: second, FetchedAt: time.Now().Add(maxAge)})
	if src, _, err := ResolveForLaunchWithin("navikt/x", "v1", maxAge); err != nil || src.SHA != third {
		t.Fatalf("cache dated in the future: %+v, %v; want %s", src, err, third)
	}

	// Older and offline: an error, not the old copy.
	writeCacheMeta(dir, cacheMeta{SHA: third, FetchedAt: time.Now().Add(-2 * maxAge)})
	RemoteURLFn = func(string) string { return "file://" + filepath.Join(repo, "gone") }
	if src, _, err := ResolveForLaunchWithin("navikt/x", "v1", maxAge); err == nil {
		t.Fatalf("cache past maxAge, offline: got %+v, want an error", src)
	}
}
