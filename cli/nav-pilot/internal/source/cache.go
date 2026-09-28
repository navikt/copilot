package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CacheDir, when set, is where a launch keeps the last checkout of a remote
// source (see [ResolveForLaunch]). The CLI sets it; empty, as in unit tests,
// every launch fetches, as [ResolveSource] does.
var CacheDir string

const (
	// launchRefreshEvery is how old a cached checkout may get before a launch
	// fetches the next one in the background.
	launchRefreshEvery = time.Hour
	// firstFetchTimeout bounds the one fetch a launch waits on: the first for
	// a source, when there is nothing cached to start from.
	firstFetchTimeout = 30 * time.Second
	// refreshTimeout bounds a background fetch, well inside the age at which
	// its lock counts as left over.
	refreshTimeout = 5 * time.Minute
)

type cacheMeta struct {
	SHA       string    `json:"sha"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// ResolveForLaunch is [ResolveSource] for a session about to start. A remote
// source is read from the checkout an earlier launch fetched, so the launch
// does not wait on github.com, and refresh fetches the next checkout while the
// session runs: the launch after gets it. Only the first launch of a source
// waits on the network, for at most firstFetchTimeout; when that fails (no
// network), the launches in the hour after do not wait again but go without
// and fetch in the background.
//
// refresh is never nil. Run it in the background and cancel its context when
// the session ends; a fetch cut short leaves the cache as it was.
func ResolveForLaunch(sourceRepo, cliVersion string) (src *Source, refresh func(context.Context), err error) {
	return ResolveForLaunchWithin(sourceRepo, cliVersion, 0)
}

// ResolveForLaunchWithin is [ResolveForLaunch] with a limit on the cached
// checkout's age: past maxAge the launch fetches a new one and waits for it
// (for at most FetchTimeout, when set), and a failed fetch is an error, as
// it is for [ResolveSource]. Zero means no limit.
func ResolveForLaunchWithin(sourceRepo, cliVersion string, maxAge time.Duration) (src *Source, refresh func(context.Context), err error) {
	refresh = func(context.Context) {}
	src, err = resolveSource("", sourceRepo, cliVersion, func(ref, repo string) (*Source, error) {
		if CacheDir == "" {
			return CloneRemoteFn(ref, repo)
		}
		dir := filepath.Join(CacheDir, cacheKey(repo))
		if s, fetched, ok := cachedCheckout(dir); ok {
			if maxAge > 0 && time.Since(fetched) >= maxAge {
				return refetch(dir, repo, fetched)
			}
			if time.Since(fetched) >= launchRefreshEvery {
				refresh = func(ctx context.Context) { _ = fetchIntoCache(ctx, dir, repo) }
			}
			return s, nil
		}
		failed := filepath.Join(dir, ".first-fetch-failed")
		if info, err := os.Stat(failed); err == nil && time.Since(info.ModTime()) < launchRefreshEvery {
			refresh = func(ctx context.Context) { _ = fetchIntoCache(ctx, dir, repo) }
			why, _ := os.ReadFile(failed)
			return nil, fmt.Errorf("no copy of %s here yet: fetching it failed %s (%s). nav-pilot tries again while this session runs",
				cacheLabel(repo), minutesAgo(time.Since(info.ModTime())), strings.TrimSpace(string(why)))
		}
		if FetchTimeout == 0 || FetchTimeout > firstFetchTimeout {
			defer func(d time.Duration) { FetchTimeout = d }(FetchTimeout)
			FetchTimeout = firstFetchTimeout
		}
		// Clone inside the cache directory, dot-named so pruneCache spares
		// it, so the move into the cache is a rename on one filesystem.
		if os.MkdirAll(dir, 0o700) == nil {
			defer func(p, pat string) { cloneTempParent, cloneTempPattern = p, pat }(cloneTempParent, cloneTempPattern)
			cloneTempParent, cloneTempPattern = dir, ".fetch-*"
		}
		s, err := CloneRemoteFn(ref, repo)
		if err != nil {
			_ = os.WriteFile(failed, []byte(err.Error()), 0o600)
			return nil, err
		}
		os.Remove(failed)
		keepInCache(dir, s)
		return s, nil
	})
	return src, refresh, err
}

// refetch fetches a new checkout into the cache and waits for it. Another
// launch already fetching holds the lock, and then the checkout there is used:
// the next launch gets the new one.
func refetch(dir, repo string, fetched time.Time) (*Source, error) {
	timeout := firstFetchTimeout
	if FetchTimeout > 0 && FetchTimeout < timeout {
		timeout = FetchTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := fetchIntoCache(ctx, dir, repo); err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("no answer in %s", timeout)
		}
		return nil, fmt.Errorf("the copy of %s is from %s, and fetching a new one failed: %w",
			cacheLabel(repo), fetched.Local().Format("2006-01-02 15:04"), err)
	}
	s, _, ok := cachedCheckout(dir)
	if !ok {
		return nil, fmt.Errorf("fetching %s left no checkout in %s", cacheLabel(repo), dir)
	}
	return s, nil
}

// minutesAgo says how long ago, in words a message can carry.
func minutesAgo(d time.Duration) string {
	if m := int(d.Minutes()); m >= 1 {
		return fmt.Sprintf("%d minute(s) ago", m)
	}
	return "just now"
}

// cacheLabel is the repo as a message names it.
func cacheLabel(repo string) string {
	if repo == "" {
		return DefaultRepo
	}
	return repo
}

// cacheKey is the directory name for a source repo: owner/name with the slash
// replaced, so it is one path element.
func cacheKey(repo string) string {
	if repo == "" {
		repo = DefaultRepo
	}
	return strings.NewReplacer("/", "__", "\\", "__", ":", "_").Replace(repo)
}

// cachedCheckout returns the checkout meta.json names, when it is there.
func cachedCheckout(dir string) (*Source, time.Time, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return nil, time.Time{}, false
	}
	var m cacheMeta
	if json.Unmarshal(data, &m) != nil || m.SHA == "" || strings.ContainsAny(m.SHA, `/\.`) {
		return nil, time.Time{}, false
	}
	co := filepath.Join(dir, m.SHA)
	if info, err := os.Stat(co); err != nil || !info.IsDir() {
		return nil, time.Time{}, false
	}
	return &Source{Dir: co, SHA: m.SHA}, m.FetchedAt, true
}

// keepInCache moves a fresh clone into the cache and points s at it. When the
// move fails, s stays the temporary clone and nothing is cached: the next
// launch fetches again. A checkout of the same commit already there (another
// launch got there first) is left alone, since that launch may be reading it.
func keepInCache(dir string, s *Source) {
	if s.TempDir == "" || s.SHA == "" || s.SHA == "unknown" {
		return
	}
	if os.MkdirAll(dir, 0o700) != nil {
		return
	}
	co := filepath.Join(dir, s.SHA)
	if _, err := os.Stat(co); err != nil && os.Rename(s.TempDir, co) != nil {
		return
	}
	s.Cleanup()
	s.Dir, s.TempDir = co, ""
	writeCacheMeta(dir, cacheMeta{SHA: s.SHA, FetchedAt: time.Now()})
}

// fetchIntoCache fetches the source's default branch into dir and makes it the
// cached checkout. The previous checkout stays until the one after, so a launch
// reading it while this runs is not left with a directory that went away.
func fetchIntoCache(ctx context.Context, dir, repo string) error {
	// Private: the source may be a private repository.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// One refresh at a time, so two sessions cannot each move the cache on
	// and prune the checkout a third launch is reading. A lock older than
	// the longest fetch is left over from a killed process.
	lock := filepath.Join(dir, ".refresh.lock")
	if info, err := os.Stat(lock); err == nil && time.Since(info.ModTime()) > 10*time.Minute {
		os.Remove(lock)
	}
	lf, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil
	}
	lf.Close()
	defer os.Remove(lock)
	ctx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()
	tmp, err := os.MkdirTemp(dir, ".fetch-")
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	if err := fetchRevision(ctx, tmp, RemoteURLFn(repo), "", &stderr); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	sha := getGitSHA(tmp)
	if sha == "unknown" {
		os.RemoveAll(tmp)
		return nil
	}
	prev, _, _ := cachedCheckout(dir)
	co := filepath.Join(dir, sha)
	if _, err := os.Stat(co); err == nil {
		os.RemoveAll(tmp)
	} else if err := os.Rename(tmp, co); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	writeCacheMeta(dir, cacheMeta{SHA: sha, FetchedAt: time.Now()})
	os.Remove(filepath.Join(dir, ".first-fetch-failed"))
	keep := map[string]bool{sha: true, "meta.json": true}
	if prev != nil {
		keep[prev.SHA] = true
	}
	pruneCache(dir, keep)
	return nil
}

// pruneCache removes checkouts other than keep, and fetches abandoned more
// than a day ago (a session that ended mid-fetch).
func pruneCache(dir string, keep map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if keep[e.Name()] {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			if info, err := e.Info(); err != nil || time.Since(info.ModTime()) < 24*time.Hour {
				continue
			}
		}
		os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}

func writeCacheMeta(dir string, m cacheMeta) {
	data, _ := json.Marshal(m)
	tmp, err := os.CreateTemp(dir, ".meta-")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return
	}
	if os.Rename(tmp.Name(), filepath.Join(dir, "meta.json")) != nil {
		os.Remove(tmp.Name())
	}
}
