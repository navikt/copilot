package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

// A client's version is asked once, then read from the cache until the
// binary changes.
func TestCachedVersionPersists(t *testing.T) {
	dir := t.TempDir()
	count := filepath.Join(dir, "count")
	bin := filepath.Join(dir, "fakeclient")
	write := func(version string) {
		script := "#!/bin/sh\necho x >> " + count + "\necho " + version + "\n"
		if err := testhome.WriteExec(bin, script); err != nil {
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

// A probe stopped by a short deadline does not answer a caller that allows
// more: IsCplt asks with 2s, and the launch gate, asking the same binary with
// 30s, used to get IsCplt's timeout back as its own fatal error.
func TestCachedVersionRetriesATimeoutWithMoreTime(t *testing.T) {
	dir := t.TempDir()
	count := filepath.Join(dir, "count")
	bin := filepath.Join(dir, "slowclient")
	script := "#!/bin/sh\necho x >> " + count + "\nsleep 0.5\necho 1.0.40\n"
	if err := testhome.WriteExec(bin, script); err != nil {
		t.Fatal(err)
	}
	VersionCacheFile = filepath.Join(dir, "client-versions.json")
	t.Cleanup(func() { VersionCacheFile = ""; versionCache.Delete(bin) })
	asked := func() int {
		b, _ := os.ReadFile(count)
		return len(b) / 2
	}

	if _, err := cachedVersion(bin, 100*time.Millisecond); err == nil {
		t.Fatal("a 0.5s probe finished inside 100ms")
	}
	n := asked()
	if _, err := cachedVersion(bin, 100*time.Millisecond); err == nil || asked() != n {
		t.Fatalf("same budget: err %v, asked %d times, want the cached timeout and no new ask", err, asked()-n)
	}
	out, err := cachedVersion(bin, 30*time.Second)
	if err != nil || strings.TrimSpace(out) != "1.0.40" {
		t.Fatalf("longer budget: %q, %v; want the version, asked again", out, err)
	}
}
