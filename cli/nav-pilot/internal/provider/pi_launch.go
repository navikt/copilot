package provider

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/artifacts"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// PiNavContextDirOverride redirects the pi context directory in tests.
var PiNavContextDirOverride string

// piNavContextDir is where nav-pilot materializes an agentpakke for pi.
//
// Pi's own global state lives in ~/.pi/agent (PI_CODING_AGENT_DIR), and its
// package auto-load paths are project-local (.pi/settings.json, .pi/extensions).
// Neither is ours to write: the first is pi's, and writing the second would put
// nav-pilot content in the user's repository. So the artifacts go in a directory
// nav-pilot owns and are handed to pi by flag instead, which is why every path
// below is passed explicitly rather than discovered.
func piNavContextDir() string {
	if PiNavContextDirOverride != "" {
		return PiNavContextDirOverride
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "nav-pilot", "pi")
	}
	return filepath.Join(home, ".nav-pilot", "pi")
}

// EnsurePiNavContext materializes the active agentpakke for pi and returns a
// one-line summary, mirroring [EnsureOpenCodeNavContext].
func EnsurePiNavContext(ref, sourceRepo string) (string, error) {
	outputDir := piNavContextDir()

	// The caller's source wins, then whatever the last sync recorded — the same
	// order the opencode path uses. Resolving with neither picks the built-in
	// default, so a user whose config pointed at their own pakke got stock
	// nav-pilot materialized for pi.
	sRepo := sourceRepo
	if sRepo == "" {
		if prev, _ := artifacts.ReadOpenCodeState(outputDir); prev != nil && prev.SourceRepo != "" {
			sRepo = prev.SourceRepo
		}
	}

	src, err := source.ResolveSource(ref, sRepo, cliVersion)
	if err != nil {
		return "", fmt.Errorf("resolving source: %w", err)
	}
	defer src.Cleanup()

	// The syncing variant, not MaterializeOpenCode: it writes the state file
	// that ContextStatus and `nav-pilot status` read, so pi's context is a
	// managed scope like opencode's rather than an untracked copy.
	skills, _, agents, instructions, conflicts, err := artifacts.SyncOpenCodeArtifacts(
		src.Dir, "", outputDir, src.Version, src.SHA, src.Repo)
	if err != nil {
		return "", err
	}
	for _, c := range conflicts {
		fmt.Fprintf(os.Stderr, "%s pi context: %s was modified locally and was left alone\n", domain.Yellow("⚠"), c)
	}
	return fmt.Sprintf("%d skill(s), %d agent(s), %d instruction section(s)", skills, agents, instructions), nil
}

// piSkillArgs returns pi's flags for the materialized artifacts.
//
// Pi takes skills as paths (--skill, repeatable, file or directory) and a
// persona as text or a file (--append-system-prompt). It has no --agent, so the
// persona is the agent file's body rather than a name, and the name only decides
// which file is read.
func piSkillArgs(contextDir, persona string) []string {
	var args []string
	if d := filepath.Join(contextDir, "skills"); dirHasEntries(d) {
		args = append(args, "--skill", d)
	}
	if persona != "" {
		if f := filepath.Join(contextDir, "agents", persona+".md"); fileExists(f) {
			args = append(args, "--append-system-prompt", f)
		}
	}
	if f := filepath.Join(contextDir, "AGENTS.md"); fileExists(f) {
		args = append(args, "--append-system-prompt", f)
	}
	return args
}

func dirHasEntries(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// piModelArg forwards the resolved model. Pi accepts "provider/id" and a bare
// id, which is the same shape opencode wants, so ToOpenCodeModel's mapping
// applies unchanged.
func piModelArg(model string) []string {
	if model == "" {
		return nil
	}
	return []string{"--model", ToOpenCodeModel(model)}
}

// PiUnsupportedConfigWarnings names the settings a pi launch still drops.
//
// Model, skills and the persona are forwarded now, so they are gone from this
// list. What remains has no pi flag: mode, reasoning effort, context tier, and
// the tool-permission settings.
func PiUnsupportedConfigWarnings(resolved domain.ResolvedConfig) []string {
	var warnings []string
	add := func(setting, value string) {
		warnings = append(warnings, fmt.Sprintf("%s %s is not forwarded to pi — pi has no equivalent flag", setting, value))
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

// LaunchPi launches pi inside the cplt sandbox with the active agentpakke's
// artifacts passed as flags.
func LaunchPi(resolved domain.ResolvedConfig) error {
	if _, err := exec.LookPath("pi"); err != nil {
		return fmt.Errorf("pi not found in PATH — install it first, or set a different client with: nav-pilot config set client copilot")
	}

	for _, msg := range PiUnsupportedConfigWarnings(resolved) {
		fmt.Fprintf(os.Stderr, "%s %s\n", domain.Yellow("⚠"), msg)
	}

	// Materialize from the source this launch resolved. Bootstrap cannot: the
	// Provider interface hands it no config, so it falls back to whatever the
	// last sync recorded and, on a first run, to the built-in default.
	if piDeclaresTier1() {
		if _, err := EnsurePiNavContext("", resolved.Source); err != nil {
			fmt.Fprintf(os.Stderr, "%s Could not materialize the agentpakke for pi: %v\n", domain.Yellow("⚠"), err)
		}
	}

	persona := resolved.Persona
	if persona == "" {
		persona = PrimaryAgent("pi")
	}
	contextDir := piNavContextDir()

	args := piSkillArgs(contextDir, persona)
	args = append(args, piModelArg(resolved.Model)...)
	args = append(args, resolved.ExtraArgs...)

	return launchViaCplt(cpltLaunch{
		agent:       "pi",
		displayName: "pi",
		cpltArgs:    []string{"--allow-read", contextDir},
		agentArgs:   args,
	})
}

// piDeclaresTier1 reports whether the active agentpakke declares pi as a Tier 1
// client, which is what makes materializing anything for it worthwhile.
func piDeclaresTier1() bool {
	m := source.ActivePakke()
	return m != nil && m.Tier("pi") == agentpakke.TierLayout
}
