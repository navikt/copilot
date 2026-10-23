package provider

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The sign-in hint after a failed opencode launch is for a client that ran,
// not for a cplt setup failure or an answered n (#1962).
func TestOpenCodeClientRanReadsTheBridgeMarker(t *testing.T) {
	dir := t.TempDir()
	env := []string{HookStateDirEnv + "=" + dir}
	marker := filepath.Join(dir, "client-started")
	start := time.Now()

	if openCodeClientRan(env, start) {
		t.Error("no marker: want false (opencode never started)")
	}
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	old := start.Add(-time.Hour)
	if err := os.Chtimes(marker, old, old); err != nil {
		t.Fatal(err)
	}
	if openCodeClientRan(env, start) {
		t.Error("marker from an earlier launch: want false")
	}
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !openCodeClientRan(env, start) {
		t.Error("marker written during this launch: want true")
	}
	if !openCodeClientRan(nil, start) {
		t.Error("no hooks bridge: want true, the hint as before")
	}
}
