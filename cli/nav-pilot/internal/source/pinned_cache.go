package source

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// pinnedUnusedFor is how long a cached pinned checkout may go unread before
// the next publish for the same repo removes it. A pakke bumping its base
// leaves the old revision behind, and nothing else would clean it up.
const pinnedUnusedFor = 30 * 24 * time.Hour

// pinnedLockStale is how old a publish lock may be before it counts as left
// over from a killed process. It matches the launch cache's refresh lock.
const pinnedLockStale = 10 * time.Minute

// resolvePinned resolves a reused base at the revision its declaration pins.
//
// A pinned revision never changes, so once fetched it is kept under
// CacheDir/pinned/<repo>/<sha> and read from there by every later install,
// sync and launch, without the network. Unlike the launch cache in
// [ResolveForLaunch] there is nothing to refresh and no age limit.
//
// The cache is skipped, and this is [ResolveSource], when there is no cache
// directory (unit tests), for a local path (a working tree, read live), for a
// repo that is not owner/name, and for anything but a full 40-character sha (a
// short sha or a ref names no single immutable revision). The returned source
// has no TempDir when it comes from the cache, so its Cleanup leaves the cache
// alone.
//
// Readers take no lock, as with the launch cache. Pruning runs under the
// publish lock and only removes revisions no one has read for
// pinnedUnusedFor, so a checkout in use is not removed in practice.
func resolvePinned(sha, repo string) (*Source, error) {
	if CacheDir == "" || filepath.IsAbs(repo) || ValidateSourceValue(repo) != nil || !isFullHexSHA(sha) {
		return ResolveSource(sha, repo, "")
	}
	// Git prints a sha in lower case and GitHub names are case-insensitive,
	// so either spelling of a pin or a repo finds the same entry.
	sha = strings.ToLower(sha)
	dir := filepath.Join(CacheDir, "pinned", cacheKey(strings.ToLower(repo)))
	co := filepath.Join(dir, sha)
	if pinnedCheckoutOK(co, sha) {
		touchPinned(co)
		return &Source{Dir: co, SHA: sha, Repo: repo}, nil
	}

	// Invalidating and publishing happen under one lock per repo, and the
	// entry is checked again once it is held: another process may have
	// published it in the meantime, and that copy must not be removed.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ResolveSource(sha, repo, "")
	}
	unlock, ok := lockPinned(dir)
	if !ok {
		// Another process is fetching this repo. Do not wait on it; this run
		// uses a clone of its own, uncached, as it did before the cache.
		return ResolveSource(sha, repo, "")
	}
	defer unlock()
	if pinnedCheckoutOK(co, sha) {
		touchPinned(co)
		return &Source{Dir: co, SHA: sha, Repo: repo}, nil
	}
	// Whatever is at co does not hold the pinned commit: a damaged entry or
	// one edited by hand. Rebuild it rather than compose from it.
	if err := os.RemoveAll(co); err != nil {
		return ResolveSource(sha, repo, "")
	}

	// Clone inside the cache, dot-named, so publishing is a rename on one
	// filesystem.
	defer func(p, pat string) { cloneTempParent, cloneTempPattern = p, pat }(cloneTempParent, cloneTempPattern)
	cloneTempParent, cloneTempPattern = dir, ".fetch-*"
	src, err := CloneRemoteFn(sha, repo)
	if err != nil {
		return nil, err
	}
	src.Repo = repo
	if src.TempDir == "" || src.SHA != sha {
		return src, nil // not something this cache can vouch for
	}
	if err := os.Rename(src.TempDir, co); err != nil {
		return src, nil // keep the temporary clone, uncached
	}
	src.Dir, src.TempDir = co, ""
	prunePinned(dir, sha)
	return src, nil
}

// touchPinned marks a checkout as read now, so pruning spares it.
func touchPinned(co string) {
	now := time.Now()
	_ = os.Chtimes(co, now, now)
}

// prunePinned removes revisions of one repo nobody has read for
// pinnedUnusedFor, and fetches abandoned more than a day ago, as pruneCache
// does for the launch cache. The caller holds the publish lock. Each entry's
// age is read again just before it is removed, so one read since the listing
// keeps it.
func prunePinned(dir, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if name == keep || name == ".publish.lock" {
			continue
		}
		limit := pinnedUnusedFor
		if strings.HasPrefix(name, ".") {
			limit = 24 * time.Hour
		}
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && time.Since(info.ModTime()) > limit {
			_ = os.RemoveAll(p)
		}
	}
}

// lockPinned takes the publish lock for one repo's pinned entries. ok is false
// when another live process holds it.
func lockPinned(dir string) (unlock func(), ok bool) {
	lock := filepath.Join(dir, ".publish.lock")
	if info, err := os.Stat(lock); err == nil && time.Since(info.ModTime()) > pinnedLockStale {
		os.Remove(lock)
	}
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, fs.ErrExist) || err != nil {
		return nil, false
	}
	f.Close()
	return func() { os.Remove(lock) }, true
}

// pinnedCheckoutOK reports whether co is a git checkout whose HEAD is sha.
// Publishing is a single rename, so this catches an entry that is missing,
// half-removed or replaced. Like the launch cache, it trusts the files in a
// checkout that does have the right HEAD: the cache is the user's own.
func pinnedCheckoutOK(co, sha string) bool {
	info, err := os.Stat(co)
	if err != nil || !info.IsDir() {
		return false
	}
	return getGitSHA(co) == sha
}

// isFullHexSHA reports whether s is forty hex characters, in either case, the
// form agentpakke declarations accept.
func isFullHexSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}
