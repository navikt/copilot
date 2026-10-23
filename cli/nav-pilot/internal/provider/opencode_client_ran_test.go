package provider

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
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

// opencode 1 with --pure loads no plugin, so the launch gets no id to wait
// for and a real failure still gets the hint.
func TestOpenCodeClientStartedIDSkippedUnderPure(t *testing.T) {
	prev := OpenCodeHookBridge
	t.Cleanup(func() { OpenCodeHookBridge = prev; versionCache.Delete("opencode") })
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	OpenCodeHookBridge = func(domain.ResolvedConfig) HookBridge { return HookBridge{Bin: "nav-pilot", Post: []BridgeHook{{}}} }
	versionCache.Store("opencode", versionAnswer{"1.18.32\n", nil, 0})
	has := func(env []string) bool {
		return slices.ContainsFunc(env, func(e string) bool {
			return len(e) > len(ClientStartedEnv) && e[:len(ClientStartedEnv)+1] == ClientStartedEnv+"="
		})
	}
	if env, _ := applyOpenCodeHooks(domain.ResolvedConfig{}, nil, nil); !has(env) {
		t.Error("plain opencode 1 launch: want a client-started id")
	}
	if env, _ := applyOpenCodeHooks(domain.ResolvedConfig{ExtraArgs: []string{"--pure"}}, nil, nil); has(env) {
		t.Error("--pure: want no client-started id, so the hint falls back")
	}
}
