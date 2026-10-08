package provider

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

func TestWithOpenCodeConfigContent(t *testing.T) {
	env := withOpenCodeConfigContent([]string{`OPENCODE_CONFIG_CONTENT={"plugin":["mine"],"theme":"x"}`},
		map[string]any{"plugin": []any{"file:///p.js"}, "share": "disabled"})
	var cfg map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(env[0], "OPENCODE_CONFIG_CONTENT=")), &cfg); err != nil {
		t.Fatal(err)
	}
	if got := cfg["plugin"].([]any); len(got) != 2 || got[0] != "mine" || got[1] != "file:///p.js" {
		t.Fatalf("plugin = %v", got)
	}
	if cfg["theme"] != "x" || cfg["share"] != "disabled" {
		t.Fatalf("cfg = %v", cfg)
	}
	env = withOpenCodeConfigContent([]string{"OPENCODE_CONFIG_CONTENT=not json"}, map[string]any{"share": "disabled"})
	if env[0] != `OPENCODE_CONFIG_CONTENT={"share":"disabled"}` {
		t.Fatalf("env = %v", env)
	}
}

func TestApplyOpenCodeHooksStagesV2PluginDir(t *testing.T) {
	prev := OpenCodeHookBridge
	t.Cleanup(func() { OpenCodeHookBridge = prev; versionCache.Delete("opencode") })
	OpenCodeHookBridge = func(domain.ResolvedConfig) HookBridge {
		return HookBridge{Post: []BridgeHook{{Name: "r", Argv: []string{"/bin/true"}}}}
	}
	for ver, want := range map[string]string{
		"1.18.35\n":          `"plugin":["file://` + filepath.Join(openCodePluginDir(), "nav-pilot-hooks.js") + `"]`,
		"opencode v2.0.24\n": `"plugins":["` + filepath.Join(openCodePluginDir(), "v2") + `"]`,
	} {
		versionCache.Store("opencode", versionAnswer{ver, nil, time.Hour})
		env, _ := applyOpenCodeHooks(domain.ResolvedConfig{}, nil, nil)
		if got := strings.Join(env, "\n"); !strings.Contains(got, want) {
			t.Errorf("%q: env = %s, want %s", ver, got, want)
		}
	}
	if b, err := os.ReadFile(filepath.Join(openCodePluginDir(), "v2", "index.js")); err != nil || !bytes.Equal(b, hooksBridgePluginV2) {
		t.Errorf("v2/index.js not the v2 bridge: %v", err)
	}
}

// opencode 2 keeps the first plugin of an id, so the bridge's id is fresh per
// launch and passed through cplt; opencode 1 gets none.
func TestApplyOpenCodeHooksV2PluginIDPerLaunch(t *testing.T) {
	prev := OpenCodeHookBridge
	t.Cleanup(func() { OpenCodeHookBridge = prev; versionCache.Delete("opencode") })
	OpenCodeHookBridge = func(domain.ResolvedConfig) HookBridge {
		return HookBridge{Post: []BridgeHook{{Name: "r", Argv: []string{"/bin/true"}}}}
	}
	id := func() (string, []string) {
		env, cplt := applyOpenCodeHooks(domain.ResolvedConfig{}, nil, nil)
		for _, e := range env {
			if v, ok := strings.CutPrefix(e, OpenCodePluginIDEnv+"="); ok {
				return v, cplt
			}
		}
		return "", cplt
	}
	versionCache.Store("opencode", versionAnswer{"1.18.35\n", nil, time.Hour})
	if v, _ := id(); v != "" {
		t.Errorf("opencode 1 got a plugin id: %q", v)
	}
	versionCache.Store("opencode", versionAnswer{"opencode v2.0.24\n", nil, time.Hour})
	a, cplt := id()
	b, _ := id()
	if !strings.HasPrefix(a, "nav-pilot-hooks-") || a == b || !strings.Contains(strings.Join(cplt, " "), "--pass-env "+OpenCodePluginIDEnv) {
		t.Errorf("ids %q, %q; cplt %v", a, b, cplt)
	}
	if !bytes.Contains(hooksBridgePluginV2, []byte("process.env."+OpenCodePluginIDEnv)) {
		t.Error("the v2 bridge does not read its id from " + OpenCodePluginIDEnv)
	}
	// Every model request kind carries tool results (model-request.ts).
	if !bytes.Contains(hooksBridgePluginV2, []byte(`["context", "compaction", "generate", "title"]`)) {
		t.Error("the v2 bridge does not redact every model request kind")
	}
}

// On opencode 2 the bridge runs the dispatch gate, so a launch with the gate
// and no other hook still stages it; on opencode 1 dispatch-gate.js does.
func TestOpenCodeHooksStageBridgeForDispatchGateOnV2(t *testing.T) {
	prev := OpenCodeHookBridge
	t.Cleanup(func() { OpenCodeHookBridge = prev })
	OpenCodeHookBridge = func(domain.ResolvedConfig) HookBridge { return HookBridge{} }
	t.Cleanup(func() { versionCache.Delete("opencode") })
	for _, c := range []struct {
		version string
		want    bool
	}{{"opencode v2.0.24\n", true}, {"opencode 1.17.0\n", false}} {
		versionCache.Store("opencode", versionAnswer{c.version, nil, time.Hour})
		env, _ := applyOpenCodeHooks(domain.ResolvedConfig{}, []string{DispatchGateEnv + "=http://127.0.0.1:1/x"}, nil)
		if got := strings.Contains(strings.Join(env, "\n"), "plugin"); got != c.want {
			t.Errorf("%s: bridge staged = %v, want %v", strings.TrimSpace(c.version), got, c.want)
		}
	}
}

// A key the user repeats is one key, so nav-pilot's policy is not outranked
// by a later copy opencode would read.
func TestWithOpenCodeConfigContentDuplicateKeys(t *testing.T) {
	env := []string{`OPENCODE_CONFIG_CONTENT={"share":"auto","permission":{"bash":"allow"},"share":"auto","permission":{"bash":"allow"}}`}
	env = withOpenCodeConfigContent(env, map[string]any{"share": "disabled", "permission": map[string]any{"bash": "ask"}})
	want := `OPENCODE_CONFIG_CONTENT={"share":"disabled","permission":{"bash":"ask"}}`
	if len(env) != 1 || env[0] != want {
		t.Errorf("env = %q\nwant  %q", env, want)
	}
}

// OPENCODE_DISABLE_PROJECT_CONFIG is the documented untrusted-repo switch;
// cplt strips any variable it is not told to pass.
func TestApplyOpenCodeHooksPassesDisableProjectConfig(t *testing.T) {
	prev := OpenCodeHookBridge
	t.Cleanup(func() { OpenCodeHookBridge = prev; versionCache.Delete("opencode") })
	OpenCodeHookBridge = nil
	for _, ver := range []string{"1.18.35\n", "opencode v2.0.24\n"} {
		versionCache.Store("opencode", versionAnswer{ver, nil, time.Hour})
		_, cplt := applyOpenCodeHooks(domain.ResolvedConfig{}, []string{"OPENCODE_DISABLE_PROJECT_CONFIG=1"}, nil)
		if !strings.Contains(strings.Join(cplt, " "), "--pass-env OPENCODE_DISABLE_PROJECT_CONFIG") {
			t.Errorf("%q: cplt = %v", ver, cplt)
		}
		if _, cplt = applyOpenCodeHooks(domain.ResolvedConfig{}, nil, nil); strings.Contains(strings.Join(cplt, " "), "OPENCODE_DISABLE_PROJECT_CONFIG") {
			t.Errorf("%q unset: cplt = %v", ver, cplt)
		}
	}
}
