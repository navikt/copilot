package provider

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// pluginTrustSetup is a v2 launch in a fresh project with a fresh HOME. It
// returns the project and a counter of prompts; answer is what the prompt says.
func pluginTrustSetup(t *testing.T, tty bool, answer *bool) (string, *int) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(t.TempDir(), "config.yaml"))
	versionCache.Store("opencode", versionAnswer{"opencode v2.1.4\n", nil, time.Hour})
	prevT, prevA := openCodePluginsTTY, askOpenCodePlugins
	asked := new(int)
	openCodePluginsTTY = func() bool { return tty }
	askOpenCodePlugins = func(p string) bool {
		*asked++
		if !strings.Contains(p, "This repo has its own OpenCode plugins (") || !strings.Contains(p, "Load them? They run with the same access as the agent. A no also turns off this repo's AGENTS.md, agents and skills.") {
			t.Errorf("prompt %q", p)
		}
		return *answer
	}
	t.Cleanup(func() { openCodePluginsTTY, askOpenCodePlugins = prevT, prevA; versionCache.Delete("opencode") })
	return t.TempDir(), asked
}

func writePlugin(t *testing.T, proj, name, body string) {
	t.Helper()
	p := filepath.Join(proj, ".opencode", "plugins", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func disabled(env []string) bool {
	return slices.Contains(env, openCodeDisableProjectConfigEnv+"=1")
}

func TestOpenCodeProjectPluginsNoneNoPrompt(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	// A user-global plugin is not the repo's: never asked about.
	writePlugin(t, os.Getenv("HOME"), "mine.ts", "x")
	env := applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 0 || disabled(env) {
		t.Fatalf("asked=%d env=%v", *asked, env)
	}
}

func TestOpenCodeProjectPluginsYesLoadsAndIsRemembered(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	writePlugin(t, proj, "p.ts", "export default {}")
	for range 2 {
		if env := applyOpenCodeProjectPlugins(nil, proj); disabled(env) {
			t.Fatalf("yes disabled the project config: %v", env)
		}
	}
	if *asked != 1 {
		t.Fatalf("asked %d times, want 1", *asked)
	}
}

func TestOpenCodeProjectPluginsNoDisablesAndIsRemembered(t *testing.T) {
	no := false
	proj, asked := pluginTrustSetup(t, true, &no)
	writePlugin(t, proj, "p.ts", "export default {}")
	for range 2 {
		if env := applyOpenCodeProjectPlugins(nil, proj); !disabled(env) {
			t.Fatalf("no left the project config on: %v", env)
		}
	}
	if *asked != 1 {
		t.Fatalf("asked %d times, want 1", *asked)
	}
}

func TestOpenCodeProjectPluginsChangeAsksAgain(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	writePlugin(t, proj, "p.ts", "one")
	applyOpenCodeProjectPlugins(nil, proj)
	writePlugin(t, proj, "p.ts", "two")
	applyOpenCodeProjectPlugins(nil, proj)
	// A plugins entry in the project's opencode.json is part of the set too.
	if err := os.WriteFile(filepath.Join(proj, "opencode.json"), []byte(`{"plugins":["some-npm-plugin"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 3 {
		t.Fatalf("asked %d times, want 3", *asked)
	}
}

func TestOpenCodeProjectPluginsHeadlessDisables(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, false, &yes)
	writePlugin(t, proj, "p.ts", "x")
	if env := applyOpenCodeProjectPlugins(nil, proj); !disabled(env) || *asked != 0 {
		t.Fatalf("asked=%d env=%v", *asked, env)
	}
	// Headless records nothing: a later terminal launch still asks.
	openCodePluginsTTY = func() bool { return true }
	if env := applyOpenCodeProjectPlugins(nil, proj); disabled(env) || *asked != 1 {
		t.Fatalf("asked=%d env=%v", *asked, env)
	}
}

// A terminal answer holds headless too: yes loads, no stays off.
func TestOpenCodeProjectPluginsHeadlessUsesStoredAnswer(t *testing.T) {
	for _, answer := range []bool{true, false} {
		a := answer
		proj, asked := pluginTrustSetup(t, true, &a)
		writePlugin(t, proj, "p.ts", "x")
		applyOpenCodeProjectPlugins(nil, proj)
		openCodePluginsTTY = func() bool { return false }
		if env := applyOpenCodeProjectPlugins(nil, proj); disabled(env) == answer || *asked != 1 {
			t.Fatalf("stored %v: asked=%d env=%v", answer, *asked, env)
		}
	}
}

func TestRunOpenCodePluginsAsksAgain(t *testing.T) {
	answer := false
	proj, asked := pluginTrustSetup(t, true, &answer)
	writePlugin(t, proj, "p.ts", "x")
	applyOpenCodeProjectPlugins(nil, proj)
	answer = true
	if err := RunOpenCodePlugins(proj); err != nil {
		t.Fatal(err)
	}
	if env := applyOpenCodeProjectPlugins(nil, proj); disabled(env) || *asked != 2 {
		t.Fatalf("asked=%d env=%v", *asked, env)
	}
	// Headless with an answer on record: show it and succeed.
	openCodePluginsTTY = func() bool { return false }
	if err := RunOpenCodePlugins(proj); err != nil || *asked != 2 {
		t.Fatalf("headless, answered: err=%v asked=%d", err, *asked)
	}
	// Headless with no answer for the current set: that one needs a terminal.
	writePlugin(t, proj, "q.ts", "y")
	if err := RunOpenCodePlugins(proj); err == nil || *asked != 2 {
		t.Fatalf("headless, unanswered: err=%v asked=%d", err, *asked)
	}
}

func TestOpenCodeProjectPluginsV1Untouched(t *testing.T) {
	yes := false
	proj, asked := pluginTrustSetup(t, true, &yes)
	versionCache.Store("opencode", versionAnswer{"1.18.32\n", nil, time.Hour})
	writePlugin(t, proj, "p.ts", "x")
	if env := applyOpenCodeProjectPlugins(nil, proj); disabled(env) || *asked != 0 {
		t.Fatalf("asked=%d env=%v", *asked, env)
	}
}
