package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
		v, err := cachedLatestCpltVersion()
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
	v, err := cachedLatestCpltVersion()
	if err == nil {
		t.Fatalf("got %q with no error, want a timeout", v)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("lookup took %s, want it bounded by the timeout", d)
	}
}

// Owner decision on #2908: upgrade checks cplt fresh, even with a fresh cache.
func TestCpltUpgradeSkipsCache(t *testing.T) {
	hits := fakeCpltReleases(t, 0)
	if _, err := cachedLatestCpltVersion(); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(t.TempDir(), "cplt")
	if err := os.WriteFile(cli, []byte("#!/bin/sh\necho 'cplt 2026.01.01-000000-0000000'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(cli)+string(os.PathListSeparator)+os.Getenv("PATH"))
	var out bytes.Buffer
	reportCpltUpgrade(&out)
	if !strings.Contains(out.String(), "out of date") {
		t.Fatalf("upgrade output %q, want cplt reported out of date", out.String())
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("GitHub asked %d times, want 2: upgrade must not use the cache", n)
	}
}
