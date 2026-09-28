package artifacts

import (
	"sync"
	"testing"
	"time"
)

// The cached check never waits on the fetch: it answers from the cache and
// fetches in the background for the next command.
func TestAssessStalenessCachedDoesNotWait(t *testing.T) {
	CacheHome = t.TempDir()
	t.Cleanup(func() { CacheHome = "" })
	refreshOnce = sync.Once{} // -count=2 runs this in one process

	release := make(chan struct{})
	fetch := func() (string, string, error) {
		<-release
		return "2026.09.24-120000-bbbbbbb", "nav-pilot/2026.09.24-120000-bbbbbbb", nil
	}
	start := time.Now()
	a := AssessStalenessCached("2026.09.01-120000-aaaaaaa", fetch)
	if took := time.Since(start); took > time.Second {
		t.Fatalf("waited %s on the fetch", took)
	}
	if a.LatestVersion != "" || a.Result != "cooldown" {
		t.Fatalf("empty cache: got %+v", a)
	}
	close(release)
	WaitForRefresh(5 * time.Second)

	a = AssessStalenessCached("2026.09.01-120000-aaaaaaa", func() (string, string, error) {
		t.Fatal("fetched again with a fresh cache")
		return "", "", nil
	})
	if a.LatestVersion != "2026.09.24-120000-bbbbbbb" {
		t.Fatalf("after the refresh: got %+v", a)
	}
}
