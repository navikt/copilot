package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	"github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// ConfigPathOverride can be set in tests to redirect the opencode config.
var ConfigPathOverride string

// NavContextDirOverride can be set in tests to redirect Nav context materialization.
var NavContextDirOverride string

// openCodeConfigPath returns the path to opencode's global config.
// Honors ConfigPathOverride (test seam).
func openCodeConfigPath() string {
	if ConfigPathOverride != "" {
		return ConfigPathOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", "opencode.json")
}

// openCodeNavContextDir returns the directory for Nav context materialization.
// Always uses the user-global opencode config dir (~/.config/opencode/) so Nav
// context is available across all repos regardless of whether the developer
// is inside a git repo or has run `nav-pilot export opencode` manually before.
// Honors NavContextDirOverride (test seam).
func openCodeNavContextDir() string {
	if NavContextDirOverride != "" {
		return NavContextDirOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode")
}

// EnsureOpenCodeNavContext resolves the Nav artifact source and materializes
// AGENTS.md, skills, agents, and commands into opencode's user config directory.
// Uses SyncOpenCodeArtifacts for conflict detection and state tracking.
// Returns a short summary string (e.g. "AGENTS.md + 3 skill(s)") suitable for
// the launch message, or an empty string if nothing was produced.
// Non-fatal: callers should warn and continue on error.
func EnsureOpenCodeNavContext() (string, error) {
	src, err := source.ResolveSource("", "", cliVersion)
	if err != nil {
		return "", fmt.Errorf("resolving source: %w", err)
	}
	defer src.Cleanup()

	outputDir := openCodeNavContextDir()

	if prevState, _ := artifacts.ReadOpenCodeState(outputDir); prevState != nil {
		assessment := assessStaleness(prevState.Version)
		recordFreshness("opencode", artifacts.OpenCodeScopeName, assessment)
	}

	skills, commands, agents, instrCount, conflicts, err := artifacts.SyncOpenCodeArtifacts(src.Dir, outputDir, src.Version, src.SHA)
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
	model := resolved.Model
	if model == "" {
		model = OpenCodeDefaultModel
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

// LaunchOpenCode launches the opencode CLI with the resolved config.
// Before launching, it materializes Nav context into opencode's user config directory.
func LaunchOpenCode(resolved domain.ResolvedConfig) error {
	opencodePath, err := exec.LookPath("opencode")
	if err != nil {
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

	args := OpenCodeArgs(resolved)

	if navSummary != "" {
		fmt.Printf("Launching %s with Nav context (%s)...\n\n", domain.Bold("opencode"), navSummary)
	} else {
		fmt.Printf("Launching %s...\n\n", domain.Bold("opencode"))
	}
	cmd := exec.Command(opencodePath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	var otelUpdated bool
	cmd.Env, otelUpdated = telemetry.ApplyOpenCodeOTelEnv(env, cliVersion)
	if !otelUpdated {
		cmd.Env = nil
	}

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "%s Could not launch opencode: %v\n", domain.Yellow("⚠"), err)
		}
		return err
	}
	return nil
}

// LaunchPi returns a clear error explaining that pi is not yet supported.
func LaunchPi() error {
	return fmt.Errorf("agent \"pi\" is not yet supported for launch — set a different agent with: nav-pilot config set agent copilot")
}
