package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

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
		// Fail safe, not open: a config that does not parse must not be what
		// switches redaction and the loop guard off. One line for the human;
		// the model reads stdout only.
		fmt.Fprintf(os.Stderr, "nav-pilot hook: %v; running with the defaults (redaction and loop guard on)\n", err)
		cfg = nil
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
		var rule string
		var err error
		out, rule, err = hook.LoopGuard(hookStateDir(), p, localLoopGuard(r))
		if err != nil {
			fmt.Fprintf(os.Stderr, "nav-pilot loop guard: cannot keep the run, so it will not trip: %v\n", err)
		}
		if rule != "" {
			// For the human, not the model: the threshold and how to move it
			// stay out of what the model reads (hook.LoopMessage).
			fmt.Fprintf(os.Stderr, "nav-pilot loop guard: %s tripped (local_loop_guard = %d; nav-pilot config set local_loop_guard <n> changes it)\n",
				rule, localLoopGuard(r))
			spoolHookEvents(p.SessionID, "loop_guard "+rule)
		}
	case "redact":
		// Local sessions too: the local guard only watches for loops, and a
		// secret in a local session's context still ends up in logs and in
		// whatever the session writes.
		if !p.HasResult {
			return
		}
		text, n := hook.RedactCount(p.Result, hook.RedactOptions{
			Secrets:       r.HookRedactSecrets,
			FNR:           r.HookRedactFNR,
			InjectionNote: r.HookInjectionNote,
		})
		if text != p.Result {
			out = hook.ModifiedResult(p.ResultType, text)
		}
		var lines []string
		for _, k := range []struct {
			kind string
			n    int
		}{{"secret", n.Secret}, {"fnr", n.FNR}, {"injection_note", n.InjectionNote}} {
			if k.n > 0 {
				lines = append(lines, fmt.Sprintf("redact %s %d", k.kind, k.n))
			}
		}
		spoolHookEvents(p.SessionID, lines...)
	}
}

// spoolHookEvents leaves the hook's telemetry for the next launch to send
// ([hook.Spool] says why not now). Opted out means nothing is written.
func spoolHookEvents(sessionID string, lines ...string) {
	if len(lines) == 0 || !telemetryEnabled() {
		return
	}
	_ = hook.Spool(hookStateDir(), sessionID, lines)
}

var (
	drainSpoolAtExit bool
	localTripsMu     sync.Mutex
	localTrips       = map[string]int{}
)

// countLocalTrip is local.OnLoopGuard: the guard proxy's trips, kept until
// exit. The launch process lives for the whole session and its periodic
// reader re-exports every cumulative counter every 10 s, which the dashboards'
// sum_over_time would count again each time. Recorded at exit, each lands in
// the one final export, like nav_pilot_local_dispatches.
func countLocalTrip(rule string) {
	localTripsMu.Lock()
	localTrips[rule]++
	localTripsMu.Unlock()
}

// recordHookEvents records the local guard's trips and, after a Copilot
// launch, what the hooks spooled. Main calls it just before the final export.
func recordHookEvents() {
	localTripsMu.Lock()
	for rule, n := range localTrips {
		for range n {
			telemetry.RecordHookLoopGuard(rule, "local")
		}
	}
	clear(localTrips)
	localTripsMu.Unlock()
	if drainSpoolAtExit {
		drainSpoolAtExit = false
		drainHookEvents()
	}
}

// drainHookEvents records what the hooks spooled since the last launch. With
// telemetry off the recorder is a no-op and the files are removed all the same.
func drainHookEvents() {
	hook.DrainSpool(hookStateDir(), func(f []string) {
		switch {
		case len(f) == 2 && f[0] == "loop_guard":
			telemetry.RecordHookLoopGuard(f[1], "cloud")
		case len(f) == 3 && f[0] == "redact":
			if n, err := strconv.ParseInt(f[2], 10, 64); err == nil && n > 0 && n < 1<<20 {
				telemetry.RecordHookRedact(f[1], n)
			}
		}
	})
}

// hookConfig is the config a hook runs with. The file is read on every call,
// so turning a hook off takes effect at once.
//
// cplt denies ~/.nav-pilot to everything in its sandbox, and a hook run by a
// sandboxed Copilot is inside it. There the settings the launch wrote into the
// hook's own command (settings, as key=value) stand in for the file. Any other
// read or parse error is returned, and the caller runs with the defaults.
//
// A key of the wrong type (hook_redact_secrets = "false") is left out, so its
// default applies; the hook says so on stderr rather than quietly running with
// a setting the user did not choose.
func hookConfig(settings []string) (*Config, error) {
	cfg, problems, err := loadConfig()
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "nav-pilot hook: %s (nav-pilot config validate)\n", strings.Join(problems, "; "))
	}
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
	// Drained when this launch exits, so the session's own events go too.
	// Only after a Copilot launch: the drain globs every Copilot session
	// directory, 25-50 ms with a thousand of them, which a launch does not
	// notice and `nav-pilot config get` would.
	drainSpoolAtExit = true
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
