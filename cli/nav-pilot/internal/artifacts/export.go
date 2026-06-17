package artifacts

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// CmdExport dispatches to the appropriate export format.
func CmdExport(format string, scope *domain.InstallScope, ref, sourceRepo, cliVersion string, dryRun, force, jsonOutput bool) error {
	switch format {
	case "opencode":
		return ExportOpenCode(scope, ref, sourceRepo, cliVersion, dryRun, force, jsonOutput)
	default:
		return fmt.Errorf("unknown export format: %q\n\nSupported formats: opencode", format)
	}
}

// ExportOpenCode transforms Nav's .github/ artifacts into OpenCode-compatible .opencode/ format.
func ExportOpenCode(scope *domain.InstallScope, ref, sourceRepo, cliVersion string, dryRun, force, jsonOutput bool) error {
	src, err := source.ResolveSource(ref, sourceRepo, cliVersion)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	outputDir := OpenCodeOutputDir(scope)

	if info, err := os.Stat(outputDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(outputDir)
		if len(entries) > 0 && !force {
			return fmt.Errorf("%s already exists and is not empty — use %s to overwrite",
				outputDir, domain.Bold("--force"))
		}
	}

	if !jsonOutput {
		if dryRun {
			fmt.Printf("%s Export to %s\n\n", domain.Dim("→"), domain.Dim(outputDir))
		} else {
			fmt.Printf("Exporting to %s\n\n", domain.Bold(outputDir))
		}
	}

	sourceDir := src.Dir
	var totalSkills, totalCommands, totalAgents, totalInstructions int

	n, err := exportSkills(sourceDir, outputDir, dryRun)
	if err != nil {
		return fmt.Errorf("exporting skills: %w", err)
	}
	totalSkills = n

	n, err = exportPrompts(sourceDir, outputDir, dryRun)
	if err != nil {
		return fmt.Errorf("exporting prompts: %w", err)
	}
	totalCommands = n

	n, err = exportAgents(sourceDir, outputDir, dryRun)
	if err != nil {
		return fmt.Errorf("exporting agents: %w", err)
	}
	totalAgents = n

	n, err = exportInstructions(sourceDir, outputDir, dryRun)
	if err != nil {
		return fmt.Errorf("exporting instructions: %w", err)
	}
	totalInstructions = n

	total := totalSkills + totalCommands + totalAgents
	if totalInstructions > 0 {
		total++
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
		domain.Green("✓"), action, total,
		ExportSummary(totalSkills, totalCommands, totalAgents, totalInstructions))

	return nil
}

// OpenCodeOutputDir returns the base output directory for OpenCode export.
// For user scope: ~/.config/opencode/ (OpenCode's native global path)
// For repo scope: <targetDir>/.opencode/
func OpenCodeOutputDir(scope *domain.InstallScope) string {
	if scope.IsUser() {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "opencode")
	}
	return filepath.Join(scope.RootDir, ".opencode")
}

func ExportSummary(skills, commands, agents, instructions int) string {
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

func exportSkills(sourceDir, outputDir string, dryRun bool) (int, error) {
	skills := source.NewSourceResolver(sourceDir).List(source.KindSkill)
	if len(skills) == 0 {
		return 0, nil
	}

	count := 0
	for _, skill := range skills {
		dstDir := filepath.Join(outputDir, "skills", skill.Name)

		if dryRun {
			files := source.CountDirFiles(skill.AbsPath)
			fmt.Printf("  %s %s → skills/%s/ (%d file(s))\n",
				domain.Dim("→"), skill.Name, skill.Name, files)
		} else {
			if err := os.MkdirAll(filepath.Dir(dstDir), 0o755); err != nil {
				return count, err
			}
			if err := copyDirSimple(skill.AbsPath, dstDir); err != nil {
				return count, fmt.Errorf("copying skill %s: %w", skill.Name, err)
			}
			fmt.Printf("  %s %s\n", domain.Green("✓"), skill.Name)
		}
		count++
	}

	if count > 0 && dryRun {
		fmt.Fprintf(os.Stderr, "")
	}
	return count, nil
}

func exportPrompts(sourceDir, outputDir string, dryRun bool) (int, error) {
	entries := source.NewSourceResolver(sourceDir).List(source.KindPrompt)
	if len(entries) == 0 {
		return 0, nil
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}

		dstPath := filepath.Join(outputDir, "commands", entry.Name+".md")

		data, err := os.ReadFile(entry.AbsPath)
		if err != nil {
			return count, fmt.Errorf("reading prompt %s: %w", entry.Name, err)
		}

		transformed := transformPrompt(data)

		if dryRun {
			fmt.Printf("  %s %s.prompt.md → commands/%s.md\n", domain.Dim("→"), entry.Name, entry.Name)
		} else {
			if err := writeFile(dstPath, transformed); err != nil {
				return count, fmt.Errorf("writing command %s: %w", entry.Name, err)
			}
			fmt.Printf("  %s %s\n", domain.Green("✓"), entry.Name)
		}
		count++
	}
	return count, nil
}

func transformPrompt(data []byte) []byte {
	fm, body, hasFM := source.SplitFrontmatter(data)
	if !hasFM {
		return data
	}
	fm = source.TransformPromptFrontmatter(fm)
	return source.Reassemble(fm, body)
}

func exportAgents(sourceDir, outputDir string, dryRun bool) (int, error) {
	agents := source.NewSourceResolver(sourceDir).List(source.KindAgent)
	if len(agents) == 0 {
		return 0, nil
	}

	count := 0
	for _, entry := range agents {
		dstPath := filepath.Join(outputDir, "agents", entry.Name+".md")

		data, err := os.ReadFile(entry.AbsPath)
		if err != nil {
			return count, fmt.Errorf("reading agent %s: %w", entry.Name, err)
		}

		transformed := transformAgent(data)

		if dryRun {
			fmt.Printf("  %s %s.agent.md → agents/%s.md\n", domain.Dim("→"), entry.Name, entry.Name)
		} else {
			if err := writeFile(dstPath, transformed); err != nil {
				return count, fmt.Errorf("writing agent %s: %w", entry.Name, err)
			}
			fmt.Printf("  %s %s\n", domain.Green("✓"), entry.Name)
		}
		count++
	}
	return count, nil
}

func transformAgent(data []byte) []byte {
	fm, body, hasFM := source.SplitFrontmatter(data)
	if !hasFM {
		return data
	}

	description, _ := source.ExtractFrontmatterValue(fm, "description")
	if description == "" {
		description = "Nav agent"
	}

	newFM := source.BuildAgentFrontmatter(description)
	return source.Reassemble(newFM, body)
}

// InstructionSection holds global instruction content to be inlined into AGENTS.md.
type InstructionSection struct {
	Name string
	Body []byte
}

// InstructionRef holds a scoped instruction file to be exported individually
// and referenced lazily from AGENTS.md.
type InstructionRef struct {
	Name    string
	ApplyTo string
	Body    []byte
}

func collectInstructionData(sourceDir string) ([]InstructionSection, []InstructionRef, error) {
	instrEntries := source.NewSourceResolver(sourceDir).List(source.KindInstruction)

	var globalSections []InstructionSection
	globalInstr := filepath.Join(sourceDir, ".github", "copilot-instructions.md")
	if data, err := os.ReadFile(globalInstr); err == nil {
		_, body, hasFM := source.SplitFrontmatter(data)
		if !hasFM {
			body = data
		}
		globalSections = append(globalSections, InstructionSection{
			Name: "Global Instructions",
			Body: body,
		})
	}

	var scopedRefs []InstructionRef
	for _, entry := range instrEntries {
		data, err := os.ReadFile(entry.AbsPath)
		if err != nil {
			return nil, nil, fmt.Errorf("reading instruction %s: %w", entry.Name, err)
		}

		fm, body, hasFM := source.SplitFrontmatter(data)
		if !hasFM {
			body = data
		}

		applyTo := ""
		if hasFM {
			applyTo, _ = source.ExtractFrontmatterValue(fm, "applyTo")
		}

		if applyTo == "" || applyTo == "**" {
			sectionName := titleCase(strings.ReplaceAll(entry.Name, "-", " "))
			globalSections = append(globalSections, InstructionSection{
				Name: sectionName,
				Body: body,
			})
		} else {
			scopedRefs = append(scopedRefs, InstructionRef{
				Name:    entry.Name,
				ApplyTo: applyTo,
				Body:    body,
			})
		}
	}

	return globalSections, scopedRefs, nil
}

func exportInstructions(sourceDir, outputDir string, dryRun bool) (int, error) {
	globalSections, scopedRefs, err := collectInstructionData(sourceDir)
	if err != nil {
		return 0, err
	}

	if len(globalSections) == 0 && len(scopedRefs) == 0 {
		return 0, nil
	}

	for _, ref := range scopedRefs {
		dstPath := filepath.Join(outputDir, "instructions", ref.Name+".md")
		if dryRun {
			fmt.Printf("  %s %s.instructions.md → instructions/%s.md\n", domain.Dim("→"), ref.Name, ref.Name)
		} else {
			if err := writeFile(dstPath, ref.Body); err != nil {
				return 0, fmt.Errorf("writing instruction %s: %w", ref.Name, err)
			}
		}
	}

	agentsMD := buildLeanAGENTSmd(globalSections, scopedRefs)
	dstPath := filepath.Join(outputDir, "AGENTS.md")
	total := len(globalSections) + len(scopedRefs)

	if dryRun {
		if len(scopedRefs) > 0 {
			fmt.Printf("  %s %d global section(s) → AGENTS.md + %d scoped file(s) → instructions/\n",
				domain.Dim("→"), len(globalSections), len(scopedRefs))
		} else {
			fmt.Printf("  %s %d section(s) → AGENTS.md\n", domain.Dim("→"), len(globalSections))
		}
	} else {
		if err := writeFile(dstPath, agentsMD); err != nil {
			return 0, fmt.Errorf("writing AGENTS.md: %w", err)
		}
		if len(scopedRefs) > 0 {
			fmt.Printf("  %s AGENTS.md (%d global) + %d instruction file(s)\n",
				domain.Green("✓"), len(globalSections), len(scopedRefs))
		} else {
			fmt.Printf("  %s AGENTS.md (%d section(s))\n", domain.Green("✓"), len(globalSections))
		}
	}

	return total, nil
}

func buildLeanAGENTSmd(globalSections []InstructionSection, refs []InstructionRef) []byte {
	var buf strings.Builder
	buf.WriteString("<!-- Auto-generated by nav-pilot export opencode — do not edit manually -->\n\n")

	for i, s := range globalSections {
		if i > 0 {
			buf.WriteString("\n---\n\n")
		}
		buf.WriteString("## " + s.Name + "\n\n")
		body := strings.TrimSpace(string(s.Body))
		buf.WriteString(body)
		buf.WriteByte('\n')
	}

	if len(refs) > 0 {
		if len(globalSections) > 0 {
			buf.WriteString("\n---\n\n")
		}
		buf.WriteString("## Context Loading\n\n")
		buf.WriteString("Load instruction files on a **need-to-know basis** only — do not preemptively load all references.\n")
		buf.WriteString("Use the Read tool to load the relevant file when about to write or review matching code:\n\n")
		for _, ref := range refs {
			buf.WriteString(fmt.Sprintf("- `%s` → @.opencode/instructions/%s.md\n", ref.ApplyTo, ref.Name))
		}
		buf.WriteString("\n**CRITICAL**: Only load a file when it matches the current task. Do not load files for languages or frameworks not in use.\n")
	}

	return []byte(buf.String())
}

// MaterializeOpenCode writes all Nav OpenCode artifacts to outputDir silently (no console output).
// Unlike ExportOpenCode it never checks for --force and never prints per-file lines —
// it just ensures the files exist and are current. Idempotent: os.WriteFile overwrites
// files with the same content on repeated calls, so running on every launch is safe.
// Returns the count of each artifact type written.
func MaterializeOpenCode(sourceDir, outputDir string) (skills, commands, agents, instructions int, err error) {
	resolver := source.NewSourceResolver(sourceDir)

	for _, skill := range resolver.List(source.KindSkill) {
		dstDir := filepath.Join(outputDir, "skills", skill.Name)
		if err := source.CheckSymlink(dstDir, outputDir); err != nil {
			return skills, commands, agents, instructions, fmt.Errorf("skill %s: %w", skill.Name, err)
		}
		if mkErr := os.MkdirAll(filepath.Dir(dstDir), 0o755); mkErr != nil {
			return skills, commands, agents, instructions, mkErr
		}
		if cpErr := copyDirSimple(skill.AbsPath, dstDir); cpErr != nil {
			return skills, commands, agents, instructions, fmt.Errorf("skill %s: %w", skill.Name, cpErr)
		}
		skills++
	}

	for _, entry := range resolver.List(source.KindPrompt) {
		if entry.IsDir {
			continue
		}
		data, readErr := os.ReadFile(entry.AbsPath)
		if readErr != nil {
			return skills, commands, agents, instructions, fmt.Errorf("prompt %s: %w", entry.Name, readErr)
		}
		dstPath := filepath.Join(outputDir, "commands", entry.Name+".md")
		if err := source.CheckSymlink(dstPath, outputDir); err != nil {
			return skills, commands, agents, instructions, fmt.Errorf("command %s: %w", entry.Name, err)
		}
		if wErr := writeFile(dstPath, transformPrompt(data)); wErr != nil {
			return skills, commands, agents, instructions, fmt.Errorf("command %s: %w", entry.Name, wErr)
		}
		commands++
	}

	for _, entry := range resolver.List(source.KindAgent) {
		data, readErr := os.ReadFile(entry.AbsPath)
		if readErr != nil {
			return skills, commands, agents, instructions, fmt.Errorf("agent %s: %w", entry.Name, readErr)
		}
		dstPath := filepath.Join(outputDir, "agents", entry.Name+".md")
		if err := source.CheckSymlink(dstPath, outputDir); err != nil {
			return skills, commands, agents, instructions, fmt.Errorf("agent %s: %w", entry.Name, err)
		}
		if wErr := writeFile(dstPath, transformAgent(data)); wErr != nil {
			return skills, commands, agents, instructions, fmt.Errorf("agent %s: %w", entry.Name, wErr)
		}
		agents++
	}

	globalSections, scopedRefs, collErr := collectInstructionData(sourceDir)
	if collErr != nil {
		return skills, commands, agents, instructions, collErr
	}
	if len(globalSections) > 0 || len(scopedRefs) > 0 {
		for _, ref := range scopedRefs {
			dstPath := filepath.Join(outputDir, "instructions", ref.Name+".md")
			if err := source.CheckSymlink(dstPath, outputDir); err != nil {
				return skills, commands, agents, instructions, fmt.Errorf("instruction %s: %w", ref.Name, err)
			}
			if wErr := writeFile(dstPath, ref.Body); wErr != nil {
				return skills, commands, agents, instructions, fmt.Errorf("instruction %s: %w", ref.Name, wErr)
			}
		}
		agentsMDPath := filepath.Join(outputDir, "AGENTS.md")
		if err := source.CheckSymlink(agentsMDPath, outputDir); err != nil {
			return skills, commands, agents, instructions, fmt.Errorf("AGENTS.md: %w", err)
		}
		agentsMD := buildLeanAGENTSmd(globalSections, scopedRefs)
		if wErr := writeFile(agentsMDPath, agentsMD); wErr != nil {
			return skills, commands, agents, instructions, fmt.Errorf("AGENTS.md: %w", wErr)
		}
		instructions = len(globalSections) + len(scopedRefs)
	}

	return skills, commands, agents, instructions, nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func copyDirSimple(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

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

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
