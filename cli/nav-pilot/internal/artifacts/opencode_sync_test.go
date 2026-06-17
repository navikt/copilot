package artifacts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

func TestValidateOpenCodeStatePath(t *testing.T) {
	valid := []string{
		"AGENTS.md",
		"skills/security-review/",
		"commands/aksel-component.md",
		"agents/nav-pilot.md",
		"instructions/accessibility.md",
	}
	for _, p := range valid {
		if err := ValidateOpenCodeStatePath(p); err != nil {
			t.Errorf("ValidateOpenCodeStatePath(%q) unexpected error: %v", p, err)
		}
	}

	invalid := []string{
		"/absolute/path",
		"../traversal",
		"some/other/path.md",
		".github/agents/foo.agent.md",
	}
	for _, p := range invalid {
		if err := ValidateOpenCodeStatePath(p); err == nil {
			t.Errorf("ValidateOpenCodeStatePath(%q) expected error, got nil", p)
		}
	}
}

func TestReadWriteOpenCodeState(t *testing.T) {
	dir := t.TempDir()

	state := &domain.StateFile{
		Collection: OpenCodeCollection,
		Version:    "2026.06.16-120000",
		Scope:      OpenCodeScopeName,
		SourceSHA:  "abc123",
		Files: []domain.InstalledFile{
			{Path: "AGENTS.md", Hash: "deadbeef"},
			{Path: "skills/foo/", Hash: "baadf00d"},
		},
	}

	if err := WriteOpenCodeState(dir, state); err != nil {
		t.Fatalf("WriteOpenCodeState: %v", err)
	}

	got, err := ReadOpenCodeState(dir)
	if err != nil {
		t.Fatalf("ReadOpenCodeState: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil state")
	}
	if got.Version != state.Version {
		t.Errorf("Version = %q, want %q", got.Version, state.Version)
	}
	if got.SourceSHA != state.SourceSHA {
		t.Errorf("SourceSHA = %q, want %q", got.SourceSHA, state.SourceSHA)
	}
	if len(got.Files) != 2 {
		t.Errorf("Files count = %d, want 2", len(got.Files))
	}
}

func TestReadOpenCodeState_ScopeMismatch(t *testing.T) {
	dir := t.TempDir()
	state := &domain.StateFile{
		Collection: "other",
		Version:    "1.0",
		Scope:      "user",
	}
	if err := WriteOpenCodeState(dir, state); err != nil {
		t.Fatalf("WriteOpenCodeState: %v", err)
	}
	_, err := ReadOpenCodeState(dir)
	if err == nil {
		t.Fatal("expected scope mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "scope mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadOpenCodeState_MissingFile(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadOpenCodeState(dir)
	if err != nil {
		t.Fatalf("unexpected error on missing state: %v", err)
	}
	if got != nil {
		t.Error("expected nil state when file missing")
	}
}

func TestSyncOpenCodeArtifacts_FirstRun(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	skills, commands, agents, instructions, conflicts, err := SyncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.16-120000", "abc123")
	if err != nil {
		t.Fatalf("SyncOpenCodeArtifacts error: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts on first run, got: %v", conflicts)
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

	state, err := ReadOpenCodeState(outputDir)
	if err != nil {
		t.Fatalf("ReadOpenCodeState: %v", err)
	}
	if state == nil {
		t.Fatal("state not written after first run")
	}
	if state.Version != "2026.06.16-120000" {
		t.Errorf("state.Version = %q, want 2026.06.16-120000", state.Version)
	}
	if state.SourceSHA != "abc123" {
		t.Errorf("state.SourceSHA = %q, want abc123", state.SourceSHA)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md not created: %v", err)
	}
}

func TestSyncOpenCodeArtifacts_Idempotent(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	s1, c1, a1, i1, conf1, err := SyncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.16-120000", "abc")
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	agentsMD1, _ := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))

	s2, c2, a2, i2, conf2, err := SyncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.16-120000", "abc")
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	if s1 != s2 || c1 != c2 || a1 != a2 || i1 != i2 {
		t.Errorf("counts differ: %d/%d/%d/%d vs %d/%d/%d/%d", s1, c1, a1, i1, s2, c2, a2, i2)
	}
	if len(conf1) != 0 || len(conf2) != 0 {
		t.Errorf("unexpected conflicts: %v / %v", conf1, conf2)
	}

	agentsMD2, _ := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))
	if string(agentsMD1) != string(agentsMD2) {
		t.Error("AGENTS.md changed between identical runs — not idempotent")
	}
}

func TestSyncOpenCodeArtifacts_ConflictNotOverwritten(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	if _, _, _, _, _, err := SyncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.01-120000", "old"); err != nil {
		t.Fatalf("first run error: %v", err)
	}

	agentsMDPath := filepath.Join(outputDir, "AGENTS.md")
	userContent := "# My custom AGENTS.md — do not overwrite\n"
	if err := os.WriteFile(agentsMDPath, []byte(userContent), 0o644); err != nil {
		t.Fatalf("writing user AGENTS.md: %v", err)
	}

	_, _, _, _, conflicts, err := SyncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.16-120000", "new")
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	found := false
	for _, c := range conflicts {
		if c == "AGENTS.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("AGENTS.md not in conflicts: %v", conflicts)
	}

	got, _ := os.ReadFile(agentsMDPath)
	if string(got) != userContent {
		t.Errorf("AGENTS.md was overwritten; got %q, want %q", string(got), userContent)
	}

	state, err := ReadOpenCodeState(outputDir)
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	conflictInState := false
	for _, f := range state.Files {
		if f.Path == "AGENTS.md" && f.Status == domain.FileStatusConflict {
			conflictInState = true
		}
	}
	if !conflictInState {
		t.Error("AGENTS.md not recorded as conflict in state")
	}
}

func TestSyncOpenCodeArtifacts_UpdatesStaleFile(t *testing.T) {
	sourceV1 := setupTestSource(t)
	sourceV2 := t.TempDir()
	mustMkdir(t, filepath.Join(sourceV2, ".github", "agents"))
	mustWrite(t, filepath.Join(sourceV2, ".github", "agents", "nav-pilot.agent.md"), `---
name: nav-pilot
description: Updated Nav-pilot agent
tools:
  - read
---

Updated content.
`)

	outputDir := t.TempDir()

	if _, _, _, _, _, err := SyncOpenCodeArtifacts(sourceV1, outputDir, "2026.06.01-120000", "v1sha"); err != nil {
		t.Fatalf("v1 run error: %v", err)
	}
	agentV1, _ := os.ReadFile(filepath.Join(outputDir, "agents", "nav-pilot.md"))

	_, _, _, _, conflicts, err := SyncOpenCodeArtifacts(sourceV2, outputDir, "2026.06.16-120000", "v2sha")
	if err != nil {
		t.Fatalf("v2 run error: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("unexpected conflicts on stale update: %v", conflicts)
	}

	agentV2, _ := os.ReadFile(filepath.Join(outputDir, "agents", "nav-pilot.md"))
	if string(agentV1) == string(agentV2) {
		t.Error("agent file was not updated when source advanced")
	}
	if !strings.Contains(string(agentV2), "Updated Nav-pilot agent") {
		t.Error("agent file missing updated description")
	}

	state, _ := ReadOpenCodeState(outputDir)
	if state == nil || state.Version != "2026.06.16-120000" {
		t.Errorf("state version not updated: %v", state)
	}
}

func TestPrintOpenCodeStatusBlock_NoError(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	if _, _, _, _, _, err := SyncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.16-120000", "abc"); err != nil {
		t.Fatalf("sync error: %v", err)
	}

	state, _ := ReadOpenCodeState(outputDir)
	if state == nil {
		t.Fatal("no state to print")
	}
	PrintOpenCodeStatusBlock(outputDir, state)
}

func TestSyncOpenCodeArtifacts_RejectsSymlink(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()
	outside := t.TempDir()

	// Create a symlink inside outputDir pointing outside the boundary.
	agentsDir := filepath.Join(outputDir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	symlinkPath := filepath.Join(agentsDir, "nav-pilot.md")
	if err := os.Symlink(filepath.Join(outside, "nav-pilot.md"), symlinkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	_, _, _, _, _, err := SyncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.16-120000", "abc")
	if err == nil {
		t.Error("SyncOpenCodeArtifacts() = nil, want error when writing through symlink")
	}
}
