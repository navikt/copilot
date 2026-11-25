package cli

import (
	"bytes"
	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeOldCpltOnPath puts a cplt older than the fake release first on PATH.
func fakeOldCpltOnPath(t *testing.T) {
	t.Helper()
	cli := filepath.Join(t.TempDir(), "cplt")
	if err := os.WriteFile(cli, []byte("#!/bin/sh\necho 'cplt 2026.01.01-000000-0000000'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(cli)+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// seedCpltCache writes a cplt cache entry checked at the given time.
func seedCpltCache(t *testing.T, version string, checked time.Time) {
	t.Helper()
	artifacts.WriteCacheAt(artifacts.CpltCacheFilePath(), &artifacts.StalenessCache{
		LastChecked:   checked.UTC().Format(time.RFC3339),
		LatestVersion: version,
	})
}

const (
	oldCached  = "2026.10.01-120000-1111111"
	newRelease = "2026.11.01-120000-abcdef0"
)

// Each entry a lookup must not trust: it asks GitHub and stores the answer.
func TestCachedLatestCpltVersionRefreshes(t *testing.T) {
	for _, tc := range []struct {
		name, version string
		checked       time.Duration
	}{
		{"expired", oldCached, -25 * time.Hour},
		{"future-dated", oldCached, time.Hour},
		{"unparseable version", "garbage", -time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hits := fakeCpltReleases(t, 0)
			seedCpltCache(t, tc.version, time.Now().Add(tc.checked))
			v, err := cachedLatestCpltVersion()
			if err != nil || v != newRelease {
				t.Fatalf("got %q, %v; want %q", v, err, newRelease)
			}
			if n := hits.Load(); n != 1 {
				t.Fatalf("GitHub asked %d times, want 1", n)
			}
			if c := artifacts.ReadCacheAt(artifacts.CpltCacheFilePath()); c == nil || c.LatestVersion != newRelease {
				t.Fatalf("cache %+v, want %q stored", c, newRelease)
			}
		})
	}
}

// A failed refresh of an expired entry returns the old answer, not an error.
func TestCachedLatestCpltVersionFailedRefreshKeepsOld(t *testing.T) {
	isolatedConfig(t)
	t.Setenv("GITHUB_TOKEN", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	orig := cpltReleasesAPI
	cpltReleasesAPI = srv.URL
	t.Cleanup(func() { cpltReleasesAPI = orig })
	seedCpltCache(t, oldCached, time.Now().Add(-25*time.Hour))
	v, err := cachedLatestCpltVersion()
	if err != nil || v != oldCached {
		t.Fatalf("got %q, %v; want the old %q", v, err, oldCached)
	}
}

// Upgrade is an explicit action: it waits longer than doctor's 1.5 s.
func TestCpltUpgradeUsesLongTimeout(t *testing.T) {
	fakeCpltReleases(t, 300*time.Millisecond)
	fakeOldCpltOnPath(t)
	orig := cpltLookupTimeout
	cpltLookupTimeout = 50 * time.Millisecond
	t.Cleanup(func() { cpltLookupTimeout = orig })
	var out bytes.Buffer
	reportCpltUpgrade(&out)
	if !strings.Contains(out.String(), "out of date") {
		t.Fatalf("upgrade output %q: it gave up at doctor's timeout", out.String())
	}
}

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
	fakeOldCpltOnPath(t)
	var out bytes.Buffer
	reportCpltUpgrade(&out)
	if !strings.Contains(out.String(), "out of date") {
		t.Fatalf("upgrade output %q, want cplt reported out of date", out.String())
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("GitHub asked %d times, want 2: upgrade must not use the cache", n)
	}
}
