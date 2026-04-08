package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestSkill(t *testing.T, dir, name string, skillContent string, metadataJSON string) {
	t.Helper()
	skillDir := filepath.Join(dir, "skills", name)
	if err := os.MkdirAll(skillDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o600); err != nil {
		t.Fatal(err)
	}
	if metadataJSON != "" {
		if err := os.WriteFile(filepath.Join(skillDir, "metadata.json"), []byte(metadataJSON), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func setupTestSkillWithReferences(t *testing.T, dir, name string, refs []string) {
	t.Helper()
	skillContent := `---
name: ` + name + `
description: Test skill
---
# ` + name + `
Content here.
`
	refsJSON := `"references": [`
	for i, ref := range refs {
		if i > 0 {
			refsJSON += ", "
		}
		refsJSON += `"` + ref + `"`
	}
	refsJSON += `]`

	metadataJSON := `{"description": "Test skill", ` + refsJSON + `}`
	setupTestSkill(t, dir, name, skillContent, metadataJSON)

	// Create actual reference files
	refsDir := filepath.Join(dir, "skills", name, "references")
	if err := os.MkdirAll(refsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs {
		refPath := filepath.Join(dir, "skills", name, ref)
		if err := os.WriteFile(refPath, []byte("# Reference content"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadSkills_WithoutReferences(t *testing.T) {
	dir := t.TempDir()
	setupTestSkill(t, dir, "simple-skill", `---
name: simple-skill
description: A simple skill
---
# Simple Skill

Some content.
`, `{"description": "A simple skill"}`)

	gen := NewGenerator("navikt", "copilot", "main")
	skills, err := gen.loadSkills(filepath.Join(dir, "skills"))
	if err != nil {
		t.Fatalf("loadSkills() error: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Name != "simple-skill" {
		t.Errorf("expected name 'simple-skill', got %q", s.Name)
	}
	if len(s.References) != 0 {
		t.Errorf("expected 0 references, got %d", len(s.References))
	}
	if !strings.HasSuffix(s.RawURL, "SKILL.md") {
		t.Errorf("RawURL should end with SKILL.md, got %q", s.RawURL)
	}
}

func TestLoadSkills_WithReferences(t *testing.T) {
	dir := t.TempDir()
	refs := []string{"references/queries.md", "references/patterns.md"}
	setupTestSkillWithReferences(t, dir, "obs-setup", refs)

	gen := NewGenerator("navikt", "copilot", "main")
	skills, err := gen.loadSkills(filepath.Join(dir, "skills"))
	if err != nil {
		t.Fatalf("loadSkills() error: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	s := skills[0]
	if len(s.References) != 2 {
		t.Fatalf("expected 2 references, got %d", len(s.References))
	}

	for _, ref := range s.References {
		if ref.Path == "" {
			t.Error("reference path should not be empty")
		}
		if ref.RawURL == "" {
			t.Error("reference RawURL should not be empty")
		}
		if !strings.Contains(ref.RawURL, "raw.githubusercontent.com") {
			t.Errorf("reference RawURL should contain raw.githubusercontent.com, got %q", ref.RawURL)
		}
		if !strings.Contains(ref.RawURL, ref.Path) {
			t.Errorf("reference RawURL should contain the path %q, got %q", ref.Path, ref.RawURL)
		}
	}
}

func TestLoadSkills_WithoutMetadata(t *testing.T) {
	dir := t.TempDir()
	setupTestSkill(t, dir, "no-meta", `---
name: no-meta
description: No metadata
---
# No Meta
`, "")

	gen := NewGenerator("navikt", "copilot", "main")
	skills, err := gen.loadSkills(filepath.Join(dir, "skills"))
	if err != nil {
		t.Fatalf("loadSkills() error: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	if len(skills[0].References) != 0 {
		t.Errorf("expected 0 references without metadata, got %d", len(skills[0].References))
	}
}

func TestLoadSkills_SkipsDirWithoutSKILLmd(t *testing.T) {
	dir := t.TempDir()

	// Create a directory without SKILL.md
	emptyDir := filepath.Join(dir, "skills", "empty-dir")
	if err := os.MkdirAll(emptyDir, 0o750); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator("navikt", "copilot", "main")
	skills, err := gen.loadSkills(filepath.Join(dir, "skills"))
	if err != nil {
		t.Fatalf("loadSkills() error: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("expected 0 skills for dir without SKILL.md, got %d", len(skills))
	}
}

func TestLoadSkills_ReferencesFromMetadataNotFilesystem(t *testing.T) {
	dir := t.TempDir()

	// Create skill with references on disk but NOT in metadata
	setupTestSkill(t, dir, "meta-only", `---
name: meta-only
description: Test
---
# Test
`, `{"description": "Test"}`)

	// Add reference file on disk (but metadata has no references)
	refsDir := filepath.Join(dir, "skills", "meta-only", "references")
	if err := os.MkdirAll(refsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refsDir, "orphan.md"), []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator("navikt", "copilot", "main")
	skills, err := gen.loadSkills(filepath.Join(dir, "skills"))
	if err != nil {
		t.Fatalf("loadSkills() error: %v", err)
	}

	// Generator reads metadata, not filesystem — should have 0 references
	if len(skills[0].References) != 0 {
		t.Errorf("generator should read references from metadata only, got %d refs from filesystem", len(skills[0].References))
	}
}

func TestLoadSkills_ExcludedSkillsFiltered(t *testing.T) {
	dir := t.TempDir()

	setupTestSkill(t, dir, "included-skill", `---
name: included-skill
description: Included
---
# Included
`, `{"description": "Included"}`)

	setupTestSkill(t, dir, "excluded-skill", `---
name: excluded-skill
description: Excluded
---
# Excluded
`, `{"description": "Excluded", "excluded": true}`)

	gen := NewGenerator("navikt", "copilot", "main")
	skills, err := gen.loadSkills(filepath.Join(dir, "skills"))
	if err != nil {
		t.Fatalf("loadSkills() error: %v", err)
	}

	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (excluded filtered), got %d", len(skills))
	}

	if skills[0].Name != "included-skill" {
		t.Errorf("expected included-skill, got %q", skills[0].Name)
	}
}
