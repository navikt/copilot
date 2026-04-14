package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// isInteractive returns true when stdin is a terminal (not piped).
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// isGitRepo returns true if dir contains a .git directory.
func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// cmdInteractive runs an interactive flow based on current state:
//  1. Not installed → prompt to pick and install a collection (repo or user home)
//  2. Installed but outdated → sync all detected scopes
//  3. Installed and up-to-date → launch cplt/copilot
func cmdInteractive() error {
	reader := bufio.NewReader(os.Stdin)

	// Check user-scope state (always available regardless of git repo)
	var userScope *InstallScope
	var userState *StateFile
	if s, err := ScopeUser(); err == nil {
		userScope = s
		userState, _ = readScopedState(userScope)
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
		return interactiveSyncAndLaunch(reader, repoScope, repoState, userScope, userState)
	}

	// Fresh install requires a git repo
	if targetDir == "" {
		return fmt.Errorf("not in a git repository (run from a repo to install, or use --user)")
	}

	return interactiveFreshInstall(reader, targetDir)
}

// interactiveSyncAndLaunch handles the case where at least one scope has an install.
// Checks for staleness in all scopes and offers to sync, then launches Copilot.
func interactiveSyncAndLaunch(reader *bufio.Reader, repoScope *InstallScope, repoState *StateFile, userScope *InstallScope, userState *StateFile) error {
	// Collect all stale scopes
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
	// Deduplicate agents
	allAgents = uniqueStrings(allAgents)

	if len(stale) > 0 {
		for _, s := range stale {
			fmt.Printf("%s Update available for %s (%s): %s → %s\n",
				yellow("⚠"), bold(s.state.Collection), s.scope.Name, s.state.Version, s.latest)
		}
		fmt.Println()

		label := "Sync now?"
		if len(stale) > 1 {
			label = "Sync both?"
		}
		fmt.Printf("%s [Y/n]: ", label)
		answer, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			return nil
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || answer == "y" || answer == "yes" {
			for _, s := range stale {
				fmt.Println()
				fmt.Printf("%s Syncing %s scope...\n", dim("→"), s.scope.Name)
				if err := cmdSync(s.scope, "", "", true, false); err != nil {
					fmt.Fprintf(os.Stderr, "%s Sync failed for %s scope: %v\n", yellow("⚠"), s.scope.Name, err)
				}
			}
		}
	}

	// Up-to-date (or user skipped sync) — offer to launch with agent selection
	offerLaunchCopilotWithAgents(reader, allAgents)
	return nil
}

// interactiveFreshInstall handles the case where no install exists.
// Prompts for collection and install scope.
func interactiveFreshInstall(reader *bufio.Reader, targetDir string) error {
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

	// Display collections
	type collectionInfo struct {
		name  string
		desc  string
		total int
	}
	var collections []collectionInfo
	for _, name := range names {
		m, err := loadManifest(src.Dir, name)
		if err != nil {
			continue
		}
		total := len(m.Agents) + len(m.Skills) + len(m.Instructions) + len(m.Prompts)
		collections = append(collections, collectionInfo{name: name, desc: m.Description, total: total})
	}

	if len(collections) == 0 {
		return fmt.Errorf("no valid collections found")
	}

	fmt.Println()
	fmt.Println(bold("Available collections:"))
	fmt.Println()
	for i, c := range collections {
		fmt.Printf("  %s  %-20s %s %s\n",
			bold(fmt.Sprintf("%d.", i+1)),
			c.name,
			c.desc,
			dim(fmt.Sprintf("(%d items)", c.total)))
	}
	fmt.Println()

	// Prompt for selection
	fmt.Printf("Select collection [1-%d]: ", len(collections))
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println()
		return nil // EOF or closed stdin — exit gracefully
	}
	input = strings.TrimSpace(input)
	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > len(collections) {
		return fmt.Errorf("invalid selection: %q", input)
	}
	selected := collections[choice-1]

	// Show preview
	fmt.Println()
	m, err := loadManifest(src.Dir, selected.name)
	if err != nil {
		return err
	}
	fmt.Printf("%s %s — %s\n", dim("→"), bold(selected.name), m.Description)
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

	// Prompt for install scope
	scope, err := promptInstallScope(reader, targetDir)
	if err != nil {
		return err
	}
	if scope == nil {
		fmt.Println(dim("Cancelled."))
		return nil
	}

	// Confirm
	fmt.Printf("Install %s to %s? [Y/n]: ", bold(selected.name), bold(scope.Label()))
	confirm, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println()
		return nil
	}
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm != "" && confirm != "y" && confirm != "yes" {
		fmt.Println(dim("Cancelled."))
		return nil
	}

	// Install
	fmt.Println()
	if err := cmdInstall(selected.name, scope, "", "", false, false); err != nil {
		return err
	}

	// Offer to launch Copilot CLI
	offerLaunchCopilot(reader)
	return nil
}

// promptInstallScope asks the user where to install: repo or user home.
// Returns nil if the user cancels.
func promptInstallScope(reader *bufio.Reader, targetDir string) (*InstallScope, error) {
	fmt.Println(bold("Where to install?"))
	fmt.Println()
	fmt.Printf("  %s  This repo %s\n", bold("1."), dim("(.github/) — full collection"))
	fmt.Printf("  %s  User home %s\n", bold("2."), dim("(~/.copilot/) — agents & skills only, works across all repos"))
	fmt.Println()
	fmt.Printf("Select [1-2] (default: 1): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println()
		return nil, nil
	}
	input = strings.TrimSpace(input)

	switch input {
	case "", "1":
		return ScopeRepo(targetDir), nil
	case "2":
		scope, err := ScopeUser()
		if err != nil {
			return nil, err
		}
		return scope, nil
	default:
		return nil, fmt.Errorf("invalid selection: %q", input)
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
		// Repo scope: ".github/agents/x.agent.md"
		// User scope: "agents/x.agent.md"
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

// launchCopilotWithAgent launches the Copilot CLI with an optional --agent flag.
func launchCopilotWithAgent(agent string) {
	cliPath, cliName := findCopilotCLI()
	if cliPath == "" {
		return
	}

	// Pass --agent after "--" so cplt forwards it to the Copilot CLI
	args := []string{}
	if agent != "" {
		args = append(args, "--", "--agent", agent)
	}

	if agent != "" {
		fmt.Printf("Launching %s with agent %s...\n\n", bold(cliName), bold(agent))
	} else {
		fmt.Printf("Launching %s...\n\n", bold(cliName))
	}
	cmd := exec.Command(cliPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not launch %s: %v\n", yellow("⚠"), cliName, err)
	}
}

// promptLaunchCopilot asks the user if they want to launch the Copilot CLI.
// Returns the CLI path and name, and whether the user confirmed.
func promptLaunchCopilot(reader *bufio.Reader) (cliPath, cliName string, ok bool) {
	cliPath, cliName = findCopilotCLI()
	if cliPath == "" {
		return "", "", false
	}

	fmt.Println()
	fmt.Printf("Launch %s now? [Y/n]: ", bold(cliName))
	answer, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println()
		return "", "", false
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		return "", "", false
	}
	return cliPath, cliName, true
}

// offerLaunchCopilot prompts the user to launch the Copilot CLI after install.
// If agents are available in the collection, offers to spawn with --agent.
func offerLaunchCopilot(reader *bufio.Reader) {
	if _, _, ok := promptLaunchCopilot(reader); !ok {
		return
	}
	fmt.Println()
	launchCopilotWithAgent("nav-pilot")
}

// offerLaunchCopilotWithAgents prompts the user to launch the Copilot CLI
// with the nav-pilot agent if it's among the installed agents.
func offerLaunchCopilotWithAgents(reader *bufio.Reader, agents []string) {
	if _, _, ok := promptLaunchCopilot(reader); !ok {
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
