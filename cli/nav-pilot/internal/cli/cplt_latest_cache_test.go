package cli

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// fakeCpltReleases points the cplt release lookup at a local server that
// counts its requests and answers after delay.
func fakeCpltReleases(t *testing.T, delay time.Duration) *atomic.Int32 {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("GITHUB_TOKEN", "")
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(`[{"tag_name":"2026.11.01-120000-abcdef0"}]`))
	}))
	t.Cleanup(srv.Close)
	orig := cpltReleasesAPI
	cpltReleasesAPI = srv.URL
	t.Cleanup(func() { cpltReleasesAPI = orig })
	return &hits
}

// #2904: a second doctor run within the day reads the cache, not GitHub.
func TestLatestCpltVersionCacheHitSkipsNetwork(t *testing.T) {
	hits := fakeCpltReleases(t, 0)
	for i := 0; i < 2; i++ {
		v, err := latestCpltVersion()
		if err != nil || v != "2026.11.01-120000-abcdef0" {
			t.Fatalf("run %d: got %q, %v", i, v, err)
		}
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("GitHub asked %d times, want 1: the second run must hit the cache", n)
	}
}

// #2904: with nothing cached, a slow GitHub gives up after the short timeout
// and reports an error, which doctor shows as "could not check".
func TestLatestCpltVersionTimesOutWithoutCache(t *testing.T) {
	fakeCpltReleases(t, 5*time.Second)
	orig := cpltLookupTimeout
	cpltLookupTimeout = 100 * time.Millisecond
	t.Cleanup(func() { cpltLookupTimeout = orig })
	start := time.Now()
	v, err := latestCpltVersion()
	if err == nil {
		t.Fatalf("got %q with no error, want a timeout", v)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("lookup took %s, want it bounded by the timeout", d)
	}
}
