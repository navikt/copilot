package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// openCodeConfigPathOverride can be set in tests to redirect the opencode config.
var openCodeConfigPathOverride string

// openCodeNavContextDirOverride can be set in tests to redirect Nav context materialization.
var openCodeNavContextDirOverride string

// openCodeConfigPath returns the path to opencode's global config.
// Honors openCodeConfigPathOverride (test seam).
func openCodeConfigPath() string {
	if openCodeConfigPathOverride != "" {
		return openCodeConfigPathOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// openCodeNavContextDir returns the directory for Nav context materialization.
// Always uses the user-global opencode config dir (~/.config/opencode/) so Nav
// context is available across all repos regardless of whether the developer
// is inside a git repo or has run `nav-pilot export opencode` manually before.
// Honors openCodeNavContextDirOverride (test seam).
func openCodeNavContextDir() string {
	if openCodeNavContextDirOverride != "" {
		return openCodeNavContextDirOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode")
}

// ensureOpenCodeNavContext resolves the Nav artifact source and materializes
// AGENTS.md, skills, agents, and commands into opencode's user config directory.
// Uses syncOpenCodeArtifacts for conflict detection and state tracking.
// Returns a short summary string (e.g. "AGENTS.md + 3 skill(s)") suitable for
// the launch message, or an empty string if nothing was produced.
// Non-fatal: callers should warn and continue on error.
func ensureOpenCodeNavContext() (string, error) {
	src, err := resolveSource("", "")
	if err != nil {
		return "", fmt.Errorf("resolving source: %w", err)
	}
	defer src.Cleanup()

	outputDir := openCodeNavContextDir()

	// Read pre-sync state for staleness telemetry before updating it.
	if prevState, _ := readOpenCodeState(outputDir); prevState != nil {
		assessment := assessStaleness(prevState.Version)
		recordFreshness("opencode", openCodeScopeName, assessment)
	}

	skills, commands, agents, instrCount, conflicts, err := syncOpenCodeArtifacts(src.Dir, outputDir, src.Version, src.SHA)
	if err != nil {
		return "", err
	}

	for _, c := range conflicts {
		fmt.Fprintf(os.Stderr, "%s Nav context file modified locally, not overwriting: %s\n", yellow("⚠"), c)
	}

	summary := exportSummary(skills, commands, agents, instrCount)
	if summary == "nothing to export" {
		return "", nil
	}
	return summary, nil
}

// openCodeArgs builds the CLI arguments for launching opencode non-interactively.
// Maps resolved config fields to opencode flags; omits unset/default fields.
//
// Mapping (nav-pilot config -> opencode flag):
//
//	model            -> --model (expects provider/model, e.g. anthropic/claude-sonnet-4-5)
//	                    defaults to openCodeDefaultModel when unset
//	mode=plan        -> --agent plan (opencode has no --mode; autopilot has no opencode flag)
//	reasoning_effort -> --variant (provider-specific reasoning, e.g. high/max)
//	allow_all_tools  -> --dangerously-skip-permissions
//	log_level        -> --log-level (translated to opencode's UPPERCASE set)
//	context_tier     -> (no opencode equivalent; ignored — warns via openCodeUnsupportedConfigWarnings)
//	ask_user         -> (no opencode equivalent; ignored — warns when explicitly set to false)
func openCodeArgs(resolved ResolvedConfig) []string {
	var args []string
	// Always pass --model so opencode uses Nav's Anthropic API credentials rather
	// than its own built-in default, which may not be configured in Nav's setup.
	model := resolved.Model
	if model == "" {
		model = openCodeDefaultModel
	}
	args = append(args, "--model", model)
	if resolved.Mode == "plan" {
		args = append(args, "--agent", "plan")
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

// openCodeUnsupportedConfigWarnings returns informational warning strings for
// config fields that are explicitly set to a non-default value but have no
// opencode equivalent. Only called when launching opencode; only non-default
// values produce warnings to keep output quiet for typical configs.
//
// Callers should print each warning to stderr with a yellow ⚠ prefix.
func openCodeUnsupportedConfigWarnings(r ResolvedConfig) []string {
	var w []string
	// mode=autopilot: opencode has no autonomous non-interactive mode;
	// --dangerously-skip-permissions (from allow_all_tools) is the closest alternative.
	if r.Mode == "autopilot" {
		w = append(w, `mode "autopilot" has no opencode equivalent — running with opencode defaults (use allow_all_tools = true to skip confirmations)`)
	}
	// context_tier: opencode does not expose a context-window tier flag.
	if r.ContextTier != "" {
		w = append(w, fmt.Sprintf("context_tier %q has no opencode equivalent — ignored", r.ContextTier))
	}
	// ask_user defaults to true; warn only when explicitly set to false.
	if !r.AskUser {
		w = append(w, "ask_user = false has no opencode equivalent — ignored")
	}
	return w
}

// openCodeLogLevel translates a nav-pilot log level to opencode's accepted set
// (DEBUG, INFO, WARN, ERROR). Returns "" for levels with no opencode equivalent
// (none/default/unset), in which case the flag is omitted and opencode uses its
// own default.
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
	default: // none, default, "" — let opencode decide
		return ""
	}
}

// applyOpenCodeOTelEnv injects OTel env vars for opencode, reusing the same
// approach as applyCopilotOTelEnv. Also sets OPENCODE_CLIENT=nav-pilot.
func applyOpenCodeOTelEnv(env []string) ([]string, bool) {
	changed := false
	endpoint := copilotOTelEndpoint(env)
	if endpoint == "" {
		return env, false
	}

	var updated bool
	env, updated = setEnvValue(env, "OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)
	changed = changed || updated

	deviceID := ""
	if telemetryEnabled() {
		deviceID = copilotDeviceID()
	}
	env, updated = applyCopilotResourceAttributes(env, normalizeTelemetryDimension(version, "dev"), deviceID)
	changed = changed || updated

	env, updated = setEnvIfAbsent(env, "OPENCODE_CLIENT", "nav-pilot")
	changed = changed || updated

	return env, changed
}

// ensureOpenCodeOTelConfig reads ~/.config/opencode/opencode.json (or creates it),
// sets experimental.openTelemetry=true without clobbering other keys, and writes back.
// When creating for the first time, seeds recommended defaults.
// Idempotent: calling twice produces identical output.
func ensureOpenCodeOTelConfig() error {
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

// launchOpenCode launches the opencode CLI with the resolved config.
// Before launching, it materializes Nav context (AGENTS.md, skills, agents, commands)
// into opencode's user config directory so the developer always gets Nav context.
func launchOpenCode(resolved ResolvedConfig) error {
	opencodePath, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode not found in PATH — install it first: https://opencode.ai")
	}

	env := os.Environ()
	if copilotOTelEndpoint(env) != "" {
		if err := ensureOpenCodeOTelConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "%s Warning: could not configure opencode OTel: %v\n", yellow("⚠"), err)
		}
	}

	// Materialize Nav context (AGENTS.md, skills, agents, commands) into opencode's
	// user config dir before every launch. Non-fatal: if source is unavailable we
	// warn and still launch so the user isn't blocked.
	navSummary, ctxErr := ensureOpenCodeNavContext()
	if ctxErr != nil {
		fmt.Fprintf(os.Stderr, "%s Warning: could not materialize Nav context for opencode: %v\n", yellow("⚠"), ctxErr)
	}

	// Warn about config fields that are explicitly set but have no opencode equivalent.
	for _, msg := range openCodeUnsupportedConfigWarnings(resolved) {
		fmt.Fprintf(os.Stderr, "%s %s\n", yellow("⚠"), msg)
	}

	args := openCodeArgs(resolved)

	if navSummary != "" {
		fmt.Printf("Launching %s with Nav context (%s)...\n\n", bold("opencode"), navSummary)
	} else {
		fmt.Printf("Launching %s...\n\n", bold("opencode"))
	}
	cmd := exec.Command(opencodePath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	var otelUpdated bool
	cmd.Env, otelUpdated = applyOpenCodeOTelEnv(env)
	if !otelUpdated {
		cmd.Env = nil
	}

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "%s Could not launch opencode: %v\n", yellow("⚠"), err)
		}
		return err
	}
	return nil
}

// launchPi returns a clear error explaining that pi is not yet supported.
func launchPi() error {
	return fmt.Errorf("agent \"pi\" is not yet supported for launch — set a different agent with: nav-pilot config set agent copilot")
}
