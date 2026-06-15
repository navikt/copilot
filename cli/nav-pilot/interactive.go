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
func cmdInteractive(overrides CLIOverrides) error {
	// On first interactive run without a config, offer the setup wizard.
	if err := maybeRunFirstRunSetup(); err != nil {
		fmt.Fprintf(os.Stderr, "%s Config setup failed: %v\n", yellow("⚠"), err)
	}

	// Resolve config once for the entire interactive session. Refuses to start
	// on a hard-invalid config; warns (non-fatal) on unknown keys / unrecognized
	// model ids.
	resolved, cfgErr := loadConfigForLaunch(overrides)
	if cfgErr != nil {
		return cfgErr
	}

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
		recordInstallState(userScope.Name, userState, readErr)
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
			recordInstallState(repoScope.Name, nil, err)
			return fmt.Errorf("reading repo state: %w", err)
		}
		recordInstallState(repoScope.Name, repoState, nil)
	}

	hasRepo := repoState != nil
	hasUser := userState != nil

	if hasRepo || hasUser {
		return interactiveSyncAndLaunch(repoScope, repoState, userScope, userState, resolved)
	}

	// Fresh install — git repo determines available scopes
	if targetDir != "" {
		return interactiveFreshInstall(targetDir, resolved)
	}

	// Not in a git repo — only user-home scope is possible
	return interactiveUserOnlyInstall(resolved)
}

// interactiveSyncAndLaunch handles the case where at least one scope has an install.
// Checks for staleness in all scopes and offers to sync, then launches Copilot.
func interactiveSyncAndLaunch(repoScope *InstallScope, repoState *StateFile, userScope *InstallScope, userState *StateFile, resolved ResolvedConfig) error {
	// Single-line greeting with discovered scopes
	var scopeParts []string
	if repoState != nil {
		scopeParts = append(scopeParts, fmt.Sprintf("repo: %s", repoState.Collection))
	}
	if userState != nil {
		label := userState.Collection
		if label == CollectionAll {
			label = "all"
		}
		scopeParts = append(scopeParts, fmt.Sprintf("user: %s", label))
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
		assessment := assessStaleness(repoState.Version)
		recordFreshness("collection", repoScope.Name, assessment)
		if assessment.LatestVersion != "" && versionNewer(assessment.LatestVersion, repoState.Version) {
			stale = append(stale, staleScope{repoScope, repoState, assessment.LatestVersion})
		}
		allAgents = append(allAgents, installedAgents(repoState)...)
	}
	if userState != nil {
		assessment := assessStaleness(userState.Version)
		recordFreshness("collection", userScope.Name, assessment)
		if assessment.LatestVersion != "" && versionNewer(assessment.LatestVersion, userState.Version) {
			stale = append(stale, staleScope{userScope, userState, assessment.LatestVersion})
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
			ref := "nav-pilot/" + s.latest
			if err := runWithCommandTelemetry("sync", "interactive", s.scope.Name, func() error {
				return cmdSync(s.scope, ref, "", true, false)
			}); err != nil {
				fmt.Fprintf(os.Stderr, "%s Sync failed for %s scope: %v\n", yellow("⚠"), s.scope.Name, err)
			}
		}
	}

	offerLaunchCopilotWithAgents(allAgents, resolved)
	return nil
}

// interactiveFreshInstall handles the case where no install exists and we're in a git repo.
// Prompts for scope first, then collection (repo) or installs everything (user).
func interactiveFreshInstall(targetDir string, resolved ResolvedConfig) error {
	fmt.Println(bold("nav-pilot") + dim(" — Nav's Copilot toolkit"))
	fmt.Println()
	fmt.Println(dim("Resolving source..."))

	src, err := resolveSource("", "")
	if err != nil {
		return err
	}
	defer src.Cleanup()

	// Scope-first: ask where to install
	scope, err := promptInstallScope(targetDir)
	if err != nil {
		return err
	}
	if scope == nil {
		fmt.Println(dim("Cancelled."))
		return nil
	}

	if scope.IsUser() {
		return interactiveUserInstall(src, resolved)
	}

	// Repo scope: pick a collection
	if err := interactiveRepoInstall(src, scope); err != nil {
		return err
	}
	offerLaunchCopilot(resolved)
	return nil
}

// interactiveUserOnlyInstall handles fresh install when not in a git repo.
// Skips the scope picker and goes straight to user-home install.
func interactiveUserOnlyInstall(resolved ResolvedConfig) error {
	fmt.Println(bold("nav-pilot") + dim(" — Nav's Copilot toolkit"))
	fmt.Println()
	fmt.Println(dim("Not in a git repository — installing to user home."))
	fmt.Println(dim("Resolving source..."))

	src, err := resolveSource("", "")
	if err != nil {
		return err
	}
	defer src.Cleanup()

	return interactiveUserInstall(src, resolved)
}

// interactiveUserInstall installs agents, skills & instructions to user home.
// Offers a two-step flow: install everything or customize selection.
// Called from interactiveFreshInstall and interactiveUserOnlyInstall (root command only).
func interactiveUserInstall(src *Source, resolved ResolvedConfig) error {
	scope, err := ScopeUser()
	if err != nil {
		return err
	}
	if err := interactiveUserInstallFromSource(scope, src); err != nil {
		return err
	}
	offerLaunchCopilot(resolved)
	return nil
}

// interactiveUserInstallFromSource is the shared implementation for user-scope interactive install.
// Used by both the root `nav-pilot` command and `nav-pilot install --user`.
func interactiveUserInstallFromSource(scope *InstallScope, src *Source) error {
	manifest, err := collectAllItems(src.Dir)
	if err != nil {
		return err
	}

	total := len(manifest.Agents) + len(manifest.Skills) + len(manifest.Instructions)
	if total == 0 {
		return fmt.Errorf("no agents, skills, or instructions found in source")
	}

	// Check for existing install to pre-select items
	existingState, _ := readScopedState(scope)

	var skippedItems []InstalledFile

	if isInteractive() {
		fmt.Println()
		var installChoice string
		err = huh.NewSelect[string]().
			Title(fmt.Sprintf("Install %d agents, skills & instructions to ~/.copilot?", total)).
			Options(
				huh.NewOption(fmt.Sprintf("Install everything (%d items)", total), "all"),
				huh.NewOption("Customize selection", "custom"),
				huh.NewOption("Cancel", "cancel"),
			).
			Value(&installChoice).
			WithTheme(navTheme()).
			Run()
		if err != nil || installChoice == "cancel" {
			fmt.Println(dim("Cancelled."))
			return nil
		}

		if installChoice == "custom" {
			selected, skipped, pickerErr := interactiveItemPicker(manifest, existingState, scope)
			if pickerErr != nil {
				return pickerErr
			}
			if selected == nil {
				fmt.Println(dim("Cancelled."))
				return nil
			}
			manifest = selected
			skippedItems = skipped
		}
	}

	// If re-installing (existing state), force-update managed files
	forceUpdate := existingState != nil && len(existingState.Files) > 0

	fmt.Println()
	return installAllFromSource(scope, src, manifest, false, forceUpdate, false, skippedItems...)
}

// buildPickerDefaults determines which items should be pre-selected in the picker.
// For a fresh install (no existing state): all items are selected.
// For a re-install: items active in state are selected, ignored items are not,
// and new items not in state are selected (so users discover new additions).
func buildPickerDefaults(full *Manifest, existingState *StateFile, scope *InstallScope) map[string][]string {
	defaults := make(map[string][]string)

	hasExisting := existingState != nil && len(existingState.Files) > 0
	if !hasExisting {
		// Fresh install: all selected
		defaults["agents"] = append([]string{}, full.Agents...)
		defaults["skills"] = append([]string{}, full.Skills...)
		defaults["instructions"] = append([]string{}, full.Instructions...)
		return defaults
	}

	// Build sets of active and ignored paths from state
	activeSet := make(map[string]bool)
	ignoredSet := make(map[string]bool)
	for _, f := range existingState.Files {
		if f.Status == fileStatusIgnored {
			ignoredSet[f.Path] = true
		} else {
			activeSet[f.Path] = true
		}
	}

	// Helper: check if an item should be selected
	isSelected := func(kind *ArtifactKind, name string) bool {
		relPath := kind.RelPathForName(scope, name)
		if ignoredSet[relPath] {
			return false // explicitly ignored
		}
		if activeSet[relPath] {
			return true // actively installed
		}
		return true // new item not in state: default to selected
	}

	type group struct {
		key   string
		kind  *ArtifactKind
		names []string
	}
	for _, g := range []group{
		{"agents", KindAgent, full.Agents},
		{"skills", KindSkill, full.Skills},
		{"instructions", KindInstruction, full.Instructions},
	} {
		for _, name := range g.names {
			if isSelected(g.kind, name) {
				defaults[g.key] = append(defaults[g.key], name)
			}
		}
	}
	return defaults
}

// interactiveItemPicker shows multiselect pickers for each artifact category.
// Returns a manifest containing only the selected items, ignored entries for
// deselected items, or (nil, nil, nil) if cancelled.
// Must be called from an interactive context (isInteractive() == true).
func interactiveItemPicker(full *Manifest, existingState *StateFile, scope *InstallScope) (*Manifest, []InstalledFile, error) {
	if !isInteractive() {
		return nil, nil, fmt.Errorf("interactive item picker requires a terminal")
	}

	defaults := buildPickerDefaults(full, existingState, scope)

	type pickerGroup struct {
		label string
		key   string
		kind  *ArtifactKind
		names []string
	}
	groups := []pickerGroup{
		{"Agents", "agents", KindAgent, full.Agents},
		{"Skills", "skills", KindSkill, full.Skills},
		{"Instructions", "instructions", KindInstruction, full.Instructions},
	}

	selected := &Manifest{
		Name:        full.Name,
		Description: full.Description,
	}

	for _, g := range groups {
		if len(g.names) == 0 {
			continue
		}

		var options []huh.Option[string]
		for _, name := range g.names {
			options = append(options, huh.NewOption(name, name))
		}

		chosen := defaults[g.key]
		err := huh.NewMultiSelect[string]().
			Title(fmt.Sprintf("%s (%d available)", g.label, len(g.names))).
			Options(options...).
			Value(&chosen).
			WithTheme(navTheme()).
			Run()
		if err != nil {
			return nil, nil, nil // user cancelled
		}

		switch g.kind {
		case KindAgent:
			selected.Agents = chosen
		case KindSkill:
			selected.Skills = chosen
		case KindInstruction:
			selected.Instructions = chosen
		}
	}

	totalSelected := len(selected.Agents) + len(selected.Skills) + len(selected.Instructions)
	totalAvailable := len(full.Agents) + len(full.Skills) + len(full.Instructions)

	if totalSelected == 0 {
		fmt.Println(dim("No items selected."))
		return nil, nil, nil
	}

	fmt.Println()
	fmt.Printf("%s Selected %d of %d items.\n", dim("→"), totalSelected, totalAvailable)

	skippedItems := computeSkippedItems(full, selected, scope)
	return selected, skippedItems, nil
}

// computeSkippedItems returns InstalledFile entries for items in full but not in selected.
// These are stored as ignored in the state file so sync and detectNewItems skip them.
func computeSkippedItems(full, selected *Manifest, scope *InstallScope) []InstalledFile {
	selectedSet := make(map[string]bool)
	for _, name := range selected.Agents {
		selectedSet["agent:"+name] = true
	}
	for _, name := range selected.Skills {
		selectedSet["skill:"+name] = true
	}
	for _, name := range selected.Instructions {
		selectedSet["instruction:"+name] = true
	}

	var skipped []InstalledFile
	addSkipped := func(kind *ArtifactKind, names []string) {
		for _, name := range names {
			if selectedSet[kind.Name+":"+name] {
				continue
			}
			skipped = append(skipped, InstalledFile{
				Path:   kind.RelPathForName(scope, name),
				Hash:   "",
				Status: fileStatusIgnored,
			})
		}
	}
	addSkipped(KindAgent, full.Agents)
	addSkipped(KindSkill, full.Skills)
	addSkipped(KindInstruction, full.Instructions)
	return skipped
}

// interactiveRepoInstall handles repo-scope collection picker flow.
func interactiveRepoInstall(src *Source, scope *InstallScope) error {
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
		return nil // user cancelled
	}

	// Show preview with contents
	m, err := loadManifest(src.Dir, selected)
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Printf("%s %s — %s\n", dim("→"), bold(selected), m.Description)
	printManifestContents(m)
	fmt.Println()

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

	// Install using the already-resolved source (avoid redundant git clone)
	fmt.Println()
	return cmdInstallFromSource(selected, src, scope, false, false, false)
}

// promptInstallScope asks the user where to install: repo or user home.
// Returns nil if the user cancels.
func promptInstallScope(targetDir string) (*InstallScope, error) {
	if !isInteractive() {
		// Non-interactive: default to repo scope
		return ScopeRepo(targetDir), nil
	}
	var choice string
	err := huh.NewSelect[string]().
		Title("Where to install?").
		Options(
			huh.NewOption("This repo (.github/) — collection with prompts, commit and push to enable", "repo"),
			huh.NewOption("User home (~/.copilot/) — agents, skills & instructions across all repos", "user"),
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

// printManifestContents prints the contents of a manifest in a readable format.
// Shows each category with names, truncating long lists with "...".
func printManifestContents(m *Manifest) {
	const maxItems = 8
	printCategory := func(label string, items []string) {
		if len(items) == 0 {
			return
		}
		display := items
		suffix := ""
		if len(items) > maxItems {
			display = items[:maxItems]
			suffix = fmt.Sprintf(", … (%d total)", len(items))
		}
		fmt.Printf("  %-16s %s%s\n", dim(fmt.Sprintf("%d %s:", len(items), label)),
			strings.Join(display, ", "), dim(suffix))
	}
	printCategory("agents", m.Agents)
	printCategory("skills", m.Skills)
	printCategory("instructions", m.Instructions)
	printCategory("prompts", m.Prompts)
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
// If the "copilot" binary is actually cplt (aliased), it's treated as cplt.
func findCopilotCLI() (path, name string) {
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

// cliDisplayName returns a user-friendly name for the CLI binary.
func cliDisplayName(name string) string {
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

// copilotAgentPersona is the Copilot CLI custom-agent persona that loads
// Nav's instructions and context. This is distinct from resolved.Agent,
// which selects the launcher (copilot vs opencode vs pi).
const copilotAgentPersona = "nav-pilot"

// buildCopilotArgs constructs the CLI arguments for launching copilot.
//
// cplt is the sandbox wrapper: its own --agent selects WHICH agent to sandbox
// and otherwise auto-detects from PATH (per `cplt --help`). Because nav-pilot is
// on the copilot launch path here, we pin `cplt --agent copilot` so a different
// agent on PATH (e.g. opencode) is never picked, then forward the copilot
// persona + flags after the "--" separator.
//
// Note: the forwarded --agent is always the nav-pilot persona; resolved.Agent
// selects the launcher and is consumed by launchAgent before reaching here.
func buildCopilotArgs(cliName string, resolved ResolvedConfig) []string {
	var args []string
	args = append(args, "--agent", copilotAgentPersona)
	args = append(args, copilotAgentArgs(copilotAgentPersona)...)
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

// launchCopilotResolved launches the Copilot CLI with the resolved launch config.
// If user-scope instructions exist, it sets COPILOT_CUSTOM_INSTRUCTIONS_DIRS
// so cplt picks up ~/.copilot/.github/instructions/*.instructions.md.
func launchCopilotResolved(resolved ResolvedConfig) error {
	cliPath, cliName := findCopilotCLI()
	if cliPath == "" {
		return fmt.Errorf("copilot cli not found")
	}
	args := buildCopilotArgs(cliName, resolved)
	displayName := cliDisplayName(cliName)
	fmt.Printf("Launching %s with agent %s...\n\n", bold(displayName), bold(copilotAgentPersona))
	cmd := exec.Command(cliPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = copilotEnv()
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not launch %s: %v\n", yellow("⚠"), displayName, err)
		return err
	}
	return nil
}

func launchAgent(resolved ResolvedConfig) error {
	switch resolved.Agent {
	case "opencode":
		return launchOpenCode(resolved)
	case "pi":
		return launchPi()
	default:
		return launchCopilotResolved(resolved)
	}
}

// offerLaunchCopilot prompts the user to launch the configured agent after install.
func offerLaunchCopilot(resolved ResolvedConfig) {
	cliPath, cliName := findCopilotCLI()
	if resolved.Agent == "copilot" && cliPath == "" {
		return
	}
	if !isInteractive() {
		return
	}

	displayName := cliDisplayName(cliName)
	if resolved.Agent == "opencode" {
		displayName = "opencode"
	}
	if resolved.Agent == "pi" {
		displayName = "pi"
	}

	fmt.Println()
	var choice string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Launch %s now?", displayName)).
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
	_ = runWithCommandTelemetry("launch", telemetryMode(), "none", func() error {
		return launchAgent(resolved)
	})
}

// offerLaunchCopilotWithAgents prompts the user to launch the Copilot CLI
// using the resolved launch config.
func offerLaunchCopilotWithAgents(agents []string, resolved ResolvedConfig) {
	_ = agents
	cliPath, cliName := findCopilotCLI()
	if resolved.Agent == "copilot" && cliPath == "" {
		return
	}
	if !isInteractive() {
		return
	}

	displayName := cliDisplayName(cliName)
	if resolved.Agent == "opencode" {
		displayName = "opencode"
	}
	if resolved.Agent == "pi" {
		displayName = "pi"
	}

	fmt.Println()
	var choice string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Launch %s now?", displayName)).
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
	_ = runWithCommandTelemetry("launch", telemetryMode(), "none", func() error {
		return launchAgent(resolved)
	})
}

// copilotEnv returns the environment for launching cplt, injecting
// COPILOT_CUSTOM_INSTRUCTIONS_DIRS if user-scope customizations exist
// (instructions and/or agents).
func copilotEnv() []string {
	copilotDir := userCopilotDir()
	env := os.Environ()
	changed := false
	key := "COPILOT_CUSTOM_INSTRUCTIONS_DIRS"
	if copilotDir != "" {
		existing := lookupEnvValue(env, key)
		if existing != "" {
			// Don't duplicate if already present
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

		var updated bool
		env, updated = setEnvValue(env, key, copilotDir)
		changed = changed || updated
	}

	var otelUpdated bool
	env, otelUpdated = applyCopilotOTelEnv(env)
	changed = changed || otelUpdated

	if !changed {
		return nil // nil inherits parent env
	}
	return env
}

func lookupEnvValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix)
		}
	}
	return ""
}

func setEnvValue(env []string, key, value string) ([]string, bool) {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			newValue := key + "=" + value
			if env[i] == newValue {
				return env, false
			}
			env[i] = newValue
			return env, true
		}
	}
	return append(env, key+"="+value), true
}

// userCopilotDir returns ~/.copilot if it contains user-scope customizations
// (instructions or agents), or "" otherwise.
func userCopilotDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	base := filepath.Join(home, ".copilot")

	// Check for instructions
	instructions, _ := filepath.Glob(filepath.Join(base, ".github", "instructions", "*.instructions.md"))
	if len(instructions) > 0 {
		return base
	}

	// Check for agents
	agents, _ := filepath.Glob(filepath.Join(base, ".github", "agents", "*.agent.md"))
	if len(agents) > 0 {
		return base
	}

	return ""
}
