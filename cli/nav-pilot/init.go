package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ─── Stack detection ────────────────────────────────────────────────────────

// DetectedStack describes the technologies found in a project directory.
type DetectedStack struct {
	Go      bool
	Node    bool
	Kotlin  bool
	Nais    bool
	RepoDir string
	Name    string // basename of repo dir
}

// detectStack checks for stack indicators in the target directory.
func detectStack(dir string) DetectedStack {
	ds := DetectedStack{
		RepoDir: dir,
		Name:    filepath.Base(dir),
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		ds.Go = true
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		ds.Node = true
	}
	for _, f := range []string{"build.gradle.kts", "build.gradle", "pom.xml"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			ds.Kotlin = true
			break
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".nais")); err == nil {
		ds.Nais = true
	}
	return ds
}

// Languages returns a human-readable list of detected languages.
func (ds DetectedStack) Languages() []string {
	var langs []string
	if ds.Go {
		langs = append(langs, "Go")
	}
	if ds.Node {
		langs = append(langs, "Node.js/TypeScript")
	}
	if ds.Kotlin {
		langs = append(langs, "Kotlin")
	}
	return langs
}

// StackLabel returns a short summary like "Go + Node.js/TypeScript on Nais".
func (ds DetectedStack) StackLabel() string {
	langs := ds.Languages()
	if len(langs) == 0 {
		if ds.Nais {
			return "Nais application"
		}
		return "unknown stack"
	}
	label := strings.Join(langs, " + ")
	if ds.Nais {
		label += " on Nais"
	}
	return label
}

// ─── Key directories ────────────────────────────────────────────────────────

// keyDirectories returns the top-level directories worth mentioning in AGENTS.md.
func keyDirectories(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	skip := map[string]bool{
		".git": true, ".github": true, ".idea": true, ".vscode": true,
		"node_modules": true, "build": true, "dist": true, "target": true,
		".gradle": true, ".nais": true, ".nav-pilot-state.json": true,
	}

	var dirs []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || skip[e.Name()] {
			continue
		}
		dirs = append(dirs, e.Name()+"/")
	}
	sort.Strings(dirs)
	return dirs
}

// ─── Templates ──────────────────────────────────────────────────────────────

func templateAgentsMD(ds DetectedStack) string {
	var b strings.Builder

	b.WriteString("# AGENTS.md — " + ds.Name + "\n\n")
	b.WriteString("<!-- TODO: Describe what this application does -->\n\n")

	// Build & Test Commands
	b.WriteString("## Build & Test Commands\n\n")
	b.WriteString("```bash\n")
	if ds.Go {
		b.WriteString("go test ./...       # Run tests\n")
		b.WriteString("go build ./...      # Build\n")
		b.WriteString("go vet ./...        # Lint\n")
	}
	if ds.Kotlin {
		b.WriteString("./gradlew test      # Run tests\n")
		b.WriteString("./gradlew build     # Build\n")
	}
	if ds.Node {
		b.WriteString("pnpm test           # Run tests\n")
		b.WriteString("pnpm build          # Build\n")
		b.WriteString("pnpm lint           # Lint\n")
	}
	if !ds.Go && !ds.Kotlin && !ds.Node {
		b.WriteString("# TODO: Add build and test commands\n")
	}
	b.WriteString("```\n\n")

	// Project Structure
	b.WriteString("## Project Structure\n\n")
	b.WriteString("```text\n")
	dirs := keyDirectories(ds.RepoDir)
	if len(dirs) > 0 {
		for _, d := range dirs {
			b.WriteString(d + "\n")
		}
	} else {
		b.WriteString("# TODO: Add key directories\n")
	}
	b.WriteString("```\n\n")

	// Code Style
	b.WriteString("## Code Style\n\n")
	b.WriteString("### Minimal Editing\n\n")
	b.WriteString("When fixing a bug or implementing a feature, change only what is necessary.\n")
	b.WriteString("Do not rename variables, restructure working code, or refactor beyond the task at hand.\n")
	b.WriteString("Keep diffs small and focused so they are easy to review.\n\n")

	// Git Workflow
	b.WriteString("## Git Workflow\n\n")
	b.WriteString("<!-- TODO: Document your branching and merge strategy -->\n\n")

	// Boundaries
	b.WriteString("## Boundaries\n\n")
	b.WriteString("### ✅ Always\n\n")
	b.WriteString("- Run tests after changes\n")
	b.WriteString("- Follow existing code patterns in the project\n")
	b.WriteString("- Preserve existing code structure — do not reorganize or refactor beyond the task\n")
	b.WriteString("- Validate all external input\n\n")
	b.WriteString("### ⚠️ Ask First\n\n")
	b.WriteString("- Changing authentication mechanisms\n")
	b.WriteString("- Adding new dependencies\n")
	b.WriteString("- Modifying database schema\n\n")
	b.WriteString("### 🚫 Never\n\n")
	b.WriteString("- Commit secrets or credentials\n")
	b.WriteString("- Skip input validation on external boundaries\n")

	return b.String()
}

func templateCopilotInstructions(ds DetectedStack) string {
	var b strings.Builder

	b.WriteString("# Copilot Instructions for " + ds.Name + "\n\n")
	b.WriteString("<!-- This file captures repository-specific context.\n")
	b.WriteString("     Nav-wide language and framework conventions are provided by installed Copilot instructions. -->\n\n")

	b.WriteString("## Repository Overview\n\n")
	b.WriteString("<!-- TODO: Describe what this application does, who uses it, and key architecture decisions -->\n\n")

	b.WriteString("## Tech Stack\n\n")
	langs := ds.Languages()
	if len(langs) > 0 {
		for _, l := range langs {
			b.WriteString("- " + l + "\n")
		}
		if ds.Nais {
			b.WriteString("- Nais (Kubernetes on GCP)\n")
		}
		b.WriteString("\n")
	} else {
		b.WriteString("<!-- TODO: List technologies used -->\n\n")
	}

	b.WriteString("## Key Patterns\n\n")
	b.WriteString("<!-- TODO: Document project-specific patterns, e.g.:\n")
	b.WriteString("     - Authentication flow\n")
	b.WriteString("     - Data access patterns\n")
	b.WriteString("     - API conventions -->\n\n")

	b.WriteString("## Minimal Editing\n\n")
	b.WriteString("When fixing a bug or implementing a feature, change only what is necessary.\n")
	b.WriteString("Do not rename variables, restructure working code, or refactor beyond the task at hand.\n")
	b.WriteString("Keep diffs small and focused so they are easy to review.\n")

	return b.String()
}

func templateReviewInstructions(ds DetectedStack) string {
	var b strings.Builder

	// Stack-specific rules
	if ds.Go {
		b.WriteString("## Go\n\n")
		b.WriteString("- Error wrapping: use `fmt.Errorf(\"context: %w\", err)`, never `%v`\n")
		b.WriteString("- Structured logging with `slog`, never `fmt.Println` or `log.Println`\n")
		b.WriteString("- All SQL queries must be parameterized (`$1`, `$2`)\n\n")
	}
	if ds.Kotlin {
		b.WriteString("## Kotlin\n\n")
		b.WriteString("- Parameterized SQL queries (`?` or named params), never string concatenation\n")
		b.WriteString("- Use Kotest matchers (`shouldBe`) in tests\n")
		b.WriteString("- Prefer sealed classes for state modeling\n\n")
	}
	if ds.Node {
		b.WriteString("## TypeScript\n\n")
		b.WriteString("- Use Aksel Design System spacing tokens (`space-16`), never Tailwind `p-*`/`m-*`\n")
		b.WriteString("- TypeScript strict mode — no `any` without justification\n")
		b.WriteString("- Named imports from `@navikt/ds-react`, never `import *`\n\n")
	}

	b.WriteString("## Norwegian text (all `.md` and user-facing strings)\n\n")
	b.WriteString("- Use Norwegian bokmål for user-facing text\n")
	b.WriteString("- Avoid unnecessary anglicisms when good Norwegian alternatives exist\n\n")

	b.WriteString("## Security\n\n")
	b.WriteString("- No secrets, tokens, or credentials in code\n")
	b.WriteString("- SQL queries must be parameterized\n")
	b.WriteString("- GitHub Actions pinned to full SHA with version comment\n\n")

	b.WriteString("## Over-editing\n\n")
	b.WriteString("Flag changes where the diff is disproportionate to the stated goal:\n\n")
	b.WriteString("- Renamed variables or parameters not related to the fix\n")
	b.WriteString("- Restructured working code without justification\n")
	b.WriteString("- Added refactoring outside the PR scope\n")

	content := b.String()

	if len(content) > 4000 {
		// Trim to fit — remove optional sections from end
		content = content[:3990] + "\n"
	}

	return content
}

// ─── Init targets ───────────────────────────────────────────────────────────

type initTarget struct {
	RelPath string // e.g. "AGENTS.md" or ".github/copilot-instructions.md"
	Content string
}

func initTargets(ds DetectedStack) []initTarget {
	return []initTarget{
		{"AGENTS.md", templateAgentsMD(ds)},
		{".github/copilot-instructions.md", templateCopilotInstructions(ds)},
		{".github/copilot-review-instructions.md", templateReviewInstructions(ds)},
	}
}

// ─── Command ────────────────────────────────────────────────────────────────

func cmdInit(targetDir string, dryRun, force bool) error {
	if _, err := os.Stat(filepath.Join(targetDir, ".git")); os.IsNotExist(err) {
		return fmt.Errorf("target %q does not appear to be a git repository (no .git directory)", targetDir)
	}

	ds := detectStack(targetDir)

	fmt.Println(bold("nav-pilot init"))
	fmt.Println()
	fmt.Printf("  %s %s\n", dim("Repo:"), ds.Name)
	fmt.Printf("  %s %s\n", dim("Stack:"), ds.StackLabel())
	fmt.Println()

	targets := initTargets(ds)

	created := 0
	skipped := 0

	for _, t := range targets {
		absPath := filepath.Join(targetDir, t.RelPath)

		if _, err := os.Stat(absPath); err == nil && !force {
			fmt.Printf("  %s %s (exists — use --force to overwrite)\n", yellow("⚠"), t.RelPath)
			skipped++
			continue
		}

		if dryRun {
			fmt.Printf("  %s %s\n", dim("→"), t.RelPath)
			created++
			continue
		}

		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", t.RelPath, err)
		}
		if err := os.WriteFile(absPath, []byte(t.Content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", t.RelPath, err)
		}
		fmt.Printf("  %s %s\n", green("✓"), t.RelPath)
		created++
	}

	fmt.Println()
	if dryRun {
		fmt.Printf("%s Would create %d file(s).\n", dim("→"), created)
	} else if created > 0 {
		fmt.Printf("%s Created %d file(s).\n", green("✓"), created)
		fmt.Println()
		fmt.Println(dim("Next steps:"))
		fmt.Println(dim("  1. Fill in the TODO placeholders"))
		fmt.Println(dim("  2. Commit and push to enable Copilot customization"))
		fmt.Println(dim("  3. Install agents and skills: nav-pilot install <collection>"))
		fmt.Println(dim("  4. See what's available: nav-pilot list"))
	}
	if skipped > 0 {
		fmt.Printf("%s Skipped %d existing file(s).\n", yellow("⚠"), skipped)
	}

	return nil
}

// hintInitIfMissing prints a suggestion to run `nav-pilot init` if repo-local
// config files are missing. Called after successful --user install when cwd is a git repo.
func hintInitIfMissing(dir string) {
	if !isGitRepo(dir) {
		return
	}

	missing := []string{}
	for _, f := range []string{
		"AGENTS.md",
		filepath.Join(".github", "copilot-instructions.md"),
		filepath.Join(".github", "copilot-review-instructions.md"),
	} {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			missing = append(missing, f)
		}
	}

	if len(missing) == 0 {
		return
	}

	fmt.Println()
	fmt.Printf("%s This repo is missing repo-local Copilot config (%d file(s)).\n",
		dim("💡"), len(missing))
	fmt.Printf("   Run %s to scaffold them.\n", bold("nav-pilot init"))
}
