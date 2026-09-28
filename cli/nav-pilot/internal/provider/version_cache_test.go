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
	t.Cleanup(func() { VersionCacheFile = "" })
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
	// An upgrade replaces the binary: asked again.
	write("1.18.30")
	future := time.Now().Add(time.Minute)
	_ = os.Chtimes(bin, future, future)
	if got := ask(); got != "1.18.30" || asked() != 2 {
		t.Fatalf("after an upgrade: %q, asked %d times", got, asked())
	}
}
