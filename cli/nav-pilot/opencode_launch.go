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
	skills, commands, agents, instrCount, err := materializeOpenCode(src.Dir, outputDir)
	if err != nil {
		return "", err
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
//	model            -> --model (expects provider/model, e.g. anthropic/claude-3-5-sonnet)
//	mode=plan        -> --agent plan (opencode has no --mode; autopilot has no opencode flag)
//	reasoning_effort -> --variant (provider-specific reasoning, e.g. high/max)
//	allow_all_tools  -> --dangerously-skip-permissions
//	log_level        -> --log-level (translated to opencode's UPPERCASE set)
//	context_tier     -> (no opencode equivalent; ignored)
//	ask_user         -> (no opencode equivalent; ignored)
func openCodeArgs(resolved ResolvedConfig) []string {
	var args []string
	if resolved.Model != "" {
		args = append(args, "--model", resolved.Model)
	}
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
