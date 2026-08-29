package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// ConfigPathOverride can be set in tests to redirect the opencode config.
var ConfigPathOverride string

// NavContextDirOverride can be set in tests to redirect Nav context materialization.
var NavContextDirOverride string

// openCodeConfigPath returns the path to opencode's global config.
// Honors ConfigPathOverride (test seam).
// Falls back to os.TempDir() when the home directory cannot be resolved so the
// returned path is always absolute.
func openCodeConfigPath() string {
	if ConfigPathOverride != "" {
		return ConfigPathOverride
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "nav-pilot", ".config", "opencode", "opencode.json")
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// openCodeNavContextDir returns the directory for Nav context materialization.
// Always uses the user-global opencode config dir (~/.config/opencode/) so Nav
// context is available across all repos regardless of whether the developer
// is inside a git repo or has run `nav-pilot export opencode` manually before.
// Honors NavContextDirOverride (test seam).
// Falls back to os.TempDir() when the home directory cannot be resolved so the
// returned path is always absolute.
func openCodeNavContextDir() string {
	if NavContextDirOverride != "" {
		return NavContextDirOverride
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "nav-pilot", ".config", "opencode")
	}
	return filepath.Join(home, ".config", "opencode")
}

// EnsureOpenCodeNavContext resolves the Nav artifact source and materializes
// AGENTS.md, skills, agents, and commands into opencode's user config directory.
// Uses SyncOpenCodeArtifacts for conflict detection and state tracking.
// Returns a short summary string (e.g. "AGENTS.md + 3 skill(s)") suitable for
// the launch message, or an empty string if nothing was produced.
// Non-fatal: callers should warn and continue on error.
func EnsureOpenCodeNavContext() (string, error) {
	outputDir := openCodeNavContextDir()
	prevState, _ := artifacts.ReadOpenCodeState(outputDir)

	sRepo := ""
	if prevState != nil && prevState.SourceRepo != "" {
		sRepo = prevState.SourceRepo
	}

	src, err := source.ResolveSource("", sRepo, cliVersion)
	if err != nil {
		return "", fmt.Errorf("resolving source: %w", err)
	}
	defer src.Cleanup()

	if prevState != nil {
		assessment := assessStaleness(prevState.Version)
		recordFreshness("opencode", artifacts.OpenCodeScopeName, assessment)
	}

	skills, commands, agents, instrCount, conflicts, err := artifacts.SyncOpenCodeArtifacts(src.Dir, outputDir, src.Version, src.SHA, src.Repo)
	if err != nil {
		return "", err
	}

	for _, c := range conflicts {
		fmt.Fprintf(os.Stderr, "%s Nav context file modified locally, not overwriting: %s\n", domain.Yellow("⚠"), c)
	}

	summary := artifacts.ExportSummary(skills, commands, agents, instrCount)
	if summary == "nothing to export" {
		return "", nil
	}
	return summary, nil
}

// OpenCodeArgs builds the CLI arguments for launching opencode non-interactively.
// Maps resolved config fields to opencode flags; omits unset/default fields.
func OpenCodeArgs(resolved domain.ResolvedConfig) []string {
	var args []string
	// The model nav-pilot sets for the session. The flag outranks opencode's own
	// config and its recent-model list, and on `opencode run` it outranks an
	// agent's frontmatter too, because there it is the request model. In the TUI,
	// which is what nav-pilot launches, an agent that declares its own `model:`
	// uses that instead (verified against opencode 1.18.25). So the order is
	// agent specialisation, then nav-pilot's session model, then whatever the
	// client would have picked on its own.
	args = append(args, "--model", ToOpenCodeModel(resolved.Model))
	if resolved.Mode == "plan" {
		// opencode's built-in read-only planning agent. Nav context still loads
		// via AGENTS.md regardless of the active agent.
		args = append(args, "--agent", "plan")
	} else {
		// Launch the materialized Nav primary agent so the session starts with
		// Nav's persona and context (parity with the copilot client persona).
		args = append(args, "--agent", PrimaryAgent("opencode"))
	}
	if resolved.ReasoningEffort != "" {
		args = append(args, "--variant", resolved.ReasoningEffort)
	}
	if resolved.AllowAllTools {
		args = append(args, "--dangerously-skip-permissions")
	}
	if lvl := openCodeLogLevel(resolved.LogLevel); lvl != "" {
		args = append(args, "--log-level", lvl)
	}
	return args
}

// openCodeAgentArgs is what a legacy opencode launch passes the client:
// [OpenCodeArgs], with the user's pass-through arguments forwarded through the
// same rules the staged path uses (openCodeClientArgs), so `run` keeps its
// place as the first argument opencode sees.
//
// Until this existed the pass-through arguments were parsed, resolved, and then
// dropped on the floor: `nav-pilot -- run "…"` started the TUI with the request
// discarded, which is a whole non-interactive dispatch thrown away in silence.
// With none of them openCodeClientArgs returns the bind untouched, so every
// launch that has ever worked is byte-identical (golden_launch_test.go).
func openCodeAgentArgs(resolved domain.ResolvedConfig) []string {
	return openCodeClientArgs(OpenCodeArgs(resolved), resolved.ExtraArgs)
}

// OpenCodeUnsupportedConfigWarnings returns informational warning strings for
// config fields that are explicitly set to a non-default value but have no
// opencode equivalent.
func OpenCodeUnsupportedConfigWarnings(r domain.ResolvedConfig) []string {
	var w []string
	if r.Mode == "autopilot" {
		w = append(w, `mode "autopilot" has no opencode equivalent — running with opencode defaults (use allow_all_tools = true to skip confirmations)`)
	}
	if r.ContextTier != "" {
		w = append(w, fmt.Sprintf("context_tier %q has no opencode equivalent — ignored", r.ContextTier))
	}
	if !r.AskUser {
		w = append(w, "ask_user = false has no opencode equivalent — ignored")
	}
	return w
}

// openCodeLogLevel translates a nav-pilot log level to opencode's accepted set
// (DEBUG, INFO, WARN, ERROR).
func openCodeLogLevel(level string) string {
	switch level {
	case "debug", "all":
		return "DEBUG"
	case "info":
		return "INFO"
	case "warning":
		return "WARN"
	case "error":
		return "ERROR"
	default:
		return ""
	}
}

// EnsureOpenCodeOTelConfig reads ~/.config/opencode/opencode.json (or creates it),
// sets experimental.openTelemetry=true without clobbering other keys, and writes back.
func EnsureOpenCodeOTelConfig() error {
	path := openCodeConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating opencode config dir: %w", err)
	}

	var cfg map[string]any

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading opencode config: %w", err)
		}
		cfg = map[string]any{
			"$schema":    "https://opencode.ai/config.json",
			"autoupdate": "notify",
			"share":      "disabled",
			"logLevel":   "INFO",
		}
	} else {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("opencode config is not valid JSON (%s): %w", path, err)
		}
		// A file holding the literal `null` parses without error and leaves cfg
		// nil, and assigning into a nil map panics. Erroring for the same reason
		// unparseable content does: the file is the developer's, and replacing it
		// with a fresh object loses whatever they meant by it.
		if cfg == nil {
			return fmt.Errorf("opencode config is not a JSON object (%s): remove or fix the file", path)
		}
	}

	experimental, _ := cfg["experimental"].(map[string]any)
	if experimental == nil {
		experimental = make(map[string]any)
	}
	if v, ok := experimental["openTelemetry"]; ok && v == true {
		return nil
	}
	experimental["openTelemetry"] = true
	cfg["experimental"] = experimental

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling opencode config: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("writing opencode config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("setting opencode config permissions: %w", err)
	}
	return nil
}

// LocalProviderID is the opencode provider id the local server is registered
// under. Shared with the generator in navikt/mlx-workspace, whose opencode-init
// task writes the same block for its benchmark workspaces.
const LocalProviderID = "mlx"

// EnsureOpenCodeLocalProvider registers the local server as an opencode
// provider, so `--model mlx/<id>` reaches this machine instead of GitHub.
//
// It is a config write rather than an environment variable because opencode
// picks its backend from the provider block and has no base-URL variable to
// override. It merges, like EnsureOpenCodeOTelConfig above and for the same
// reason: the file is the developer's, not nav-pilot's.
//
// The three limits are the manifest's, not this package's. They were measured:
// a context declared lower than the model's real one keeps each auto-compaction
// small, and the chunk timeout is the gap opencode tolerates between streamed
// tokens — at 96k tokens we measured a single token taking three and a half
// minutes, and the default timeout drops the connection mid-generation and
// returns an empty response the client does not report as a failure.
func EnsureOpenCodeLocalProvider(m local.Model) error {
	path := openCodeConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating opencode config dir: %w", err)
	}
	cfg := map[string]any{"$schema": "https://opencode.ai/config.json"}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("opencode config is not valid JSON (%s): %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading opencode config: %w", err)
	}

	providers, _ := cfg["provider"].(map[string]any)
	if providers == nil {
		providers = make(map[string]any)
	}
	providers[LocalProviderID] = map[string]any{
		"npm":  "@ai-sdk/openai-compatible",
		"name": "Local (nav-pilot)",
		"options": map[string]any{
			// The loop guard, not the server: every completion has to pass
			// through the thing that can stop a runaway loop.
			"baseURL":      local.GuardURL() + "/v1",
			"apiKey":       "nav-pilot",
			"chunkTimeout": localParamInt(m, "MLX_OPENCODE_CHUNK_TIMEOUT", 600000),
			"timeout":      false,
		},
		"models": map[string]any{
			m.Model: map[string]any{
				"limit": map[string]any{
					"context": localParamInt(m, "MLX_OPENCODE_CONTEXT", 32768),
					"output":  localParamInt(m, "MLX_OPENCODE_OUTPUT", 8192),
				},
			},
		},
	}
	cfg["provider"] = providers

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling opencode config: %w", err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing opencode config: %w", err)
	}
	return os.Chmod(path, 0o600)
}

// RemoveOpenCodeLocalProvider takes the local provider block back out of
// opencode's config, which is what `alpha local off` owes the developer.
//
// Turning dispatch off in nav-pilot's own config only stops nav-pilot from
// choosing the model. The block [EnsureOpenCodeLocalProvider] wrote stays in a
// file opencode reads on its own, so a developer running opencode directly
// could still pick the model and reach the guard's port — which after `off` is
// whatever happens to be listening there. `start` writes the block back.
//
// A config with no local block, and a config that is not there at all, are both
// nothing to do rather than errors: off must work on a machine where opencode
// was never configured.
func RemoveOpenCodeLocalProvider() error {
	path := openCodeConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading opencode config: %w", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("opencode config is not valid JSON (%s): %w", path, err)
	}
	providers, _ := cfg["provider"].(map[string]any)
	if _, found := providers[LocalProviderID]; !found {
		return nil
	}
	delete(providers, LocalProviderID)
	// An empty "provider": {} left behind is nav-pilot's litter in someone
	// else's file, so it goes too — but only when nav-pilot emptied it.
	if len(providers) == 0 {
		delete(cfg, "provider")
	} else {
		cfg["provider"] = providers
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling opencode config: %w", err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o600); err != nil {
		return fmt.Errorf("writing opencode config: %w", err)
	}
	return os.Chmod(path, 0o600)
}

// localParamInt reads one MLX_ param as an integer, falling back to a
// conservative default. A malformed value is not fatal: the manifest's job is
// to tune this, and a bad number should cost tuning, not the session.
func localParamInt(m local.Model, key string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(m.Params[key])); err == nil && n > 0 {
		return n
	}
	return fallback
}

// LaunchOpenCode launches opencode inside the cplt sandbox with the resolved config.
// Before launching, it materializes Nav context into opencode's user config directory.
// cplt sandboxes the opencode binary, so opencode must also be installed on PATH.
func LaunchOpenCode(resolved domain.ResolvedConfig) error {
	if _, err := exec.LookPath("opencode"); err != nil {
		return fmt.Errorf("opencode not found in PATH — install it first: https://opencode.ai")
	}

	env := os.Environ()
	if telemetry.CopilotOTelEndpointConfigured(env) {
		if err := EnsureOpenCodeOTelConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "%s Warning: could not configure opencode OTel: %v\n", domain.Yellow("⚠"), err)
		}
	}

	navSummary, ctxErr := EnsureOpenCodeNavContext()
	if ctxErr != nil {
		fmt.Fprintf(os.Stderr, "%s Warning: could not materialize Nav context for opencode: %v\n", domain.Yellow("⚠"), ctxErr)
	}

	for _, msg := range OpenCodeUnsupportedConfigWarnings(resolved) {
		fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), msg)
	}

	launchEnv, _ := telemetry.ApplyOpenCodeOTelEnv(env, cliVersion)

	guard, err := startLoopGuard(resolved.Model)
	if err != nil {
		return err
	}
	if guard != nil {
		defer guard.Close()
		fmt.Fprintf(os.Stderr, "%s Local model: nav-pilot ends a turn after %d identical tool calls in a row.\n",
			domain.Dim("ℹ"), local.LoopGuardRepeat())
	}

	suffix := ""
	if navSummary != "" {
		suffix = fmt.Sprintf(" with Nav context (%s)", navSummary)
	}

	return launchViaCplt(cpltLaunch{
		agent:         "opencode",
		agentArgs:     openCodeAgentArgs(resolved),
		env:           launchEnv,
		displayName:   "opencode",
		messageSuffix: suffix,
	})
}

// startLoopGuard puts nav-pilot's loop guard between the client and the local
// server for the length of the session, and returns (nil, nil) for a hosted
// launch — which is every launch for the ~650 developers who never turn local
// on. launchViaCplt blocks until the client exits, so the guard lives exactly
// as long as the dispatch it guards and needs no daemon.
//
// The client reaches it by address, not by environment: `nav-pilot alpha local
// start` writes an opencode provider pointing at the guard's fixed port,
// because opencode selects a backend through its provider config and has no
// base-URL variable to override.
//
// The gate is a function rather than an `if` at the call site so a test can
// hold it: with local disabled nothing here listens and nothing here writes,
// pinned by TestHostedLaunchStartsNoLoopGuard, and moving the guard out from
// behind the gate now fails a test instead of nothing.
//
// [local.EnsureOwnServer] first, and the launch is refused when it fails. The
// guard forwards to a fixed 127.0.0.1:8080, so a recorded server that died
// hours ago and left the port to whatever bound it next would have every prompt
// of the session proxied to a stranger, with nothing on screen to say so.
// Server.Start refuses a port nav-pilot does not own; this is the same rule
// where the prompts actually flow.
func startLoopGuard(model string) (*local.Guard, error) {
	if !local.IsLocal(model) {
		return nil, nil
	}
	if err := local.EnsureOwnServer(); err != nil {
		return nil, err
	}
	return local.StartGuard(local.ServerURL())
}

// LaunchPi launches pi inside the cplt sandbox. pi must also be installed on
// PATH (cplt sandboxes the pi binary). Nav-pilot config is not forwarded: pi
// uses its own defaults, and PiUnsupportedConfigWarnings says which settings
// were dropped. Nav context is still available via AGENTS.md in the project
// root. Pass-through arguments after "--" are forwarded, as they are for every
// other client: without them `nav-pilot --client pi -- run "..."` started pi
// with no request at all. cplt is required; if it is absent, launchViaCplt
// fails with guidance.
func LaunchPi(resolved domain.ResolvedConfig) error {
	if _, err := exec.LookPath("pi"); err != nil {
		return fmt.Errorf("pi not found in PATH — install it first, or set a different client with: nav-pilot config set client copilot")
	}

	for _, msg := range PiUnsupportedConfigWarnings(resolved) {
		fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), msg)
	}

	return launchViaCplt(cpltLaunch{
		agent:       "pi",
		displayName: "pi",
		agentArgs:   resolved.ExtraArgs,
	})
}

// clientForwardsModel reports whether launching a client puts the resolved
// model on its command line. Only pi does not: [LaunchPi] passes no nav-pilot
// config at all, so a launch notice naming a model for pi would contradict the
// warning [PiUnsupportedConfigWarnings] prints one line later, and would name a
// model the session does not run on.
//
// One place, next to the launch that does the dropping, so the predicate cannot
// drift from it.
func clientForwardsModel(client string) bool { return client != "pi" }

// PiUnsupportedConfigWarnings reports nav-pilot config that a pi launch drops,
// so users understand pi will use its own defaults instead.
//
// The list mirrors the settings LaunchPi does not put on the command line: the
// full launch-relevant half of domain.ResolvedConfig, not just model and mode.
// The silent ones were the dangerous ones: allow_all_tools and ask_user read
// as permission settings, and a user who turned them off had no way to see that
// pi never received them.
func PiUnsupportedConfigWarnings(resolved domain.ResolvedConfig) []string {
	var warnings []string
	add := func(setting, value string) {
		warnings = append(warnings, fmt.Sprintf("%s %s is not forwarded to pi yet — pi will use its own default", setting, value))
	}
	if resolved.Model != "" {
		add("model", fmt.Sprintf("%q", resolved.Model))
	}
	if resolved.Mode != "" && resolved.Mode != "default" {
		add("mode", fmt.Sprintf("%q", resolved.Mode))
	}
	if resolved.ReasoningEffort != "" {
		add("reasoning_effort", fmt.Sprintf("%q", resolved.ReasoningEffort))
	}
	if resolved.ContextTier != "" && resolved.ContextTier != "default" {
		add("context_tier", fmt.Sprintf("%q", resolved.ContextTier))
	}
	if resolved.AllowAllTools {
		add("allow_all_tools", "true")
	}
	if !resolved.AskUser {
		add("ask_user", "false")
	}
	if resolved.LogLevel != "" {
		add("log_level", fmt.Sprintf("%q", resolved.LogLevel))
	}
	return warnings
}
