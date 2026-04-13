package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type installResult struct {
	Installed int
	Skipped   int
	Conflicts int
	Files     []InstalledFile
}

func installItems(sourceDir, targetDir string, manifest *Manifest, dryRun, force bool) (*installResult, error) {
	result := &installResult{}

	for _, group := range []struct {
		label   string
		names   []string
		install func(string, string, string, bool, bool, *installResult) error
	}{
		{"Agents", manifest.Agents, installAgent},
		{"Skills", manifest.Skills, installSkill},
		{"Instructions", manifest.Instructions, installInstruction},
		{"Prompts", manifest.Prompts, installPrompt},
	} {
		if len(group.names) == 0 {
			continue
		}
		fmt.Println(bold(fmt.Sprintf("%s (%d):", group.label, len(group.names))))
		for _, name := range group.names {
			if err := group.install(sourceDir, targetDir, name, dryRun, force, result); err != nil {
				return result, err
			}
		}
		fmt.Println()
	}

	return result, nil
}

func installAgent(sourceDir, targetDir, name string, dryRun, force bool, result *installResult) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid agent name: %w", err)
	}
	srcFile := filepath.Join(sourceDir, ".github", "agents", name+".agent.md")
	srcMeta := filepath.Join(sourceDir, ".github", "agents", name+".metadata.json")
	dstFile := filepath.Join(targetDir, ".github", "agents", name+".agent.md")
	dstMeta := filepath.Join(targetDir, ".github", "agents", name+".metadata.json")

	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		fmt.Printf("  %s Agent not found: %s\n", yellow("⚠"), name)
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

	relPath := filepath.Join(".github", "agents", name+".agent.md")
	if dryRun {
		fmt.Printf("  %s %s\n", dim("→"), relPath)
		result.Installed++
		return nil
	}

	if err := copyFile(srcFile, dstFile); err != nil {
		return fmt.Errorf("copying agent %s: %w", name, err)
	}
	hash, err := fileHash(dstFile)
	if err != nil {
		return fmt.Errorf("hashing installed agent %s: %w", name, err)
	}
	result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})

	if _, err := os.Stat(srcMeta); err == nil {
		if err := copyFile(srcMeta, dstMeta); err != nil {
			return fmt.Errorf("copying agent metadata %s: %w", name, err)
		}
		metaRel := filepath.Join(".github", "agents", name+".metadata.json")
		metaHash, err := fileHash(dstMeta)
		if err != nil {
			return fmt.Errorf("hashing agent metadata %s: %w", name, err)
		}
		result.Files = append(result.Files, InstalledFile{Path: metaRel, Hash: metaHash})
	}

	fmt.Printf("  %s %s\n", green("✓"), name)
	result.Installed++
	return nil
}

func installSkill(sourceDir, targetDir, name string, dryRun, force bool, result *installResult) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid skill name: %w", err)
	}
	srcDir := filepath.Join(sourceDir, ".github", "skills", name)
	dstDir := filepath.Join(targetDir, ".github", "skills", name)

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

	relPath := filepath.Join(".github", "skills", name) + "/"
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

func installInstruction(sourceDir, targetDir, name string, dryRun, force bool, result *installResult) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid instruction name: %w", err)
	}
	srcFile := filepath.Join(sourceDir, ".github", "instructions", name+".instructions.md")
	dstFile := filepath.Join(targetDir, ".github", "instructions", name+".instructions.md")

	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		fmt.Printf("  %s Instruction not found: %s\n", yellow("⚠"), name)
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

	relPath := filepath.Join(".github", "instructions", name+".instructions.md")
	if dryRun {
		fmt.Printf("  %s %s\n", dim("→"), relPath)
		result.Installed++
		return nil
	}

	if err := copyFile(srcFile, dstFile); err != nil {
		return fmt.Errorf("copying instruction %s: %w", name, err)
	}
	hash, err := fileHash(dstFile)
	if err != nil {
		return fmt.Errorf("hashing installed instruction %s: %w", name, err)
	}
	result.Files = append(result.Files, InstalledFile{Path: relPath, Hash: hash})

	fmt.Printf("  %s %s\n", green("✓"), name)
	result.Installed++
	return nil
}

func installPrompt(sourceDir, targetDir, name string, dryRun, force bool, result *installResult) error {
	if err := validateName(name); err != nil {
		return fmt.Errorf("invalid prompt name: %w", err)
	}
	srcDir := filepath.Join(sourceDir, ".github", "prompts", name)
	srcFile := filepath.Join(sourceDir, ".github", "prompts", name+".prompt.md")

	// Try directory first, then flat file
	if info, err := os.Stat(srcDir); err == nil && info.IsDir() {
		dstDir := filepath.Join(targetDir, ".github", "prompts", name)

		if c, err := checkConflict(dstDir, srcDir, true); err != nil {
			return err
		} else if c != nil && !force {
			fmt.Printf("  %s %s (exists, differs — use --force to overwrite)\n", yellow("⚠"), name)
			result.Conflicts++
			return nil
		}

		relPath := filepath.Join(".github", "prompts", name) + "/"
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
	dstFile := filepath.Join(targetDir, ".github", "prompts", name+".prompt.md")

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

	relPath := filepath.Join(".github", "prompts", name+".prompt.md")
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

func installGlobalInstructions(src *Source, targetDir string, result *installResult) {
	globalSrc := filepath.Join(src.Dir, ".github", "copilot-instructions.md")
	globalDst := filepath.Join(targetDir, ".github", "copilot-instructions.md")
	if _, err := os.Stat(globalSrc); err != nil {
		return
	}
	if _, err := os.Stat(globalDst); !os.IsNotExist(err) {
		return // already exists
	}
	if err := copyFile(globalSrc, globalDst); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not copy copilot-instructions.md: %v\n", yellow("⚠"), err)
		return
	}
	hash, err := fileHash(globalDst)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not hash copilot-instructions.md: %v\n", yellow("⚠"), err)
		return
	}
	result.Files = append(result.Files, InstalledFile{
		Path: ".github/copilot-instructions.md",
		Hash: hash,
	})
	fmt.Printf("%s Copied global copilot-instructions.md\n", green("✓"))
}

// removeEmptyGitHubDirs cleans up empty .github subdirectories after uninstall.
func removeEmptyGitHubDirs(targetDir string) {
	for _, sub := range []string{"agents", "skills", "instructions", "prompts"} {
		dir := filepath.Join(targetDir, ".github", sub)
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			os.Remove(dir)
		}
	}
}

// ─── Commands ───────────────────────────────────────────────────────────────

func cmdList(ref string, showItems bool) error {
	fmt.Println(dim("Resolving source..."))
	src, err := resolveSource(ref, "")
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

	// Prompts
	if entries, err := filepath.Glob(filepath.Join(ghDir, "prompts", "*.prompt.md")); err == nil && len(entries) > 0 {
		fmt.Println(bold("Available prompts:"))
		for _, e := range entries {
			name := strings.TrimSuffix(filepath.Base(e), ".prompt.md")
			fmt.Printf("  %-30s %s\n", name, dim("nav-pilot add prompt "+name))
		}
		fmt.Println()
	}

	return nil
}

func cmdInstall(collection, targetDir, ref string, dryRun, force bool) error {
	if !dryRun {
		if _, err := os.Stat(filepath.Join(targetDir, ".git")); os.IsNotExist(err) {
			return fmt.Errorf("target %q does not appear to be a git repository (no .git directory)", targetDir)
		}
	}

	fmt.Println(dim("Resolving source..."))
	src, err := resolveSource(ref, "")
	if err != nil {
		return err
	}
	defer src.Cleanup()

	manifest, err := loadManifest(src.Dir, collection)
	if err != nil {
		return err
	}

	fmt.Println()
	if dryRun {
		fmt.Println(bold(fmt.Sprintf("Dry run: %s", collection)))
	} else {
		fmt.Println(bold(fmt.Sprintf("Installing: %s", collection)))
	}
	fmt.Printf("%s %s\n", dim("Source:"), dim(fmt.Sprintf("navikt/copilot@%s", src.SHA)))
	fmt.Printf("%s %s\n", dim("Target:"), dim(targetDir))
	fmt.Println()

	result, err := installItems(src.Dir, targetDir, manifest, dryRun, force)
	if err != nil {
		return err
	}

	if !dryRun {
		installGlobalInstructions(src, targetDir, result)
	}

	if result.Conflicts > 0 {
		fmt.Printf("%s %d file(s) skipped due to conflicts. Use %s to overwrite.\n",
			yellow("⚠"), result.Conflicts, bold("--force"))
	}

	if dryRun {
		fmt.Printf("%s Would install %d items from %q.\n",
			dim("→"), result.Installed, collection)
		return nil
	}

	state := &StateFile{
		Collection:  collection,
		Version:     manifest.Version,
		SourceSHA:   src.SHA,
		InstalledAt: timeNow().UTC().Format("2006-01-02T15:04:05Z07:00"),
		Files:       result.Files,
	}
	if err := writeState(targetDir, state); err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not write state file: %v\n", yellow("⚠"), err)
	}

	fmt.Printf("%s Installed %d items from %q (v%s, %s).\n",
		green("✓"), result.Installed, collection, manifest.Version, src.SHA)
	fmt.Println()
	fmt.Println(dim("Next steps:"))
	fmt.Println(dim("  1. Review the installed files in .github/"))
	fmt.Println(dim("  2. Commit and push to enable Copilot customization"))
	fmt.Println(dim("  3. Use @nav-pilot in Copilot to start planning"))

	return nil
}

func cmdStatus(targetDir string) error {
	state, err := readState(targetDir)
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if state == nil {
		fmt.Println("No nav-pilot collection installed.")
		fmt.Printf("Install with: %s\n", bold("nav-pilot install <collection>"))
		return nil
	}

	fmt.Println(bold("nav-pilot install status"))
	fmt.Println()
	fmt.Printf("  Collection:  %s\n", bold(state.Collection))
	fmt.Printf("  Version:     %s\n", state.Version)
	fmt.Printf("  Source:      %s\n", state.SourceSHA)
	fmt.Printf("  Installed:   %s\n", state.InstalledAt)
	fmt.Printf("  Files:       %d\n", len(state.Files))
	fmt.Println()

	missing := 0
	modified := 0
	ok := 0
	for _, f := range state.Files {
		path := filepath.Join(targetDir, f.Path)
		var currentHash string
		var hashErr error
		if strings.HasSuffix(f.Path, "/") {
			currentHash, hashErr = dirHash(path)
		} else {
			currentHash, hashErr = fileHash(path)
		}
		if hashErr != nil {
			missing++
			continue
		}
		if currentHash != f.Hash {
			modified++
			fmt.Printf("  %s %s (modified locally)\n", yellow("~"), f.Path)
		} else {
			ok++
		}
	}

	fmt.Printf("\n  %s %d ok, %s %d modified, %s %d missing\n",
		green("✓"), ok, yellow("~"), modified, red("✗"), missing)
	return nil
}

func cmdUninstall(targetDir string, dryRun bool) error {
	state, err := readState(targetDir)
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
		path := filepath.Join(targetDir, f.Path)

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
		stPath := filepath.Join(targetDir, stateFilePath)
		os.Remove(stPath)
		removeEmptyGitHubDirs(targetDir)
	}

	fmt.Println()
	if dryRun {
		fmt.Printf("%s Would remove %d items.\n", dim("→"), removed)
	} else {
		fmt.Printf("%s Removed %d items.\n", green("✓"), removed)
	}
	return nil
}
