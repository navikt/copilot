package artifacts

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/navikt/copilot/cli/nav-pilot/internal/agentpakke"
	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
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

// refuseNonCanonicalPakke stops an export that would otherwise produce wrong or
// empty output.
//
// export reads a source's canonical agents//skills/instructions/prompts
// directories directly; it does not go through the agentpakke layout resolver
// that install and sync use. A source whose manifest puts its content anywhere
// else would therefore export whatever happens to sit at the canonical paths —
// in practice nothing at all, silently. Until export is migrated onto the
// manifest (M2), that is a hard error. A manifest declaring the canonical
// layout, and a manifest-less source, export exactly as before.
func refuseNonCanonicalPakke(src *source.Source) error {
	m, err := agentpakke.Load(src.Dir)
	if err != nil {
		// No manifest is the legacy case. A manifest that fails to load is
		// install and sync's fail-closed error to report, in the words those
		// commands use; export does not second-guess it here.
		return nil //nolint:nilerr // deliberate: only a usable manifest changes export's behaviour
	}
	if isCanonicalLayout(m.Layout) {
		return nil
	}
	label := src.Repo
	if label == "" {
		label = source.DefaultRepo
	}
	return fmt.Errorf(
		"export does not support agentpakke sources yet: %s ships an agentpakke (%q) whose content is not at the canonical agents/, skills/, instructions/, prompts/ paths.\n"+
			"Exporting it would silently produce an empty or wrong opencode tree, so nothing was written.\n\n"+
			"  Install the agentpakke instead:    %s\n"+
			"  Or export from a canonical source: %s",
		label, m.Name,
		domain.Bold("nav-pilot install --source "+label+" "+m.Name),
		domain.Bold("nav-pilot export opencode --source "+source.DefaultRepo))
}

// isCanonicalLayout reports whether a manifest's layout is the one export
// already reads: the canonical directory names. A Tier 2-only manifest declares
// no layout at all, which is equally not something export can read.
func isCanonicalLayout(l *agentpakke.Layout) bool {
	if l == nil {
		return false
	}
	for _, d := range []struct{ declared, canonical string }{
		{l.Agents, "agents"},
		{l.Skills, "skills"},
		{l.Instructions, "instructions"},
		{l.Prompts, "prompts"},
	} {
		declared := strings.TrimSpace(d.declared)
		if declared != "" && path.Clean(declared) != d.canonical {
			return false
		}
	}
	return true
}

// ExportOpenCode transforms Nav's .github/ artifacts into OpenCode-compatible .opencode/ format.
func ExportOpenCode(scope *domain.InstallScope, ref, sourceRepo, cliVersion string, dryRun, force, jsonOutput bool) error {
	src, err := source.ResolveSource(ref, sourceRepo, cliVersion)
	if err != nil {
		return err
	}
	defer src.Cleanup()

	if err := refuseNonCanonicalPakke(src); err != nil {
		return err
	}

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

// agentEntries lists the agents to materialize.
//
// It is [source.SourceResolver.List] minus [local.WorkerAgent] while local
// dispatch is off, which is every launch for the ~650 developers who never run
// `nav-pilot alpha local init`. That agent's description promises work that
// costs no premium requests, and it can only keep that promise when the launch
// has bound it to the local provider model — so shipping it to a machine with
// no local model offers a worker that quietly runs on the session's own model
// and bills for it.
//
// Filtered at the listing rather than at each write because both the export
// command and the launch-time sync materialize from this same list, and a gate
// in one of them is a gate in neither. Turning local off takes an already
// materialized copy back out: the sync deletes what its state file names and
// the new listing does not.
func agentEntries(sourceDir string) []source.Resolved {
	entries := source.NewSourceResolver(sourceDir).List(source.KindAgent)
	if local.Enabled() {
		return entries
	}
	return slices.DeleteFunc(entries, func(e source.Resolved) bool { return e.Name == local.WorkerAgent })
}

func exportAgents(sourceDir, outputDir string, dryRun bool) (int, error) {
	agents := agentEntries(sourceDir)
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

		transformed := transformAgent(data, entry.Name)

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

func transformAgent(data []byte, name string) []byte {
	fm, body, hasFM := source.SplitFrontmatter(data)
	if !hasFM {
		return data
	}

	description, _ := source.ExtractFrontmatterValue(fm, "description")
	if description == "" {
		description = "Nav agent"
	}

	mode := source.OpenCodeAgentMode(name, source.ActivePakke().PrimaryAgents("opencode"))
	newFM := source.BuildAgentFrontmatter(description, mode, openCodeAgentModel(fm, name))
	return source.Reassemble(newFM, body)
}

// openCodeAgentModel resolves an agent's frontmatter model to the
// provider-qualified id opencode wants, or "" when there is nothing safe to
// write.
//
// Nav agents declare a display name ("Claude Sonnet 4.6"); opencode wants
// "github-copilot/claude-sonnet-4.6". Before this, the rebuilt frontmatter
// carried description and mode only, so the model line on most Nav agents
// never reached opencode and per-agent model selection was inert there.
//
// An unrecognised name writes no model line rather than a guessed id, and says
// so on stderr: the agent still works on the session's model, and the author of
// the source repo is the one who has to fix the name. The warning goes to
// whoever runs the sync, which is the only place it can be noticed, since
// materialization has no other output channel.
func openCodeAgentModel(fm []byte, name string) string {
	declared, ok := source.ExtractFrontmatterValue(fm, "model")
	if !ok || strings.TrimSpace(declared) == "" {
		return ""
	}
	model := domain.OpenCodeModelForLabel(declared)
	if model == "" {
		fmt.Fprintf(os.Stderr, "%s agent %s declares model %q, which is not a known Copilot model. Materializing it without a model line; it will run on the session default. Fix the name in the source repo, or add the model to domain.KnownCopilotModels.\n",
			domain.Yellow("⚠"), name, declared)
	}
	return model
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
	globalInstr := filepath.Join(sourceDir, "copilot-instructions.md")
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
		if wErr := writeFile(dstPath, transformAgent(data, entry.Name)); wErr != nil {
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
