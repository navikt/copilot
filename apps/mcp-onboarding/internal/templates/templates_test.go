package templates

import (
	"strings"
	"testing"
)

func TestGenerateAgentsMD_GoProject(t *testing.T) {
	info := &RepoInfo{
		Owner:     "navikt",
		Repo:      "my-api",
		Languages: []string{"Go", "Dockerfile"},
		HasGoMod:  true,
		HasNais:   true,
	}

	output := GenerateAgentsMD(info)

	checks := []string{
		"# AGENTS.md \u2014 navikt/my-api",
		"Go",
		"go build ./...",
		"go test ./...",
		"go vet ./...",
		"Follow Go standards",
		"staticcheck",
		"Nais",
		"/isalive",
		"✅ Always",
		"⚠️ Ask First",
		"🚫 Never",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected output to contain %q", check)
		}
	}
}

func TestGenerateAgentsMD_TypeScriptProject(t *testing.T) {
	info := &RepoInfo{
		Owner:          "navikt",
		Repo:           "frontend-app",
		Languages:      []string{"TypeScript", "CSS"},
		HasPackageJSON: true,
		PackageManager: "pnpm",
	}

	output := GenerateAgentsMD(info)

	checks := []string{
		"navikt/frontend-app",
		"pnpm install",
		"pnpm run build",
		"pnpm test",
		"TypeScript strict mode",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected output to contain %q", check)
		}
	}

	if strings.Contains(output, "Nais") {
		t.Error("should not mention Nais when HasNais is false")
	}
}

func TestGenerateAgentsMD_KotlinGradleProject(t *testing.T) {
	info := &RepoInfo{
		Owner:        "navikt",
		Repo:         "kotlin-service",
		Languages:    []string{"Kotlin"},
		HasGradleKts: true,
		HasNais:      true,
	}

	output := GenerateAgentsMD(info)

	if !strings.Contains(output, "./gradlew build") {
		t.Error("expected Gradle build commands")
	}
	if !strings.Contains(output, "sealed classes") {
		t.Error("expected Kotlin code standards")
	}
}

func TestGenerateAgentsMD_KotlinMavenProject(t *testing.T) {
	info := &RepoInfo{
		Owner:     "navikt",
		Repo:      "legacy-service",
		Languages: []string{"Java", "Kotlin"},
		HasPomXML: true,
	}

	output := GenerateAgentsMD(info)

	if !strings.Contains(output, "mvn compile") {
		t.Error("expected Maven build commands")
	}
}

func TestGenerateAgentsMD_NpmDefault(t *testing.T) {
	info := &RepoInfo{
		Owner:          "navikt",
		Repo:           "app",
		Languages:      []string{"JavaScript"},
		HasPackageJSON: true,
	}

	output := GenerateAgentsMD(info)

	if !strings.Contains(output, "npm install") {
		t.Error("expected npm as default package manager")
	}
}

func TestGenerateAgentsMD_UnknownStack(t *testing.T) {
	info := &RepoInfo{
		Owner:     "navikt",
		Repo:      "scripts",
		Languages: []string{"Shell"},
	}

	output := GenerateAgentsMD(info)

	if !strings.Contains(output, "TODO: Add your build and test commands") {
		t.Error("expected TODO placeholder for unknown stack")
	}
	if !strings.Contains(output, "Follow existing code patterns") {
		t.Error("expected generic code standards")
	}
}

func TestGenerateSetupSteps_GoProject(t *testing.T) {
	info := &RepoInfo{
		Owner:     "navikt",
		Repo:      "my-api",
		Languages: []string{"Go"},
		HasGoMod:  true,
	}

	output := GenerateSetupSteps(info)

	checks := []string{
		"name: Copilot Setup Steps",
		"on: workflow_dispatch",
		"actions/checkout@v4",
		"actions/setup-go@v5",
		"go mod download",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected output to contain %q", check)
		}
	}
}

func TestGenerateSetupSteps_PnpmProject(t *testing.T) {
	info := &RepoInfo{
		Owner:          "navikt",
		Repo:           "frontend",
		Languages:      []string{"TypeScript"},
		HasPackageJSON: true,
		PackageManager: "pnpm",
	}

	output := GenerateSetupSteps(info)

	if !strings.Contains(output, "pnpm/action-setup@v4") {
		t.Error("expected pnpm action setup")
	}
	if !strings.Contains(output, "pnpm install --frozen-lockfile") {
		t.Error("expected pnpm install")
	}
	if !strings.Contains(output, "actions/setup-node@v4") {
		t.Error("expected Node.js setup")
	}
}

func TestGenerateSetupSteps_YarnProject(t *testing.T) {
	info := &RepoInfo{
		Owner:          "navikt",
		Repo:           "app",
		Languages:      []string{"TypeScript"},
		HasPackageJSON: true,
		PackageManager: "yarn",
	}

	output := GenerateSetupSteps(info)

	if !strings.Contains(output, "yarn install --frozen-lockfile") {
		t.Error("expected yarn install")
	}
}

func TestGenerateSetupSteps_NpmProject(t *testing.T) {
	info := &RepoInfo{
		Owner:          "navikt",
		Repo:           "app",
		Languages:      []string{"JavaScript"},
		HasPackageJSON: true,
	}

	output := GenerateSetupSteps(info)

	if !strings.Contains(output, "npm ci") {
		t.Error("expected npm ci")
	}
}

func TestGenerateSetupSteps_GradleProject(t *testing.T) {
	info := &RepoInfo{
		Owner:        "navikt",
		Repo:         "kotlin-service",
		Languages:    []string{"Kotlin"},
		HasGradleKts: true,
	}

	output := GenerateSetupSteps(info)

	checks := []string{
		"actions/setup-java@v4",
		"java-version: '21'",
		"gradle/actions/setup-gradle@v4",
		"./gradlew dependencies",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected output to contain %q", check)
		}
	}
}

func TestGenerateSetupSteps_MavenProject(t *testing.T) {
	info := &RepoInfo{
		Owner:     "navikt",
		Repo:      "legacy",
		Languages: []string{"Java"},
		HasPomXML: true,
	}

	output := GenerateSetupSteps(info)

	if !strings.Contains(output, "mvn dependency:resolve") {
		t.Error("expected Maven dependency resolution")
	}
}

func TestGenerateSetupSteps_UnknownStack(t *testing.T) {
	info := &RepoInfo{
		Owner:     "navikt",
		Repo:      "scripts",
		Languages: []string{"Shell"},
	}

	output := GenerateSetupSteps(info)

	if !strings.Contains(output, "TODO: Add your language setup") {
		t.Error("expected TODO placeholder")
	}
}

func TestGenerateAgentsMD_NextJSProject(t *testing.T) {
	info := &RepoInfo{
		Owner:          "navikt",
		Repo:           "my-portal",
		Languages:      []string{"TypeScript", "CSS"},
		HasPackageJSON: true,
		HasNextConfig:  true,
		PackageManager: "pnpm",
	}

	output := GenerateAgentsMD(info)

	checks := []string{
		"Next.js Agent Rules",
		"BEGIN:nextjs-agent-rules",
		"node_modules/next/dist/docs/",
		"Server Components",
		"App Router conventions",
		"Nav Design System",
		"spacing tokens",
		"Read bundled Next.js docs",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected output to contain %q", check)
		}
	}
}

func TestGenerateAgentsMD_SpringBootProject(t *testing.T) {
	info := &RepoInfo{
		Owner:        "navikt",
		Repo:         "spring-service",
		Languages:    []string{"Kotlin"},
		HasGradleKts: true,
		HasAppYml:    true,
		HasNais:      true,
	}

	output := GenerateAgentsMD(info)

	checks := []string{
		"Spring Boot Patterns",
		"Controllers → Services → Repositories",
		"@WebMvcTest",
		"@DataJpaTest",
		"@Configuration",
		"@ConfigurationProperties",
		"constructor injection",
		"Never use `@Component`/`@Service`/`@Repository`",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected output to contain %q", check)
		}
	}

	if strings.Contains(output, "ApplicationBuilder") {
		t.Error("Spring Boot project should not include Ktor patterns")
	}
}

func TestGenerateAgentsMD_KtorProject(t *testing.T) {
	info := &RepoInfo{
		Owner:        "navikt",
		Repo:         "ktor-service",
		Languages:    []string{"Kotlin"},
		HasGradleKts: true,
		HasAppYml:    false,
		HasNais:      true,
	}

	output := GenerateAgentsMD(info)

	checks := []string{
		"Ktor Patterns",
		"Application.module()",
		"routing { }",
		"testApplication",
		"Rapids & Rivers",
		"ApplicationBuilder pattern",
		"Kotliquery",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("expected output to contain %q", check)
		}
	}

	if strings.Contains(output, "Spring Boot") {
		t.Error("Ktor project should not include Spring Boot patterns")
	}
}

func TestGenerateSetupSteps_NextJSProject(t *testing.T) {
	info := &RepoInfo{
		Owner:          "navikt",
		Repo:           "my-portal",
		Languages:      []string{"TypeScript"},
		HasPackageJSON: true,
		HasNextConfig:  true,
		PackageManager: "pnpm",
	}

	output := GenerateSetupSteps(info)

	if !strings.Contains(output, "pnpm run build") {
		t.Error("expected Next.js build step")
	}
	if !strings.Contains(output, "Next.js") {
		t.Error("expected Next.js comment")
	}
}

func TestGenerateSetupSteps_SpringBootGradleProject(t *testing.T) {
	info := &RepoInfo{
		Owner:        "navikt",
		Repo:         "spring-service",
		Languages:    []string{"Kotlin"},
		HasGradleKts: true,
		HasAppYml:    true,
	}

	output := GenerateSetupSteps(info)

	if !strings.Contains(output, "compileKotlin") {
		t.Error("expected Spring Boot compile step")
	}
}

func TestGenerateSetupSteps_SpringBootMavenProject(t *testing.T) {
	info := &RepoInfo{
		Owner:     "navikt",
		Repo:      "spring-maven",
		Languages: []string{"Kotlin"},
		HasPomXML: true,
		HasAppYml: true,
	}

	output := GenerateSetupSteps(info)

	if !strings.Contains(output, "mvn compile -DskipTests") {
		t.Error("expected Spring Boot Maven compile step")
	}
}

func TestStackDetection(t *testing.T) {
	tests := []struct {
		name   string
		info   RepoInfo
		nextjs bool
		spring bool
		ktor   bool
	}{
		{
			name:   "Next.js project",
			info:   RepoInfo{HasPackageJSON: true, HasNextConfig: true, Languages: []string{"TypeScript"}},
			nextjs: true,
		},
		{
			name: "Plain TypeScript (not Next.js)",
			info: RepoInfo{HasPackageJSON: true, Languages: []string{"TypeScript"}},
		},
		{
			name:   "Spring Boot with Gradle",
			info:   RepoInfo{HasGradleKts: true, HasAppYml: true, Languages: []string{"Kotlin"}},
			spring: true,
		},
		{
			name:   "Spring Boot with Maven",
			info:   RepoInfo{HasPomXML: true, HasAppYml: true, Languages: []string{"Java"}},
			spring: true,
		},
		{
			name: "Ktor project",
			info: RepoInfo{HasGradleKts: true, Languages: []string{"Kotlin"}},
			ktor: true,
		},
		{
			name: "Generic Kotlin Maven (no application.yml)",
			info: RepoInfo{HasPomXML: true, Languages: []string{"Kotlin"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.isNextJS(); got != tt.nextjs {
				t.Errorf("isNextJS() = %v, want %v", got, tt.nextjs)
			}
			if got := tt.info.isSpringBoot(); got != tt.spring {
				t.Errorf("isSpringBoot() = %v, want %v", got, tt.spring)
			}
			if got := tt.info.isKtor(); got != tt.ktor {
				t.Errorf("isKtor() = %v, want %v", got, tt.ktor)
			}
		})
	}
}
