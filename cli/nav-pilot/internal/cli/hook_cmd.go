package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
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

// hookFailClosedEnv asks a hook to say it failed rather than answer "{}".
const hookFailClosedEnv = "NAV_PILOT_HOOK_FAIL_CLOSED"

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
//
// With NAV_PILOT_HOOK_FAIL_CLOSED=1 in the environment, which only the
// OpenCode bridge sets and only for redact, a failure answers {"error": …}
// instead, so the caller can withhold the output rather than pass on text
// nothing has looked at (hooks-bridge.js). Copilot cannot act on that, so its
// entries never set it.
func runHookCommand(args []string, stdin io.Reader, stdout io.Writer) {
	out := hook.NoChange
	failed := func(why string) {
		if os.Getenv(hookFailClosedEnv) == "1" {
			b, _ := json.Marshal(map[string]string{"error": why})
			out = string(b)
		}
	}
	defer func() {
		if recover() != nil {
			out = hook.NoChange
			failed("the hook crashed")
		}
		fmt.Fprintln(stdout, out)
	}()
	if len(args) == 0 {
		failed("no hook named")
		return
	}
	data, err := io.ReadAll(io.LimitReader(stdin, maxHookPayload+1))
	if err != nil || len(data) > maxHookPayload {
		failed("the tool output could not be read, or is over 16 MiB")
		return
	}
	p, err := hook.ParsePayload(data)
	if err != nil {
		failed("the payload is not JSON")
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
	for _, dir := range []string{hookStateDir(), providerpkg.OpenCodeHookStateDir()} {
		hook.DrainSpool(dir, recordHookEvent)
	}
}

func recordHookEvent(f []string) {
	switch {
	case len(f) == 2 && f[0] == "loop_guard":
		telemetry.RecordHookLoopGuard(f[1], "cloud")
	case len(f) == 3 && f[0] == "redact":
		if n, err := strconv.ParseInt(f[2], 10, 64); err == nil && n > 0 && n < 1<<20 {
			telemetry.RecordHookRedact(f[1], n)
		}
	}
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
//
// An OpenCode launch points it elsewhere with NAV_PILOT_HOOK_STATE_DIR, a
// directory of nav-pilot's own that the launch lets the sandbox write.
func hookStateDir() string {
	if d := os.Getenv(providerpkg.HookStateDirEnv); d != "" && filepath.IsAbs(d) {
		return d
	}
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
	if r.Client == "opencode" {
		// The OpenCode bridge spools to a directory of its own, and the
		// launch hands the hooks over itself (openCodeHookBridge).
		drainSpoolAtExit = true
	}
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
	var added []string
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
		_, statErr := os.Stat(path)
		if err := source.WriteUserHook(dir, entry); err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not write the %s hook: %v\n", yellow("⚠"), h.name,
				explainSandboxedWrite(err, KindHook, dir))
		} else if os.IsNotExist(statErr) {
			added = append(added, h.name)
		}
	}
	announceBuiltinHooks(dir, added)
}

// announceBuiltinHooks says once per machine, on stderr, what the launch just
// put in ~/.copilot/hooks and how to turn it off. Hooks run on every tool
// call, so a file appearing there unannounced is worth one paragraph.
func announceBuiltinHooks(dir string, added []string) {
	if len(added) == 0 || !providerpkg.FirstTime("builtin-hooks-notice") {
		return
	}
	what := map[string]string{
		"nav-pilot-loop-guard":         "tells the agent when it repeats the same tool call. Off: nav-pilot config set hook_loop_guard false",
		"nav-pilot-redact-tool-output": "masks secrets and fødselsnumre in tool output before the model reads it. Off: nav-pilot config set hook_redact_secrets false (and hook_redact_fnr, hook_injection_note)",
	}
	fmt.Fprintf(os.Stderr, "%s nav-pilot added Copilot hooks to %s. They run after every tool call:\n", dim("ℹ"), dir)
	for _, name := range added {
		fmt.Fprintf(os.Stderr, "  %s %s\n", bold(name), what[name])
	}
	fmt.Fprintln(os.Stderr)
}

func init() { providerpkg.OpenCodeHookBridge = openCodeHookBridge }

// openCodeHookBridge is the OpenCode side of the same hooks: the built-in ones
// with the argv their Copilot entries run, and the gates nav-pilot installed
// for Copilot, read out of Copilot's own hook configs so there is one list.
//
// Repo gates run without Copilot's folder-trust check. OpenCode has none: it
// loads a repo's .opencode/plugins as code on its own, so a repo's gate
// entries give it nothing it did not have.
func openCodeHookBridge(r ResolvedConfig) providerpkg.HookBridge {
	var b providerpkg.HookBridge
	// An absolute path either way: a bare name is resolved inside the session,
	// after a chdir, where a relative PATH entry would let a file in the repo
	// answer for redaction. The PATH entry itself (a Homebrew symlink, say)
	// survives an upgrade; the running binary's path is the fallback.
	bin, err := exec.LookPath("nav-pilot")
	if err == nil {
		bin, err = filepath.Abs(bin)
	}
	if err != nil {
		if bin, err = os.Executable(); err != nil {
			bin = "nav-pilot"
		}
	}
	b.Bin = bin
	// builtinHooks lists the loop guard first, so redaction runs last and
	// nothing another hook adds reaches the model unchecked.
	for _, h := range builtinHooks {
		if !h.enabled(r) {
			continue
		}
		argv := []string{bin, "hook", h.arg}
		for _, k := range h.keys {
			argv = append(argv, k+"="+resolvedFieldStr(r, k))
		}
		// Redaction fails closed: the output is withheld if it cannot be
		// checked. The loop guard fails open, as under Copilot.
		redact := h.arg == "redact"
		b.Post = append(b.Post, providerpkg.BridgeHook{Name: h.name, Argv: argv, Timeout: 5,
			FailClosed: redact, SkipLocal: !redact})
	}
	b.LocalProvider = providerpkg.LocalProviderID

	if scope, err := ScopeUser(); err == nil {
		dir := scope.DstPath(KindHook.Dir)
		files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
		n := len(b.Pre)
		for _, f := range files {
			b.Pre = append(b.Pre, bridgeHooks(source.PreToolUseHooks(f))...)
		}
		if len(b.Pre) > n {
			b.ReadDirs = append(b.ReadDirs, dir)
		}
	}
	dir := r.ProjectDir
	if dir == "" {
		dir = "."
	}
	// A user who turned off OpenCode's project config turned off the repo's
	// own plugins, which is the argument for running its gates without a trust
	// check; so they do not run either.
	if v := strings.ToLower(os.Getenv("OPENCODE_DISABLE_PROJECT_CONFIG")); v == "true" || v == "1" {
		return b
	}
	if root := source.FindGitRoot(dir); root != "" {
		b.Pre = append(b.Pre, bridgeHooks(source.PreToolUseHooks(filepath.Join(ScopeRepo(root).DstPath(KindHook.Dir), source.RepoHooksConfig)))...)
	}
	return b
}

func bridgeHooks(entries []source.HookEntry) []providerpkg.BridgeHook {
	out := make([]providerpkg.BridgeHook, 0, len(entries))
	for _, e := range entries {
		out = append(out, providerpkg.BridgeHook{Name: e.Name, Command: e.Command, Matcher: e.Matcher, Timeout: e.Timeout})
	}
	return out
}
