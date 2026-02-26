// Package templates generates Copilot customization file content
// tailored to a repository's tech stack.
package templates

import (
	"fmt"
	"strings"
)

// RepoInfo holds detected information about a repository's tech stack.
type RepoInfo struct {
	Owner     string
	Repo      string
	Languages []string
	HasNais   bool

	HasPackageJSON bool
	PackageManager string
	HasNextConfig  bool
	HasGoMod       bool
	HasPomXML      bool
	HasGradleKts   bool
	HasDockerfile  bool
	HasAppYml      bool // application.yml or application.properties (Spring Boot indicator)
}

// isNextJS returns true if the repo appears to be a Next.js project.
func (r *RepoInfo) isNextJS() bool {
	return r.HasPackageJSON && r.HasNextConfig
}

// isSpringBoot returns true if the repo appears to be a Spring Boot project.
func (r *RepoInfo) isSpringBoot() bool {
	return (r.HasGradleKts || r.HasPomXML) && r.HasAppYml
}

// isKtor returns true if the repo appears to be a Kotlin/Ktor project (Gradle without Spring Boot markers).
func (r *RepoInfo) isKtor() bool {
	langs := langSet(r.Languages)
	return r.HasGradleKts && !r.HasAppYml && langs["Kotlin"]
}

// GenerateAgentsMD generates an AGENTS.md tailored to the repo.
// AGENTS.md is a cross-agent standard that works with Copilot, Claude, Codex, and others.
func GenerateAgentsMD(info *RepoInfo) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# AGENTS.md — %s/%s\n\n", info.Owner, info.Repo)

	b.WriteString("## Repository Overview\n\n")
	b.WriteString("<!-- Describe what this repository does and its main purpose -->\n\n")

	if len(info.Languages) > 0 {
		b.WriteString("## Tech Stack\n\n")
		for _, lang := range info.Languages {
			fmt.Fprintf(&b, "- %s\n", lang)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Build & Test Commands\n\n")
	writeBuildCommands(&b, info)

	b.WriteString("## Code Standards\n\n")
	writeCodeStandards(&b, info)

	writeStackSpecificGuidance(&b, info)

	if info.HasNais {
		b.WriteString("## Deployment\n\n")
		b.WriteString("- Platform: Nais (Kubernetes on GCP)\n")
		b.WriteString("- Manifests in `.nais/` directory\n")
		b.WriteString("- Required endpoints: `/isalive`, `/isready`, `/metrics`\n\n")
	}

	b.WriteString("## Boundaries\n\n")
	b.WriteString("### ✅ Always\n\n")
	writeBoundariesAlways(&b, info)
	b.WriteString("\n### ⚠️ Ask First\n\n")
	b.WriteString("- Changing authentication mechanisms\n")
	b.WriteString("- Modifying production configurations\n")
	b.WriteString("- Adding new dependencies\n\n")
	b.WriteString("### 🚫 Never\n\n")
	b.WriteString("- Commit secrets or credentials to git\n")
	b.WriteString("- Skip input validation\n")
	b.WriteString("- Bypass security controls\n")

	return b.String()
}

func writeBuildCommands(b *strings.Builder, info *RepoInfo) {
	if info.HasPackageJSON {
		pm := info.PackageManager
		if pm == "" {
			pm = "npm"
		}
		fmt.Fprintf(b, "```bash\n%s install    # Install dependencies\n", pm)
		fmt.Fprintf(b, "%s run build  # Build project\n", pm)
		fmt.Fprintf(b, "%s test       # Run tests\n", pm)
		fmt.Fprintf(b, "%s run lint   # Lint code\n```\n\n", pm)
		return
	}

	if info.HasGoMod {
		b.WriteString("```bash\ngo build ./...   # Build\ngo test ./...    # Run tests\ngo vet ./...     # Vet code\n```\n\n")
		return
	}

	if info.HasGradleKts {
		b.WriteString("```bash\n./gradlew build  # Build\n./gradlew test   # Run tests\n```\n\n")
		return
	}

	if info.HasPomXML {
		b.WriteString("```bash\nmvn compile      # Build\nmvn test         # Run tests\n```\n\n")
		return
	}

	b.WriteString("```bash\n# TODO: Add your build and test commands\n```\n\n")
}

func writeCodeStandards(b *strings.Builder, info *RepoInfo) {
	langs := langSet(info.Languages)

	if langs["TypeScript"] || langs["JavaScript"] {
		b.WriteString("- Use TypeScript strict mode\n")
		b.WriteString("- Follow existing component patterns\n")
		b.WriteString("- Write tests for utilities and business logic\n")
		if info.isNextJS() {
			b.WriteString("- Use App Router conventions (page.tsx, layout.tsx, loading.tsx)\n")
			b.WriteString("- Server Components by default, 'use client' only when needed\n")
		}
		b.WriteString("\n")
	}

	if langs["Kotlin"] || langs["Java"] {
		switch {
		case info.isSpringBoot():
			b.WriteString("- Follow Kotlin coding conventions\n")
			b.WriteString("- Use constructor injection (never field injection)\n")
			b.WriteString("- Use `@Configuration` classes for bean definitions (not `@Component`/`@Service`)\n")
			b.WriteString("- Use sealed classes for type-safe state hierarchies\n")
			b.WriteString("- Use data classes for DTOs, value classes for typed IDs\n")
			b.WriteString("- Write tests for all public APIs\n\n")
		case info.isKtor():
			b.WriteString("- Follow Kotlin coding conventions\n")
			b.WriteString("- Use sealed classes for environment configuration (Dev/Prod/Local)\n")
			b.WriteString("- Use Kotliquery with HikariCP for database access\n")
			b.WriteString("- ApplicationBuilder pattern for bootstrapping\n")
			b.WriteString("- Write tests for all public APIs\n\n")
		default:
			b.WriteString("- Follow Kotlin coding conventions\n")
			b.WriteString("- Use sealed classes for configuration\n")
			b.WriteString("- Write tests for all public APIs\n\n")
		}
	}

	if langs["Go"] {
		b.WriteString("- Follow Go standards and idioms\n")
		b.WriteString("- Use `go vet` and `staticcheck`\n")
		b.WriteString("- Write table-driven tests\n\n")
	}

	if !langs["TypeScript"] && !langs["JavaScript"] && !langs["Kotlin"] && !langs["Java"] && !langs["Go"] {
		b.WriteString("- Follow existing code patterns in the project\n")
		b.WriteString("- Write tests for new functionality\n\n")
	}
}

func writeStackSpecificGuidance(b *strings.Builder, info *RepoInfo) {
	if info.isNextJS() {
		b.WriteString("## Next.js Agent Rules\n\n")
		b.WriteString("<!-- BEGIN:nextjs-agent-rules -->\n")
		b.WriteString("Before any Next.js work, find and read the relevant doc in `node_modules/next/dist/docs/`.\n")
		b.WriteString("Your training data may be outdated — the bundled docs are the source of truth.\n")
		b.WriteString("<!-- END:nextjs-agent-rules -->\n\n")
		b.WriteString("- Prefer Server Components; add `'use client'` only for interactivity\n")
		b.WriteString("- Use Nav Design System (`@navikt/ds-react`) spacing tokens, never Tailwind padding/margin\n")
		b.WriteString("- Use responsive `Box` props (`paddingBlock`, `paddingInline`) with breakpoints (`xs`, `sm`, `md`, `lg`)\n")
		b.WriteString("- Test components with React Testing Library; test utilities with Jest\n\n")
	}

	if info.isSpringBoot() {
		b.WriteString("## Spring Boot Patterns\n\n")
		b.WriteString("### Architecture\n\n")
		b.WriteString("- Layered: Controllers → Services → Repositories\n")
		b.WriteString("- Package-by-feature: each feature gets its own package\n")
		b.WriteString("- DTOs only in the controller layer; services work with entities\n\n")
		b.WriteString("### Testing\n\n")
		b.WriteString("- `@WebMvcTest` for controller tests (mocks service layer)\n")
		b.WriteString("- `@DataJpaTest` for repository integration tests\n")
		b.WriteString("- MockK for unit test mocking, AssertJ for assertions\n")
		b.WriteString("- Descriptive test names with backticks: `` fun `GET should return 404 when not found`() ``\n\n")
		b.WriteString("### Configuration\n\n")
		b.WriteString("- `application.yml` for config, `@ConfigurationProperties` for type-safe binding\n")
		b.WriteString("- Flyway for database migrations (`src/main/resources/db/migration/`)\n\n")
	}

	if info.isKtor() {
		b.WriteString("## Ktor Patterns\n\n")
		b.WriteString("- Use `Application.module()` extension functions for feature installation\n")
		b.WriteString("- Route definitions via `routing { }` DSL\n")
		b.WriteString("- Use `testApplication { }` for integration tests\n")
		b.WriteString("- Flyway for database migrations\n")
		b.WriteString("- Rapids & Rivers pattern for Kafka event handling\n\n")
	}
}

func writeBoundariesAlways(b *strings.Builder, info *RepoInfo) {
	b.WriteString("- Follow existing code patterns\n")

	if info.HasPackageJSON {
		pm := info.PackageManager
		if pm == "" {
			pm = "npm"
		}
		fmt.Fprintf(b, "- Run `%s test` before committing\n", pm)
	}

	if info.HasGoMod {
		b.WriteString("- Run `go test ./...` before committing\n")
	}

	if info.HasGradleKts || info.HasPomXML {
		b.WriteString("- Run tests before committing\n")
	}

	b.WriteString("- Use parameterized queries for database access\n")

	if info.isNextJS() {
		b.WriteString("- Use Nav DS spacing tokens (`space-8`, `space-16`, etc.), never Tailwind `p-`/`m-` utilities\n")
		b.WriteString("- Read bundled Next.js docs before implementing Next.js features\n")
	}

	if info.isSpringBoot() {
		b.WriteString("- Use constructor injection, never field injection\n")
		b.WriteString("- Never use `@Component`/`@Service`/`@Repository` — use `@Configuration` + `@Bean`\n")
	}

	if info.isKtor() {
		b.WriteString("- Use sealed classes for environment configuration\n")
	}
}

// GenerateSetupSteps generates a copilot-setup-steps.yml workflow for the coding agent.
func GenerateSetupSteps(info *RepoInfo) string {
	var b strings.Builder

	b.WriteString("# This workflow is triggered by GitHub Copilot to set up\n")
	b.WriteString("# the environment for the coding agent.\n")
	b.WriteString("# https://docs.github.com/en/copilot/customizing-copilot/customizing-the-development-environment-for-copilot-coding-agent\n\n")
	b.WriteString("name: Copilot Setup Steps\n\n")
	b.WriteString("on: workflow_dispatch\n\n")
	b.WriteString("jobs:\n")
	b.WriteString("  setup:\n")
	b.WriteString("    runs-on: ubuntu-latest\n")
	b.WriteString("    permissions:\n")
	b.WriteString("      contents: read\n")
	b.WriteString("    steps:\n")
	b.WriteString("      - uses: actions/checkout@v4\n\n")

	writeSetupSteps(&b, info)

	return b.String()
}

func writeSetupSteps(b *strings.Builder, info *RepoInfo) {
	langs := langSet(info.Languages)

	if info.HasPackageJSON {
		nodeVersion := "22"

		b.WriteString("      - uses: actions/setup-node@v4\n")
		b.WriteString("        with:\n")
		fmt.Fprintf(b, "          node-version: '%s'\n\n", nodeVersion)

		pm := info.PackageManager
		if pm == "" {
			pm = "npm"
		}

		switch pm {
		case "pnpm":
			b.WriteString("      - uses: pnpm/action-setup@v4\n\n")
			b.WriteString("      - run: pnpm install --frozen-lockfile\n\n")
		case "yarn":
			b.WriteString("      - run: yarn install --frozen-lockfile\n\n")
		default:
			b.WriteString("      - run: npm ci\n\n")
		}

		if info.isNextJS() {
			b.WriteString("      # Next.js: build to validate TypeScript and generate types\n")
			switch pm {
			case "pnpm":
				b.WriteString("      - run: pnpm run build\n\n")
			case "yarn":
				b.WriteString("      - run: yarn build\n\n")
			default:
				b.WriteString("      - run: npm run build\n\n")
			}
		}
	}

	if info.HasGoMod {
		goVersion := "1.23"
		b.WriteString("      - uses: actions/setup-go@v5\n")
		b.WriteString("        with:\n")
		fmt.Fprintf(b, "          go-version: '%s'\n\n", goVersion)
		b.WriteString("      - run: go mod download\n\n")
	}

	if langs["Kotlin"] || langs["Java"] {
		javaVersion := "21"
		if info.HasGradleKts {
			b.WriteString("      - uses: actions/setup-java@v4\n")
			b.WriteString("        with:\n")
			b.WriteString("          distribution: 'temurin'\n")
			fmt.Fprintf(b, "          java-version: '%s'\n\n", javaVersion)
			b.WriteString("      - uses: gradle/actions/setup-gradle@v4\n\n")
			b.WriteString("      - run: ./gradlew dependencies\n\n")

			if info.isSpringBoot() {
				b.WriteString("      # Spring Boot: compile to verify configuration\n")
				b.WriteString("      - run: ./gradlew compileKotlin\n\n")
			}
		} else if info.HasPomXML {
			b.WriteString("      - uses: actions/setup-java@v4\n")
			b.WriteString("        with:\n")
			b.WriteString("          distribution: 'temurin'\n")
			fmt.Fprintf(b, "          java-version: '%s'\n\n", javaVersion)
			b.WriteString("      - run: mvn dependency:resolve\n\n")

			if info.isSpringBoot() {
				b.WriteString("      # Spring Boot: compile to verify configuration\n")
				b.WriteString("      - run: mvn compile -DskipTests\n\n")
			}
		}
	}

	if !info.HasPackageJSON && !info.HasGoMod && !info.HasGradleKts && !info.HasPomXML {
		b.WriteString("      # TODO: Add your language setup and dependency installation steps\n")
		b.WriteString("      # See https://docs.github.com/en/copilot/customizing-copilot/customizing-the-development-environment-for-copilot-coding-agent\n")
	}
}

func langSet(langs []string) map[string]bool {
	m := make(map[string]bool, len(langs))
	for _, l := range langs {
		m[l] = true
	}
	return m
}
