package testhome

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(Run(m)) }

func TestHomeIsRedirected(t *testing.T) {
	home := os.Getenv("HOME")
	if !strings.Contains(filepath.Base(home), "nav-pilot-test-home-") {
		t.Fatalf("HOME = %q, want the temporary test home", home)
	}
	for _, k := range Vars[1:] {
		if !strings.HasPrefix(os.Getenv(k), home) {
			t.Errorf("%s = %q, not under the test home %s", k, os.Getenv(k), home)
		}
	}
}

func TestGuardSeesAWrite(t *testing.T) {
	home := t.TempDir()
	before := snapshot(home)
	if p := diff(before, snapshot(home)); p != "" {
		t.Fatalf("nothing written, yet diff = %q", p)
	}
	if err := os.MkdirAll(filepath.Join(home, ".nav-pilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".nav-pilot", "cache.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if diff(before, snapshot(home)) == "" {
		t.Fatal("a new ~/.nav-pilot/cache.json went unnoticed")
	}
}
