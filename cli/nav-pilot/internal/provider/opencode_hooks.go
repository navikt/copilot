package provider

import (
	"bytes"
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
	"go.yaml.in/yaml/v3"
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
	// OpenCodePluginIDEnv carries the opencode 2 bridge's plugin id, fresh
	// for each launch. opencode 2 keeps the first plugin of an id and drops
	// the rest, so a fixed id lets a project plugin that claims it first
	// turn the bridge off.
	OpenCodePluginIDEnv = "NAV_PILOT_OPENCODE_PLUGIN_ID"
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
	v2 := openCodeMajor() >= 2
	// On opencode 2 the bridge also runs the dispatch gate (dispatch-gate.js
	// is an opencode 1 plugin).
	gate := v2 && slices.ContainsFunc(env, func(e string) bool { return strings.HasPrefix(e, DispatchGateEnv+"=") })
	if len(b.Post) == 0 && len(b.Pre) == 0 && !blocked && !gate {
		return env, cpltArgs
	}
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
		env, _ = telemetry.SetEnvValue(env, OpenCodePluginIDEnv, "nav-pilot-hooks-"+rand.Text())
		cpltArgs = append(cpltArgs, "--pass-env", OpenCodePluginIDEnv)
		env = withOpenCodeConfigContent(env, map[string]any{"plugins": []any{filepath.Dir(plugin)}})
	} else {
		env = withOpenCodeConfigContent(env, map[string]any{"plugin": []any{(&url.URL{Scheme: "file", Path: plugin}).String()}})
	}
	// opencode 2 has no --pure; the launch drops it (openCodeV2Args).
	if !v2 && slices.Contains(r.ExtraArgs, "--pure") {
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
//
// Key order is kept: opencode resolves permission rules in order, the last
// match winning. add's own maps are written in sorted key order; a
// *yaml.Node value is written in its own order. Output is not byte-identical
// to the earlier map-based merge: top-level keys follow the user's order then
// add's, nested maps are sorted; what opencode reads is the same.
func withOpenCodeConfigContent(env []string, add map[string]any) []string {
	cfg := &yaml.Node{Kind: yaml.MappingNode}
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, openCodeConfigContentEnv+"="); ok && v != "" {
			n, err := jsonNode([]byte(v))
			if err != nil || n.Kind != yaml.MappingNode {
				fmt.Fprintf(os.Stderr, "%s %s is not a JSON object; nav-pilot replaces it for this session.\n", domain.Yellow("⚠"), openCodeConfigContentEnv)
				n = &yaml.Node{Kind: yaml.MappingNode}
			}
			cfg = n
		}
	}
	for _, k := range slices.Sorted(maps.Keys(add)) {
		a := toNode(add[k])
		have := nodeGet(cfg, k)
		if a.Kind == yaml.SequenceNode {
			list := &yaml.Node{Kind: yaml.SequenceNode}
			switch {
			case have != nil && have.Kind == yaml.SequenceNode:
				list = have
			case have != nil && have.Kind == yaml.ScalarNode && have.Tag == "!!str":
				list.Content = []*yaml.Node{have}
			}
			for _, item := range a.Content {
				if !slices.ContainsFunc(list.Content, func(h *yaml.Node) bool { return string(nodeJSON(h)) == string(nodeJSON(item)) }) {
					list.Content = append(list.Content, item)
				}
			}
			nodeSet(cfg, k, list)
			continue
		}
		nodeSet(cfg, k, mergeNode(have, a))
	}
	env, _ = telemetry.SetEnvValue(env, openCodeConfigContentEnv, string(nodeJSON(cfg)))
	return env
}

// toNode turns a JSON-shaped value into a node, maps in sorted key order.
func toNode(v any) *yaml.Node {
	if n, ok := v.(*yaml.Node); ok {
		return n
	}
	b, _ := json.Marshal(v)
	n, _ := jsonNode(b)
	return n
}

// jsonNode decodes JSON into a node, keeping key order.
func jsonNode(b []byte) (*yaml.Node, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	n, err := jsonNodeFrom(dec)
	if err == nil {
		if _, e := dec.Token(); e != io.EOF {
			err = fmt.Errorf("trailing data after JSON value")
		}
	}
	return n, err
}

func jsonNodeFrom(dec *json.Decoder) (*yaml.Node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		n := &yaml.Node{Kind: yaml.SequenceNode}
		if t == '{' {
			n.Kind = yaml.MappingNode
		}
		for dec.More() {
			if n.Kind == yaml.MappingNode {
				k, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := jsonNodeFrom(dec)
				if err != nil {
					return nil, err
				}
				// A duplicate key keeps its first place and its last value, as
				// JSON.parse does; two copies would let the user's outrank policy.
				nodeSet(n, k.(string), v)
				continue
			}
			v, err := jsonNodeFrom(dec)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, v)
		}
		_, err := dec.Token()
		return n, err
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t}, nil
	case json.Number:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: t.String()}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(t)}, nil
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
}

// nodeJSON writes a node as compact JSON in its own key order. Mapping keys
// become strings, as JavaScript makes them.
func nodeJSON(n *yaml.Node) []byte {
	var b bytes.Buffer
	writeNodeJSON(&b, n)
	return b.Bytes()
}

func writeNodeJSON(b *bytes.Buffer, n *yaml.Node) {
	switch n.Kind {
	case yaml.AliasNode:
		writeNodeJSON(b, n.Alias)
		return
	case yaml.MappingNode, yaml.SequenceNode:
		open, close := byte('['), byte(']')
		if n.Kind == yaml.MappingNode {
			open, close = '{', '}'
		}
		b.WriteByte(open)
		step := 1
		if n.Kind == yaml.MappingNode {
			step = 2
		}
		for i := 0; i+step <= len(n.Content); i += step {
			if i > 0 {
				b.WriteByte(',')
			}
			if step == 2 {
				k, _ := json.Marshal(n.Content[i].Value)
				b.Write(k)
				b.WriteByte(':')
			}
			writeNodeJSON(b, n.Content[i+step-1])
		}
		b.WriteByte(close)
		return
	case yaml.ScalarNode:
		if n.Tag == "!!float" || n.Tag == "!!int" {
			if json.Valid([]byte(n.Value)) {
				b.WriteString(n.Value)
				return
			}
		}
		var v any
		if n.Tag == "!!str" {
			v = n.Value
		} else if n.Decode(&v) != nil {
			v = n.Value
		}
		if out, err := json.Marshal(v); err == nil {
			b.Write(out)
			return
		}
	}
	b.WriteString("null")
}

func nodeGet(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func nodeSet(m *yaml.Node, key string, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = v
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, v)
}

func nodeDelete(m *yaml.Node, key string) {
	for i := 0; m != nil && i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = slices.Delete(m.Content, i, i+2)
			return
		}
	}
}

// mergeNode merges add over have: mappings key by key, anything else
// replaced. A key have already holds keeps its place; a new one goes last.
// have is changed in place.
func mergeNode(have, add *yaml.Node) *yaml.Node {
	if have == nil || have.Kind != yaml.MappingNode || add.Kind != yaml.MappingNode {
		return add
	}
	for i := 0; i+1 < len(add.Content); i += 2 {
		k := add.Content[i].Value
		nodeSet(have, k, mergeNode(nodeGet(have, k), add.Content[i+1]))
	}
	return have
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
