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
//  1. Not installed → prompt to pick and install a collection
//  2. Installed but outdated → prompt to sync/upgrade
//  3. Installed and up-to-date → launch cplt/copilot
func cmdInteractive() error {
	// I3: Use git root, not CWD (which could be a subdirectory)
	targetDir := findGitRoot(".")
	if targetDir == "" {
		return fmt.Errorf("not in a git repository")
	}

	// If already installed, check for updates or launch Copilot
	state, err := readState(targetDir)
	if err != nil {
		return fmt.Errorf("reading install state: %w", err)
	}
	if state != nil {
		reader := bufio.NewReader(os.Stdin)

		// Fast staleness check (cached, 2s timeout)
		if latest := checkStaleness(state.Version); latest != "" {
			fmt.Printf("%s Update available for %s: %s → %s\n",
				yellow("⚠"), bold(state.Collection), state.Version, latest)
			fmt.Println()
			fmt.Printf("Sync now? [Y/n]: ")
			answer, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println()
				return nil
			}
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer == "" || answer == "y" || answer == "yes" {
				fmt.Println()
				return cmdSync(targetDir, "", "", true, false)
			}
		}

		// Up-to-date (or user skipped sync) — offer to launch with agent selection
		agents := installedAgents(state)
		offerLaunchCopilotWithAgents(reader, agents)
		return nil
	}

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
	reader := bufio.NewReader(os.Stdin)
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

	// Confirm
	fmt.Printf("Install %s? [Y/n]: ", bold(selected.name))
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
	if err := cmdInstall(selected.name, targetDir, "", "", false, false); err != nil {
		return err
	}

	// Offer to launch Copilot CLI
	offerLaunchCopilot(reader)
	return nil
}

// installedAgents extracts agent names from the state file's installed files.
// Agent files follow the pattern .github/agents/<name>.agent.md.
func installedAgents(state *StateFile) []string {
	var agents []string
	for _, f := range state.Files {
		if strings.HasPrefix(f.Path, ".github/agents/") && strings.HasSuffix(f.Path, ".agent.md") {
			name := filepath.Base(f.Path)
			name = strings.TrimSuffix(name, ".agent.md")
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

	args := []string{}
	if agent != "" {
		args = append(args, "--agent", agent)
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

// offerLaunchCopilot prompts the user to launch the Copilot CLI after install.
// If agents are available in the collection, offers to spawn with --agent.
func offerLaunchCopilot(reader *bufio.Reader) {
	cliPath, cliName := findCopilotCLI()
	if cliPath == "" {
		return
	}

	fmt.Println()
	fmt.Printf("Launch %s now? [Y/n]: ", bold(cliName))
	answer, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println()
		return
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		return
	}

	fmt.Println()
	launchCopilotWithAgent("nav-pilot")
}

// offerLaunchCopilotWithAgents prompts the user to launch the Copilot CLI
// and lets them pick an agent from the installed collection.
func offerLaunchCopilotWithAgents(reader *bufio.Reader, agents []string) {
	cliPath, cliName := findCopilotCLI()
	if cliPath == "" {
		return
	}

	fmt.Println()
	fmt.Printf("Launch %s now? [Y/n]: ", bold(cliName))
	answer, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println()
		return
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		return
	}

	agent := ""
	if len(agents) > 0 {
		fmt.Println()
		fmt.Println(bold("Available agents:"))
		fmt.Println()
		for i, a := range agents {
			fmt.Printf("  %s  %s\n", bold(fmt.Sprintf("%d.", i+1)), a)
		}
		fmt.Printf("  %s  %s\n", bold(fmt.Sprintf("%d.", len(agents)+1)), dim("(no agent — default mode)"))
		fmt.Println()
		fmt.Printf("Select agent [1-%d]: ", len(agents)+1)
		input, err := reader.ReadString('\n')
		if err == nil {
			input = strings.TrimSpace(input)
			if choice, err := strconv.Atoi(input); err == nil && choice >= 1 && choice <= len(agents) {
				agent = agents[choice-1]
			}
		}
	}

	fmt.Println()
	launchCopilotWithAgent(agent)
}
