package provider

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// The hooks Copilot CLI runs for nav-pilot, run in OpenCode as well (#1025).
//
// One plugin, hooks-bridge.js, runs the same commands with the same payloads:
// `nav-pilot hook loop-guard` and `nav-pilot hook redact` after every tool
// call, and the gates nav-pilot installed before one. There is no second
// implementation of any hook; see docs/opencode-hooks.md.
//
// The plugin is not written into the user's opencode config. It lives in a
// directory of nav-pilot's own and is named for one launch in
// OPENCODE_CONFIG_CONTENT, which opencode merges after the user's and the
// project's config (#500). A plain `opencode` never loads it.

//go:embed hooks-bridge.js
var hooksBridgePlugin []byte

const (
	// OpenCodeHooksEnv carries the hooks, as JSON, to the plugin.
	OpenCodeHooksEnv = "NAV_PILOT_OPENCODE_HOOKS"
	// HookStateDirEnv tells `nav-pilot hook` where to keep the loop guard's
	// run and the spooled telemetry, in place of Copilot's session directory.
	HookStateDirEnv = "NAV_PILOT_HOOK_STATE_DIR"
	// openCodeConfigContentEnv is opencode's per-process config.
	openCodeConfigContentEnv = "OPENCODE_CONFIG_CONTENT"
)

// BridgeHook is one hook as the plugin runs it. A post hook is an argv (the
// nav-pilot binary), a pre hook a shell command (the installed gate's entry,
// as Copilot runs it).
type BridgeHook struct {
	Name       string   `json:"name"`
	Argv       []string `json:"argv,omitempty"`
	Command    string   `json:"command,omitempty"`
	Matcher    string   `json:"matcher,omitempty"`
	Timeout    int      `json:"timeout,omitempty"`
	FailClosed bool     `json:"failClosed,omitempty"`
	SkipLocal  bool     `json:"skipLocal,omitempty"`
}

// HookBridge is what one launch hands the plugin. ReadDirs are directories the
// hooks' scripts live in outside the project, which the sandbox has to let the
// session read.
type HookBridge struct {
	Post          []BridgeHook `json:"post,omitempty"`
	Pre           []BridgeHook `json:"pre,omitempty"`
	LocalProvider string       `json:"localProvider,omitempty"`
	ReadDirs      []string     `json:"-"`
}

// OpenCodeHookBridge builds the hooks for a launch. The cli package sets it,
// since the hook settings and the installed gates are its business. Nil means
// no hooks.
var OpenCodeHookBridge func(domain.ResolvedConfig) HookBridge

// navPilotDataDir is nav-pilot's directory for files a sandboxed client reads
// or writes. Not ~/.nav-pilot: cplt denies that to everything it sandboxes.
func navPilotDataDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "nav-pilot")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "nav-pilot")
	}
	return filepath.Join(home, ".local", "share", "nav-pilot")
}

func openCodePluginDir() string { return filepath.Join(navPilotDataDir(), "opencode-plugin") }

// OpenCodeHookStateDir is where the OpenCode bridge's hooks keep their state
// and spool their telemetry.
func OpenCodeHookStateDir() string { return filepath.Join(navPilotDataDir(), "opencode-hook-state") }

// applyOpenCodeHooks wires the bridge into one launch: the plugin file, its
// place in OPENCODE_CONFIG_CONTENT, and what cplt has to let through. With no
// hooks it changes nothing, so the launch is what it was before the bridge.
//
// A failure to write the plugin is a warning and the launch goes on, as a
// failed Copilot hook write is: the session is not held hostage to it.
func applyOpenCodeHooks(r domain.ResolvedConfig, env []string, cpltArgs []string) ([]string, []string) {
	if OpenCodeHookBridge == nil {
		return env, cpltArgs
	}
	b := OpenCodeHookBridge(r)
	if len(b.Post) == 0 && len(b.Pre) == 0 {
		return env, cpltArgs
	}
	plugin, err := writeHooksBridgePlugin()
	if err == nil {
		err = os.MkdirAll(OpenCodeHookStateDir(), 0o700)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s nav-pilot's hooks (redaction, loop guard, gates) will not run in this session: %v\n", domain.Yellow("⚠"), err)
		return env, cpltArgs
	}
	cfg, _ := json.Marshal(b)
	env, _ = telemetry.SetEnvValue(env, OpenCodeHooksEnv, string(cfg))
	env, _ = telemetry.SetEnvValue(env, HookStateDirEnv, OpenCodeHookStateDir())
	env = withOpenCodeConfigContent(env, map[string]any{"plugin": []any{(&url.URL{Scheme: "file", Path: plugin}).String()}})
	if slices.Contains(r.ExtraArgs, "--pure") {
		fmt.Fprintf(os.Stderr, "%s nav-pilot's hooks (redaction, loop guard, gates) do not run with --pure: opencode loads no plugins then.\n", domain.Yellow("⚠"))
	}
	cpltArgs = append(cpltArgs,
		"--allow-read", filepath.Dir(plugin),
		"--allow-write", OpenCodeHookStateDir(),
		"--pass-env", OpenCodeHooksEnv,
		"--pass-env", HookStateDirEnv,
		"--pass-env", openCodeConfigContentEnv)
	for _, d := range b.ReadDirs {
		cpltArgs = append(cpltArgs, "--allow-read", d)
	}
	return env, cpltArgs
}

// writeHooksBridgePlugin writes the plugin, only when it differs, so two
// launches at once never truncate the file under each other's plugin scan.
func writeHooksBridgePlugin() (string, error) {
	path := filepath.Join(openCodePluginDir(), "nav-pilot-hooks.js")
	if cur, err := os.ReadFile(path); err == nil && bytes.Equal(cur, hooksBridgePlugin) {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return path, writeConfigAtomically(path, hooksBridgePlugin)
}

// withOpenCodeConfigContent merges add into OPENCODE_CONFIG_CONTENT. A value
// the user set is kept and added to; a list (plugin) is appended to. One the
// user set that is not a JSON object is replaced, with a warning, because
// what nav-pilot puts there is policy it does not skip.
func withOpenCodeConfigContent(env []string, add map[string]any) []string {
	cfg := map[string]any{}
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, openCodeConfigContentEnv+"="); ok && v != "" {
			if err := json.Unmarshal([]byte(v), &cfg); err != nil || cfg == nil {
				fmt.Fprintf(os.Stderr, "%s %s is not a JSON object; nav-pilot replaces it for this session.\n", domain.Yellow("⚠"), openCodeConfigContentEnv)
				cfg = map[string]any{}
			}
		}
	}
	for k, v := range add {
		if list, ok := v.([]any); ok {
			have, _ := cfg[k].([]any)
			for _, item := range list {
				if !slices.Contains(have, item) {
					have = append(have, item)
				}
			}
			cfg[k] = have
			continue
		}
		cfg[k] = v
	}
	out, _ := json.Marshal(cfg)
	env, _ = telemetry.SetEnvValue(env, openCodeConfigContentEnv, string(out))
	return env
}
