package discovery

import (
	"strings"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	service := NewService("navikt", "copilot", "main", "")

	if err := service.LoadManifest(); err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	manifest := service.GetManifest()
	if manifest == nil {
		t.Fatal("GetManifest() returned nil")
	}

	if len(manifest.Agents) == 0 {
		t.Error("Expected at least one agent in embedded manifest")
	}
}

func TestSearch(t *testing.T) {
	service := NewService("navikt", "copilot", "main", "")
	if err := service.LoadManifest(); err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	results := service.Search("kafka", "", nil)
	if len(results) == 0 {
		t.Error("Search() returned 0 results for 'kafka'")
	}
}

func TestListByType(t *testing.T) {
	service := NewService("navikt", "copilot", "main", "")
	if err := service.LoadManifest(); err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	results := service.ListByType(TypeAgent, "")
	if len(results) == 0 {
		t.Error("ListByType() returned 0 agents")
	}
}

func TestSkillReferencesInManifest(t *testing.T) {
	service := NewService("navikt", "copilot", "main", "")
	if err := service.LoadManifest(); err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	skills := service.ListByType(TypeSkill, "")
	if len(skills) == 0 {
		t.Fatal("Expected at least one skill in manifest")
	}

	var withRefs []Customization
	for _, s := range skills {
		if len(s.References) > 0 {
			withRefs = append(withRefs, s)
		}
	}

	if len(withRefs) == 0 {
		t.Fatal("Expected at least one skill with references in manifest")
	}

	for _, s := range withRefs {
		for _, ref := range s.References {
			if ref.Path == "" {
				t.Errorf("Skill %q has reference with empty path", s.Name)
			}
			if ref.RawURL == "" {
				t.Errorf("Skill %q has reference with empty RawURL", s.Name)
			}
			if !strings.Contains(ref.RawURL, "raw.githubusercontent.com") {
				t.Errorf("Skill %q reference %q has invalid RawURL: %s", s.Name, ref.Path, ref.RawURL)
			}
		}
	}
}

func newTestService(skills []Customization) *Service {
	return &Service{
		manifest: &CustomizationsManifest{
			Skills: skills,
		},
	}
}

func TestGenerateInstallationGuide_SkillWithoutReferences(t *testing.T) {
	service := newTestService([]Customization{
		{
			Type:        TypeSkill,
			Name:        "aksel-spacing",
			DisplayName: "aksel-spacing",
			Description: "Responsive layout",
			RawURL:      "https://raw.githubusercontent.com/navikt/copilot/main/.github/skills/aksel-spacing/SKILL.md",
		},
	})

	guide, err := service.GenerateInstallationGuide(TypeSkill, "aksel-spacing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(guide, `mkdir -p ".github/skills/aksel-spacing"`) {
		t.Error("guide should create skill directory")
	}
	if !strings.Contains(guide, "curl -fsSL") {
		t.Error("guide should use curl -fsSL")
	}
	if !strings.Contains(guide, "SKILL.md") {
		t.Error("guide should download SKILL.md")
	}
	if strings.Contains(guide, "mkdir -p \".github/skills/aksel-spacing/references\"") {
		t.Error("guide should NOT create references dir for skill without references")
	}
}

func TestGenerateInstallationGuide_SkillWithReferences(t *testing.T) {
	service := newTestService([]Customization{
		{
			Type:        TypeSkill,
			Name:        "observability-setup",
			DisplayName: "observability-setup",
			Description: "Prometheus metrics and tracing",
			RawURL:      "https://raw.githubusercontent.com/navikt/copilot/main/.github/skills/observability-setup/SKILL.md",
			References: []SkillReference{
				{
					Path:   "references/grafana-queries.md",
					RawURL: "https://raw.githubusercontent.com/navikt/copilot/main/.github/skills/observability-setup/references/grafana-queries.md",
				},
				{
					Path:   "references/production-patterns.md",
					RawURL: "https://raw.githubusercontent.com/navikt/copilot/main/.github/skills/observability-setup/references/production-patterns.md",
				},
			},
		},
	})

	guide, err := service.GenerateInstallationGuide(TypeSkill, "observability-setup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(guide, `mkdir -p ".github/skills/observability-setup"`) {
		t.Error("guide should create skill directory")
	}
	if !strings.Contains(guide, `mkdir -p ".github/skills/observability-setup/references"`) {
		t.Error("guide should create references subdirectory")
	}
	if !strings.Contains(guide, "SKILL.md") {
		t.Error("guide should download SKILL.md")
	}
	if !strings.Contains(guide, "grafana-queries.md") {
		t.Error("guide should download grafana-queries.md reference")
	}
	if !strings.Contains(guide, "production-patterns.md") {
		t.Error("guide should download production-patterns.md reference")
	}
	if strings.Count(guide, "curl -fsSL") != 3 {
		t.Errorf("guide should have 3 curl commands (SKILL.md + 2 refs), got %d", strings.Count(guide, "curl -fsSL"))
	}
}

func TestGenerateInstallationGuide_NotFound(t *testing.T) {
	service := newTestService(nil)

	_, err := service.GenerateInstallationGuide(TypeSkill, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestGenerateInstallationGuide_ManifestNotLoaded(t *testing.T) {
	service := &Service{}

	_, err := service.GenerateInstallationGuide(TypeSkill, "any")
	if err == nil {
		t.Error("expected error when manifest not loaded")
	}
}
