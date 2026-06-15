package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// openCodeConfigPathOverride can be set in tests to redirect the opencode config.
var openCodeConfigPathOverride string

// openCodeConfigPath returns the path to opencode's global config.
// Honors openCodeConfigPathOverride (test seam).
func openCodeConfigPath() string {
	if openCodeConfigPathOverride != "" {
		return openCodeConfigPathOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "opencode.json")
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
	return nil
}

// launchOpenCode launches the opencode CLI with the resolved config.
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

	args := openCodeArgs(resolved)

	fmt.Printf("Launching %s...\n\n", bold("opencode"))
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
		fmt.Fprintf(os.Stderr, "%s Could not launch opencode: %v\n", yellow("⚠"), err)
		return err
	}
	return nil
}

// launchPi returns a clear error explaining that pi is not yet supported.
func launchPi() error {
	return fmt.Errorf("agent \"pi\" is not yet supported for launch — set a different agent with: nav-pilot config set agent copilot")
}
