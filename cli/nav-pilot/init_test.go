package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectStack(t *testing.T) {
	tests := []struct {
		name   string
		files  []string // files/dirs to create
		wantGo bool
		wantNd bool // Node
		wantKt bool // Kotlin
		wantNs bool // Nais
	}{
		{
			name:   "Go project",
			files:  []string{"go.mod"},
			wantGo: true,
		},
		{
			name:   "Node project",
			files:  []string{"package.json"},
			wantNd: true,
		},
		{
			name:   "Kotlin Gradle project",
			files:  []string{"build.gradle.kts"},
			wantKt: true,
		},
		{
			name:   "Kotlin Maven project",
			files:  []string{"pom.xml"},
			wantKt: true,
		},
		{
			name:   "Go on Nais",
			files:  []string{"go.mod", ".nais/"},
			wantGo: true,
			wantNs: true,
		},
		{
			name:   "Fullstack Go + Node",
			files:  []string{"go.mod", "package.json"},
			wantGo: true,
			wantNd: true,
		},
		{
			name:  "Empty directory",
			files: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				if strings.HasSuffix(f, "/") {
					os.MkdirAll(filepath.Join(dir, f), 0o755)
				} else {
					os.WriteFile(filepath.Join(dir, f), []byte{}, 0o644)
				}
			}

			ds := detectStack(dir)
			if ds.Go != tt.wantGo {
				t.Errorf("Go: got %v, want %v", ds.Go, tt.wantGo)
			}
			if ds.Node != tt.wantNd {
				t.Errorf("Node: got %v, want %v", ds.Node, tt.wantNd)
			}
			if ds.Kotlin != tt.wantKt {
				t.Errorf("Kotlin: got %v, want %v", ds.Kotlin, tt.wantKt)
			}
			if ds.Nais != tt.wantNs {
				t.Errorf("Nais: got %v, want %v", ds.Nais, tt.wantNs)
			}
		})
	}
}

func TestStackLabel(t *testing.T) {
	tests := []struct {
		name  string
		stack DetectedStack
		want  string
	}{
		{"Go only", DetectedStack{Go: true}, "Go"},
		{"Go on Nais", DetectedStack{Go: true, Nais: true}, "Go on Nais"},
		{"Fullstack", DetectedStack{Go: true, Node: true, Nais: true}, "Go + Node.js/TypeScript on Nais"},
		{"Unknown", DetectedStack{}, "unknown stack"},
		{"Nais only", DetectedStack{Nais: true}, "Nais application"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stack.StackLabel()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKeyDirectories(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.MkdirAll(filepath.Join(dir, "cmd"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644)

	dirs := keyDirectories(dir)
	if len(dirs) != 2 {
		t.Fatalf("got %d dirs, want 2: %v", len(dirs), dirs)
	}
	if dirs[0] != "cmd/" || dirs[1] != "src/" {
		t.Errorf("got %v, want [cmd/ src/]", dirs)
	}
}

func TestTemplateAgentsMD(t *testing.T) {
	ds := DetectedStack{Name: "my-app", Go: true, Nais: true, RepoDir: t.TempDir()}
	os.MkdirAll(filepath.Join(ds.RepoDir, "cmd"), 0o755)
	os.MkdirAll(filepath.Join(ds.RepoDir, "internal"), 0o755)

	content := templateAgentsMD(ds)

	checks := []string{
		"# AGENTS.md — my-app",
		"go test ./...",
		"go build ./...",
		"cmd/",
		"internal/",
		"Minimal Editing",
		"✅ Always",
		"⚠️ Ask First",
		"🚫 Never",
	}
	for _, c := range checks {
		if !strings.Contains(content, c) {
			t.Errorf("AGENTS.md missing %q", c)
		}
	}

	// Should NOT contain Kotlin/Node commands
	if strings.Contains(content, "gradlew") {
		t.Error("AGENTS.md should not contain Gradle commands for Go project")
	}
}

func TestTemplateCopilotInstructions(t *testing.T) {
	ds := DetectedStack{Name: "my-service", Node: true, Nais: true}
	content := templateCopilotInstructions(ds)

	checks := []string{
		"# Copilot Instructions for my-service",
		"Nav-wide language and framework conventions",
		"- Node.js/TypeScript",
		"- Nais",
		"Minimal Editing",
	}
	for _, c := range checks {
		if !strings.Contains(content, c) {
			t.Errorf("copilot-instructions.md missing %q", c)
		}
	}
}

func TestTemplateReviewInstructions(t *testing.T) {
	ds := DetectedStack{Go: true}
	content := templateReviewInstructions(ds)

	if len(content) > 4000 {
		t.Errorf("review instructions exceed 4000 char limit: %d", len(content))
	}

	checks := []string{
		"## Go",
		"slog",
		"Security",
		"Over-editing",
		"Norwegian text",
	}
	for _, c := range checks {
		if !strings.Contains(content, c) {
			t.Errorf("review instructions missing %q", c)
		}
	}

	// Should NOT contain TypeScript/Kotlin sections
	if strings.Contains(content, "## TypeScript") {
		t.Error("review instructions should not contain TypeScript for Go-only project")
	}
}

func TestTemplateReviewInstructionsCharLimit(t *testing.T) {
	// Worst case: all stacks detected
	ds := DetectedStack{Go: true, Kotlin: true, Node: true}
	content := templateReviewInstructions(ds)
	if len(content) > 4000 {
		t.Errorf("review instructions with all stacks exceed 4000 char limit: %d chars", len(content))
	}
}

func TestCmdInit_DryRun(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0o644)

	err := cmdInit(dir, true, false)
	if err != nil {
		t.Fatalf("cmdInit dry run: %v", err)
	}

	// Verify no files were created
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Error("AGENTS.md should not exist after dry run")
	}
}

func TestCmdInit_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0o644)
	os.MkdirAll(filepath.Join(dir, "cmd"), 0o755)

	err := cmdInit(dir, false, false)
	if err != nil {
		t.Fatalf("cmdInit: %v", err)
	}

	// All three files should exist
	for _, f := range []string{
		"AGENTS.md",
		filepath.Join(".github", "copilot-instructions.md"),
		filepath.Join(".github", "copilot-review-instructions.md"),
	} {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", f)
		}
	}

	// AGENTS.md should contain Go commands
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !strings.Contains(string(data), "go test") {
		t.Error("AGENTS.md should contain go test command")
	}
}

func TestCmdInit_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)

	// Pre-create AGENTS.md with custom content
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("custom content"), 0o644)

	err := cmdInit(dir, false, false)
	if err != nil {
		t.Fatalf("cmdInit: %v", err)
	}

	// AGENTS.md should be unchanged
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if string(data) != "custom content" {
		t.Error("AGENTS.md should not have been overwritten")
	}

	// Other files should be created
	if _, err := os.Stat(filepath.Join(dir, ".github", "copilot-instructions.md")); os.IsNotExist(err) {
		t.Error("copilot-instructions.md should have been created")
	}
}

func TestCmdInit_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("old"), 0o644)

	err := cmdInit(dir, false, true)
	if err != nil {
		t.Fatalf("cmdInit --force: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if string(data) == "old" {
		t.Error("AGENTS.md should have been overwritten with --force")
	}
}

func TestCmdInit_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	err := cmdInit(dir, false, false)
	if err == nil {
		t.Error("cmdInit should fail outside git repo")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHintInitIfMissing(t *testing.T) {
	// Not a git repo — should not panic
	dir := t.TempDir()
	hintInitIfMissing(dir) // should be a no-op

	// Git repo with all files — no hint
	gitDir := t.TempDir()
	os.MkdirAll(filepath.Join(gitDir, ".git"), 0o755)
	os.WriteFile(filepath.Join(gitDir, "AGENTS.md"), []byte(""), 0o644)
	os.MkdirAll(filepath.Join(gitDir, ".github"), 0o755)
	os.WriteFile(filepath.Join(gitDir, ".github", "copilot-instructions.md"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(gitDir, ".github", "copilot-review-instructions.md"), []byte(""), 0o644)
	hintInitIfMissing(gitDir) // should be a no-op (all files present)

	// Git repo with missing files — should print (we just verify it doesn't error)
	missingDir := t.TempDir()
	os.MkdirAll(filepath.Join(missingDir, ".git"), 0o755)
	hintInitIfMissing(missingDir)
}
