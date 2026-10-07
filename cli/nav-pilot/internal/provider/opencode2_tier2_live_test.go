package provider

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// TestOpenCode2LiveTier2UnderCplt runs `opencode debug config` and `debug
// agents` under real cplt with the arguments and environment a Tier 2 launch
// builds (buildStagedOpenCodeSpec): the payload's agent is loaded (cplt
// overlays OPENCODE_CONFIG_DIR) and the user's opencode.json is read as
// OPENCODE_CONFIG. Controls: without --pass-env OPENCODE_CONFIG the user's
// file is gone, and without OPENCODE_CONFIG_DIR the payload's agent is.
// Opt-in: NAV_PILOT_OPENCODE2 and NAV_PILOT_CPLT.
func TestOpenCode2LiveTier2UnderCplt(t *testing.T) {
	oc, _ := liveOpenCode2(t)
	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24\n", nil, time.Hour})
	t.Cleanup(func() { versionCache.Delete("opencode") })
	SetActivePakke(stagedFixturePakke())
	t.Cleanup(func() { SetActivePakke(nil) })

	work := liveDir(t)
	payload, proj := filepath.Join(work, "payload"), filepath.Join(work, "proj")
	mustWrite(t, filepath.Join(payload, "agents", "grillmester.md"), "---\ndescription: payload probe\nmode: primary\n---\nPAYLOAD-AGENT-PROMPT\n")
	_ = os.MkdirAll(proj, 0o755)
	_ = exec.Command("git", "-C", proj, "init", "-q").Run()
	user := filepath.Join(openCodeConfigDir(), "opencode.json")
	mustWrite(t, user, `{"instructions":["user-probe.md"]}`)

	spec := buildStagedSpec(t, "opencode", domain.ResolvedConfig{}, StagedLaunch{PakkeName: "grillmester", Dir: payload, Context: "full"})
	if !slices.Contains(spec.env, "OPENCODE_CONFIG="+user) {
		t.Fatalf("the user's config is not OPENCODE_CONFIG: %v", spec.cpltArgs)
	}
	llm := &fakeLLM{}
	srv := newFakeLLMServer(t, llm)
	env := withOpenCodeConfigContent(spec.env, map[string]any{"provider": map[string]any{"fake": map[string]any{
		"npm": "@ai-sdk/openai-compatible", "name": "Fake", "options": map[string]any{"baseURL": srv + "/v1", "apiKey": "x"},
		"models": map[string]any{"m": map[string]any{"name": "m", "tool_call": true}}}}})
	// As LaunchOpenCodeStaged: the hooks step passes OPENCODE_CONFIG_CONTENT.
	prev := OpenCodeHookBridge
	t.Cleanup(func() { OpenCodeHookBridge = prev })
	OpenCodeHookBridge = func(domain.ResolvedConfig) HookBridge { return HookBridge{} }
	env, spec.cpltArgs = applyOpenCodeHooks(domain.ResolvedConfig{}, env, spec.cpltArgs)
	without := func(args []string, flag string) []string {
		i := slices.Index(args, flag)
		return slices.Delete(slices.Clone(args), i-1, i+1)
	}
	noDir := slices.DeleteFunc(slices.Clone(env), func(e string) bool { return strings.HasPrefix(e, "OPENCODE_CONFIG_DIR=") })

	// The user's file, as opencode lists its config sources.
	for _, c := range []struct {
		name     string
		cpltArgs []string
		found    bool
	}{
		{"user config read", spec.cpltArgs, true},
		{"control: user config not passed", without(spec.cpltArgs, "OPENCODE_CONFIG"), false},
	} {
		out := runOpenCode2(t, oc, proj, []string{"debug", "config"}, env, c.cpltArgs)
		if strings.Contains(out, user) != c.found {
			t.Errorf("%s: %s listed = %v, want %v\n%s", c.name, user, !c.found, c.found, tail(out, 1500))
		}
	}
	// The payload's agent, by its prompt reaching the model.
	run := []string{"run", "--agent", "grillmester", "--model", "fake/m", "go"}
	for _, c := range []struct {
		name  string
		env   []string
		found bool
	}{
		{"payload agent loaded", env, true},
		{"control: no payload dir", noDir, false},
	} {
		llm.mu.Lock()
		llm.bodies = nil
		llm.mu.Unlock()
		out := runOpenCode2(t, oc, proj, run, c.env, spec.cpltArgs, srv)
		if strings.Contains(llm.requests(), "PAYLOAD-AGENT-PROMPT") != c.found {
			t.Errorf("%s: payload prompt sent = %v, want %v\n%s", c.name, !c.found, c.found, tail(out, 1500))
		}
	}
}
