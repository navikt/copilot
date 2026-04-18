package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestSource creates a temporary source tree mimicking .github/ structure.
func setupTestSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Skills
	skillDir := filepath.Join(dir, ".github", "skills", "security-review")
	mustMkdir(t, skillDir)
	mustWrite(t, filepath.Join(skillDir, "SKILL.md"), `---
name: security-review
description: Security code review
license: MIT
metadata:
  domain: security
  tags:
    - security
    - review
---

# Security Review

Run security checks on your code.
`)
	mustWrite(t, filepath.Join(skillDir, "checklist.md"), "## Checklist\n\n- [ ] Check for secrets\n")

	// Agents
	agentDir := filepath.Join(dir, ".github", "agents")
	mustMkdir(t, agentDir)
	mustWrite(t, filepath.Join(agentDir, "nav-pilot.agent.md"), `---
name: nav-pilot
description: Plan and build Nav applications
tools:
  - execute
  - read
  - edit
  - search
---

You are nav-pilot, an expert on Nav's platform.
`)
	mustWrite(t, filepath.Join(agentDir, "auth.agent.md"), `---
name: auth
description: Authentication and authorization expert
tools:
  - read
  - search
---

You handle auth flows for Nav apps.
`)

	// Prompts
	promptDir := filepath.Join(dir, ".github", "prompts")
	mustMkdir(t, promptDir)
	mustWrite(t, filepath.Join(promptDir, "aksel-component.prompt.md"), `---
name: aksel-component
description: Generate Aksel components
---

Create a responsive React component using Aksel Design System.
`)

	// Instructions
	instrDir := filepath.Join(dir, ".github", "instructions")
	mustMkdir(t, instrDir)
	mustWrite(t, filepath.Join(instrDir, "accessibility.instructions.md"), `---
applyTo: "**/*.tsx"
---

# Accessibility Standards

Always use semantic HTML elements.
`)
	mustWrite(t, filepath.Join(instrDir, "database.instructions.md"), `---
applyTo: "**/db/migration/**/*.sql"
---

# Database Migration Standards

Follow Flyway naming convention.
`)

	// Global instructions
	mustWrite(t, filepath.Join(dir, ".github", "copilot-instructions.md"), `---
applyTo: "**"
---

# Global Nav Standards

These apply everywhere.
`)

	return dir
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExportSkills(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	n, err := exportSkills(sourceDir, outputDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("exported %d skills, want 1", n)
	}

	// Check SKILL.md was copied
	skillMD := filepath.Join(outputDir, "skills", "security-review", "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatalf("SKILL.md not found: %v", err)
	}
	if !strings.Contains(string(data), "name: security-review") {
		t.Error("SKILL.md missing name field")
	}

	// Check reference file was copied
	checklist := filepath.Join(outputDir, "skills", "security-review", "checklist.md")
	if _, err := os.Stat(checklist); os.IsNotExist(err) {
		t.Error("reference file checklist.md not copied")
	}
}

func TestExportPrompts(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	n, err := exportPrompts(sourceDir, outputDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("exported %d prompts, want 1", n)
	}

	commandMD := filepath.Join(outputDir, "commands", "aksel-component.md")
	data, err := os.ReadFile(commandMD)
	if err != nil {
		t.Fatalf("command file not found: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "name:") {
		t.Error("command file should not contain name: field")
	}
	if !strings.Contains(content, "description: Generate Aksel components") {
		t.Error("command file missing description")
	}
	if !strings.Contains(content, "Create a responsive React component") {
		t.Error("command file missing body content")
	}
}

func TestExportAgents(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	n, err := exportAgents(sourceDir, outputDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("exported %d agents, want 2", n)
	}

	// Check nav-pilot agent
	agentMD := filepath.Join(outputDir, "agents", "nav-pilot.md")
	data, err := os.ReadFile(agentMD)
	if err != nil {
		t.Fatalf("agent file not found: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "description: Plan and build Nav applications") {
		t.Error("agent file missing description")
	}
	if !strings.Contains(content, "mode: subagent") {
		t.Error("agent file missing mode: subagent")
	}
	if strings.Contains(content, "tools:") {
		t.Error("agent file should not contain tools: field")
	}
	if !strings.Contains(content, "You are nav-pilot") {
		t.Error("agent file missing body content")
	}
}

func TestExportInstructions(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	n, err := exportInstructions(sourceDir, outputDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 { // global + accessibility + database
		t.Fatalf("merged %d sections, want 3", n)
	}

	agentsMD := filepath.Join(outputDir, "AGENTS.md")
	data, err := os.ReadFile(agentsMD)
	if err != nil {
		t.Fatalf("AGENTS.md not found: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Auto-generated by nav-pilot") {
		t.Error("AGENTS.md missing auto-generated header")
	}
	if !strings.Contains(content, "## Global Instructions") {
		t.Error("AGENTS.md missing global instructions section")
	}
	if !strings.Contains(content, "## Accessibility") {
		t.Error("AGENTS.md missing accessibility section")
	}
	if !strings.Contains(content, "## Database") {
		t.Error("AGENTS.md missing database section")
	}
	if strings.Contains(content, "applyTo:") {
		t.Error("AGENTS.md should not contain applyTo frontmatter")
	}

	// Verify order: global first, then alphabetical
	globalIdx := strings.Index(content, "## Global Instructions")
	accIdx := strings.Index(content, "## Accessibility")
	dbIdx := strings.Index(content, "## Database")
	if globalIdx > accIdx {
		t.Error("global instructions should come before accessibility")
	}
	if accIdx > dbIdx {
		t.Error("sections not in alphabetical order: accessibility should come before database")
	}
}

func TestExportInstructionsGlobalOnly(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".github"))
	mustWrite(t, filepath.Join(dir, ".github", "copilot-instructions.md"), `---
applyTo: "**"
---

# Global Standards

These apply everywhere.
`)

	outputDir := t.TempDir()
	n, err := exportInstructions(dir, outputDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("merged %d sections, want 1", n)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not found: %v", err)
	}
	if !strings.Contains(string(data), "## Global Instructions") {
		t.Error("AGENTS.md missing global instructions")
	}
}

func TestExportDryRun(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	// Run all exports in dry-run mode
	for _, fn := range []func(string, string, bool) (int, error){
		exportSkills, exportPrompts, exportAgents, exportInstructions,
	} {
		if _, err := fn(sourceDir, outputDir, true); err != nil {
			t.Fatal(err)
		}
	}

	// Verify nothing was written
	entries, _ := os.ReadDir(outputDir)
	if len(entries) > 0 {
		t.Errorf("dry run created %d entries, want 0", len(entries))
	}
}

func TestExportEmptySource(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := t.TempDir()

	for _, fn := range []struct {
		name string
		fn   func(string, string, bool) (int, error)
	}{
		{"skills", exportSkills},
		{"prompts", exportPrompts},
		{"agents", exportAgents},
		{"instructions", exportInstructions},
	} {
		n, err := fn.fn(sourceDir, outputDir, false)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", fn.name, err)
		}
		if n != 0 {
			t.Errorf("%s: exported %d, want 0", fn.name, n)
		}
	}
}

func TestTransformAgent(t *testing.T) {
	input := `---
name: nav-pilot
description: Plan and build Nav applications
tools:
  - execute
  - read
  - edit
---

You are nav-pilot, an expert on Nav's platform.
`
	got := transformAgent([]byte(input))
	content := string(got)

	if !strings.Contains(content, "description: Plan and build Nav applications") {
		t.Error("missing description")
	}
	if !strings.Contains(content, "mode: subagent") {
		t.Error("missing mode: subagent")
	}
	if strings.Contains(content, "name:") {
		t.Error("should not contain name:")
	}
	if strings.Contains(content, "tools:") {
		t.Error("should not contain tools:")
	}
	if !strings.Contains(content, "You are nav-pilot") {
		t.Error("body content missing")
	}
}

func TestTransformPromptFull(t *testing.T) {
	input := `---
name: aksel-component
description: Generate Aksel components
---

Create a responsive React component using Aksel Design System.
`
	got := transformPrompt([]byte(input))
	content := string(got)

	if strings.Contains(content, "name:") {
		t.Error("should not contain name:")
	}
	if !strings.Contains(content, "description: Generate Aksel components") {
		t.Error("missing description")
	}
	if !strings.Contains(content, "Create a responsive React component") {
		t.Error("body content missing")
	}
}

func TestExportBlocksWithoutForce(t *testing.T) {
	outputDir := t.TempDir()
	openCodeDir := filepath.Join(outputDir, ".opencode")
	mustMkdir(t, openCodeDir)
	mustWrite(t, filepath.Join(openCodeDir, "existing.md"), "existing content")

	scope := ScopeRepo(outputDir)

	// Without --force, should error
	err := exportOpenCode(scope, "", "", false, false, false)
	if err == nil {
		t.Fatal("expected error without --force, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExportSummary(t *testing.T) {
	tests := []struct {
		name                                       string
		skills, commands, agents, instructions      int
		want                                       string
	}{
		{"all types", 3, 2, 4, 2, "3 skill(s), 2 command(s), 4 agent(s), AGENTS.md"},
		{"skills only", 1, 0, 0, 0, "1 skill(s)"},
		{"instructions only", 0, 0, 0, 5, "AGENTS.md"},
		{"nothing", 0, 0, 0, 0, "nothing to export"},
		{"commands and agents", 0, 3, 1, 0, "3 command(s), 1 agent(s)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exportSummary(tt.skills, tt.commands, tt.agents, tt.instructions)
			if got != tt.want {
				t.Errorf("exportSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTransformAgentNoFrontmatter(t *testing.T) {
	input := "You are an agent without frontmatter.\n"
	got := transformAgent([]byte(input))
	if string(got) != input {
		t.Errorf("transformAgent with no frontmatter should return input unchanged\ngot:  %q\nwant: %q", string(got), input)
	}
}

func TestTransformAgentNoDescription(t *testing.T) {
	input := "---\nname: bare-agent\ntools:\n  - read\n---\n\nAgent body.\n"
	got := string(transformAgent([]byte(input)))
	if !strings.Contains(got, "description: Nav agent") {
		t.Error("expected fallback description 'Nav agent'")
	}
	if !strings.Contains(got, "mode: subagent") {
		t.Error("expected mode: subagent")
	}
}

func TestTransformPromptNoFrontmatter(t *testing.T) {
	input := "Just a prompt with no frontmatter.\n"
	got := transformPrompt([]byte(input))
	if string(got) != input {
		t.Errorf("transformPrompt with no frontmatter should return input unchanged\ngot:  %q\nwant: %q", string(got), input)
	}
}

func TestCopyDirSimpleRejectsSymlinks(t *testing.T) {
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "real.txt"), "real content")
	if err := os.Symlink(filepath.Join(src, "real.txt"), filepath.Join(src, "link.txt")); err != nil {
		t.Skip("cannot create symlink on this OS")
	}

	dst := t.TempDir()
	err := copyDirSimple(src, dst)
	if err == nil {
		t.Fatal("expected error for symlink in source, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("expected symlink error, got: %v", err)
	}
}

func TestCmdExportUnknownFormat(t *testing.T) {
	scope := ScopeRepo(t.TempDir())
	err := cmdExport("zed", scope, "", "", false, false, false)
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown export format") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOpenCodeOutputDir(t *testing.T) {
	t.Run("repo scope", func(t *testing.T) {
		dir := t.TempDir()
		scope := ScopeRepo(dir)
		got := openCodeOutputDir(scope)
		want := filepath.Join(dir, ".opencode")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("user scope", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot determine home dir")
		}
		scope, err := ScopeUser()
		if err != nil {
			t.Fatal(err)
		}
		got := openCodeOutputDir(scope)
		want := filepath.Join(home, ".config", "opencode")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestTitleCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "Hello World"},
		{"accessibility", "Accessibility"},
		{"database migration", "Database Migration"},
		{"github actions", "Github Actions"},
	}
	for _, tt := range tests {
		got := titleCase(tt.input)
		if got != tt.want {
			t.Errorf("titleCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExportSkills_RootLevel(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := t.TempDir()

	// Root-level skill
	skillDir := filepath.Join(sourceDir, "skills", "my-skill")
	mustMkdir(t, skillDir)
	mustWrite(t, filepath.Join(skillDir, "SKILL.md"), "# My Skill\n")
	mustWrite(t, filepath.Join(skillDir, "reference.md"), "## Reference\n")

	n, err := exportSkills(sourceDir, outputDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("exported %d skills, want 1", n)
	}

	// Verify output
	got, err := os.ReadFile(filepath.Join(outputDir, "skills", "my-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("SKILL.md not found: %v", err)
	}
	if string(got) != "# My Skill\n" {
		t.Errorf("SKILL.md = %q", string(got))
	}
}

func TestExportSkills_MergesBothDirs(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := t.TempDir()

	// Root-level skill
	mustMkdir(t, filepath.Join(sourceDir, "skills", "alpha"))
	mustWrite(t, filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), "# Alpha root\n")

	// Legacy skill
	mustMkdir(t, filepath.Join(sourceDir, ".github", "skills", "beta"))
	mustWrite(t, filepath.Join(sourceDir, ".github", "skills", "beta", "SKILL.md"), "# Beta legacy\n")

	n, err := exportSkills(sourceDir, outputDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("exported %d skills, want 2", n)
	}
}

func TestExportSkills_InvalidRootFallsBack(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := t.TempDir()

	// Root dir exists but no SKILL.md — invalid
	mustMkdir(t, filepath.Join(sourceDir, "skills", "broken"))

	// Legacy has valid SKILL.md
	mustMkdir(t, filepath.Join(sourceDir, ".github", "skills", "broken"))
	mustWrite(t, filepath.Join(sourceDir, ".github", "skills", "broken", "SKILL.md"), "# Legacy\n")

	n, err := exportSkills(sourceDir, outputDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("exported %d skills, want 1", n)
	}

	got, _ := os.ReadFile(filepath.Join(outputDir, "skills", "broken", "SKILL.md"))
	if string(got) != "# Legacy\n" {
		t.Errorf("expected legacy content, got %q", string(got))
	}
}
