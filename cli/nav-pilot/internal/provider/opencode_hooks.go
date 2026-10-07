package provider

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
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

// hooksBridgePluginV2 is the same bridge for opencode 2's plugin API. opencode
// 2 loads a configured plugin from a directory (its index.js), not a file.
//
//go:embed hooks-bridge-v2.js
var hooksBridgePluginV2 []byte

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
	// Bin is the nav-pilot the post hooks run, for the launch's exec check.
	Bin string `json:"-"`
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
	// The session's policy (share, autoupdate, MCP) rides in this variable
	// whether or not any hook runs, and cplt passes only what it is told to.
	cpltArgs = append(cpltArgs, "--pass-env", openCodeConfigContentEnv)
	if OpenCodeHookBridge == nil {
		return env, cpltArgs
	}
	b := OpenCodeHookBridge(r)
	blocked := slices.ContainsFunc(env, func(e string) bool { return strings.HasPrefix(e, MCPBlockedEnv+"=") })
	if len(b.Post) == 0 && len(b.Pre) == 0 && !blocked {
		return env, cpltArgs
	}
	v2 := openCodeMajor() >= 2
	plugin, err := writeHooksBridgePlugin(v2)
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
	if v2 {
		env = withOpenCodeConfigContent(env, map[string]any{"plugins": []any{filepath.Dir(plugin)}})
	} else {
		env = withOpenCodeConfigContent(env, map[string]any{"plugin": []any{(&url.URL{Scheme: "file", Path: plugin}).String()}})
	}
	if slices.Contains(r.ExtraArgs, "--pure") {
		fmt.Fprintf(os.Stderr, "%s nav-pilot's hooks (redaction, loop guard, gates) do not run with --pure: opencode loads no plugins then.\n", domain.Yellow("⚠"))
		if blocked {
			fmt.Fprintf(os.Stderr, "%s With --pure, an MCP server turned off for this session can be connected again from /mcp and used.\n", domain.Yellow("⚠"))
		}
	}
	// Only a sandboxed session is limited in where it may execute from.
	_, launcher := FindCopilotCLI()
	if dir := filepath.Dir(b.Bin); launcher == "cplt" && len(b.Post) > 0 && filepath.IsAbs(b.Bin) && !underCpltExecRoot(dir) {
		fmt.Fprintf(os.Stderr, "%s nav-pilot runs from %s, where the cplt sandbox may not let the session start it. If so, redaction withholds every tool output. Install nav-pilot under ~/.local/bin or Homebrew.\n", domain.Yellow("⚠"), dir)
	}
	cpltArgs = append(cpltArgs,
		"--allow-read", filepath.Dir(plugin),
		"--allow-write", OpenCodeHookStateDir(),
		"--pass-env", OpenCodeHooksEnv,
		"--pass-env", HookStateDirEnv,
		"--pass-env", MCPBlockedEnv)
	for _, d := range b.ReadDirs {
		cpltArgs = append(cpltArgs, "--allow-read", d)
	}
	env, checkFlags := withActionCheckServer(r, env)
	return env, append(cpltArgs, checkFlags...)
}

// writeHooksBridgePlugin writes the plugin, only when it differs, so two
// launches at once never truncate the file under each other's plugin scan.
// For opencode 2 the plugin is the index.js of a directory of its own.
func writeHooksBridgePlugin(v2 bool) (string, error) {
	path, src := filepath.Join(openCodePluginDir(), "nav-pilot-hooks.js"), hooksBridgePlugin
	if v2 {
		path, src = filepath.Join(openCodePluginDir(), "v2", "index.js"), hooksBridgePluginV2
	}
	if cur, err := os.ReadFile(path); err == nil && bytes.Equal(cur, src) {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return path, writeConfigAtomically(path, src)
}

// withOpenCodeConfigContent merges add into OPENCODE_CONFIG_CONTENT. A value
// the user set is kept and added to; a list (plugin) is appended to. A value
// that is not a JSON object is replaced, with a warning, because what
// nav-pilot puts there is policy it does not skip.
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
			have, isList := cfg[k].([]any)
			if one, isStr := cfg[k].(string); !isList && isStr {
				have = []any{one}
			}
			for _, item := range list {
				if !slices.Contains(have, item) {
					have = append(have, item)
				}
			}
			cfg[k] = have
			continue
		}
		cfg[k] = mergeJSON(cfg[k], v)
	}
	out, _ := json.Marshal(cfg)
	env, _ = telemetry.SetEnvValue(env, openCodeConfigContentEnv, string(out))
	return env
}

// underCpltExecRoot reports whether dir is under a tree cplt lets a sandboxed
// session execute from by default.
//
// ponytail: a copy of cplt's default exec roots (cplt --print-profile, macOS,
// 2026.09.24); it only decides whether to warn, so a stale list costs a
// needless warning or a missing one, never a launch.
func underCpltExecRoot(dir string) bool {
	home, _ := os.UserHomeDir()
	roots := []string{"/opt/homebrew", "/usr/local", "/usr/bin", "/bin", "/usr/sbin", "/nix"}
	// Only on Linux: on macOS /home is an autofs mount, and resolving a path
	// under it asks automountd, which can take seconds (#1276).
	if runtime.GOOS == "linux" {
		roots = append(roots, "/home/linuxbrew/.linuxbrew")
	}
	for _, r := range []string{".local/bin", "go/bin", ".cargo/bin", ".mise", ".bun", ".local/share/mise"} {
		roots = append(roots, filepath.Join(home, r))
	}
	for _, r := range roots {
		if domain.PathWithinRoot(r, dir) {
			return true
		}
	}
	return false
}

// mergeJSON merges add over have: objects key by key, anything else replaced.
func mergeJSON(have, add any) any {
	h, ok1 := have.(map[string]any)
	a, ok2 := add.(map[string]any)
	if !ok1 || !ok2 {
		return add
	}
	for k, v := range a {
		h[k] = mergeJSON(h[k], v)
	}
	return h
}
