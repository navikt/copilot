package provider

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
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
		gates := p == "This repo has its own hook gates (.github/hooks). Run them? They run with the same access as the agent. [y/N] "
		if !gates && (!strings.Contains(p, "This repo has its own OpenCode plugins or hook gates (") || !strings.Contains(p, "Load them? They run with the same access as the agent. A no also turns off this repo's AGENTS.md, agents and skills.")) {
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

// OpenCode reads OPENCODE_CONFIG_PROJECT_DISABLE first, so a user's =0 would
// hide our switch: a decline sets both, and the repo's gates go with it.
func TestOpenCodeProjectPluginsDeclineOverridesFalsyFirstKey(t *testing.T) {
	no := false
	proj, _ := pluginTrustSetup(t, true, &no)
	writePlugin(t, proj, "p.ts", "export default {}")
	env := applyOpenCodeProjectPlugins([]string{"OPENCODE_CONFIG_PROJECT_DISABLE=0"}, proj)
	for _, k := range OpenCodeProjectDisableEnvs {
		if !slices.Contains(env, k+"=1") || slices.Contains(env, k+"=0") {
			t.Fatalf("%s not forced on: %v", k, env)
		}
	}
	prev := OpenCodeHookBridge
	t.Cleanup(func() { OpenCodeHookBridge = prev })
	OpenCodeHookBridge = func(domain.ResolvedConfig) HookBridge {
		return HookBridge{Pre: []BridgeHook{{Name: "user-gate", Command: "true"}, {Name: "repo-gate", Command: "evil", Repo: true}}}
	}
	out, _ := applyOpenCodeHooks(domain.ResolvedConfig{}, env, nil)
	if got := openCodeEnvGetter(out)(OpenCodeHooksEnv); strings.Contains(got, "repo-gate") {
		t.Fatalf("repo gate kept: %s", got)
	}
}

func TestOpenCodeProjectDisabledFirstSetKeyWins(t *testing.T) {
	for env, want := range map[string]bool{
		"OPENCODE_CONFIG_PROJECT_DISABLE=0 OPENCODE_DISABLE_PROJECT_CONFIG=1":    false,
		"OPENCODE_CONFIG_PROJECT_DISABLE= OPENCODE_DISABLE_PROJECT_CONFIG=1":     false,
		"OPENCODE_CONFIG_PROJECT_DISABLE=true OPENCODE_DISABLE_PROJECT_CONFIG=0": true,
		"OPENCODE_DISABLE_PROJECT_CONFIG=1":                                      true,
		"X=1":                                                                    false,
	} {
		if got := OpenCodeProjectDisabled(openCodeEnvLookup(strings.Fields(env))); got != want {
			t.Errorf("%s: got %v, want %v", env, got, want)
		}
	}
}

func writeRepoGate(t *testing.T, proj, command string) {
	t.Helper()
	p := filepath.Join(proj, ".github", "hooks", "copilot-hooks.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":1,"hooks":{"preToolUse":[{"type":"command","navPilot":"g","command":"` + command + `"}]}}`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Repo gates are shell commands the bridge runs; with no plugins they still
// need a yes. A no, or no answer, drops only the gates: the project config stays.
func TestOpenCodeRepoGatesOnlyDeclineKeepsProjectConfig(t *testing.T) {
	for _, tty := range []bool{true, false} {
		no := false
		proj, asked := pluginTrustSetup(t, tty, &no)
		writeRepoGate(t, proj, "true")
		env := applyOpenCodeProjectPlugins(nil, proj)
		if OpenCodeProjectDisabled(openCodeEnvLookup(env)) || *asked != map[bool]int{true: 1}[tty] {
			t.Fatalf("tty=%v asked=%d env=%v", tty, *asked, env)
		}
		if got := bridgeRepoGates(t, env); strings.Contains(got, "repo-gate") {
			t.Fatalf("tty=%v repo gate kept: %s", tty, got)
		}
	}
}

// With plugins, a no still turns off the whole project config.
func TestOpenCodeRepoGatesWithPluginsDeclineDisablesProject(t *testing.T) {
	no := false
	proj, _ := pluginTrustSetup(t, true, &no)
	writePlugin(t, proj, "p.ts", "x")
	writeRepoGate(t, proj, "true")
	if env := applyOpenCodeProjectPlugins(nil, proj); !disabled(env) || strings.Contains(bridgeRepoGates(t, env), "repo-gate") {
		t.Fatalf("env=%v", env)
	}
}

func bridgeRepoGates(t *testing.T, env []string) string {
	t.Helper()
	prev := OpenCodeHookBridge
	t.Cleanup(func() { OpenCodeHookBridge = prev })
	OpenCodeHookBridge = func(domain.ResolvedConfig) HookBridge {
		return HookBridge{Pre: []BridgeHook{{Name: "user-gate", Command: "true"}, {Name: "repo-gate", Command: "evil", Repo: true}}}
	}
	out, _ := applyOpenCodeHooks(domain.ResolvedConfig{}, env, nil)
	return openCodeEnvGetter(out)(OpenCodeHooksEnv)
}

// A hooks file over pluginMaxRead is hashed by its entries, not name and size.
func TestOpenCodeRepoGateLargeFileSwapAsksAgain(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	p := filepath.Join(proj, ".github", "hooks", "copilot-hooks.json")
	write := func(cmd string) {
		body := `{"version":1,"hooks":{"preToolUse":[{"type":"command","navPilot":"g","command":"` + cmd + `"}]},"pad":"` + strings.Repeat("x", pluginMaxRead) + `"}`
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("true")
	applyOpenCodeProjectPlugins(nil, proj)
	write("evil") // same length
	applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 2 {
		t.Fatalf("asked %d times, want 2", *asked)
	}
}

// Gates run scripts in .github/hooks; changing one asks again.
func TestOpenCodeRepoGateScriptChangeAsksAgain(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	writeRepoGate(t, proj, "python3 .github/hooks/g.py")
	script := filepath.Join(proj, ".github", "hooks", "g.py")
	if err := os.WriteFile(script, []byte("print(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	applyOpenCodeProjectPlugins(nil, proj)
	if err := os.WriteFile(script, []byte("print(2)"), 0o644); err != nil {
		t.Fatal(err)
	}
	applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 2 {
		t.Fatalf("asked %d times, want 2", *asked)
	}
}

func TestOpenCodeRepoGateChangeAsksAgain(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	writePlugin(t, proj, "p.ts", "x")
	writeRepoGate(t, proj, "true")
	applyOpenCodeProjectPlugins(nil, proj)
	writeRepoGate(t, proj, "curl evil")
	applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 2 {
		t.Fatalf("asked %d times, want 2", *asked)
	}
}

func TestOpenCodeRepoGatesAllowClearsInheritedDropEnv(t *testing.T) {
	yes := true
	proj, _ := pluginTrustSetup(t, true, &yes)
	writeRepoGate(t, proj, "true")
	env := applyOpenCodeProjectPlugins([]string{openCodeDropRepoGatesEnv + "=1"}, proj)
	if got := bridgeRepoGates(t, env); !strings.Contains(got, "repo-gate") {
		t.Fatalf("inherited drop env kept repo gate off: %s", got)
	}
}

func TestOpenCodeRepoGateLargeScriptSwapAsksAgain(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	writeRepoGate(t, proj, "python3 .github/hooks/g.py")
	script := filepath.Join(proj, ".github", "hooks", "g.py")
	pad := strings.Repeat("a", pluginMaxRead+1)
	if err := os.WriteFile(script, []byte("# "+pad), 0o644); err != nil {
		t.Fatal(err)
	}
	applyOpenCodeProjectPlugins(nil, proj)
	if err := os.WriteFile(script, []byte("# b"+pad[1:]), 0o644); err != nil {
		t.Fatal(err)
	}
	applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 2 {
		t.Fatalf("asked %d times, want 2", *asked)
	}
}

func TestOpenCodeRepoGateSymlinkOutsideRepoIsUnknown(t *testing.T) {
	yes := true
	proj, _ := pluginTrustSetup(t, true, &yes)
	writeRepoGate(t, proj, "true")
	ext := t.TempDir()
	if err := os.Symlink(ext, filepath.Join(proj, ".github", "hooks", "ext")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", ".."), filepath.Join(proj, ".github", "hooks", "up")); err != nil {
		t.Fatal(err)
	}
	set := openCodeProjectPlugins(proj, nil)
	if !strings.HasPrefix(set.sum, "unknown:") {
		t.Fatalf("sum = %q, want unknown:", set.sum)
	}
}

func TestOpenCodeRepoGateFileSymlinkOutsideRepoIsUnknown(t *testing.T) {
	yes := true
	proj, asked := pluginTrustSetup(t, true, &yes)
	writeRepoGate(t, proj, "python3 .github/hooks/g.py")
	outside := filepath.Join(t.TempDir(), "outside.py")
	if err := os.WriteFile(outside, []byte("print(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(proj, ".github", "hooks", "g.py")); err != nil {
		t.Fatal(err)
	}
	set := openCodeProjectPlugins(proj, nil)
	if !strings.HasPrefix(set.sum, "unknown:") {
		t.Fatalf("sum = %q, want unknown:", set.sum)
	}
	applyOpenCodeProjectPlugins(nil, proj)
	applyOpenCodeProjectPlugins(nil, proj)
	if *asked != 2 {
		t.Fatalf("asked %d times, want 2", *asked)
	}
}
