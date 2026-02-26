// Package readiness provides agent readiness assessment for GitHub repositories.
package readiness

import (
	"fmt"
	"strings"

	"github.com/navikt/copilot/mcp-onboarding/internal/discovery"
)

// Level represents the agent readiness level of a repository.
type Level string

// Readiness levels from none to advanced.
const (
	LevelNone         Level = "none"
	LevelBasic        Level = "basic"
	LevelIntermediate Level = "intermediate"
	LevelAdvanced     Level = "advanced"
)

// FileCheck represents the existence check for a Copilot customization file or directory.
type FileCheck struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Count  int    `json:"count,omitempty"`
}

// Report contains the agent readiness assessment results for a repository.
type Report struct {
	Owner                string       `json:"owner"`
	Repo                 string       `json:"repo"`
	Level                Level        `json:"level"`
	Score                int          `json:"score"`
	MaxScore             int          `json:"maxScore"`
	VerificationScore    int          `json:"verificationScore"`
	VerificationMaxScore int          `json:"verificationMaxScore"`
	Files                []FileCheck  `json:"files"`
	Verifications        []FileCheck  `json:"verifications,omitempty"`
	Languages            []string     `json:"languages"`
	Recommendations      []string     `json:"recommendations"`
	Suggestions          []Suggestion `json:"suggestions,omitempty"`
}

// Suggestion represents a recommended Nav Copilot customization for a repository.
type Suggestion struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	InstallURL  string `json:"installUrl"`
	Reason      string `json:"reason"`
}

// RepoContents holds the discovered Copilot customization state of a repository.
type RepoContents struct {
	CopilotInstructions bool
	InstructionsCount   int
	AgentsCount         int
	PromptsCount        int
	SkillsCount         int
	SetupSteps          bool
	HooksConfig         bool
	AgentsMD            bool
	Languages           []string

	// Stack detection fields
	HasNextConfig  bool // next.config.ts/js/mjs detected
	HasAppYml      bool // application.yml or application.properties (Spring Boot)
	HasGradleKts   bool
	HasPomXML      bool
	HasPackageJSON bool

	// Verification infrastructure fields
	HasCIWorkflows  bool // .github/workflows/ has files
	HasLinterConfig bool // eslint/golangci/detekt config found
	HasTypeChecking bool // tsconfig.json or inherently typed language
	HasTestConfig   bool // jest/vitest config or Go/JVM build tool
	HasDependabot   bool // .github/dependabot.yml
	HasReadme       bool // README.md present
}

// Assess evaluates a repository's agent readiness based on its contents.
func Assess(contents *RepoContents) *Report {
	r := &Report{
		MaxScore:             8,
		VerificationMaxScore: 6,
	}

	checks := []struct {
		path   string
		exists bool
		count  int
	}{
		{".github/copilot-instructions.md", contents.CopilotInstructions, 0},
		{".github/instructions/", contents.InstructionsCount > 0, contents.InstructionsCount},
		{".github/agents/", contents.AgentsCount > 0, contents.AgentsCount},
		{".github/prompts/", contents.PromptsCount > 0, contents.PromptsCount},
		{".github/skills/", contents.SkillsCount > 0, contents.SkillsCount},
		{".github/workflows/copilot-setup-steps.yml", contents.SetupSteps, 0},
		{".github/hooks/copilot-hooks.json", contents.HooksConfig, 0},
		{"AGENTS.md", contents.AgentsMD, 0},
	}

	for _, c := range checks {
		fc := FileCheck{Path: c.path, Exists: c.exists, Count: c.count}
		r.Files = append(r.Files, fc)
		if c.exists {
			r.Score++
		}
	}

	verChecks := []struct {
		path   string
		exists bool
	}{
		{"CI/CD workflows", contents.HasCIWorkflows},
		{"Linter configuration", contents.HasLinterConfig},
		{"Type checking", contents.HasTypeChecking},
		{"Test configuration", contents.HasTestConfig},
		{"Dependency updates (Dependabot)", contents.HasDependabot},
		{"README.md", contents.HasReadme},
	}

	for _, c := range verChecks {
		fc := FileCheck{Path: c.path, Exists: c.exists}
		r.Verifications = append(r.Verifications, fc)
		if c.exists {
			r.VerificationScore++
		}
	}

	r.Languages = contents.Languages

	total := r.Score + r.VerificationScore
	switch {
	case total == 0:
		r.Level = LevelNone
	case total <= 3:
		r.Level = LevelBasic
	case total <= 9:
		r.Level = LevelIntermediate
	default:
		r.Level = LevelAdvanced
	}

	r.Recommendations = recommend(contents)
	return r
}

func recommend(c *RepoContents) []string {
	var recs []string
	langs := langSet(c.Languages)

	if !c.AgentsMD {
		rec := "Create AGENTS.md at the repo root — this is the most impactful first step. Works across Copilot, Claude, Codex, and other agents. Include build commands, testing patterns, project structure, code style, and git workflow."
		if isNextJS(c) {
			rec += " For Next.js projects, include `<!-- BEGIN:nextjs-agent-rules -->` markers and point agents to bundled docs at `node_modules/next/dist/docs/`. Run `npx @next/codemod@canary agents-md` to auto-generate."
		}
		if isSpringBoot(c) {
			rec += " For Spring Boot, document layered architecture (Controllers → Services → Repositories), testing patterns (@WebMvcTest, @DataJpaTest), and bean configuration conventions."
		}
		if isKtor(c) {
			rec += " For Ktor, document ApplicationBuilder patterns, routing DSL conventions, sealed class environment config, and testApplication usage."
		}
		recs = append(recs, rec)
	}

	if !c.HasCIWorkflows {
		rec := "Add CI/CD workflows in .github/workflows/ — agents need automated pipelines to validate their changes compile, pass tests, and meet quality standards."
		if langs["Go"] {
			rec += " For Go: include `go build ./...`, `go test ./...`, and `go vet ./...` steps."
		}
		if langs["Kotlin"] || langs["Java"] {
			rec += " For JVM: include Gradle/Maven build and test steps with the correct JDK version."
		}
		if langs["TypeScript"] || langs["JavaScript"] {
			rec += " For Node.js: include `npm ci`, `npm run build`, and `npm test` steps."
		}
		recs = append(recs, rec)
	}

	if !c.HasTestConfig {
		rec := "Add test infrastructure — agents need a runnable test suite to verify their changes. Without tests, agents cannot validate correctness and are more likely to introduce regressions."
		if langs["TypeScript"] || langs["JavaScript"] {
			rec += " Add jest.config.js or vitest.config.ts with comprehensive test coverage."
		}
		recs = append(recs, rec)
	}

	if !c.SetupSteps {
		rec := "Add .github/workflows/copilot-setup-steps.yml — enables the Copilot coding agent to pre-install dependencies and run autonomously on GitHub issues."
		if langs["Kotlin"] || langs["Java"] {
			rec += " Critical for JVM projects: specify exact JDK version (e.g., temurin 21) so the coding agent can compile and test."
		}
		if isNextJS(c) {
			rec += " Include a build step (`npm run build`) so the agent can verify TypeScript types and Next.js compilation."
		}
		recs = append(recs, rec)
	}

	if !c.HasTypeChecking {
		rec := "Enable type checking — type systems give agents immediate feedback on errors, dramatically improving code quality."
		if langs["TypeScript"] || langs["JavaScript"] {
			rec += " Ensure tsconfig.json exists with strict mode enabled."
		}
		recs = append(recs, rec)
	}

	if !c.HasLinterConfig {
		rec := "Add linter configuration — opinionated linters help agents produce code that matches your team's standards consistently."
		if langs["TypeScript"] || langs["JavaScript"] {
			rec += " Add eslint.config.mjs with strict rules for your framework."
		}
		if langs["Go"] {
			rec += " Add .golangci.yml with golangci-lint configuration."
		}
		if langs["Kotlin"] || langs["Java"] {
			rec += " Add detekt.yml for Kotlin static analysis."
		}
		recs = append(recs, rec)
	}

	if c.InstructionsCount == 0 {
		rec := "Add scoped instructions in .github/instructions/ — use applyTo patterns to give Copilot file-specific guidance."
		if isNextJS(c) {
			rec += " For Next.js: create instructions for *.tsx (component patterns, Server vs Client Components), *.test.tsx (React Testing Library), and *.css (Tailwind/Aksel tokens)."
		}
		if isSpringBoot(c) {
			rec += " For Spring Boot: create instructions for *.kt (Kotlin conventions, sealed classes), *Test.kt (MockK, AssertJ, test slices), and *.sql (Flyway migration naming)."
		}
		if isKtor(c) {
			rec += " For Ktor: create instructions for *.kt (routing DSL, Kotliquery patterns) and *Test.kt (testApplication usage)."
		}
		recs = append(recs, rec)
	}

	if !c.HasReadme {
		recs = append(recs, "Add README.md — this is the first file agents read when exploring your codebase. Include project overview, setup instructions, architecture, and how to run tests.")
	}

	if !c.CopilotInstructions {
		recs = append(recs, "Add .github/copilot-instructions.md — Copilot-specific instructions that supplement AGENTS.md with GitHub-specific features. Lower priority if you already have AGENTS.md.")
	}

	if c.AgentsCount == 0 {
		recs = append(recs, "Add custom agents in .github/agents/ — create specialized personas (e.g., @security-reviewer, @test-writer) with tool access and MCP server connections.")
	}

	if c.PromptsCount == 0 {
		recs = append(recs, "Add reusable prompts in .github/prompts/ — template repetitive tasks like code review checklists, PR descriptions, or migration patterns.")
	}

	if c.SkillsCount == 0 {
		recs = append(recs, "Add skills in .github/skills/ — bundle domain knowledge with scripts and assets for specialized capabilities like deployment, database migrations, or API design.")
	}

	if !c.HooksConfig {
		recs = append(recs, "Add .github/hooks/copilot-hooks.json — automate linting, formatting, or validation on sessionStart/sessionEnd events.")
	}

	if !c.HasDependabot {
		recs = append(recs, "Add .github/dependabot.yml — automated dependency updates keep your project secure and prevent agents from working with vulnerable dependencies.")
	}

	return recs
}

func isNextJS(c *RepoContents) bool {
	return c.HasPackageJSON && c.HasNextConfig
}

func isSpringBoot(c *RepoContents) bool {
	return (c.HasGradleKts || c.HasPomXML) && c.HasAppYml
}

func isKtor(c *RepoContents) bool {
	langs := langSet(c.Languages)
	return c.HasGradleKts && !c.HasAppYml && langs["Kotlin"]
}

// SuggestCustomizations returns Nav Copilot customizations relevant to the repository's tech stack.
func SuggestCustomizations(contents *RepoContents, manifest *discovery.CustomizationsManifest) []Suggestion {
	if manifest == nil {
		return nil
	}

	var suggestions []Suggestion
	langs := langSet(contents.Languages)

	if langs["TypeScript"] || langs["JavaScript"] {
		if isNextJS(contents) {
			suggestions = appendMatching(suggestions, manifest, "instruction", "nextjs-aksel",
				"Your repo uses Next.js — this provides Nav Design System spacing rules, responsive breakpoints, and Server/Client Component patterns")
			suggestions = appendMatching(suggestions, manifest, "agent", "aksel-agent",
				"Provides expert guidance on Nav Aksel Design System for your Next.js components")
		} else {
			suggestions = appendMatching(suggestions, manifest, "instruction", "nextjs-aksel",
				"Your repo uses TypeScript/JavaScript — this provides Nav Design System spacing rules and component patterns")
			suggestions = appendMatching(suggestions, manifest, "agent", "aksel-agent",
				"Provides expert guidance on Nav Aksel Design System for your TypeScript/JavaScript project")
		}
	}

	if langs["Kotlin"] || langs["Java"] {
		switch {
		case isSpringBoot(contents):
			suggestions = appendMatching(suggestions, manifest, "instruction", "kotlin-ktor",
				"Your repo uses Kotlin — this provides coding conventions, sealed classes, and testing patterns applicable to Spring Boot projects")
		case isKtor(contents):
			suggestions = appendMatching(suggestions, manifest, "instruction", "kotlin-ktor",
				"Your repo uses Kotlin/Ktor — this provides Ktor routing patterns, ApplicationBuilder conventions, and Kotliquery database patterns")
		default:
			suggestions = appendMatching(suggestions, manifest, "instruction", "kotlin-ktor",
				"Your repo uses Kotlin/Java — this provides Ktor patterns, sealed class configs, and Kotliquery conventions")
		}
		suggestions = appendMatching(suggestions, manifest, "agent", "kafka-events",
			"Provides Rapids & Rivers pattern guidance for Kafka event handling in JVM projects")
	}

	suggestions = appendMatching(suggestions, manifest, "instruction", "testing",
		"Testing instructions applicable to all Nav projects — patterns for Jest, Kotest, and integration tests")

	suggestions = appendMatching(suggestions, manifest, "agent", "security-champion",
		"Security review agent for all Nav repos — OWASP, dependency scanning, secrets management")

	suggestions = appendMatching(suggestions, manifest, "agent", "nais-platform",
		"Platform agent for Nais deployment, resource configuration, and infrastructure guidance")

	suggestions = appendMatching(suggestions, manifest, "agent", "observability",
		"Observability agent for Prometheus metrics, OpenTelemetry tracing, and health endpoints")

	if langs["Kotlin"] || langs["Java"] || langs["TypeScript"] || langs["JavaScript"] {
		suggestions = appendMatching(suggestions, manifest, "instruction", "database",
			"Database migration patterns with Flyway versioned SQL scripts")
	}

	return suggestions
}

// RepoReadiness holds a lightweight readiness snapshot for a single repo.
type RepoReadiness struct {
	Repo       string `json:"repo"`
	AgentsMD   bool   `json:"agentsMd"`
	CopilotMD  bool   `json:"copilotInstructions"`
	SetupSteps bool   `json:"setupSteps"`
	Level      Level  `json:"level"`
}

// TeamSummary aggregates readiness across a team's repos.
type TeamSummary struct {
	Org   string          `json:"org"`
	Team  string          `json:"team"`
	Repos []RepoReadiness `json:"repos"`
	Total int             `json:"total"`
}

// AssessRepoLight does a lightweight readiness check based on 3 key files.
func AssessRepoLight(agentsMD, copilotMD, setupSteps bool) Level {
	score := 0
	if agentsMD {
		score++
	}
	if copilotMD {
		score++
	}
	if setupSteps {
		score++
	}
	switch score {
	case 0:
		return LevelNone
	case 1:
		return LevelBasic
	case 2:
		return LevelIntermediate
	default:
		return LevelAdvanced
	}
}

// FormatTeamSummary renders a team readiness summary as markdown.
func FormatTeamSummary(s *TeamSummary) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Agent Readiness: %s/%s\n\n", s.Org, s.Team)

	counts := map[Level]int{}
	agentsMDCount := 0
	copilotMDCount := 0
	setupCount := 0
	for _, r := range s.Repos {
		counts[r.Level]++
		if r.AgentsMD {
			agentsMDCount++
		}
		if r.CopilotMD {
			copilotMDCount++
		}
		if r.SetupSteps {
			setupCount++
		}
	}

	fmt.Fprintf(&b, "**%d repos** scanned\n\n", s.Total)
	fmt.Fprintf(&b, "| Metric | Count | Percentage |\n")
	fmt.Fprintf(&b, "|--------|-------|------------|\n")
	fmt.Fprintf(&b, "| AGENTS.md | %d | %d%% |\n", agentsMDCount, pct(agentsMDCount, s.Total))
	fmt.Fprintf(&b, "| copilot-instructions.md | %d | %d%% |\n", copilotMDCount, pct(copilotMDCount, s.Total))
	fmt.Fprintf(&b, "| copilot-setup-steps.yml | %d | %d%% |\n", setupCount, pct(setupCount, s.Total))

	b.WriteString("\n**Readiness levels:**\n\n")
	for _, level := range []Level{LevelAdvanced, LevelIntermediate, LevelBasic, LevelNone} {
		if c := counts[level]; c > 0 {
			fmt.Fprintf(&b, "- %s: %d repos\n", levelEmoji(level), c)
		}
	}

	b.WriteString("\n## Per-repo breakdown\n\n")
	b.WriteString("| Repository | AGENTS.md | Instructions | Setup Steps | Level |\n")
	b.WriteString("|------------|-----------|--------------|-------------|-------|\n")
	for _, r := range s.Repos {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			r.Repo, yn(r.AgentsMD), yn(r.CopilotMD), yn(r.SetupSteps), levelEmoji(r.Level))
	}

	return b.String()
}

func pct(count, total int) int {
	if total == 0 {
		return 0
	}
	return count * 100 / total
}

func yn(v bool) string {
	if v {
		return "✅"
	}
	return "❌"
}

// FormatReport renders a readiness report as human-readable markdown.
func FormatReport(r *Report) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Agent Readiness Report: %s/%s\n\n", r.Owner, r.Repo)

	total := r.Score + r.VerificationScore
	totalMax := r.MaxScore + r.VerificationMaxScore
	fmt.Fprintf(&b, "**Level**: %s (%d/%d)\n\n", levelEmoji(r.Level), total, totalMax)
	fmt.Fprintf(&b, "**Customizations**: %d/%d | **Verification**: %d/%d\n\n", r.Score, r.MaxScore, r.VerificationScore, r.VerificationMaxScore)

	if len(r.Languages) > 0 {
		fmt.Fprintf(&b, "**Languages**: %s\n\n", strings.Join(r.Languages, ", "))
	}

	fmt.Fprintf(&b, "## Agent Customizations (%d/%d)\n\n", r.Score, r.MaxScore)
	for _, f := range r.Files {
		check := "❌"
		if f.Exists {
			check = "✅"
		}
		line := fmt.Sprintf("%s %s", check, f.Path)
		if f.Count > 0 {
			line += fmt.Sprintf(" (%d files)", f.Count)
		}
		b.WriteString(line + "\n")
	}

	if len(r.Verifications) > 0 {
		fmt.Fprintf(&b, "\n## Verification Infrastructure (%d/%d)\n\n", r.VerificationScore, r.VerificationMaxScore)
		for _, f := range r.Verifications {
			check := "❌"
			if f.Exists {
				check = "✅"
			}
			fmt.Fprintf(&b, "%s %s\n", check, f.Path)
		}
	}

	if len(r.Recommendations) > 0 {
		b.WriteString("\n## Recommendations (in priority order)\n\n")
		for i, rec := range r.Recommendations {
			fmt.Fprintf(&b, "%d. %s\n\n", i+1, rec)
		}
	}

	if len(r.Suggestions) > 0 {
		b.WriteString("\n## Suggested Nav Customizations\n\n")
		for _, s := range r.Suggestions {
			fmt.Fprintf(&b, "- **%s** (%s): %s\n  Install: %s\n\n", s.Name, s.Type, s.Reason, s.InstallURL)
		}
	}

	return b.String()
}

func levelEmoji(l Level) string {
	switch l {
	case LevelNone:
		return "🔴 None"
	case LevelBasic:
		return "🟡 Basic"
	case LevelIntermediate:
		return "🟠 Intermediate"
	case LevelAdvanced:
		return "🟢 Advanced"
	default:
		return string(l)
	}
}

func langSet(langs []string) map[string]bool {
	m := make(map[string]bool, len(langs))
	for _, l := range langs {
		m[l] = true
	}
	return m
}

func appendMatching(suggestions []Suggestion, manifest *discovery.CustomizationsManifest, typ, name, reason string) []Suggestion {
	var items []discovery.Customization
	switch typ {
	case "agent":
		items = manifest.Agents
	case "instruction":
		items = manifest.Instructions
	case "prompt":
		items = manifest.Prompts
	case "skill":
		items = manifest.Skills
	}

	for _, item := range items {
		if strings.Contains(item.Name, name) {
			suggestions = append(suggestions, Suggestion{
				Type:        typ,
				Name:        item.DisplayName,
				Description: item.Description,
				InstallURL:  item.InstallURL,
				Reason:      reason,
			})
			return suggestions
		}
	}
	return suggestions
}
