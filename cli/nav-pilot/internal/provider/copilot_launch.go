package provider

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

// CopilotAgentPersona is the Copilot CLI custom-agent persona that loads
// Nav's instructions and context. This is distinct from resolved.Client,
// which selects the launcher (copilot vs opencode vs pi).
const CopilotAgentPersona = "nav-pilot"

// FindCopilotCLI returns the path to cplt or copilot CLI.
// Prefers cplt (unambiguous GitHub Copilot CLI).
// If the "copilot" binary is actually cplt (aliased), it's treated as cplt.
func FindCopilotCLI() (path, name string) {
	if p, err := exec.LookPath("cplt"); err == nil {
		return p, "cplt"
	}
	if p, err := exec.LookPath("copilot"); err == nil {
		if isCplt(p) {
			return p, "cplt"
		}
		return p, "copilot"
	}
	return "", ""
}

// isCplt checks if a binary is actually cplt (Copilot Sandbox) by inspecting
// its version output. Returns true if the binary identifies as cplt/sandbox.
func isCplt(binPath string) bool {
	out, err := exec.Command(binPath, "--version").CombinedOutput()
	if err != nil {
		return false
	}
	s := strings.ToLower(string(out))
	return strings.Contains(s, "cplt") || strings.Contains(s, "copilot-sandbox")
}

// CLIDisplayName returns a user-friendly name for the CLI binary.
func CLIDisplayName(name string) string {
	if name == "cplt" {
		return "Copilot Sandbox (cplt)"
	}
	return name
}

// copilotAgentArgs returns extra CLI flags for a given agent.
// Keep this empty by default so model/effort selection follows agent defaults
// (or explicit user overrides in the CLI), not hardcoded launch arguments.
func copilotAgentArgs(agent string) []string {
	_ = agent
	return nil
}

// BuildCopilotArgs constructs the CLI arguments for launching copilot.
//
// cplt is the sandbox wrapper: its own --agent selects WHICH agent to sandbox
// and otherwise auto-detects from PATH (per `cplt --help`). Because nav-pilot is
// on the copilot launch path here, we pin `cplt --agent copilot` so a different
// agent on PATH (e.g. opencode) is never picked, then forward the copilot
// persona + flags after the "--" separator.
//
// Note: the forwarded --agent is always the nav-pilot persona; resolved.Client
// selects the launcher and is consumed by launchClient before reaching here.
func BuildCopilotArgs(cliName string, resolved domain.ResolvedConfig) []string {
	var args []string
	args = append(args, "--agent", CopilotAgentPersona)
	args = append(args, copilotAgentArgs(CopilotAgentPersona)...)
	if resolved.Model != "" {
		args = append(args, "--model", resolved.Model)
	}
	if resolved.Mode != "" && resolved.Mode != "default" {
		args = append(args, "--mode", resolved.Mode)
	}
	if resolved.ReasoningEffort != "" {
		args = append(args, "--effort", resolved.ReasoningEffort)
	}
	if resolved.ContextTier != "" && resolved.ContextTier != "default" {
		args = append(args, "--context", resolved.ContextTier)
	}
	if resolved.AllowAllTools {
		args = append(args, "--allow-all-tools")
	}
	if !resolved.AskUser {
		args = append(args, "--no-ask-user")
	}
	if resolved.LogLevel != "" {
		args = append(args, "--log-level", resolved.LogLevel)
	}
	if cliName == "cplt" {
		return append([]string{"--agent", "copilot", "--"}, args...)
	}
	return args
}

// LaunchCopilotResolved launches the Copilot CLI with the resolved launch config.
// If user-scope instructions exist, it sets COPILOT_CUSTOM_INSTRUCTIONS_DIRS
// so cplt picks up ~/.copilot/.github/instructions/*.instructions.md.
func LaunchCopilotResolved(resolved domain.ResolvedConfig) error {
	cliPath, cliName := FindCopilotCLI()
	if cliPath == "" {
		telemetryRecorder.RecordLaunchError("copilot", "client_not_found")
		return fmt.Errorf("copilot cli not found")
	}
	if cliName == "cplt" {
		PrintCpltSandboxHint()
	}
	PrintModelAvailabilityHint(resolved.Model)
	args := BuildCopilotArgs(cliName, resolved)
	displayName := CLIDisplayName(cliName)
	fmt.Printf("Launching %s with agent %s...\n\n", domain.Bold(displayName), domain.Bold(CopilotAgentPersona))
	cmd := exec.Command(cliPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = CopilotEnv(resolved.OtelLogLevel)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "%s Could not launch %s: %v\n", domain.Yellow("⚠"), displayName, err)
		}
		telemetryRecorder.RecordLaunchError("copilot", classifyLaunchError(err))
		return err
	}
	return nil
}

// cpltSandboxHintShown tracks whether the cplt sandbox hint has been shown this session.
var cpltSandboxHintShown bool

// isTerminal returns true when stdin is a terminal (not piped/redirected).
// Used to suppress informational hints in non-interactive contexts.
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// PrintCpltSandboxHint prints a one-time tip about cplt sandbox configuration
// for users who may not know how to configure cplt outside of nav-pilot.
// Suppressed by NAV_PILOT_CPLT_HINT=0 or in non-interactive mode.
func PrintCpltSandboxHint() {
	if cpltSandboxHintShown || !isTerminal() {
		return
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("NAV_PILOT_CPLT_HINT")), "0") {
		return
	}
	cpltSandboxHintShown = true
	fmt.Printf("%s Launching via cplt (Copilot Sandbox). Sandbox settings are managed by cplt, not nav-pilot.\n", domain.Dim("ℹ"))
	fmt.Printf("  View current settings: %s\n", domain.Bold("cplt config list"))
	fmt.Printf("  Change a setting:      %s\n", domain.Bold("cplt config set <key> <value>"))
	fmt.Printf("  Suppress this hint:    set %s in your shell\n\n", domain.Bold("NAV_PILOT_CPLT_HINT=0"))
}

// PrintModelAvailabilityHint shows a note when a specific model is configured.
// Warns on provider-qualified format (e.g. github-copilot/claude-sonnet-4.5)
// and reminds users about org-level availability restrictions.
func PrintModelAvailabilityHint(model string) {
	if !isTerminal() {
		return
	}
	if model == "" || model == "auto" {
		return
	}
	if strings.Contains(model, "/") {
		shortID := strings.SplitN(model, "/", 2)[1]
		if shortID == "" {
			shortID = model
		}
		fmt.Printf("%s Model %s is in provider-qualified format. nav-pilot translates it, but the canonical form is preferred: %s\n\n",
			domain.Yellow("⚠"), domain.Bold(model), domain.Bold("nav-pilot config set model "+shortID))
		return
	}
	fmt.Printf("%s Model: %s — if unavailable in your org, run: %s\n\n",
		domain.Dim("ℹ"), domain.Bold(model), domain.Bold("nav-pilot config set model auto"))
}

// CopilotEnv returns the environment for launching cplt, injecting
// COPILOT_CUSTOM_INSTRUCTIONS_DIRS if user-scope customizations exist
// (instructions and/or agents), and OTEL_LOG_LEVEL if otelLogLevel is set.
func CopilotEnv(otelLogLevel string) []string {
	copilotDir := userCopilotDir()
	env := os.Environ()
	key := "COPILOT_CUSTOM_INSTRUCTIONS_DIRS"
	if copilotDir != "" {
		existing := telemetrypkg.LookupEnvValue(env, key)
		if existing != "" {
			alreadyPresent := false
			for _, p := range strings.Split(existing, ",") {
				if strings.TrimSpace(p) == copilotDir {
					alreadyPresent = true
					break
				}
			}
			if !alreadyPresent {
				copilotDir = existing + "," + copilotDir
			} else {
				copilotDir = existing
			}
		}

		env, _ = telemetrypkg.SetEnvValue(env, key, copilotDir)
	}

	env, _ = telemetrypkg.ApplyCopilotOTelEnv(env, cliVersion)

	if strings.TrimSpace(otelLogLevel) != "" {
		env, _ = telemetrypkg.SetEnvIfAbsent(env, "OTEL_LOG_LEVEL", strings.TrimSpace(otelLogLevel))
	}

	return env
}

// userCopilotDir returns ~/.copilot if it contains user-scope customizations
// (instructions or agents), or "" otherwise.
func userCopilotDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	base := filepath.Join(home, ".copilot")

	instructions, _ := filepath.Glob(filepath.Join(base, ".github", "instructions", "*.instructions.md"))
	if len(instructions) > 0 {
		return base
	}

	agents, _ := filepath.Glob(filepath.Join(base, "agents", "*.agent.md"))
	if len(agents) > 0 {
		return base
	}

	return ""
}
