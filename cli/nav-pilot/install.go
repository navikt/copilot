package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type installResult struct {
	Installed   int
	Skipped     int
	Conflicts   int
	Unsupported []string
	Files       []InstalledFile
}

func installItems(sourceDir string, scope *InstallScope, manifest *Manifest, dryRun, force bool) (*installResult, error) {
	result := &installResult{}

	for _, group := range []struct {
		label   string
		names   []string
		itemType string
		install func(string, *InstallScope, string, bool, bool, *installResult) error
	}{
		{"Agents", manifest.Agents, "agent", installAgent},
		{"Skills", manifest.Skills, "skill", installSkill},
		{"Instructions", manifest.Instructions, "instruction", installInstruction},
		{"Prompts", manifest.Prompts, "prompt", installPrompt},
	} {
		if len(group.names) == 0 {
			continue
		}
		if !scope.SupportsType(group.itemType) {
			result.Unsupported = append(result.Unsupported, fmt.Sprintf("%d %s", len(group.names), group.label))
			continue
		}
		fmt.Println(bold(fmt.Sprintf("%s (%d):", group.label, len(group.names))))
		for _, name := range group.names {
			if err := group.install(sourceDir, scope, name, dryRun, force, result); err != nil {
				return result, err
			}
		}
		fmt.Println()
	}

	return result, nil
}

// installSingleFile handles the common install pattern for single-file artifacts
// (agents, instructions). Returns true if the file was actually written to disk.
func installSingleFile(sourceDir string, scope *InstallScope, dir, extension, label, name string, dryRun, force bool, result *installResult) (bool, error) {
	if err := validateName(name); err != nil {
		return false, fmt.Errorf("invalid %s name: %w", strings.ToLower(label), err)
	}
	fileName := name + extension
	srcFile, found := resolveArtifactFile(sourceDir, dir, fileName)
	dstFile := scope.DstPath(dir, fileName)

	if !found {
		fmt.Printf("  %s %s not found: %s\n", yellow("⚠"), label, name)
		result.Skipped++
		return false, nil
	}

	if c, err := checkConflict(dstFile, srcFile, false); err != nil {
		return false, err
	} else if c != nil && !force {
		fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)
		result.Conflicts++
		return false, nil
	}

	relPath := scope.RelPath(dir, fileName)
	if dryRun {
		fmt.Printf("  %s %s\n", dim("→"), relPath)
		result.Installed++
		return false, nil
	}

	if err := copyFile(srcFile, dstFile, scope.RootDir); err != nil {
		return false, fmt.Errorf("copying %s %s: %w", strings.ToLower(label), name, err)
	}
	hash, err := fileHash(dstFile)
	if err != nil {
		return false, fmt.Errorf("hashing installed %s %s: %w", strings.ToLower(label), name, err)
	}
	result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})

	fmt.Printf("  %s %s\n", green("✓"), name)
	result.Installed++
	return true, nil
}

func installAgent(sourceDir string, scope *InstallScope, name string, dryRun, force bool, result *installResult) error {
	written, err := installSingleFile(sourceDir, scope, "agents", ".agent.md", "Agent", name, dryRun, force, result)
	if err != nil || !written {
		return err
	}

	// Copy metadata if the scope supports it
	if scope.ShouldInstallMetadata() {
		srcMeta, hasMeta := resolveArtifactFile(sourceDir, "agents", name+".metadata.json")
		dstMeta := scope.DstPath("agents", name+".metadata.json")
		if hasMeta {
			if err := copyFile(srcMeta, dstMeta, scope.RootDir); err != nil {
				return fmt.Errorf("copying agent metadata %s: %w", name, err)
			}
			metaRel := scope.RelPath("agents", name+".metadata.json")
			metaHash, err := fileHash(dstMeta)
			if err != nil {
				return fmt.Errorf("hashing agent metadata %s: %w", name, err)
			}
			result.Files = append(result.Files, InstalledFile{Path: metaRel, Hash: metaHash})
		}
	}
	return nil
}

func installSkill(sourceDir string, scope *InstallScope, name string, dryRun, force bool, result *installResult) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid skill name: %w", err)
	}
	// Skills may live at root level (gh skill convention) or under .github/skills/ (legacy).
	srcDir, found := resolveSkillDir(sourceDir, name)
	if !found {
		srcDir = filepath.Join(sourceDir, ".github", "skills", name)
	}
	dstDir := scope.DstPath("skills", name)

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		fmt.Printf("  %s Skill not found: %s\n", yellow("⚠"), name)
		result.Skipped++
		return nil
	}

	if c, err := checkConflict(dstDir, srcDir, true); err != nil {
		return err
	} else if c != nil && !force {
		fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)
		result.Conflicts++
		return nil
	}

	relPath := scope.RelPath("skills", name) + "/"
	if dryRun {
		refCount := countDirFiles(filepath.Join(srcDir, "references"))
		extra := ""
		if refCount > 0 {
			extra = dim(fmt.Sprintf(" (%d reference file(s))", refCount))
		}
		fmt.Printf("  %s %s%s\n", dim("→"), relPath, extra)
		result.Installed++
		return nil
	}

	if err := copyDir(srcDir, dstDir, scope.RootDir); err != nil {
		return fmt.Errorf("copying skill %s: %w", name, err)
	}

	hash, err := dirHash(dstDir)
	if err != nil {
		return fmt.Errorf("hashing installed skill %s: %w", name, err)
	}
	result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})

	fmt.Printf("  %s %s\n", green("✓"), name)
	result.Installed++
	return nil
}

func installInstruction(sourceDir string, scope *InstallScope, name string, dryRun, force bool, result *installResult) error {
	_, err := installSingleFile(sourceDir, scope, "instructions", ".instructions.md", "Instruction", name, dryRun, force, result)
	return err
}

func installPrompt(sourceDir string, scope *InstallScope, name string, dryRun, force bool, result *installResult) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid prompt name: %w", err)
	}

	src, isDir, found := resolvePrompt(sourceDir, name)
	if !found {
		fmt.Printf("  %s Prompt not found: %s\n", yellow("⚠"), name)
		result.Skipped++
		return nil
	}

	if isDir {
		dstDir := scope.DstPath("prompts", name)

		if c, err := checkConflict(dstDir, src, true); err != nil {
			return err
		} else if c != nil && !force {
			fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)
			result.Conflicts++
			return nil
		}

		relPath := scope.RelPath("prompts", name) + "/"
		if dryRun {
			fmt.Printf("  %s %s\n", dim("→"), relPath)
			result.Installed++
			return nil
		}

		if err := copyDir(src, dstDir, scope.RootDir); err != nil {
			return fmt.Errorf("copying prompt dir %s: %w", name, err)
		}
		hash, err := dirHash(dstDir)
		if err != nil {
			return fmt.Errorf("hashing installed prompt %s: %w", name, err)
		}
		result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})
		fmt.Printf("  %s %s\n", green("✓"), name)
		result.Installed++
		return nil
	}

	// Flat file
	dstFile := scope.DstPath("prompts", name+".prompt.md")

	if c, err := checkConflict(dstFile, src, false); err != nil {
		return err
	} else if c != nil && !force {
		fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)
		result.Conflicts++
		return nil
	}

	relPath := scope.RelPath("prompts", name+".prompt.md")
	if dryRun {
		fmt.Printf("  %s %s\n", dim("→"), relPath)
		result.Installed++
		return nil
	}

	if err := copyFile(src, dstFile, scope.RootDir); err != nil {
		return fmt.Errorf("copying prompt %s: %w", name, err)
	}
	hash, err := fileHash(dstFile)
	if err != nil {
		return fmt.Errorf("hashing installed prompt %s: %w", name, err)
	}
	result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})
	fmt.Printf("  %s %s\n", green("✓"), name)
	result.Installed++
	return nil
}

// ─── Commands ───────────────────────────────────────────────────────────────

func cmdList(ref, sourceRepo string, showItems bool, jsonOutput bool) error {
	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	names, err := listCollectionDirs(src.Dir)
	if err != nil {
		return err
	}

	if jsonOutput {
		type collectionInfo struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Items       int    `json:"items"`
		}
		var collections []collectionInfo
		for _, name := range names {
			m, err := loadManifest(src.Dir, name)
			if err != nil {
				continue
			}
			total := len(m.Agents) + len(m.Skills) + len(m.Instructions) + len(m.Prompts)
			collections = append(collections, collectionInfo{Name: name, Description: m.Description, Items: total})
		}
		result := map[string]interface{}{"collections": collections}
		if showItems {
			result["items"] = collectAvailableItems(src.Dir)
		}
		return outputJSON(result)
	}

	fmt.Println()
	fmt.Println(bold("Available collections:"))
	fmt.Println()
	for _, name := range names {
		m, err := loadManifest(src.Dir, name)
		if err != nil {
			continue
		}
		total := len(m.Agents) + len(m.Skills) + len(m.Instructions) + len(m.Prompts)
		fmt.Printf("  %-20s %s %s\n", bold(name), m.Description, dim(fmt.Sprintf("(%d items)", total)))
	}
	fmt.Println()
	fmt.Printf("Install with: %s\n", bold("nav-pilot install <collection>"))
	fmt.Printf("Install everything to user home: %s\n", bold("nav-pilot install --user"))

	if showItems {
		fmt.Println()
		if err := listAvailableItems(src.Dir); err != nil {
			return err
		}
	} else {
		fmt.Printf("Show individual items: %s\n", bold("nav-pilot list --items"))
	}
	return nil
}

// listAvailableItems prints all agents, skills, instructions, and prompts in the source.
func listAvailableItems(sourceDir string) error {
	// Agents — scan both root-level and .github/
	if agents := scanArtifactFiles(sourceDir, "agents", ".agent.md"); len(agents) > 0 {
		fmt.Println(bold("Available agents:"))
		for _, a := range agents {
			fmt.Printf("  %-30s %s\n", a.Name, dim("nav-pilot add agent "+a.Name))
		}
		fmt.Println()
	}

	// Skills — scan both root-level and .github/skills/
	if skills := scanSkillDirs(sourceDir); len(skills) > 0 {
		fmt.Println(bold("Available skills:"))
		for _, s := range skills {
			fmt.Printf("  %-30s %s\n", s.Name, dim("nav-pilot add skill "+s.Name))
		}
		fmt.Println()
	}

	// Instructions — scan both root-level and .github/
	if instrs := scanArtifactFiles(sourceDir, "instructions", ".instructions.md"); len(instrs) > 0 {
		fmt.Println(bold("Available instructions:"))
		for _, i := range instrs {
			fmt.Printf("  %-30s %s\n", i.Name, dim("nav-pilot add instruction "+i.Name))
		}
		fmt.Println()
	}

	// Prompts — scan both root-level and .github/
	if prompts := scanPromptEntries(sourceDir); len(prompts) > 0 {
		fmt.Println(bold("Available prompts:"))
		for _, p := range prompts {
			fmt.Printf("  %-30s %s\n", p.Name, dim("nav-pilot add prompt "+p.Name))
		}
		fmt.Println()
	}

	return nil
}

// collectAvailableItems returns all available items as a structured map for JSON output.
func collectAvailableItems(sourceDir string) map[string][]string {
	result := make(map[string][]string)

	for _, a := range scanArtifactFiles(sourceDir, "agents", ".agent.md") {
		result["agents"] = append(result["agents"], a.Name)
	}
	for _, s := range scanSkillDirs(sourceDir) {
		result["skills"] = append(result["skills"], s.Name)
	}
	for _, i := range scanArtifactFiles(sourceDir, "instructions", ".instructions.md") {
		result["instructions"] = append(result["instructions"], i.Name)
	}
	for _, p := range scanPromptEntries(sourceDir) {
		result["prompts"] = append(result["prompts"], p.Name)
	}
	return result
}

// cmdInstallAll installs all agents and skills to user scope by scanning the source.
// Used when `nav-pilot install --user` is run without a collection name.
func cmdInstallAll(scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error {
	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	return installAllFromSource(scope, src, nil, dryRun, force, jsonOutput)
}

// installAllFromSource installs all agents+skills from source.
// If manifest is nil, it scans the source directory to discover items.
// Extracted so both cmdInstallAll and the interactive flow can share this.
func installAllFromSource(scope *InstallScope, src *Source, manifest *Manifest, dryRun, force bool, jsonOutput bool) error {
	if manifest == nil {
		var err error
		manifest, err = collectAllItems(src.Dir)
		if err != nil {
			return err
		}
	}

	total := len(manifest.Agents) + len(manifest.Skills) + len(manifest.Instructions)
	if total == 0 {
		return fmt.Errorf("no agents, skills, or instructions found in source")
	}

	sourceLabel := "navikt/copilot"

	if !jsonOutput {
		fmt.Println()
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: all agents, skills & instructions (%d items)", total)))
		} else {
			fmt.Println(bold(fmt.Sprintf("Installing: all agents, skills & instructions (%d items)", total)))
		}
		fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, src.SHA)))
		fmt.Printf("%s %s\n", dim("Target:"), dim(scope.Label()))
		fmt.Println()
	}

	result, err := installItems(src.Dir, scope, manifest, dryRun, force)
	if err != nil {
		return err
	}

	if !jsonOutput && result.Conflicts > 0 {
		fmt.Printf("%s %d file(s) skipped due to conflicts. Use %s to overwrite.\n",
			yellow("⚠"), result.Conflicts, bold("--force"))
	}

	if jsonOutput {
		return outputJSON(map[string]interface{}{
			"command":    "install",
			"collection": CollectionAll,
			"scope":      scope.Name,
			"source_sha": src.SHA,
			"version":    src.Version,
			"installed":  result.Installed,
			"conflicts":  result.Conflicts,
			"dry_run":    dryRun,
		})
	}

	if dryRun {
		fmt.Printf("%s Would install %d items.\n", dim("→"), result.Installed)
		return nil
	}

	stateVersion := src.Version

	state := &StateFile{
		Collection:  CollectionAll,
		Version:     stateVersion,
		Scope:       scope.Name,
		SourceSHA:   src.SHA,
		InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		Files:       result.Files,
	}
	if err := writeScopedState(scope, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}

	fmt.Printf("%s Installed %d items to %s (v%s, %s).\n",
		green("✓"), result.Installed, scope.Label(), stateVersion, src.SHA)
	fmt.Println()
	fmt.Println(dim("Agents and skills are now available across all your repos."))
	fmt.Println(dim("Use @nav-pilot in Copilot Chat or copilot --agent nav-pilot"))

	if len(manifest.Instructions) > 0 && scope.IsUser() {
		fmt.Println()
		fmt.Println(dim("Instructions are available when launching cplt via nav-pilot."))
		fmt.Println(dim("For direct cplt usage, add to your shell profile:"))
		fmt.Printf("  %s\n", dim("eval \"$(nav-pilot env)\""))
	}

	return nil
}

func cmdInstall(collection string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error {
	if !dryRun && !scope.IsUser() {
		if _, err := os.Stat(filepath.Join(scope.RootDir, ".git")); os.IsNotExist(err) {
			return fmt.Errorf("target %q does not appear to be a git repository (no .git directory)", scope.RootDir)
		}
	}

	if !jsonOutput {
		fmt.Println(dim("Resolving source..."))
	}
	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	manifest, err := loadManifest(src.Dir, collection)
	if err != nil {
		return err
	}

	sourceLabel := "navikt/copilot"
	if sourceRepo != "" {
		sourceLabel = sourceRepo
	}

	if !jsonOutput {
		fmt.Println()
		if dryRun {
			fmt.Println(bold(fmt.Sprintf("Dry run: %s", collection)))
		} else {
			fmt.Println(bold(fmt.Sprintf("Installing: %s", collection)))
		}
		fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, src.SHA)))
		fmt.Printf("%s %s\n", dim("Target:"), dim(scope.Label()))
		fmt.Println()
	}

	result, err := installItems(src.Dir, scope, manifest, dryRun, force)
	if err != nil {
		return err
	}

	if jsonOutput {
		return outputJSON(map[string]interface{}{
			"command":     "install",
			"collection":  collection,
			"scope":       scope.Name,
			"source_sha":  src.SHA,
			"version":     src.Version,
			"installed":   result.Installed,
			"conflicts":   result.Conflicts,
			"unsupported": result.Unsupported,
			"dry_run":     dryRun,
		})
	}

	if result.Conflicts > 0 {
		fmt.Printf("%s %d file(s) skipped due to conflicts. Use %s to overwrite.\n",
			yellow("⚠"), result.Conflicts, bold("--force"))
	}

	if len(result.Unsupported) > 0 {
		fmt.Printf("%s Skipped (not supported in %s scope): %s\n",
			yellow("⚠"), scope.Name, strings.Join(result.Unsupported, ", "))
	}

	if dryRun {
		fmt.Printf("%s Would install %d items from %q.\n",
			dim("→"), result.Installed, collection)
		return nil
	}

	stateVersion := src.Version

	state := &StateFile{
		Collection:  collection,
		Version:     stateVersion,
		Scope:       scope.Name,
		SourceSHA:   src.SHA,
		InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		Files:       result.Files,
	}
	if err := writeScopedState(scope, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}

	fmt.Printf("%s Installed %d items from %q (v%s, %s).\n",
		green("✓"), result.Installed, collection, stateVersion, src.SHA)
	fmt.Println()
	if scope.IsUser() {
		fmt.Println(dim("Agents and skills are now available across all your repos."))
		fmt.Println(dim("Use @nav-pilot in Copilot Chat or copilot --agent nav-pilot"))
	} else {
		fmt.Println(dim("Next steps:"))
		fmt.Println(dim("  1. Review the installed files in .github/"))
		fmt.Println(dim("  2. Commit and push to enable Copilot customization"))
		fmt.Println(dim("  3. Use @nav-pilot in Copilot to start planning"))
	}

	return nil
}

func cmdStatus(scope *InstallScope, jsonOutput bool) error {
	return cmdStatusScoped(scope, false, jsonOutput)
}

// cmdStatusAuto shows status for all detected scopes (repo + user) when the
// user didn't explicitly pick one with --user or --target.
func cmdStatusAuto(repoDir string, jsonOutput bool) error {
	repoScope := ScopeRepo(repoDir)
	repoState, _ := readScopedState(repoScope)

	userScope, userErr := ScopeUser()
	var userState *StateFile
	if userErr == nil {
		userState, _ = readScopedState(userScope)
	}

	if repoState == nil && userState == nil {
		if jsonOutput {
			return outputJSON(map[string]interface{}{"installed": false})
		}
		fmt.Println("No nav-pilot collection installed (repo or user scope).")
		fmt.Printf("Install with: %s\n", bold("nav-pilot install <collection>"))
		return nil
	}

	if jsonOutput {
		scopes := []map[string]interface{}{}
		if repoState != nil {
			ok, modified, missing, ignored, _ := countFileIntegrity(repoScope.RootDir, repoState)
			scopes = append(scopes, map[string]interface{}{
				"scope": "repo", "collection": repoState.Collection,
				"version": repoState.Version, "source_sha": repoState.SourceSHA,
				"installed_at": repoState.InstalledAt, "files": len(repoState.Files),
				"ok": ok, "modified": modified, "missing": missing, "ignored": ignored,
			})
		}
		if userState != nil {
			ok, modified, missing, ignored, _ := countFileIntegrity(userScope.RootDir, userState)
			scopes = append(scopes, map[string]interface{}{
				"scope": "user", "collection": userState.Collection,
				"version": userState.Version, "source_sha": userState.SourceSHA,
				"installed_at": userState.InstalledAt, "files": len(userState.Files),
				"ok": ok, "modified": modified, "missing": missing, "ignored": ignored,
			})
		}
		return outputJSON(map[string]interface{}{"installed": true, "scopes": scopes})
	}

	if repoState != nil {
		printStatusBlock(repoScope, repoState)
	}
	if userState != nil {
		if repoState != nil {
			fmt.Println()
		}
		printStatusBlock(userScope, userState)
	}
	return nil
}

func cmdStatusScoped(scope *InstallScope, _ bool, jsonOutput bool) error {
	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		if jsonOutput {
			return outputJSON(map[string]interface{}{"installed": false})
		}
		if scope.IsUser() {
			fmt.Println("No nav-pilot collection installed in user home (~/.copilot).")
		} else {
			fmt.Println("No nav-pilot collection installed.")
		}
		fmt.Printf("Install with: %s\n", bold("nav-pilot install <collection>"))
		return nil
	}

	if jsonOutput {
		ok, modified, missing, ignored, _ := countFileIntegrity(scope.RootDir, state)
		return outputJSON(map[string]interface{}{
			"installed":    true,
			"collection":   state.Collection,
			"version":      state.Version,
			"scope":        scope.Name,
			"source_sha":   state.SourceSHA,
			"installed_at": state.InstalledAt,
			"files":        len(state.Files),
			"ok":           ok,
			"modified":     modified,
			"missing":      missing,
			"ignored":      ignored,
		})
	}

	printStatusBlock(scope, state)
	return nil
}

func printStatusBlock(scope *InstallScope, state *StateFile) {
	ok, modified, missing, ignored, modifiedPaths := countFileIntegrity(scope.RootDir, state)

	fmt.Println(bold(fmt.Sprintf("nav-pilot install status (%s)", scope.Name)))
	fmt.Println()
	fmt.Printf("  Collection:  %s\n", bold(state.Collection))
	fmt.Printf("  Version:     %s\n", state.Version)
	fmt.Printf("  Scope:       %s\n", scope.Name)
	fmt.Printf("  Source:      %s\n", state.SourceSHA)
	fmt.Printf("  Installed:   %s\n", state.InstalledAt)
	fmt.Printf("  Files:       %d\n", len(state.Files))
	fmt.Println()

	for _, p := range modifiedPaths {
		fmt.Printf("  %s %s (modified locally)\n", yellow("~"), p)
	}

	statusLine := fmt.Sprintf("\n  %s %d ok, %s %d modified, %s %d missing",
		green("✓"), ok, yellow("~"), modified, red("✗"), missing)
	if ignored > 0 {
		statusLine += fmt.Sprintf(", %s %d ignored", dim("⊘"), ignored)
	}
	fmt.Println(statusLine)
}

func cmdUninstall(scope *InstallScope, dryRun bool) error {
	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		fmt.Println("No nav-pilot collection installed. Nothing to uninstall.")
		return nil
	}

	if dryRun {
		fmt.Println(bold("Dry run: would uninstall"))
	} else {
		fmt.Println(bold(fmt.Sprintf("Uninstalling: %s", state.Collection)))
	}
	fmt.Println()

	removed := 0
	for _, f := range state.Files {
		path := filepath.Join(scope.RootDir, f.Path)

		if dryRun {
			fmt.Printf("  %s %s\n", dim("×"), f.Path)
			removed++
			continue
		}

		if strings.HasSuffix(f.Path, "/") {
			if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
				fmt.Printf("  %s Could not remove %s: %v\n", yellow("⚠"), f.Path, err)
				continue
			}
		} else {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				fmt.Printf("  %s Could not remove %s: %v\n", yellow("⚠"), f.Path, err)
				continue
			}
		}
		fmt.Printf("  %s %s\n", red("×"), f.Path)
		removed++
	}

	if !dryRun {
		os.Remove(scope.StatePath())
		scope.CleanupDirs()
	}

	fmt.Println()
	if dryRun {
		fmt.Printf("%s Would remove %d items.\n", dim("→"), removed)
	} else {
		fmt.Printf("%s Removed %d items.\n", green("✓"), removed)
	}
	return nil
}
