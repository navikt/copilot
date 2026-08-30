package provider

import (
	"encoding/json"
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

	suffix := ""
	if navSummary != "" {
		suffix = fmt.Sprintf(" with Nav context (%s)", navSummary)
	}

	return launchViaCplt(cpltLaunch{
		agent:         "opencode",
		agentArgs:     OpenCodeArgs(resolved),
		env:           launchEnv,
		displayName:   "opencode",
		messageSuffix: suffix,
	})
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
