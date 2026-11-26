package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// The sign-in hint after a failed opencode launch is for a client that ran,
// not for a cplt setup failure or an answered n (#1962).
func TestOpenCodeClientRanReadsThisLaunchsMarker(t *testing.T) {
	dir := t.TempDir()
	env := []string{HookStateDirEnv + "=" + dir, ClientStartedEnv + "=this"}
	mark := func(id string) {
		if err := os.WriteFile(filepath.Join(dir, "client-started-"+id), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if openCodeClientRan(env) {
		t.Error("no marker: want false (opencode never started)")
	}
	mark("other") // another launch, even in the same second
	if openCodeClientRan(env) {
		t.Error("another launch's marker: want false")
	}
	mark("this")
	if !openCodeClientRan(env) {
		t.Error("this launch's marker: want true")
	}
	if openCodeClientRan(env) {
		t.Error("marker is consumed: a retry with the same id must not reuse it")
	}
	if !openCodeClientRan([]string{HookStateDirEnv + "=" + dir}) {
		t.Error("no launch id (no bridge, or --pure): want true, the hint as before")
	}
}
