package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cmdExport dispatches to the appropriate export format.
func cmdExport(format string, scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error {
	switch format {
	case "opencode":
		return exportOpenCode(scope, ref, sourceRepo, dryRun, force, jsonOutput)
	default:
		return fmt.Errorf("unknown export format: %q\n\nSupported formats: opencode", format)
	}
}

// exportOpenCode transforms Nav's .github/ artifacts into OpenCode-compatible .opencode/ format.
func exportOpenCode(scope *InstallScope, ref, sourceRepo string, dryRun, force bool, jsonOutput bool) error {
	src, err := resolveSource(ref, sourceRepo)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	// Determine output directory
	outputDir := openCodeOutputDir(scope)

	// Check if output already exists
	if info, err := os.Stat(outputDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(outputDir)
		if len(entries) > 0 && !force {
			return fmt.Errorf("%s already exists and is not empty — use %s to overwrite",
				outputDir, bold("--force"))
		}
	}

	if !jsonOutput {
		if dryRun {
			fmt.Printf("%s Export to %s\n\n", dim("→"), dim(outputDir))
		} else {
			fmt.Printf("Exporting to %s\n\n", bold(outputDir))
		}
	}

	sourceDir := src.Dir
	var totalSkills, totalCommands, totalAgents, totalInstructions int

	// Skills (1:1 copy)
	n, err := exportSkills(sourceDir, outputDir, dryRun)
	if err != nil {
		return fmt.Errorf("exporting skills: %w", err)
	}
	totalSkills = n

	// Prompts → Commands
	n, err = exportPrompts(sourceDir, outputDir, dryRun)
	if err != nil {
		return fmt.Errorf("exporting prompts: %w", err)
	}
	totalCommands = n

	// Agents
	n, err = exportAgents(sourceDir, outputDir, dryRun)
	if err != nil {
		return fmt.Errorf("exporting agents: %w", err)
	}
	totalAgents = n

	// Instructions → AGENTS.md
	n, err = exportInstructions(sourceDir, outputDir, dryRun)
	if err != nil {
		return fmt.Errorf("exporting instructions: %w", err)
	}
	totalInstructions = n

	// Summary
	total := totalSkills + totalCommands + totalAgents
	if totalInstructions > 0 {
		total++ // AGENTS.md counts as 1
	}

	if jsonOutput {
		return outputJSON(map[string]interface{}{
			"command":      "export",
			"format":       "opencode",
			"output_dir":   outputDir,
			"total":        total,
			"skills":       totalSkills,
			"commands":     totalCommands,
			"agents":       totalAgents,
			"instructions": totalInstructions,
			"dry_run":      dryRun,
		})
	}

	action := "Exported"
	if dryRun {
		action = "Would export"
	}
	fmt.Printf("\n%s %s %d artifact(s): %s\n",
		green("✓"), action, total,
		exportSummary(totalSkills, totalCommands, totalAgents, totalInstructions))

	return nil
}

// openCodeOutputDir returns the base output directory for OpenCode export.
// For user scope: ~/.config/opencode/ (OpenCode's native global path)
// For repo scope: <targetDir>/.opencode/
func openCodeOutputDir(scope *InstallScope) string {
	if scope.IsUser() {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "opencode")
	}
	return filepath.Join(scope.RootDir, ".opencode")
}

func exportSummary(skills, commands, agents, instructions int) string {
	var parts []string
	if skills > 0 {
		parts = append(parts, fmt.Sprintf("%d skill(s)", skills))
	}
	if commands > 0 {
		parts = append(parts, fmt.Sprintf("%d command(s)", commands))
	}
	if agents > 0 {
		parts = append(parts, fmt.Sprintf("%d agent(s)", agents))
	}
	if instructions > 0 {
		parts = append(parts, "AGENTS.md")
	}
	if len(parts) == 0 {
		return "nothing to export"
	}
	return strings.Join(parts, ", ")
}

// ─── Skills (1:1 copy) ──────────────────────────────────────────────────────

func exportSkills(sourceDir, outputDir string, dryRun bool) (int, error) {
	skills := scanSkillDirs(sourceDir)
	if len(skills) == 0 {
		return 0, nil
	}

	count := 0
	for _, skill := range skills {
		dstDir := filepath.Join(outputDir, "skills", skill.Name)

		if dryRun {
			files := countDirFiles(skill.Dir)
			fmt.Printf("  %s %s → skills/%s/ (%d file(s))\n",
				dim("→"), skill.Name, skill.Name, files)
		} else {
			if err := os.MkdirAll(filepath.Dir(dstDir), 0o755); err != nil {
				return count, err
			}
			if err := copyDirSimple(skill.Dir, dstDir); err != nil {
				return count, fmt.Errorf("copying skill %s: %w", skill.Name, err)
			}
			fmt.Printf("  %s %s\n", green("✓"), skill.Name)
		}
		count++
	}

	if count > 0 {
		if dryRun {
			fmt.Fprintf(os.Stderr, "")
		}
	}
	return count, nil
}

// ─── Prompts → Commands ─────────────────────────────────────────────────────

func exportPrompts(sourceDir, outputDir string, dryRun bool) (int, error) {
	promptsDir := filepath.Join(sourceDir, ".github", "prompts")
	if _, err := os.Stat(promptsDir); os.IsNotExist(err) {
		return 0, nil
	}

	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".prompt.md") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".prompt.md")
		srcPath := filepath.Join(promptsDir, entry.Name())
		dstPath := filepath.Join(outputDir, "commands", name+".md")

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return count, fmt.Errorf("reading prompt %s: %w", name, err)
		}

		transformed := transformPrompt(data)

		if dryRun {
			fmt.Printf("  %s %s.prompt.md → commands/%s.md\n", dim("→"), name, name)
		} else {
			if err := writeFile(dstPath, transformed); err != nil {
				return count, fmt.Errorf("writing command %s: %w", name, err)
			}
			fmt.Printf("  %s %s\n", green("✓"), name)
		}
		count++
	}
	return count, nil
}

// transformPrompt strips `name` from frontmatter (OpenCode derives it from filename).
func transformPrompt(data []byte) []byte {
	fm, body, hasFM := splitFrontmatter(data)
	if !hasFM {
		return data
	}
	fm = transformPromptFrontmatter(fm)
	return reassemble(fm, body)
}

// ─── Agents ─────────────────────────────────────────────────────────────────

func exportAgents(sourceDir, outputDir string, dryRun bool) (int, error) {
	agentsDir := filepath.Join(sourceDir, ".github", "agents")
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		return 0, nil
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".agent.md") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".agent.md")
		srcPath := filepath.Join(agentsDir, entry.Name())
		dstPath := filepath.Join(outputDir, "agents", name+".md")

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return count, fmt.Errorf("reading agent %s: %w", name, err)
		}

		transformed := transformAgent(data)

		if dryRun {
			fmt.Printf("  %s %s.agent.md → agents/%s.md\n", dim("→"), name, name)
		} else {
			if err := writeFile(dstPath, transformed); err != nil {
				return count, fmt.Errorf("writing agent %s: %w", name, err)
			}
			fmt.Printf("  %s %s\n", green("✓"), name)
		}
		count++
	}
	return count, nil
}

// transformAgent replaces Nav agent frontmatter with OpenCode-compatible frontmatter.
// Extracts description, sets mode: subagent, drops VS Code tool IDs.
func transformAgent(data []byte) []byte {
	fm, body, hasFM := splitFrontmatter(data)
	if !hasFM {
		return data
	}

	description, _ := extractFrontmatterValue(fm, "description")
	if description == "" {
		description = "Nav agent"
	}

	newFM := buildAgentFrontmatter(description)
	return reassemble(newFM, body)
}

// ─── Instructions → AGENTS.md ───────────────────────────────────────────────

func exportInstructions(sourceDir, outputDir string, dryRun bool) (int, error) {
	instrDir := filepath.Join(sourceDir, ".github", "instructions")

	// Collect instruction files, sorted for deterministic output
	var instrFiles []string
	if entries, err := os.ReadDir(instrDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".instructions.md") {
				instrFiles = append(instrFiles, entry.Name())
			}
		}
		sort.Strings(instrFiles)
	}

	// Also include copilot-instructions.md if it exists
	var sections []instructionSection
	globalInstr := filepath.Join(sourceDir, ".github", "copilot-instructions.md")
	if data, err := os.ReadFile(globalInstr); err == nil {
		_, body, hasFM := splitFrontmatter(data)
		if !hasFM {
			body = data
		}
		sections = append(sections, instructionSection{
			name: "Global Instructions",
			body: body,
		})
	}

	for _, name := range instrFiles {
		data, err := os.ReadFile(filepath.Join(instrDir, name))
		if err != nil {
			return 0, fmt.Errorf("reading instruction %s: %w", name, err)
		}

		sectionName := strings.TrimSuffix(name, ".instructions.md")
		sectionName = strings.ReplaceAll(sectionName, "-", " ")
		sectionName = titleCase(sectionName)

		_, body, hasFM := splitFrontmatter(data)
		if !hasFM {
			body = data
		}

		sections = append(sections, instructionSection{
			name: sectionName,
			body: body,
		})
	}

	if len(sections) == 0 {
		return 0, nil
	}

	agentsMD := buildAGENTSmd(sections)
	dstPath := filepath.Join(outputDir, "AGENTS.md")

	if dryRun {
		fmt.Printf("  %s %d section(s) → AGENTS.md\n", dim("→"), len(sections))
	} else {
		if err := writeFile(dstPath, agentsMD); err != nil {
			return 0, fmt.Errorf("writing AGENTS.md: %w", err)
		}
		fmt.Printf("  %s AGENTS.md (%d section(s) merged)\n", green("✓"), len(sections))
	}

	return len(sections), nil
}

type instructionSection struct {
	name string
	body []byte
}

func buildAGENTSmd(sections []instructionSection) []byte {
	var buf strings.Builder
	buf.WriteString("<!-- Auto-generated by nav-pilot export opencode — do not edit manually -->\n\n")

	for i, s := range sections {
		if i > 0 {
			buf.WriteString("\n---\n\n")
		}
		buf.WriteString("## " + s.name + "\n\n")
		body := strings.TrimSpace(string(s.body))
		buf.WriteString(body)
		buf.WriteByte('\n')
	}

	return []byte(buf.String())
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// writeFile writes data to path, creating parent directories.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// copyDirSimple copies a directory recursively, rejecting symlinks.
// Used for export where source symlinks could leak unexpected content.
func copyDirSimple(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Reject symlinks in source
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to follow symlink: %s", path)
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeFile(target, data)
	})
}

// titleCase converts "hello world" to "Hello World".
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
