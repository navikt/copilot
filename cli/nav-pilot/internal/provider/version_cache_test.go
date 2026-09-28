package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A client's version is asked once, then read from the cache until the
// binary changes.
func TestCachedVersionPersists(t *testing.T) {
	dir := t.TempDir()
	count := filepath.Join(dir, "count")
	bin := filepath.Join(dir, "fakeclient")
	write := func(version string) {
		script := "#!/bin/sh\necho x >> " + count + "\necho " + version + "\n"
		if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("1.18.25")
	VersionCacheFile = filepath.Join(dir, "client-versions.json")
	t.Cleanup(func() { VersionCacheFile = ""; versionCache.Delete(bin) })
	asked := func() int {
		b, _ := os.ReadFile(count)
		return len(b) / 2
	}
	ask := func() string {
		versionCache.Delete(bin) // a new process
		out, err := cachedVersion(bin, 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}

	if got := ask(); got != "1.18.25" || asked() != 1 {
		t.Fatalf("first run: %q, asked %d times", got, asked())
	}
	if got := ask(); got != "1.18.25" || asked() != 1 {
		t.Fatalf("second run: %q, asked %d times, want the cached answer", got, asked())
	}
	// Overwritten in place with the same size and mtime (cp -p): asked
	// again, because the status-change time moved.
	fi, _ := os.Stat(bin)
	write("1.18.26")
	_ = os.Chtimes(bin, fi.ModTime(), fi.ModTime())
	if got := ask(); got != "1.18.26" || asked() != 2 {
		t.Fatalf("after an overwrite in place: %q, asked %d times", got, asked())
	}
	// An upgrade replaces the binary: asked again.
	write("1.18.30")
	future := time.Now().Add(time.Minute)
	_ = os.Chtimes(bin, future, future)
	if got := ask(); got != "1.18.30" || asked() != 3 {
		t.Fatalf("after an upgrade: %q, asked %d times", got, asked())
	}
}
