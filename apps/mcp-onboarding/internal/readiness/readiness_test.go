package readiness

import (
	"strings"
	"testing"

	"github.com/navikt/copilot/mcp-onboarding/internal/discovery"
)

func TestAssess_EmptyRepo(t *testing.T) {
	contents := &RepoContents{}
	report := Assess(contents)

	if report.Level != LevelNone {
		t.Errorf("expected level none, got %s", report.Level)
	}
	if report.Score != 0 {
		t.Errorf("expected score 0, got %d", report.Score)
	}
	if report.VerificationScore != 0 {
		t.Errorf("expected verification score 0, got %d", report.VerificationScore)
	}
	if len(report.Recommendations) != 14 {
		t.Errorf("expected 14 recommendations, got %d", len(report.Recommendations))
	}
}

func TestAssess_BasicRepo(t *testing.T) {
	contents := &RepoContents{
		CopilotInstructions: true,
		HasReadme:           true,
	}
	report := Assess(contents)

	if report.Level != LevelBasic {
		t.Errorf("expected level basic, got %s", report.Level)
	}
	if report.Score != 1 {
		t.Errorf("expected score 1, got %d", report.Score)
	}
	if report.VerificationScore != 1 {
		t.Errorf("expected verification score 1, got %d", report.VerificationScore)
	}
	if len(report.Recommendations) != 12 {
		t.Errorf("expected 12 recommendations, got %d", len(report.Recommendations))
	}
}

func TestAssess_IntermediateRepo(t *testing.T) {
	contents := &RepoContents{
		CopilotInstructions: true,
		InstructionsCount:   2,
		AgentsCount:         1,
		HasReadme:           true,
		HasCIWorkflows:      true,
		HasLinterConfig:     true,
	}
	report := Assess(contents)

	if report.Level != LevelIntermediate {
		t.Errorf("expected level intermediate, got %s", report.Level)
	}
	if report.Score != 3 {
		t.Errorf("expected score 3, got %d", report.Score)
	}
	if report.VerificationScore != 3 {
		t.Errorf("expected verification score 3, got %d", report.VerificationScore)
	}
}

func TestAssess_AdvancedRepo(t *testing.T) {
	contents := &RepoContents{
		CopilotInstructions: true,
		InstructionsCount:   3,
		AgentsCount:         2,
		PromptsCount:        1,
		SkillsCount:         1,
		SetupSteps:          true,
		HooksConfig:         true,
		AgentsMD:            true,
		HasCIWorkflows:      true,
		HasLinterConfig:     true,
		HasTypeChecking:     true,
		HasTestConfig:       true,
		HasDependabot:       true,
		HasReadme:           true,
	}
	report := Assess(contents)

	if report.Level != LevelAdvanced {
		t.Errorf("expected level advanced, got %s", report.Level)
	}
	if report.Score != 8 {
		t.Errorf("expected score 8, got %d", report.Score)
	}
	if report.VerificationScore != 6 {
		t.Errorf("expected verification score 6, got %d", report.VerificationScore)
	}
	if len(report.Recommendations) != 0 {
		t.Errorf("expected 0 recommendations, got %d", len(report.Recommendations))
	}
}

func TestAssess_FileChecks(t *testing.T) {
	contents := &RepoContents{
		CopilotInstructions: true,
		InstructionsCount:   3,
		Languages:           []string{"Go", "TypeScript"},
	}
	report := Assess(contents)

	if len(report.Files) != 8 {
		t.Fatalf("expected 8 file checks, got %d", len(report.Files))
	}

	checks := map[string]struct {
		exists bool
		count  int
	}{
		".github/copilot-instructions.md":           {true, 0},
		".github/instructions/":                     {true, 3},
		".github/agents/":                           {false, 0},
		".github/prompts/":                          {false, 0},
		".github/skills/":                           {false, 0},
		".github/workflows/copilot-setup-steps.yml": {false, 0},
		".github/hooks/copilot-hooks.json":          {false, 0},
		"AGENTS.md":                                 {false, 0},
	}

	for _, f := range report.Files {
		expected, ok := checks[f.Path]
		if !ok {
			t.Errorf("unexpected file check path: %s", f.Path)
			continue
		}
		if f.Exists != expected.exists {
			t.Errorf("path %s: expected exists=%v, got %v", f.Path, expected.exists, f.Exists)
		}
		if f.Count != expected.count {
			t.Errorf("path %s: expected count=%d, got %d", f.Path, expected.count, f.Count)
		}
	}

	if len(report.Languages) != 2 {
		t.Errorf("expected 2 languages, got %d", len(report.Languages))
	}
}

func TestSuggestCustomizations_TypeScript(t *testing.T) {
	manifest := &discovery.CustomizationsManifest{
		Agents: []discovery.Customization{
			{Name: "aksel-agent", DisplayName: "@aksel-agent", Type: "agent", InstallURL: "vscode:..."},
			{Name: "security-champion", DisplayName: "@security-champion", Type: "agent", InstallURL: "vscode:..."},
			{Name: "nais-platform", DisplayName: "@nais-platform", Type: "agent", InstallURL: "vscode:..."},
			{Name: "observability", DisplayName: "@observability", Type: "agent", InstallURL: "vscode:..."},
		},
		Instructions: []discovery.Customization{
			{Name: "nextjs-aksel", DisplayName: "nextjs-aksel", Type: "instruction", InstallURL: "vscode:..."},
			{Name: "testing", DisplayName: "testing", Type: "instruction", InstallURL: "vscode:..."},
			{Name: "database", DisplayName: "database", Type: "instruction", InstallURL: "vscode:..."},
		},
	}

	contents := &RepoContents{Languages: []string{"TypeScript", "CSS"}}
	suggestions := SuggestCustomizations(contents, manifest)

	names := make(map[string]bool)
	for _, s := range suggestions {
		names[s.Name] = true
	}

	if !names["@aksel-agent"] {
		t.Error("expected aksel-agent suggestion for TypeScript repo")
	}
	if !names["nextjs-aksel"] {
		t.Error("expected nextjs-aksel suggestion for TypeScript repo")
	}
	if !names["@security-champion"] {
		t.Error("expected security-champion for all repos")
	}
}

func TestSuggestCustomizations_Kotlin(t *testing.T) {
	manifest := &discovery.CustomizationsManifest{
		Agents: []discovery.Customization{
			{Name: "kafka-events", DisplayName: "@kafka-events", Type: "agent", InstallURL: "vscode:..."},
			{Name: "security-champion", DisplayName: "@security-champion", Type: "agent", InstallURL: "vscode:..."},
			{Name: "nais-platform", DisplayName: "@nais-platform", Type: "agent", InstallURL: "vscode:..."},
			{Name: "observability", DisplayName: "@observability", Type: "agent", InstallURL: "vscode:..."},
		},
		Instructions: []discovery.Customization{
			{Name: "kotlin-ktor", DisplayName: "kotlin-ktor", Type: "instruction", InstallURL: "vscode:..."},
			{Name: "testing", DisplayName: "testing", Type: "instruction", InstallURL: "vscode:..."},
			{Name: "database", DisplayName: "database", Type: "instruction", InstallURL: "vscode:..."},
		},
	}

	contents := &RepoContents{Languages: []string{"Kotlin"}}
	suggestions := SuggestCustomizations(contents, manifest)

	names := make(map[string]bool)
	for _, s := range suggestions {
		names[s.Name] = true
	}

	if !names["kotlin-ktor"] {
		t.Error("expected kotlin-ktor suggestion for Kotlin repo")
	}
	if !names["@kafka-events"] {
		t.Error("expected kafka-events suggestion for Kotlin repo")
	}
}

func TestSuggestCustomizations_NilManifest(t *testing.T) {
	contents := &RepoContents{Languages: []string{"Go"}}
	suggestions := SuggestCustomizations(contents, nil)

	if suggestions != nil {
		t.Errorf("expected nil suggestions with nil manifest, got %d", len(suggestions))
	}
}

func TestFormatReport(t *testing.T) {
	report := &Report{
		Owner:                "navikt",
		Repo:                 "my-app",
		Level:                LevelBasic,
		Score:                2,
		MaxScore:             8,
		VerificationScore:    1,
		VerificationMaxScore: 6,
		Files: []FileCheck{
			{Path: ".github/copilot-instructions.md", Exists: true},
			{Path: ".github/instructions/", Exists: true, Count: 2},
			{Path: ".github/agents/", Exists: false},
		},
		Verifications: []FileCheck{
			{Path: "CI/CD workflows", Exists: true},
			{Path: "Linter configuration", Exists: false},
		},
		Languages:       []string{"Kotlin", "Dockerfile"},
		Recommendations: []string{"Add AGENTS.md"},
		Suggestions: []Suggestion{
			{Type: "instruction", Name: "kotlin-ktor", Reason: "Kotlin detected", InstallURL: "vscode:..."},
		},
	}

	output := FormatReport(report)

	if !strings.Contains(output, "navikt/my-app") {
		t.Error("expected repo name in output")
	}
	if !strings.Contains(output, "Basic") {
		t.Error("expected Basic level in output")
	}
	if !strings.Contains(output, "3/14") {
		t.Error("expected combined score 3/14 in output")
	}
	if !strings.Contains(output, "Agent Customizations (2/8)") {
		t.Error("expected customizations section header")
	}
	if !strings.Contains(output, "Verification Infrastructure (1/6)") {
		t.Error("expected verification section header")
	}
	if !strings.Contains(output, "✅ .github/copilot-instructions.md") {
		t.Error("expected checkmark for existing file")
	}
	if !strings.Contains(output, "❌ .github/agents/") {
		t.Error("expected X for missing file")
	}
	if !strings.Contains(output, "(2 files)") {
		t.Error("expected file count for instructions")
	}
	if !strings.Contains(output, "✅ CI/CD workflows") {
		t.Error("expected checkmark for CI/CD in verification section")
	}
	if !strings.Contains(output, "❌ Linter configuration") {
		t.Error("expected X for linter in verification section")
	}
	if !strings.Contains(output, "Kotlin") {
		t.Error("expected language in output")
	}
	if !strings.Contains(output, "kotlin-ktor") {
		t.Error("expected suggestion in output")
	}
}

func TestAssessRepoLight_AllPresent(t *testing.T) {
	level := AssessRepoLight(true, true, true)
	if level != LevelAdvanced {
		t.Errorf("expected Advanced, got %s", level)
	}
}

func TestAssessRepoLight_None(t *testing.T) {
	level := AssessRepoLight(false, false, false)
	if level != LevelNone {
		t.Errorf("expected None, got %s", level)
	}
}

func TestAssessRepoLight_OneFile(t *testing.T) {
	level := AssessRepoLight(true, false, false)
	if level != LevelBasic {
		t.Errorf("expected Basic, got %s", level)
	}
}

func TestAssessRepoLight_TwoFiles(t *testing.T) {
	level := AssessRepoLight(true, true, false)
	if level != LevelIntermediate {
		t.Errorf("expected Intermediate, got %s", level)
	}
}

func TestFormatTeamSummary(t *testing.T) {
	summary := &TeamSummary{
		Org:   "navikt",
		Team:  "dagpenger",
		Total: 3,
		Repos: []RepoReadiness{
			{Repo: "dp-inntekt", AgentsMD: true, CopilotMD: true, SetupSteps: true, Level: LevelAdvanced},
			{Repo: "dp-soknad", AgentsMD: true, CopilotMD: false, SetupSteps: false, Level: LevelBasic},
			{Repo: "dp-ui", AgentsMD: false, CopilotMD: false, SetupSteps: false, Level: LevelNone},
		},
	}

	output := FormatTeamSummary(summary)

	if !strings.Contains(output, "navikt/dagpenger") {
		t.Error("expected org/team in header")
	}
	if !strings.Contains(output, "3 repos") {
		t.Error("expected repo count")
	}
	if !strings.Contains(output, "AGENTS.md | 2 | 66%") {
		t.Error("expected AGENTS.md count and percentage")
	}
	if !strings.Contains(output, "🟢 Advanced: 1 repos") {
		t.Error("expected Advanced count")
	}
	if !strings.Contains(output, "🔴 None: 1 repos") {
		t.Error("expected None count")
	}
	if !strings.Contains(output, "dp-inntekt") {
		t.Error("expected repo names in table")
	}
}

func TestFormatTeamSummary_Empty(t *testing.T) {
	summary := &TeamSummary{
		Org:   "navikt",
		Team:  "empty-team",
		Total: 0,
		Repos: nil,
	}

	output := FormatTeamSummary(summary)

	if !strings.Contains(output, "0 repos") {
		t.Error("expected 0 repos")
	}
}

func TestRecommend_NextJSSpecific(t *testing.T) {
	contents := &RepoContents{
		Languages:      []string{"TypeScript", "CSS"},
		HasPackageJSON: true,
		HasNextConfig:  true,
	}

	recs := recommend(contents)

	found := false
	for _, r := range recs {
		if strings.Contains(r, "Next.js") && strings.Contains(r, "bundled docs") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Next.js-specific recommendation mentioning bundled docs")
	}
}

func TestRecommend_SpringBootSpecific(t *testing.T) {
	contents := &RepoContents{
		Languages:    []string{"Kotlin"},
		HasGradleKts: true,
		HasAppYml:    true,
	}

	recs := recommend(contents)

	found := false
	for _, r := range recs {
		if strings.Contains(r, "Spring Boot") && strings.Contains(r, "layered architecture") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Spring Boot-specific recommendation mentioning layered architecture")
	}
}

func TestRecommend_KtorSpecific(t *testing.T) {
	contents := &RepoContents{
		Languages:    []string{"Kotlin"},
		HasGradleKts: true,
	}

	recs := recommend(contents)

	found := false
	for _, r := range recs {
		if strings.Contains(r, "Ktor") && strings.Contains(r, "ApplicationBuilder") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Ktor-specific recommendation mentioning ApplicationBuilder")
	}
}

func TestSuggestCustomizations_SpringBoot(t *testing.T) {
	manifest := &discovery.CustomizationsManifest{
		Agents: []discovery.Customization{
			{Name: "kafka-events", DisplayName: "@kafka-events", Type: "agent", InstallURL: "vscode:..."},
			{Name: "security-champion", DisplayName: "@security-champion", Type: "agent", InstallURL: "vscode:..."},
			{Name: "nais-platform", DisplayName: "@nais-platform", Type: "agent", InstallURL: "vscode:..."},
			{Name: "observability", DisplayName: "@observability", Type: "agent", InstallURL: "vscode:..."},
		},
		Instructions: []discovery.Customization{
			{Name: "kotlin-ktor", DisplayName: "kotlin-ktor", Type: "instruction", InstallURL: "vscode:..."},
			{Name: "testing", DisplayName: "testing", Type: "instruction", InstallURL: "vscode:..."},
			{Name: "database", DisplayName: "database", Type: "instruction", InstallURL: "vscode:..."},
		},
	}

	contents := &RepoContents{
		Languages:    []string{"Kotlin"},
		HasGradleKts: true,
		HasAppYml:    true,
	}
	suggestions := SuggestCustomizations(contents, manifest)

	var ktReason string
	for _, s := range suggestions {
		if s.Name == "kotlin-ktor" {
			ktReason = s.Reason
		}
	}

	if !strings.Contains(ktReason, "Spring Boot") {
		t.Errorf("expected Spring Boot-specific reason for kotlin-ktor, got %q", ktReason)
	}
}

func TestSuggestCustomizations_NextJS(t *testing.T) {
	manifest := &discovery.CustomizationsManifest{
		Agents: []discovery.Customization{
			{Name: "aksel-agent", DisplayName: "@aksel-agent", Type: "agent", InstallURL: "vscode:..."},
			{Name: "security-champion", DisplayName: "@security-champion", Type: "agent", InstallURL: "vscode:..."},
			{Name: "nais-platform", DisplayName: "@nais-platform", Type: "agent", InstallURL: "vscode:..."},
			{Name: "observability", DisplayName: "@observability", Type: "agent", InstallURL: "vscode:..."},
		},
		Instructions: []discovery.Customization{
			{Name: "nextjs-aksel", DisplayName: "nextjs-aksel", Type: "instruction", InstallURL: "vscode:..."},
			{Name: "testing", DisplayName: "testing", Type: "instruction", InstallURL: "vscode:..."},
		},
	}

	contents := &RepoContents{
		Languages:      []string{"TypeScript", "CSS"},
		HasPackageJSON: true,
		HasNextConfig:  true,
	}
	suggestions := SuggestCustomizations(contents, manifest)

	var akselReason string
	for _, s := range suggestions {
		if s.Name == "nextjs-aksel" {
			akselReason = s.Reason
		}
	}

	if !strings.Contains(akselReason, "Next.js") {
		t.Errorf("expected Next.js-specific reason for nextjs-aksel, got %q", akselReason)
	}
}

func TestStackDetectionHelpers(t *testing.T) {
	nextjsContents := &RepoContents{HasPackageJSON: true, HasNextConfig: true}
	if !isNextJS(nextjsContents) {
		t.Error("expected isNextJS to return true")
	}

	springContents := &RepoContents{HasGradleKts: true, HasAppYml: true, Languages: []string{"Kotlin"}}
	if !isSpringBoot(springContents) {
		t.Error("expected isSpringBoot to return true")
	}

	ktorContents := &RepoContents{HasGradleKts: true, Languages: []string{"Kotlin"}}
	if !isKtor(ktorContents) {
		t.Error("expected isKtor to return true")
	}

	plainKotlin := &RepoContents{HasPomXML: true, Languages: []string{"Kotlin"}}
	if isKtor(plainKotlin) {
		t.Error("expected isKtor to return false for Maven project")
	}
}

func TestAssess_VerificationOnly(t *testing.T) {
	contents := &RepoContents{
		HasCIWorkflows:  true,
		HasLinterConfig: true,
		HasTypeChecking: true,
		HasTestConfig:   true,
		HasDependabot:   true,
		HasReadme:       true,
	}
	report := Assess(contents)

	if report.Score != 0 {
		t.Errorf("expected customization score 0, got %d", report.Score)
	}
	if report.VerificationScore != 6 {
		t.Errorf("expected verification score 6, got %d", report.VerificationScore)
	}
	if report.Level != LevelIntermediate {
		t.Errorf("expected level intermediate (verification alone), got %s", report.Level)
	}
	if len(report.Verifications) != 6 {
		t.Fatalf("expected 6 verification checks, got %d", len(report.Verifications))
	}
	for _, v := range report.Verifications {
		if !v.Exists {
			t.Errorf("expected all verifications to exist, %s was false", v.Path)
		}
	}
}

func TestAssess_GoRepoInheritedVerification(t *testing.T) {
	contents := &RepoContents{
		Languages: []string{"Go"},
	}
	report := Assess(contents)

	if contents.HasTypeChecking {
		t.Error("HasTypeChecking should not be set on RepoContents directly — it's set by detectVerificationInfra in mcp.go")
	}
	if report.VerificationScore != 0 {
		t.Errorf("expected verification score 0 without detection, got %d", report.VerificationScore)
	}
}

func TestRecommend_CIWorkflows(t *testing.T) {
	contents := &RepoContents{
		Languages: []string{"Go"},
	}
	recs := recommend(contents)

	found := false
	for _, r := range recs {
		if strings.Contains(r, "CI/CD workflows") && strings.Contains(r, "go build") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected CI/CD workflow recommendation with Go-specific advice")
	}
}

func TestRecommend_LinterConfig(t *testing.T) {
	contents := &RepoContents{
		Languages: []string{"TypeScript"},
	}
	recs := recommend(contents)

	found := false
	for _, r := range recs {
		if strings.Contains(r, "linter") && strings.Contains(r, "eslint") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected linter recommendation with ESLint advice for TypeScript")
	}
}
