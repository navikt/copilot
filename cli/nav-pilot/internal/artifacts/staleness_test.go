package artifacts

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckStaleness_SkipsDevVersion(t *testing.T) {
	setupTestCache(t)
	result := CheckStaleness("dev", func() (string, string, error) {
		t.Fatal("fetchFn should not be called for dev version")
		return "", "", nil
	})
	if result != "" {
		t.Errorf("expected empty for dev version, got %q", result)
	}
}

func TestCheckStaleness_SkipsEmptyVersion(t *testing.T) {
	setupTestCache(t)
	result := CheckStaleness("", func() (string, string, error) {
		t.Fatal("fetchFn should not be called for empty version")
		return "", "", nil
	})
	if result != "" {
		t.Errorf("expected empty for empty version, got %q", result)
	}
}

func TestCheckStaleness_DetectsUpdate(t *testing.T) {
	setupTestCache(t)

	result := CheckStaleness("2026.01.01-080000-old1234", func() (string, string, error) {
		return "2026.04.13-170138-abc1234", "nav-pilot/2026.04.13-170138-abc1234", nil
	})
	if result != "2026.04.13-170138-abc1234" {
		t.Errorf("expected update version, got %q", result)
	}
}

func TestCheckStaleness_UpToDate(t *testing.T) {
	setupTestCache(t)

	result := CheckStaleness("2026.04.13-170138-abc1234", func() (string, string, error) {
		return "2026.04.13-170138-abc1234", "nav-pilot/2026.04.13-170138-abc1234", nil
	})
	if result != "" {
		t.Errorf("expected empty for up-to-date version, got %q", result)
	}
}

func TestCheckStaleness_UsesCachedResult(t *testing.T) {
	setupTestCache(t)

	WriteCache(&StalenessCache{
		LastChecked:   time.Now().UTC().Format(time.RFC3339),
		LatestVersion: "2026.05.01-120000-new1234",
	})

	called := false
	result := CheckStaleness("2026.01.01-080000-old1234", func() (string, string, error) {
		called = true
		return "2026.05.01-120000-new1234", "nav-pilot/2026.05.01-120000-new1234", nil
	})
	if called {
		t.Error("expected cache hit, but fetchFn was called")
	}
	if result != "2026.05.01-120000-new1234" {
		t.Errorf("expected cached version, got %q", result)
	}
}

func TestCheckStaleness_CachedOlderVersionNoWarning(t *testing.T) {
	setupTestCache(t)

	WriteCache(&StalenessCache{
		LastChecked:   time.Now().UTC().Format(time.RFC3339),
		LatestVersion: "2026.04.14-120650-71dcb83",
	})

	result := CheckStaleness("2026.04.14-202800-a25f6c3", func() (string, string, error) {
		t.Fatal("fetchFn should not be called with fresh cache")
		return "", "", nil
	})
	if result != "" {
		t.Errorf("expected no warning when current is newer than cached, got %q", result)
	}
}

func TestCheckStaleness_ExpiredCacheRefetches(t *testing.T) {
	setupTestCache(t)

	WriteCache(&StalenessCache{
		LastChecked:   time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339),
		LatestVersion: "2026.03.01-060000-old",
	})

	result := CheckStaleness("2026.01.01-080000-old1234", func() (string, string, error) {
		return "2026.05.01-120000-new1234", "nav-pilot/2026.05.01-120000-new1234", nil
	})
	if result != "2026.05.01-120000-new1234" {
		t.Errorf("expected new version from fetchFn, got %q", result)
	}
}

func TestCheckStaleness_NetworkErrorSkips(t *testing.T) {
	setupTestCache(t)

	result := CheckStaleness("2026.01.01-080000-old1234", func() (string, string, error) {
		return "", "", errors.New("network error")
	})
	if result != "" {
		t.Errorf("expected empty on network error, got %q", result)
	}
}

func TestCacheFilePath(t *testing.T) {
	path := CacheFilePath()
	if path == "" {
		t.Skip("no home directory available")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}
	if filepath.Base(path) != "cache.json" {
		t.Errorf("expected cache.json, got %q", filepath.Base(path))
	}
}

func setupTestCache(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	origHome := CacheHome
	CacheHome = dir
	t.Cleanup(func() { CacheHome = origHome })
}
