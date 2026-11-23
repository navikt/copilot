package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGuardsSwitch(t *testing.T) {
	isolatedConfig(t)
	on, off := true, false
	all := func(r ResolvedConfig) [4]bool {
		return [4]bool{r.HookLoopGuard, r.HookRedactSecrets, r.HookRedactFNR, r.HookInjectionNote}
	}
	if got := all(resolve(&Config{}, CLIOverrides{})); got != [4]bool{true, true, true, true} {
		t.Errorf("default: %v, want all on", got)
	}
	if got := all(resolve(&Config{Guards: &off}, CLIOverrides{})); got != [4]bool{} {
		t.Errorf("guards=false: %v, want all off", got)
	}
	if got := all(resolve(&Config{Guards: &off, HookRedactSecrets: &on}, CLIOverrides{})); got != [4]bool{false, true, false, false} {
		t.Errorf("guards=false, hook_redact_secrets=true: %v", got)
	}
	if got := all(resolve(&Config{HookLoopGuard: &off}, CLIOverrides{})); got != [4]bool{false, true, true, true} {
		t.Errorf("hook_loop_guard=false: %v", got)
	}
	for _, k := range userKeyNames() {
		if strings.HasPrefix(k, "hook_") && k != "hook_action_check" {
			t.Errorf("old key %s still offered", k)
		}
	}
}

func TestGuardsNudgeOnce(t *testing.T) {
	isolatedConfig(t)
	off := false
	if out := captureStderrFor(t, func() { nudgeGuardsKey(resolve(&Config{}, CLIOverrides{})) }); out != "" {
		t.Errorf("nudge without old keys: %q", out)
	}
	r := resolve(&Config{HookRedactFNR: &off}, CLIOverrides{})
	if out := captureStderrFor(t, func() { nudgeGuardsKey(r) }); !strings.Contains(out, "guards") {
		t.Errorf("first nudge missing: %q", out)
	}
	if out := captureStderrFor(t, func() { nudgeGuardsKey(r) }); out != "" {
		t.Errorf("second nudge: %q", out)
	}

	// OpenCode launches return early in syncBuiltinHooks; the nudge must still come.
	oc := resolve(&Config{HookLoopGuard: &off}, CLIOverrides{})
	oc.Client = "opencode"
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	if out := captureStderrFor(t, func() { syncBuiltinHooks(oc) }); !strings.Contains(out, "guards") {
		t.Errorf("OpenCode launch: nudge missing: %q", out)
	}
}

func TestGuardsShowsLegacyOverride(t *testing.T) {
	isolatedConfig(t)
	off := false
	got := guardsOverrides(resolve(&Config{HookRedactSecrets: &off}, CLIOverrides{}), "guards")
	want := ", except: secret masking off (hook_redact_secrets = false; remove it with: nav-pilot config unset hook_redact_secrets)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := guardsOverrides(resolve(&Config{}, CLIOverrides{}), "guards"); got != "" {
		t.Errorf("no override: %q", got)
	}
}
