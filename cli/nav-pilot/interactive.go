package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// isInteractive returns true when stdin is a terminal (not piped).
func isInteractive() bool {
	if forceNonInteractive {
		return false
	}
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// forceNonInteractive can be set in tests to prevent huh from blocking.
var forceNonInteractive bool

// navTheme returns a huh theme with radio-button-style indicators (● / blank)
// instead of the default "> " cursor.
func navTheme() *huh.Theme {
	t := huh.ThemeBase()
	t.Focused.SelectSelector = lipgloss.NewStyle().SetString("● ").Foreground(lipgloss.Color("6"))
	return t
}

// isGitRepo returns true if dir contains a .git directory.
func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// cmdInteractive runs an interactive flow based on current state:
//  1. Not installed → prompt to pick and install a collection (repo or user home)
//  2. Installed but outdated → sync all detected scopes
//  3. Installed and up-to-date → launch Copilot Sandbox / copilot
//
// Safety: huh prompts are guarded by isInteractive() at each call site.
// In tests, forceNonInteractive=true causes isInteractive() to return false,
// which makes prompt-guarded functions (offerLaunchCopilot, etc.) return early.
// The run() entry point also gates cmdInteractive behind isInteractive().
func cmdInteractive() error {
	// Check user-scope state (always available regardless of git repo)
	var userScope *InstallScope
	var userState *StateFile
	if s, err := ScopeUser(); err == nil {
		userScope = s
		var readErr error
		userState, readErr = readScopedState(userScope)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "⚠  Warning: user-scope state may be corrupted: %v\n", readErr)
		}
	}

	// Check repo-scope state (only if in a git repo)
	targetDir := findGitRoot(".")
	var repoScope *InstallScope
	var repoState *StateFile
	if targetDir != "" {
		repoScope = ScopeRepo(targetDir)
		var err error
		repoState, err = readScopedState(repoScope)
		if err != nil {
			return fmt.Errorf("reading repo state: %w", err)
		}
	}

	hasRepo := repoState != nil
	hasUser := userState != nil

	if hasRepo || hasUser {
		return interactiveSyncAndLaunch(repoScope, repoState, userScope, userState)
	}

	// Fresh install requires a git repo
	if targetDir == "" {
		return fmt.Errorf("not in a git repository (run from a repo to install, or use --user)")
	}

	return interactiveFreshInstall(targetDir)
}

// interactiveSyncAndLaunch handles the case where at least one scope has an install.
// Checks for staleness in all scopes and offers to sync, then launches Copilot.
func interactiveSyncAndLaunch(repoScope *InstallScope, repoState *StateFile, userScope *InstallScope, userState *StateFile) error {
	// Single-line greeting with discovered scopes
	var scopeParts []string
	if repoState != nil {
		scopeParts = append(scopeParts, fmt.Sprintf("repo: %s", repoState.Collection))
	}
	if userState != nil {
		scopeParts = append(scopeParts, fmt.Sprintf("user: %s", userState.Collection))
	}
	fmt.Printf("%s  %s\n", bold("🧭 nav-pilot"), dim(strings.Join(scopeParts, "  ·  ")))

	type staleScope struct {
		scope  *InstallScope
		state  *StateFile
		latest string
	}
	var stale []staleScope
	var allAgents []string

	if repoState != nil {
		if latest := checkStaleness(repoState.Version); latest != "" {
			stale = append(stale, staleScope{repoScope, repoState, latest})
		}
		allAgents = append(allAgents, installedAgents(repoState)...)
	}
	if userState != nil {
		if latest := checkStaleness(userState.Version); latest != "" {
			stale = append(stale, staleScope{userScope, userState, latest})
		}
		allAgents = append(allAgents, installedAgents(userState)...)
	}
	allAgents = uniqueStrings(allAgents)

	if len(stale) > 0 {
		for _, s := range stale {
			fmt.Printf("%s Update available for %s (%s): %s → %s\n",
				yellow("⚠"), bold(s.state.Collection), s.scope.Name, s.state.Version, s.latest)
		}
		fmt.Println()

		label := "Sync now?"
		if len(stale) > 1 {
			label = "Sync all?"
		}

		var choice string
		err := huh.NewSelect[string]().
			Title(label).
			Options(
				huh.NewOption("Yes", "yes"),
				huh.NewOption("No", "no"),
			).
			Value(&choice).
			WithTheme(navTheme()).
			Run()
		if err != nil || choice != "yes" {
			return nil
		}

		for _, s := range stale {
			fmt.Println()
			fmt.Printf("%s Syncing %s scope...\n", dim("→"), s.scope.Name)
			if err := cmdSync(s.scope, "", "", true, false); err != nil {
				fmt.Fprintf(os.Stderr, "%s Sync failed for %s scope: %v\n", yellow("⚠"), s.scope.Name, err)
			}
		}
	}

	offerLaunchCopilotWithAgents(allAgents)
	return nil
}

// interactiveFreshInstall handles the case where no install exists.
// Prompts for collection and install scope.
func interactiveFreshInstall(targetDir string) error {
	fmt.Println(bold("nav-pilot") + dim(" — Nav's Copilot toolkit"))
	fmt.Println()
	fmt.Println(dim("Resolving source..."))

	src, err := resolveSource("", "")
	if err != nil {
		return err
	}
	defer src.Cleanup()

	names, err := listCollectionDirs(src.Dir)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("no collections found")
	}

	// Build collection options
	var options []huh.Option[string]
	for _, name := range names {
		m, err := loadManifest(src.Dir, name)
		if err != nil {
			continue
		}
		total := len(m.Agents) + len(m.Skills) + len(m.Instructions) + len(m.Prompts)
		label := fmt.Sprintf("%-20s %s (%d items)", name, m.Description, total)
		options = append(options, huh.NewOption(label, name))
	}

	if len(options) == 0 {
		return fmt.Errorf("no valid collections found")
	}

	// Select collection
	var selected string
	err = huh.NewSelect[string]().
		Title("Choose collection").
		Options(options...).
		Value(&selected).
		WithTheme(navTheme()).
		Run()
	if err != nil {
		return nil // user cancelled (Ctrl+C / Esc)
	}

	// Show preview
	m, err := loadManifest(src.Dir, selected)
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Printf("%s %s — %s\n", dim("→"), bold(selected), m.Description)
	parts := []string{}
	if len(m.Agents) > 0 {
		parts = append(parts, fmt.Sprintf("%d agents", len(m.Agents)))
	}
	if len(m.Skills) > 0 {
		parts = append(parts, fmt.Sprintf("%d skills", len(m.Skills)))
	}
	if len(m.Instructions) > 0 {
		parts = append(parts, fmt.Sprintf("%d instructions", len(m.Instructions)))
	}
	if len(m.Prompts) > 0 {
		parts = append(parts, fmt.Sprintf("%d prompts", len(m.Prompts)))
	}
	fmt.Printf("  %s\n", dim(strings.Join(parts, ", ")))
	fmt.Println()

	// Select install scope
	scope, err := promptInstallScope(targetDir)
	if err != nil {
		return err
	}
	if scope == nil {
		fmt.Println(dim("Cancelled."))
		return nil
	}

	// Confirm install
	var installChoice string
	err = huh.NewSelect[string]().
		Title(fmt.Sprintf("Install %s to %s?", selected, scope.Label())).
		Options(
			huh.NewOption("Yes", "yes"),
			huh.NewOption("No", "no"),
		).
		Value(&installChoice).
		WithTheme(navTheme()).
		Run()
	if err != nil || installChoice != "yes" {
		fmt.Println(dim("Cancelled."))
		return nil
	}

	// Install
	fmt.Println()
	if err := cmdInstall(selected, scope, "", "", false, false); err != nil {
		return err
	}

	offerLaunchCopilot()
	return nil
}

// promptInstallScope asks the user where to install: repo or user home.
// Returns nil if the user cancels.
func promptInstallScope(targetDir string) (*InstallScope, error) {
	var choice string
	err := huh.NewSelect[string]().
		Title("Where to install?").
		Options(
			huh.NewOption("This repo (.github/) — full collection", "repo"),
			huh.NewOption("User home (~/.copilot/) — agents & skills only, works across all repos", "user"),
		).
		Value(&choice).
		WithTheme(navTheme()).
		Run()
	if err != nil {
		return nil, nil // user cancelled
	}

	switch choice {
	case "repo":
		return ScopeRepo(targetDir), nil
	case "user":
		return ScopeUser()
	default:
		return nil, fmt.Errorf("invalid selection: %q", choice)
	}
}

// uniqueStrings returns a sorted slice with duplicates removed.
func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	sort.Strings(result)
	return result
}

// installedAgents extracts agent names from the state file's installed files.
// Agent files follow the pattern .github/agents/<name>.agent.md.
func installedAgents(state *StateFile) []string {
	var agents []string
	for _, f := range state.Files {
		base := filepath.Base(f.Path)
		if !strings.HasSuffix(base, ".agent.md") {
			continue
		}
		dir := filepath.Dir(f.Path)
		if dir == filepath.Join(".github", "agents") || dir == "agents" {
			name := strings.TrimSuffix(base, ".agent.md")
			agents = append(agents, name)
		}
	}
	sort.Strings(agents)
	return agents
}

// findCopilotCLI returns the path to cplt or copilot CLI.
// Prefers cplt (unambiguous GitHub Copilot CLI).
func findCopilotCLI() (path, name string) {
	if p, err := exec.LookPath("cplt"); err == nil {
		return p, "cplt"
	}
	if p, err := exec.LookPath("copilot"); err == nil {
		return p, "copilot"
	}
	return "", ""
}

// cliDisplayName returns a user-friendly name for the CLI binary.
func cliDisplayName(name string) string {
	if name == "cplt" {
		return "Copilot Sandbox (cplt)"
	}
	return name
}

// launchCopilotWithAgent launches the Copilot CLI with an optional --agent flag.
func launchCopilotWithAgent(agent string) {
	cliPath, cliName := findCopilotCLI()
	if cliPath == "" {
		return
	}

	args := []string{}
	if agent != "" {
		if cliName == "cplt" {
			// cplt requires "--" to forward flags to the underlying Copilot CLI
			args = append(args, "--", "--agent", agent)
		} else {
			// copilot CLI accepts --agent directly
			args = append(args, "--agent", agent)
		}
	}

	displayName := cliDisplayName(cliName)
	if agent != "" {
		fmt.Printf("Launching %s with agent %s...\n\n", bold(displayName), bold(agent))
	} else {
		fmt.Printf("Launching %s...\n\n", bold(displayName))
	}
	cmd := exec.Command(cliPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not launch %s: %v\n", yellow("⚠"), displayName, err)
	}
}

// offerLaunchCopilot prompts the user to launch the Copilot CLI after install.
func offerLaunchCopilot() {
	cliPath, cliName := findCopilotCLI()
	if cliPath == "" || !isInteractive() {
		return
	}

	fmt.Println()
	var choice string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Launch %s now?", cliDisplayName(cliName))).
		Options(
			huh.NewOption("Yes", "yes"),
			huh.NewOption("No", "no"),
		).
		Value(&choice).
		WithTheme(navTheme()).
		Run()
	if err != nil || choice != "yes" {
		return
	}
	fmt.Println()
	launchCopilotWithAgent("nav-pilot")
}

// offerLaunchCopilotWithAgents prompts the user to launch the Copilot CLI
// with the nav-pilot agent if it's among the installed agents.
func offerLaunchCopilotWithAgents(agents []string) {
	cliPath, cliName := findCopilotCLI()
	if cliPath == "" || !isInteractive() {
		return
	}

	fmt.Println()
	var choice string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Launch %s now?", cliDisplayName(cliName))).
		Options(
			huh.NewOption("Yes", "yes"),
			huh.NewOption("No", "no"),
		).
		Value(&choice).
		WithTheme(navTheme()).
		Run()
	if err != nil || choice != "yes" {
		return
	}

	// Launch with nav-pilot agent if installed, otherwise plain launch
	agent := ""
	for _, a := range agents {
		if a == "nav-pilot" {
			agent = "nav-pilot"
			break
		}
	}

	fmt.Println()
	launchCopilotWithAgent(agent)
}
