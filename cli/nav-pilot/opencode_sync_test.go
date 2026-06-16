package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── validateOpenCodeStatePath ────────────────────────────────────────────────

func TestValidateOpenCodeStatePath(t *testing.T) {
	valid := []string{
		"AGENTS.md",
		"skills/security-review/",
		"commands/aksel-component.md",
		"agents/nav-pilot.md",
		"instructions/accessibility.md",
	}
	for _, p := range valid {
		if err := validateOpenCodeStatePath(p); err != nil {
			t.Errorf("validateOpenCodeStatePath(%q) unexpected error: %v", p, err)
		}
	}

	invalid := []string{
		"/absolute/path",
		"../traversal",
		"some/other/path.md",
		".github/agents/foo.agent.md",
	}
	for _, p := range invalid {
		if err := validateOpenCodeStatePath(p); err == nil {
			t.Errorf("validateOpenCodeStatePath(%q) expected error, got nil", p)
		}
	}
}

// ─── readOpenCodeState / writeOpenCodeState ───────────────────────────────────

func TestReadWriteOpenCodeState(t *testing.T) {
	dir := t.TempDir()

	state := &StateFile{
		Collection: openCodeCollection,
		Version:    "2026.06.16-120000",
		Scope:      openCodeScopeName,
		SourceSHA:  "abc123",
		Files: []InstalledFile{
			{Path: "AGENTS.md", Hash: "deadbeef"},
			{Path: "skills/foo/", Hash: "baadf00d"},
		},
	}

	if err := writeOpenCodeState(dir, state); err != nil {
		t.Fatalf("writeOpenCodeState: %v", err)
	}

	got, err := readOpenCodeState(dir)
	if err != nil {
		t.Fatalf("readOpenCodeState: %v", err)
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
	// Write a state with the wrong scope name
	state := &StateFile{
		Collection: "other",
		Version:    "1.0",
		Scope:      "user", // wrong scope
	}
	if err := writeOpenCodeState(dir, state); err != nil {
		t.Fatalf("writeOpenCodeState: %v", err)
	}
	_, err := readOpenCodeState(dir)
	if err == nil {
		t.Fatal("expected scope mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "scope mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadOpenCodeState_MissingFile(t *testing.T) {
	dir := t.TempDir()
	got, err := readOpenCodeState(dir)
	if err != nil {
		t.Fatalf("unexpected error on missing state: %v", err)
	}
	if got != nil {
		t.Error("expected nil state when file missing")
	}
}

// ─── syncOpenCodeArtifacts ────────────────────────────────────────────────────

func TestSyncOpenCodeArtifacts_FirstRun(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	skills, commands, agents, instructions, conflicts, err := syncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.16-120000", "abc123")
	if err != nil {
		t.Fatalf("syncOpenCodeArtifacts error: %v", err)
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

	// State must be written
	state, err := readOpenCodeState(outputDir)
	if err != nil {
		t.Fatalf("readOpenCodeState: %v", err)
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

	// AGENTS.md must exist
	if _, err := os.Stat(filepath.Join(outputDir, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md not created: %v", err)
	}
}

func TestSyncOpenCodeArtifacts_Idempotent(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	// First run
	s1, c1, a1, i1, conf1, err := syncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.16-120000", "abc")
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	agentsMD1, _ := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))

	// Second run — same source, same version
	s2, c2, a2, i2, conf2, err := syncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.16-120000", "abc")
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	// Counts and conflicts must match
	if s1 != s2 || c1 != c2 || a1 != a2 || i1 != i2 {
		t.Errorf("counts differ: %d/%d/%d/%d vs %d/%d/%d/%d", s1, c1, a1, i1, s2, c2, a2, i2)
	}
	if len(conf1) != 0 || len(conf2) != 0 {
		t.Errorf("unexpected conflicts: %v / %v", conf1, conf2)
	}

	// AGENTS.md must be identical
	agentsMD2, _ := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))
	if string(agentsMD1) != string(agentsMD2) {
		t.Error("AGENTS.md changed between identical runs — not idempotent")
	}
}

func TestSyncOpenCodeArtifacts_ConflictNotOverwritten(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	// First run — establishes state
	_, _, _, _, _, err := syncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.01-120000", "old")
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	// User modifies AGENTS.md
	agentsMDPath := filepath.Join(outputDir, "AGENTS.md")
	userContent := "# My custom AGENTS.md — do not overwrite\n"
	if err := os.WriteFile(agentsMDPath, []byte(userContent), 0o644); err != nil {
		t.Fatalf("writing user AGENTS.md: %v", err)
	}

	// Second run — same source, newer version
	_, _, _, _, conflicts, err := syncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.16-120000", "new")
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	// AGENTS.md must appear in conflicts
	found := false
	for _, c := range conflicts {
		if c == "AGENTS.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("AGENTS.md not in conflicts: %v", conflicts)
	}

	// AGENTS.md must NOT have been overwritten
	got, _ := os.ReadFile(agentsMDPath)
	if string(got) != userContent {
		t.Errorf("AGENTS.md was overwritten; got %q, want %q", string(got), userContent)
	}

	// State must record it as conflict
	state, err := readOpenCodeState(outputDir)
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	conflictInState := false
	for _, f := range state.Files {
		if f.Path == "AGENTS.md" && f.Status == fileStatusConflict {
			conflictInState = true
		}
	}
	if !conflictInState {
		t.Error("AGENTS.md not recorded as conflict in state")
	}
}

func TestSyncOpenCodeArtifacts_UpdatesStaleFile(t *testing.T) {
	// Two source dirs: v1 and v2 with different content
	sourceV1 := setupTestSource(t)
	sourceV2 := t.TempDir()
	// Copy v1 structure then overwrite an agent with new content
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

	// Establish state with v1
	_, _, _, _, _, err := syncOpenCodeArtifacts(sourceV1, outputDir, "2026.06.01-120000", "v1sha")
	if err != nil {
		t.Fatalf("v1 run error: %v", err)
	}
	agentV1, _ := os.ReadFile(filepath.Join(outputDir, "agents", "nav-pilot.md"))

	// Run with v2 (source advanced) — should update the agent
	_, _, _, _, conflicts, err := syncOpenCodeArtifacts(sourceV2, outputDir, "2026.06.16-120000", "v2sha")
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

	// State version must reflect v2
	state, _ := readOpenCodeState(outputDir)
	if state == nil || state.Version != "2026.06.16-120000" {
		t.Errorf("state version not updated: %v", state)
	}
}

// ─── printOpenCodeStatusBlock smoke test ─────────────────────────────────────

func TestPrintOpenCodeStatusBlock_NoError(t *testing.T) {
	sourceDir := setupTestSource(t)
	outputDir := t.TempDir()

	_, _, _, _, _, err := syncOpenCodeArtifacts(sourceDir, outputDir, "2026.06.16-120000", "abc")
	if err != nil {
		t.Fatalf("sync error: %v", err)
	}

	state, _ := readOpenCodeState(outputDir)
	if state == nil {
		t.Fatal("no state to print")
	}
	// Should not panic; output goes to stdout which we don't capture here
	printOpenCodeStatusBlock(outputDir, state)
}

// ─── cmdStatusAuto includes opencode ─────────────────────────────────────────

func TestCmdStatusAutoIncludesOpenCode(t *testing.T) {
	// Set opencode nav context dir to a temp dir
	old := openCodeNavContextDirOverride
	ocDir := t.TempDir()
	openCodeNavContextDirOverride = ocDir
	defer func() { openCodeNavContextDirOverride = old }()

	// Write a minimal opencode state
	state := &StateFile{
		Collection: openCodeCollection,
		Version:    "2026.06.16-120000",
		Scope:      openCodeScopeName,
		SourceSHA:  "abc",
		Files: []InstalledFile{
			{Path: "AGENTS.md", Hash: "deadbeef"},
		},
	}
	if err := writeOpenCodeState(ocDir, state); err != nil {
		t.Fatalf("writeOpenCodeState: %v", err)
	}

	// Create a fake AGENTS.md so integrity check doesn't fail on missing
	if err := os.WriteFile(filepath.Join(ocDir, "AGENTS.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("writing AGENTS.md: %v", err)
	}

	// cmdStatusAuto should NOT return an error even though no .github/ scope is installed
	// (it will print "No nav-pilot collection installed" for the .github/ scopes
	//  and then also show the opencode status block)
	// We just verify it doesn't error out
	err := cmdStatusAuto(t.TempDir(), false)
	if err != nil {
		t.Errorf("cmdStatusAuto error: %v", err)
	}
}
