package artifacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// setupTestSource creates a temporary source tree mimicking .github/ structure.
func setupTestSource(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	skillDir := filepath.Join(dir, "skills", "security-review")
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

	agentDir := filepath.Join(dir, "agents")
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

	promptDir := filepath.Join(dir, "prompts")
	mustMkdir(t, promptDir)
	mustWrite(t, filepath.Join(promptDir, "aksel-component.prompt.md"), `---
name: aksel-component
description: Generate Aksel components
---

Create a responsive React component using Aksel Design System.
`)

	instrDir := filepath.Join(dir, "instructions")
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

	mustWrite(t, filepath.Join(dir, "copilot-instructions.md"), `---
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

	skillMD := filepath.Join(outputDir, "skills", "security-review", "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatalf("SKILL.md not found: %v", err)
	}
	if !strings.Contains(string(data), "name: security-review") {
		t.Error("SKILL.md missing name field")
	}

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

	agentMD := filepath.Join(outputDir, "agents", "nav-pilot.md")
	data, err := os.ReadFile(agentMD)
	if err != nil {
		t.Fatalf("agent file not found: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "description: Plan and build Nav applications") {
		t.Error("agent file missing description")
	}
	if !strings.Contains(content, "mode: primary") {
		t.Error("nav-pilot agent file should be a primary agent")
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
	if n != 3 {
		t.Fatalf("exported %d items, want 3", n)
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
	if !strings.Contains(content, "## Context Loading") {
		t.Error("AGENTS.md missing context loading section")
	}
	if !strings.Contains(content, "accessibility.md") {
		t.Error("AGENTS.md missing lazy-load reference to accessibility.md")
	}
	if !strings.Contains(content, "database.md") {
		t.Error("AGENTS.md missing lazy-load reference to database.md")
	}
	if strings.Contains(content, "## Accessibility") {
		t.Error("AGENTS.md should not inline accessibility section")
	}
	if strings.Contains(content, "## Database") {
		t.Error("AGENTS.md should not inline database section")
	}
	if strings.Contains(content, "applyTo:") {
		t.Error("AGENTS.md should not contain applyTo frontmatter")
	}

	accData, err := os.ReadFile(filepath.Join(outputDir, "instructions", "accessibility.md"))
	if err != nil {
		t.Fatalf("instructions/accessibility.md not found: %v", err)
	}
	if !strings.Contains(string(accData), "Always use semantic HTML") {
		t.Error("accessibility.md missing content")
	}
	if strings.Contains(string(accData), "applyTo:") {
		t.Error("accessibility.md should not contain applyTo frontmatter")
	}

	dbData, err := os.ReadFile(filepath.Join(outputDir, "instructions", "database.md"))
	if err != nil {
		t.Fatalf("instructions/database.md not found: %v", err)
	}
	if !strings.Contains(string(dbData), "Follow Flyway naming convention") {
		t.Error("database.md missing content")
	}
}

func TestExportInstructionsGlobalOnly(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "copilot-instructions.md"), `---
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

	for _, fn := range []func(string, string, bool) (int, error){
		exportSkills, exportPrompts, exportAgents, exportInstructions,
	} {
		if _, err := fn(sourceDir, outputDir, true); err != nil {
			t.Fatal(err)
		}
	}

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
	got := transformAgent([]byte(input), "nav-pilot")
	content := string(got)

	if !strings.Contains(content, "description: Plan and build Nav applications") {
		t.Error("missing description")
	}
	if !strings.Contains(content, "mode: primary") {
		t.Error("nav-pilot should be exported as a primary agent")
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

	scope := domain.ScopeRepo(outputDir)
	origClone := source.CloneRemoteFn
	t.Cleanup(func() { source.CloneRemoteFn = origClone })

	err := ExportOpenCode(scope, "", "", "dev", false, false, false)
	if err == nil {
		t.Fatal("expected error without --force, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExportSummary(t *testing.T) {
	tests := []struct {
		name                                   string
		skills, commands, agents, instructions int
		want                                   string
	}{
		{"all types", 3, 2, 4, 2, "3 skill(s), 2 command(s), 4 agent(s), AGENTS.md"},
		{"skills only", 1, 0, 0, 0, "1 skill(s)"},
		{"instructions only", 0, 0, 0, 5, "AGENTS.md"},
		{"nothing", 0, 0, 0, 0, "nothing to export"},
		{"commands and agents", 0, 3, 1, 0, "3 command(s), 1 agent(s)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExportSummary(tt.skills, tt.commands, tt.agents, tt.instructions)
			if got != tt.want {
				t.Errorf("ExportSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTransformAgentNoFrontmatter(t *testing.T) {
	input := "You are an agent without frontmatter.\n"
	got := transformAgent([]byte(input), "auth")
	if string(got) != input {
		t.Errorf("transformAgent with no frontmatter should return input unchanged\ngot:  %q\nwant: %q", string(got), input)
	}
}

func TestTransformAgentNoDescription(t *testing.T) {
	input := "---\nname: bare-agent\ntools:\n  - read\n---\n\nAgent body.\n"
	got := string(transformAgent([]byte(input), "auth"))
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
	scope := domain.ScopeRepo(t.TempDir())
	err := CmdExport("zed", scope, "", "", "dev", false, false, false)
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
		scope := domain.ScopeRepo(dir)
		got := OpenCodeOutputDir(scope)
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
		scope, err := domain.ScopeUser()
		if err != nil {
			t.Fatal(err)
		}
		got := OpenCodeOutputDir(scope)
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

	got, err := os.ReadFile(filepath.Join(outputDir, "skills", "my-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("SKILL.md not found: %v", err)
	}
	if string(got) != "# My Skill\n" {
		t.Errorf("SKILL.md = %q", string(got))
	}
}

func TestMaterializeOpenCode(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	skills, commands, agents, instructions, err := MaterializeOpenCode(sourceDir, outputDir)
	if err != nil {
		t.Fatalf("MaterializeOpenCode() error: %v", err)
	}
	if skills != 1 {
		t.Errorf("skills = %d, want 1", skills)
	}
	if commands != 1 {
		t.Errorf("commands = %d, want 1", commands)
	}
	if agents != 2 {
		t.Errorf("agents = %d, want 2", agents)
	}
	if instructions != 3 {
		t.Errorf("instructions = %d, want 3", instructions)
	}

	agentsMD := filepath.Join(outputDir, "AGENTS.md")
	data, err := os.ReadFile(agentsMD)
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	if !strings.Contains(string(data), "Auto-generated by nav-pilot") {
		t.Error("AGENTS.md missing auto-generated header")
	}

	if _, err := os.Stat(filepath.Join(outputDir, "skills", "security-review", "SKILL.md")); err != nil {
		t.Errorf("skill SKILL.md missing: %v", err)
	}

	cmdData, err := os.ReadFile(filepath.Join(outputDir, "commands", "aksel-component.md"))
	if err != nil {
		t.Fatalf("command file missing: %v", err)
	}
	if strings.Contains(string(cmdData), "name:") {
		t.Error("command file should not contain name: field")
	}

	agentData, err := os.ReadFile(filepath.Join(outputDir, "agents", "nav-pilot.md"))
	if err != nil {
		t.Fatalf("agent file missing: %v", err)
	}
	if !strings.Contains(string(agentData), "mode: primary") {
		t.Error("nav-pilot agent should be a primary agent")
	}
}

func TestMaterializeOpenCodeIdempotent(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	s1, c1, a1, i1, err := MaterializeOpenCode(sourceDir, outputDir)
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	first, err := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}

	s2, c2, a2, i2, err := MaterializeOpenCode(sourceDir, outputDir)
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	if s1 != s2 || c1 != c2 || a1 != a2 || i1 != i2 {
		t.Errorf("counts differ: first=%d/%d/%d/%d second=%d/%d/%d/%d",
			s1, c1, a1, i1, s2, c2, a2, i2)
	}

	second, _ := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))
	if string(first) != string(second) {
		t.Errorf("AGENTS.md not idempotent:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestMaterializeOpenCodeEmpty(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := t.TempDir()

	skills, commands, agents, instructions, err := MaterializeOpenCode(sourceDir, outputDir)
	if err != nil {
		t.Fatalf("unexpected error on empty source: %v", err)
	}
	if skills != 0 || commands != 0 || agents != 0 || instructions != 0 {
		t.Errorf("expected all zeros on empty source, got %d/%d/%d/%d", skills, commands, agents, instructions)
	}
}
