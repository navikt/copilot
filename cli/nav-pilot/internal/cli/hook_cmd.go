package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/navikt/copilot/cli/nav-pilot/internal/hook"
	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// `nav-pilot hook <name>` is what nav-pilot's built-in Copilot CLI hooks run.
// It is not a user command and is not in the usage text: the CLI calls it with
// a payload on stdin after every tool call, and it answers on stdout.
//
// Main dispatches it before anything else runs — before telemetry, the update
// check or the local config — because it runs once per tool call and has to be
// fast, and because none of that is its business.

// maxHookPayload caps what a hook reads. A tool result past it is passed
// through untouched rather than read into memory.
const maxHookPayload = 16 << 20

// builtinHook is one hook nav-pilot writes to ~/.copilot/hooks/ itself.
type builtinHook struct {
	name    string // file and marker name
	arg     string // `nav-pilot hook <arg>`
	keys    []string
	enabled func(ResolvedConfig) bool
}

var builtinHooks = []builtinHook{
	{name: "nav-pilot-loop-guard", arg: "loop-guard", keys: []string{"hook_loop_guard", "local_loop_guard"},
		enabled: func(r ResolvedConfig) bool { return r.HookLoopGuard }},
	{name: "nav-pilot-redact-tool-output", arg: "redact", keys: []string{"hook_redact_secrets", "hook_redact_fnr", "hook_injection_note"},
		enabled: func(r ResolvedConfig) bool {
			return r.HookRedactSecrets || r.HookRedactFNR || r.HookInjectionNote
		}},
}

// runHookCommand runs one built-in hook and always exits 0 with a JSON answer.
// Every failure — an unreadable payload, a broken config, a panic — prints
// "{}", which leaves the tool result as it was: a hook that fails must never be
// what breaks a session.
func runHookCommand(args []string, stdin io.Reader, stdout io.Writer) {
	out := hook.NoChange
	defer func() {
		if recover() != nil {
			out = hook.NoChange
		}
		fmt.Fprintln(stdout, out)
	}()
	if len(args) == 0 {
		return
	}
	data, err := io.ReadAll(io.LimitReader(stdin, maxHookPayload+1))
	if err != nil || len(data) > maxHookPayload {
		return
	}
	p, err := hook.ParsePayload(data)
	if err != nil {
		return
	}
	cfg, err := hookConfig(args[1:])
	if err != nil {
		return
	}
	r := resolve(cfg, CLIOverrides{})

	switch args[0] {
	case "loop-guard":
		// A local session already has the guard in front of the model, which
		// ends the turn on the same rule. Warning here as well would give the
		// model two messages about one loop, one of them after the turn ended.
		if !r.HookLoopGuard || os.Getenv("COPILOT_PROVIDER_API_KEY") == providerpkg.LocalProviderAPIKey {
			return
		}
		var err error
		out, err = hook.LoopGuard(hookStateDir(), p, localLoopGuard(r))
		if err != nil {
			fmt.Fprintf(os.Stderr, "nav-pilot loop guard: cannot keep the run, so it will not trip: %v\n", err)
		}
	case "redact":
		// Local sessions too: the local guard only watches for loops, and a
		// secret in a local session's context still ends up in logs and in
		// whatever the session writes.
		if !p.HasResult {
			return
		}
		text, changed := hook.Redact(p.Result, hook.RedactOptions{
			Secrets:       r.HookRedactSecrets,
			FNR:           r.HookRedactFNR,
			InjectionNote: r.HookInjectionNote,
		})
		if changed {
			out = hook.ModifiedResult(p.ResultType, text)
		}
	}
}

// hookConfig is the config a hook runs with. The file is read on every call,
// so turning a hook off takes effect at once.
//
// cplt denies ~/.nav-pilot to everything in its sandbox, and a hook run by a
// sandboxed Copilot is inside it. There the settings the launch wrote into the
// hook's own command (settings, as key=value) stand in for the file. Any other
// read or parse error passes: a config that cannot be read may be the one that
// turned this hook off.
func hookConfig(settings []string) (*Config, error) {
	cfg, err := readConfig()
	if err == nil || !errors.Is(err, fs.ErrPermission) || len(settings) == 0 {
		return cfg, err
	}
	var c Config
	if _, err := toml.Decode(strings.Join(settings, "\n"), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// hookStateDir is where the loop guard keeps its run: in Copilot's own
// session directory, ~/.copilot/session-state/<sessionId>. ~/.nav-pilot is out
// of reach inside cplt; the session directory is where Copilot itself writes.
func hookStateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".copilot", "session-state")
}

// builtinHookCommand is the shell command an entry runs. nav-pilot is found
// on PATH rather than by absolute path: an upgrade moves the binary, and a hook
// pointing at the old one would fail on every tool call until the next launch.
// Without nav-pilot on PATH the entry answers "{}", no change.
//
// The hook's settings ride along as key=value arguments, for when the config
// file is out of reach ([hookConfig]). They are the values at the last launch.
func builtinHookCommand(h builtinHook, r ResolvedConfig) string {
	cmd := "nav-pilot hook " + h.arg
	for _, k := range h.keys {
		cmd += " " + k + "=" + resolvedFieldStr(r, k)
	}
	return "command -v nav-pilot >/dev/null 2>&1 && " + cmd + " || echo '{}'"
}

// syncBuiltinHooks writes each enabled built-in hook to the user's Copilot
// hooks directory and removes each disabled one, at launch. It is the whole
// install: there is nothing to run by hand, and turning a hook off in config
// takes it away at the next launch (the hook also checks the config on every
// call, so off means off at once).
//
// A write that fails is reported and the launch goes on: a missing hook costs
// a warning, not the session.
func syncBuiltinHooks(r ResolvedConfig) {
	if r.Client != "copilot" {
		return
	}
	scope, err := ScopeUser()
	if err != nil {
		return
	}
	dir := scope.DstPath(KindHook.Dir)
	for _, h := range builtinHooks {
		path := filepath.Join(dir, source.UserHookConfigName(h.name))
		if !h.enabled(r) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "%s Could not remove %s: %v\n", yellow("⚠"), path, err)
			}
			continue
		}
		entry := source.HookEntry{
			Name:    h.name,
			Command: builtinHookCommand(h, r),
			Timeout: 5,
			Event:   source.HookEventPostToolUse,
		}
		if err := source.WriteUserHook(dir, entry); err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not write the %s hook: %v\n", yellow("⚠"), h.name,
				explainSandboxedWrite(err, KindHook, dir))
		}
	}
}
