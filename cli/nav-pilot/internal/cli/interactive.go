package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

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

	maybePromptRtkSetup(resolved)

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

func launchClient(resolved ResolvedConfig) error {
	p, err := providerFor(resolved.Client)
	if err != nil {
		return err
	}
	return p.Launch(resolved)
}

// offerLaunchCopilot prompts the user to launch the configured agent after install.
// If the provider binary is not found in PATH, the prompt is skipped.
func offerLaunchCopilot(resolved ResolvedConfig) {
	p, err := providerFor(resolved.Client)
	if err != nil || !p.Available() {
		return
	}

	if resolved.AutoLaunch {
		fmt.Println()
		fmt.Printf("%s Launching %s...\n", dim("→"), p.DisplayName())
		_ = runWithCommandTelemetry("launch", telemetryMode(), "none", func() error {
			return launchClient(resolved)
		})
		return
	}

	if !isInteractive() {
		return
	}

	fmt.Println()
	var choice string
	err = huh.NewSelect[string]().
		Title(fmt.Sprintf("Launch %s now?", p.DisplayName())).
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
		return launchClient(resolved)
	})
}

// offerLaunchCopilotWithAgents prompts the user to launch the configured agent
// using the resolved launch config. If the provider binary is not found, skipped.
func offerLaunchCopilotWithAgents(agents []string, resolved ResolvedConfig) {
	_ = agents
	p, err := providerFor(resolved.Client)
	if err != nil || !p.Available() {
		return
	}

	if resolved.AutoLaunch {
		fmt.Println()
		fmt.Printf("%s Launching %s...\n", dim("→"), p.DisplayName())
		_ = runWithCommandTelemetry("launch", telemetryMode(), "none", func() error {
			return launchClient(resolved)
		})
		return
	}

	if !isInteractive() {
		return
	}

	fmt.Println()
	var choice string
	err = huh.NewSelect[string]().
		Title(fmt.Sprintf("Launch %s now?", p.DisplayName())).
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
		return launchClient(resolved)
	})
}

// patchOpenCodeConfig ensures the given opencode config file has the rtk plugin configured.
func patchOpenCodeConfig(opencodePath string) error {
	// Resolve symlinks to avoid overwriting the symlink itself with a regular file during atomic rename
	realPath, err := filepath.EvalSymlinks(opencodePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, nothing to do
		}
		return fmt.Errorf("failed to evaluate symlink for opencode config: %w", err)
	}

	info, err := os.Stat(realPath)
	if err != nil {
		return fmt.Errorf("failed to stat opencode config: %w", err)
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		return fmt.Errorf("failed to read opencode config: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		// Might be JSONC or invalid JSON. We abort safely.
		return fmt.Errorf("failed to unmarshal opencode config: %w", err)
	}

	pluginsRaw, exists := config["plugin"]
	if !exists {
		config["plugin"] = []string{"~/.config/opencode/plugins/rtk.ts"}
	} else {
		// Handle the case where 'plugin' is a string instead of an array
		if singleStr, ok := pluginsRaw.(string); ok {
			config["plugin"] = []string{singleStr, "~/.config/opencode/plugins/rtk.ts"}
		} else if plugins, ok := pluginsRaw.([]interface{}); ok {
			hasPlugin := false
			for _, p := range plugins {
				if str, ok := p.(string); ok && str == "~/.config/opencode/plugins/rtk.ts" {
					hasPlugin = true
					break
				}
			}

			if !hasPlugin {
				config["plugin"] = append(plugins, "~/.config/opencode/plugins/rtk.ts")
			} else {
				return nil // already patched
			}
		} else {
			return fmt.Errorf("'plugin' field is not a string or array")
		}
	}

	patchedData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal patched config: %w", err)
	}

	// Atomic write: write to temp file then rename
	tmpPath := realPath + ".tmp"
	if err := os.WriteFile(tmpPath, patchedData, info.Mode()); err != nil {
		return fmt.Errorf("failed to write temporary config file: %w", err)
	}
	if err := os.Rename(tmpPath, realPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit patched config file: %w", err)
	}
	return nil
}

// maybePromptRtkSetup asks the user if they want to install and set up rtk to save tokens.
func maybePromptRtkSetup(cfg ResolvedConfig) {
	// Split prompted clients and check if the current client is already prompted
	promptedClients := strings.Split(cfg.RtkPromptedClient, ",")
	for _, pc := range promptedClients {
		if pc == cfg.Client {
			return // already prompted for this client
		}
	}

	if !isInteractive() {
		return
	}

	hasRtk := false
	rtkPath, err := exec.LookPath("rtk")
	if err == nil {
		hasRtk = true
	}

	hasBrew := false
	if _, err := exec.LookPath("brew"); err == nil {
		hasBrew = true
	}

	fmt.Println()
	fmt.Printf("%s Terminal Token Optimizer (rtk)\n", bold("🚀"))
	fmt.Println(dim("  We recommend installing the Terminal Token Optimizer (rtk) to save 60-90% on token costs for terminal commands."))
	fmt.Println()

	var choice string
	err = huh.NewSelect[string]().
		Title(fmt.Sprintf("Install and set up Terminal Token Optimizer (rtk) for %s now?", cfg.Client)).
		Options(
			huh.NewOption("Yes, set it up", "yes"),
			huh.NewOption("No thanks", "no"),
		).
		Value(&choice).
		WithTheme(navTheme()).
		Run()

	// Only mark as prompted if the user actually made a choice (didn't abort via Ctrl-C).
	if err == nil && (choice == "yes" || choice == "no") {
		newClients := cfg.Client
		if cfg.RtkPromptedClient != "" {
			newClients = cfg.RtkPromptedClient + "," + cfg.Client
		}
		if setErr := cmdConfigSet("rtk_prompted_client", newClients); setErr != nil {
			fmt.Fprintf(os.Stderr, "%s Warning: Could not save rtk config: %v\n", yellow("⚠"), setErr)
		}
		if setErr := cmdConfigSet("rtk_prompted_at", time.Now().Format(time.RFC3339)); setErr != nil {
			fmt.Fprintf(os.Stderr, "%s Warning: Could not save rtk timestamp: %v\n", yellow("⚠"), setErr)
		}
	}

	if err != nil || choice != "yes" {
		return
	}

	fmt.Println()
	if !hasRtk {
		if hasBrew {
			fmt.Printf("%s Installing rtk via brew...\n", dim("→"))
			cmd := exec.Command("brew", "install", "navikt/tap/rtk")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "%s Failed to install rtk: %v\n", yellow("⚠"), err)
				return
			}

			// Resolve correct path after install
			if p, err := exec.LookPath("rtk"); err == nil {
				rtkPath = p
			} else {
				// Fallback to brew prefix if LookPath fails
				if out, err := exec.Command("brew", "--prefix").Output(); err == nil {
					rtkPath = filepath.Join(strings.TrimSpace(string(out)), "bin", "rtk")
				} else {
					rtkPath = "rtk"
				}
			}
		} else {
			fmt.Printf("%s 'brew' not found and 'rtk' is not installed. Please install rtk manually for your platform, then run 'rtk init'.\n", yellow("⚠"))
			return
		}
	}

	fmt.Printf("%s Initializing rtk hooks...\n", dim("→"))
	// Setup global hooks for the selected client.
	args := []string{"init", "--global"}
	if cfg.Client == "copilot" {
		args = append(args, "--copilot")
	} else if cfg.Client == "opencode" {
		args = append(args, "--opencode")
		home, homeErr := os.UserHomeDir()
		if homeErr == nil {
			opencodePath := filepath.Join(home, ".config", "opencode", "opencode.json")
			if patchErr := patchOpenCodeConfig(opencodePath); patchErr != nil {
				fmt.Fprintf(os.Stderr, "%s Warning: Could not auto-patch opencode.json: %v\n", yellow("⚠"), patchErr)
			}
		}
	} else if cfg.Client == "pi" {
		args = append(args, "--agent", "pi")
	} else {
		// Fallback for claude or others if added
		args = append(args, "--agent", "claude")
	}

	cmd := exec.Command(rtkPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s Failed to init rtk hooks: %v\n", yellow("⚠"), err)
		return
	}

	fmt.Printf("%s rtk is now set up! Please restart your shell afterwards to apply hooks.\n\n", green("✓"))
}
