package source

import (
	"bytes"
	"context"
	"encoding/json"
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
	firstFetchTimeout = 60 * time.Second
)

type cacheMeta struct {
	SHA       string    `json:"sha"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// ResolveForLaunch is [ResolveSource] for a session about to start. A remote
// source is read from the checkout an earlier launch fetched, so the launch
// does not wait on github.com, and refresh fetches the next checkout while the
// session runs: the launch after gets it. Only the first launch of a source
// waits on the network, for at most firstFetchTimeout.
//
// refresh is never nil. Run it in the background and cancel its context when
// the session ends; a fetch cut short leaves the cache as it was.
func ResolveForLaunch(sourceRepo, cliVersion string) (src *Source, refresh func(context.Context), err error) {
	refresh = func(context.Context) {}
	src, err = resolveSource("", sourceRepo, cliVersion, func(ref, repo string) (*Source, error) {
		if CacheDir == "" {
			return CloneRemoteFn(ref, repo)
		}
		dir := filepath.Join(CacheDir, cacheKey(repo))
		if s, fetched, ok := cachedCheckout(dir); ok {
			if time.Since(fetched) >= launchRefreshEvery {
				refresh = func(ctx context.Context) { _ = fetchIntoCache(ctx, dir, repo) }
			}
			return s, nil
		}
		if FetchTimeout == 0 || FetchTimeout > firstFetchTimeout {
			defer func(d time.Duration) { FetchTimeout = d }(FetchTimeout)
			FetchTimeout = firstFetchTimeout
		}
		s, err := CloneRemoteFn(ref, repo)
		if err != nil {
			return nil, err
		}
		keepInCache(dir, s)
		return s, nil
	})
	return src, refresh, err
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
// move fails (the temp directory on another filesystem), s stays the
// temporary clone and nothing is cached: the next launch fetches again.
func keepInCache(dir string, s *Source) {
	if s.TempDir == "" || s.SHA == "" || s.SHA == "unknown" {
		return
	}
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	co := filepath.Join(dir, s.SHA)
	_ = os.RemoveAll(co)
	if os.Rename(s.TempDir, co) != nil {
		return
	}
	s.Dir, s.TempDir = co, ""
	writeCacheMeta(dir, cacheMeta{SHA: s.SHA, FetchedAt: time.Now()})
}

// fetchIntoCache fetches the source's default branch into dir and makes it the
// cached checkout. The previous checkout stays until the one after, so a launch
// reading it while this runs is not left with a directory that went away.
func fetchIntoCache(ctx context.Context, dir, repo string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
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
