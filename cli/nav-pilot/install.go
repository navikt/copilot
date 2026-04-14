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
	srcFile := filepath.Join(sourceDir, ".github", dir, fileName)
	dstFile := scope.DstPath(dir, fileName)

	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
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

	if err := copyFile(srcFile, dstFile); err != nil {
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
		srcMeta := filepath.Join(sourceDir, ".github", "agents", name+".metadata.json")
		dstMeta := scope.DstPath("agents", name+".metadata.json")
		if _, err := os.Stat(srcMeta); err == nil {
			if err := copyFile(srcMeta, dstMeta); err != nil {
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
	srcDir := filepath.Join(sourceDir, ".github", "skills", name)
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

	if err := copyDir(srcDir, dstDir); err != nil {
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
	srcDir := filepath.Join(sourceDir, ".github", "prompts", name)
	srcFile := filepath.Join(sourceDir, ".github", "prompts", name+".prompt.md")

	// Try directory first, then flat file
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		dstDir := scope.DstPath("prompts", name)

		if c, err := checkConflict(dstDir, srcDir, true); err != nil {
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

		if err := copyDir(srcDir, dstDir); err != nil {
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

	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		fmt.Printf("  %s Prompt not found: %s\n", yellow("⚠"), name)
		result.Skipped++
		return nil
	}

	if c, err := checkConflict(dstFile, srcFile, false); err != nil {
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

	if err := copyFile(srcFile, dstFile); err != nil {
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

func cmdList(ref, sourceRepo string, showItems bool) error {
	fmt.Println(dim("Resolving source..."))
	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	names, err := listCollectionDirs(src.Dir)
	if err != nil {
		return err
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
	ghDir := filepath.Join(sourceDir, ".github")

	// Agents
	if entries, err := filepath.Glob(filepath.Join(ghDir, "agents", "*.agent.md")); err == nil && len(entries) > 0 {
		fmt.Println(bold("Available agents:"))
		for _, e := range entries {
			name := strings.TrimSuffix(filepath.Base(e), ".agent.md")
			fmt.Printf("  %-30s %s\n", name, dim("nav-pilot add agent "+name))
		}
		fmt.Println()
	}

	// Skills
	if entries, err := os.ReadDir(filepath.Join(ghDir, "skills")); err == nil && len(entries) > 0 {
		fmt.Println(bold("Available skills:"))
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skill := filepath.Join(ghDir, "skills", e.Name(), "SKILL.md")
			if _, err := os.Stat(skill); err == nil {
				fmt.Printf("  %-30s %s\n", e.Name(), dim("nav-pilot add skill "+e.Name()))
			}
		}
		fmt.Println()
	}

	// Instructions
	if entries, err := filepath.Glob(filepath.Join(ghDir, "instructions", "*.instructions.md")); err == nil && len(entries) > 0 {
		fmt.Println(bold("Available instructions:"))
		for _, e := range entries {
			name := strings.TrimSuffix(filepath.Base(e), ".instructions.md")
			fmt.Printf("  %-30s %s\n", name, dim("nav-pilot add instruction "+name))
		}
		fmt.Println()
	}

	// Prompts — both flat files and directories
	promptsDir := filepath.Join(ghDir, "prompts")
	promptSeen := make(map[string]bool)
	// Scan directories first
	if entries, err := os.ReadDir(promptsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				promptSeen[e.Name()] = true
			}
		}
	}
	// Scan flat files, skip if directory version exists
	if entries, err := filepath.Glob(filepath.Join(promptsDir, "*.prompt.md")); err == nil {
		for _, e := range entries {
			name := strings.TrimSuffix(filepath.Base(e), ".prompt.md")
			promptSeen[name] = true
		}
	}
	if len(promptSeen) > 0 {
		fmt.Println(bold("Available prompts:"))
		for name := range promptSeen {
			fmt.Printf("  %-30s %s\n", name, dim("nav-pilot add prompt "+name))
		}
		fmt.Println()
	}

	return nil
}

func cmdInstall(collection string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool) error {
	if !dryRun && !scope.IsUser() {
		if _, err := os.Stat(filepath.Join(scope.RootDir, ".git")); os.IsNotExist(err) {
			return fmt.Errorf("target %q does not appear to be a git repository (no .git directory)", scope.RootDir)
		}
	}

	fmt.Println(dim("Resolving source..."))
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

	fmt.Println()
	if dryRun {
		fmt.Println(bold(fmt.Sprintf("Dry run: %s", collection)))
	} else {
		fmt.Println(bold(fmt.Sprintf("Installing: %s", collection)))
	}
	fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("%s@%s", sourceLabel, src.SHA)))
	fmt.Printf("%s %s\n", dim("Target:"), dim(scope.Label()))
	fmt.Println()

	result, err := installItems(src.Dir, scope, manifest, dryRun, force)
	if err != nil {
		return err
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

	// Use the binary's release version directly.
	// "dev" means local/unreleased build — checkStaleness() skips it.
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

func cmdStatus(scope *InstallScope) error {
	state, err := readScopedState(scope)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		if scope.IsUser() {
			fmt.Println("No nav-pilot collection installed in user home (~/.copilot).")
		} else {
			fmt.Println("No nav-pilot collection installed.")
		}
		fmt.Printf("Install with: %s\n", bold("nav-pilot install <collection>"))
		return nil
	}

	fmt.Println(bold("nav-pilot install status"))
	fmt.Println()
	fmt.Printf("  Collection:  %s\n", bold(state.Collection))
	fmt.Printf("  Version:     %s\n", state.Version)
	fmt.Printf("  Scope:       %s\n", scope.Name)
	fmt.Printf("  Source:      %s\n", state.SourceSHA)
	fmt.Printf("  Installed:   %s\n", state.InstalledAt)
	fmt.Printf("  Files:       %d\n", len(state.Files))
	fmt.Println()

	ok, modified, missing, modifiedPaths := countFileIntegrity(scope.RootDir, state)
	for _, p := range modifiedPaths {
		fmt.Printf("  %s %s (modified locally)\n", yellow("~"), p)
	}

	fmt.Printf("\n  %s %d ok, %s %d modified, %s %d missing\n",
		green("✓"), ok, yellow("~"), modified, red("✗"), missing)
	return nil
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
