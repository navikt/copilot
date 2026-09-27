package testhome

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) { os.Exit(Run(m)) }

func TestHomeIsRedirected(t *testing.T) {
	home := os.Getenv("HOME")
	if !strings.Contains(filepath.Base(home), "nav-pilot-test-home-") {
		t.Fatalf("HOME = %q, want the temporary test home", home)
	}
	if RealHome() == home {
		t.Fatalf("RealHome() = %q, the temporary home", home)
	}
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "NAV_PILOT_CONFIG"} {
		if !strings.HasPrefix(os.Getenv(k), home) {
			t.Errorf("%s = %q, not under the test home %s", k, os.Getenv(k), home)
		}
	}
}

func TestGuardSeesAWrite(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, ".nav-pilot", "cache.json")
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(home)
	if p := diff(before, snapshot(home)); p != "" {
		t.Fatalf("nothing written, yet diff = %q", p)
	}

	// An in-place rewrite leaves the directory's mtime alone.
	later := time.Now().Add(time.Second)
	if err := os.Chtimes(cache, later, later); err != nil {
		t.Fatal(err)
	}
	if got := diff(before, snapshot(home)); got != cache {
		t.Fatalf("rewritten cache.json: diff = %q, want %q", got, cache)
	}

	// A new file in a watched tree.
	before = snapshot(home)
	hook := filepath.Join(home, ".copilot", "hooks", "gate.json")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if diff(before, snapshot(home)) == "" {
		t.Fatal("a new ~/.copilot/hooks/gate.json went unnoticed")
	}
}

// Every test package must call Run from its TestMain, or its tests run
// against the real home.
func TestEveryTestPackageCallsRun(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	has := map[string]bool{}  // dir has a _test.go file
	runs := map[string]bool{} // dir calls Run
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		dir := filepath.Dir(p)
		has[dir] = true
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), "testhome.Run(") || strings.Contains(string(body), "os.Exit(Run(m))") {
			runs[dir] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for dir := range has {
		if !runs[dir] {
			rel, _ := filepath.Rel(root, dir)
			t.Errorf("%s has tests but no TestMain calling testhome.Run(m); see internal/testhome", rel)
		}
	}
}
