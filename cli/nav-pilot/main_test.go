package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── CLI parsing tests ──────────────────────────────────────────────────────

func TestRun_NoArgs(t *testing.T) {
	// Prevent TUI from blocking when running in an interactive terminal
	forceNonInteractive = true
	t.Cleanup(func() { forceNonInteractive = false })

	err := run([]string{})
	if err != nil {
		t.Fatalf("expected no error for no args (shows usage), got: %v", err)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := run([]string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_UnknownFlag(t *testing.T) {
	err := run([]string{"install", "--bogus"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_InstallNoCollection(t *testing.T) {
	err := run([]string{"install"})
	if err == nil {
		t.Fatal("expected error when no collection given")
	}
	if !strings.Contains(err.Error(), "collection name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_Version(t *testing.T) {
	err := run([]string{"version"})
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
}

func TestRun_Help(t *testing.T) {
	err := run([]string{"help"})
	if err != nil {
		t.Fatalf("help command failed: %v", err)
	}
}

func TestRun_TargetMissingValue(t *testing.T) {
	err := run([]string{"install", "--target"})
	if err == nil {
		t.Fatal("expected error for --target without value")
	}
}

func TestRun_RefMissingValue(t *testing.T) {
	err := run([]string{"install", "--ref"})
	if err == nil {
		t.Fatal("expected error for --ref without value")
	}
}

func TestIsGitRepo(t *testing.T) {
	// Temp dir with .git
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !isGitRepo(dir) {
		t.Error("expected true for dir with .git")
	}

	// Temp dir without .git
	dir2 := t.TempDir()
	if isGitRepo(dir2) {
		t.Error("expected false for dir without .git")
	}
}

// ─── Manifest tests ────────────────────────────────────────────────────────

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	collectionsDir := filepath.Join(dir, ".github", "collections", "test-collection")
	if err := os.MkdirAll(collectionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := Manifest{
		Name:         "test-collection",
		Description:  "Test collection",
		Version:      "2025.01",
		Agents:       []string{"agent-a", "agent-b"},
		Skills:       []string{"skill-a"},
		Instructions: []string{"instr-a"},
		Prompts:      []string{"prompt-a"},
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(collectionsDir, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := loadManifest(dir, "test-collection")
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if got.Name != "test-collection" {
		t.Errorf("name = %q, want %q", got.Name, "test-collection")
	}
	if len(got.Agents) != 2 {
		t.Errorf("agents count = %d, want 2", len(got.Agents))
	}
	if got.Version != "2025.01" {
		t.Errorf("version = %q, want %q", got.Version, "2025.01")
	}
}

func TestLoadManifest_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := loadManifest(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent collection")
	}
}

func TestLoadManifest_InvalidAgent(t *testing.T) {
	dir := t.TempDir()
	collectionsDir := filepath.Join(dir, ".github", "collections", "bad")
	os.MkdirAll(collectionsDir, 0o755)
	os.WriteFile(filepath.Join(collectionsDir, "manifest.json"),
		[]byte(`{"name":"bad","agents":["../etc/passwd"]}`), 0o644)

	_, err := loadManifest(dir, "bad")
	if err == nil {
		t.Fatal("expected error for path traversal agent name")
	}
	if !strings.Contains(err.Error(), "invalid agent") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadManifest_DuplicateSkill(t *testing.T) {
	dir := t.TempDir()
	collectionsDir := filepath.Join(dir, ".github", "collections", "dup")
	os.MkdirAll(collectionsDir, 0o755)
	os.WriteFile(filepath.Join(collectionsDir, "manifest.json"),
		[]byte(`{"name":"dup","skills":["a","a"]}`), 0o644)

	_, err := loadManifest(dir, "dup")
	if err == nil {
		t.Fatal("expected error for duplicate skill")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadManifest_EmptyName(t *testing.T) {
	dir := t.TempDir()
	collectionsDir := filepath.Join(dir, ".github", "collections", "empty")
	os.MkdirAll(collectionsDir, 0o755)
	os.WriteFile(filepath.Join(collectionsDir, "manifest.json"),
		[]byte(`{"name":""}`), 0o644)

	_, err := loadManifest(dir, "empty")
	if err == nil {
		t.Fatal("expected error for empty manifest name")
	}
}

func TestValidateManifest_Valid(t *testing.T) {
	m := &Manifest{
		Name:   "test",
		Agents: []string{"auth", "nais"},
		Skills: []string{"api-design"},
	}
	if err := validateManifest(m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListCollectionDirs(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		cDir := filepath.Join(dir, ".github", "collections", name)
		os.MkdirAll(cDir, 0o755)
		os.WriteFile(filepath.Join(cDir, "manifest.json"), []byte(`{"name":"`+name+`"}`), 0o644)
	}
	// Dir without manifest should be ignored
	os.MkdirAll(filepath.Join(dir, ".github", "collections", "no-manifest"), 0o755)

	names, err := listCollectionDirs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("got %d collections, want 2", len(names))
	}
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("got %v, want [alpha, beta]", names)
	}
}

func TestStateFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".github"), 0o755)

	state := &StateFile{
		Collection:  "kotlin-backend",
		Version:     "2025.07",
		SourceSHA:   "abc1234",
		InstalledAt: "2025-07-01T12:00:00Z",
		Files: []InstalledFile{
			{Path: ".github/agents/test.agent.md", Hash: "deadbeef12345678"},
			{Path: ".github/skills/test-skill/", Hash: "cafebabe12345678"},
		},
	}

	if err := writeState(dir, state); err != nil {
		t.Fatalf("writeState: %v", err)
	}

	got, err := readState(dir)
	if err != nil {
		t.Fatalf("readState: %v", err)
	}
	if got.Collection != "kotlin-backend" {
		t.Errorf("collection = %q, want %q", got.Collection, "kotlin-backend")
	}
	if len(got.Files) != 2 {
		t.Errorf("files count = %d, want 2", len(got.Files))
	}
	if got.Files[0].Hash != "deadbeef12345678" {
		t.Errorf("hash = %q, want %q", got.Files[0].Hash, "deadbeef12345678")
	}
}

func TestReadState_NoFile(t *testing.T) {
	dir := t.TempDir()
	state, err := readState(dir)
	if err != nil {
		t.Fatalf("readState: %v", err)
	}
	if state != nil {
		t.Errorf("expected nil state, got %+v", state)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "subdir", "dst.txt")

	os.WriteFile(src, []byte("hello world"), 0o644)

	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello world" {
		t.Errorf("got %q, want %q", string(got), "hello world")
	}
}

func TestCopyDir(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	// Create source structure
	os.MkdirAll(filepath.Join(src, "refs"), 0o755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# Skill"), 0o644)
	os.WriteFile(filepath.Join(src, "metadata.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(src, "refs", "data.md"), []byte("# Data"), 0o644)

	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	// Verify all files exist
	for _, relPath := range []string{"SKILL.md", "metadata.json", "refs/data.md"} {
		path := filepath.Join(dst, relPath)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", relPath, err)
		}
	}
}

func TestCopyDir_RemovesStaleFiles(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	// First version: src has two files
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "keep.md"), []byte("keep"), 0o644)
	os.WriteFile(filepath.Join(src, "old.md"), []byte("old"), 0o644)

	copyDir(src, dst)

	// Second version: remove old.md from source
	os.Remove(filepath.Join(src, "old.md"))
	copyDir(src, dst)

	// old.md should not exist in destination
	if _, err := os.Stat(filepath.Join(dst, "old.md")); !os.IsNotExist(err) {
		t.Error("stale file old.md should have been removed")
	}
}

func TestFileHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0o644)

	h1, err := fileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(h1) != 16 {
		t.Errorf("hash length = %d, want 16", len(h1))
	}

	// Same content = same hash
	path2 := filepath.Join(dir, "test2.txt")
	os.WriteFile(path2, []byte("hello"), 0o644)
	h2, _ := fileHash(path2)
	if h1 != h2 {
		t.Error("identical files should have identical hashes")
	}

	// Different content = different hash
	os.WriteFile(path2, []byte("world"), 0o644)
	h3, _ := fileHash(path2)
	if h1 == h3 {
		t.Error("different files should have different hashes")
	}
}

func TestCheckConflict_NoConflictNewFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "nonexistent.txt")
	os.WriteFile(src, []byte("hello"), 0o644)

	c, err := checkConflict(dst, src, false)
	if err != nil {
		t.Fatal(err)
	}
	if c != nil {
		t.Error("expected no conflict for new file")
	}
}

func TestCheckConflict_NoConflictIdentical(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	os.WriteFile(src, []byte("hello"), 0o644)
	os.WriteFile(dst, []byte("hello"), 0o644)

	c, err := checkConflict(dst, src, false)
	if err != nil {
		t.Fatal(err)
	}
	if c != nil {
		t.Error("expected no conflict for identical files")
	}
}

func TestCheckConflict_ConflictDiffers(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	os.WriteFile(src, []byte("hello"), 0o644)
	os.WriteFile(dst, []byte("modified locally"), 0o644)

	c, err := checkConflict(dst, src, false)
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected conflict for differing files")
	}
	if c.Current == c.New {
		t.Error("conflict hashes should differ")
	}
}

func TestInstallAgent(t *testing.T) {
	// Set up source
	srcDir := t.TempDir()
	agentsDir := filepath.Join(srcDir, ".github", "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "test.agent.md"), []byte("---\nname: test\n---\n# Test"), 0o644)
	os.WriteFile(filepath.Join(agentsDir, "test.metadata.json"), []byte(`{"domain":"general"}`), 0o644)

	// Set up target
	dstDir := t.TempDir()
	result := &installResult{}

	err := installAgent(srcDir, ScopeRepo(dstDir), "test", false, false, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Installed != 1 {
		t.Errorf("installed = %d, want 1", result.Installed)
	}
	if len(result.Files) != 2 { // agent.md + metadata.json
		t.Errorf("files count = %d, want 2", len(result.Files))
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(dstDir, ".github", "agents", "test.agent.md")); err != nil {
		t.Error("agent.md not created")
	}
	if _, err := os.Stat(filepath.Join(dstDir, ".github", "agents", "test.metadata.json")); err != nil {
		t.Error("metadata.json not created")
	}
}

func TestInstallAgent_NotFound(t *testing.T) {
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, ".github", "agents"), 0o755)
	dstDir := t.TempDir()
	result := &installResult{}

	err := installAgent(srcDir, ScopeRepo(dstDir), "nonexistent", false, false, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", result.Skipped)
	}
}

func TestInstallSkill(t *testing.T) {
	srcDir := t.TempDir()
	skillDir := filepath.Join(srcDir, ".github", "skills", "my-skill")
	refsDir := filepath.Join(skillDir, "references")
	os.MkdirAll(refsDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0o644)
	os.WriteFile(filepath.Join(skillDir, "metadata.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(refsDir, "data.md"), []byte("# Data"), 0o644)

	dstDir := t.TempDir()
	result := &installResult{}

	err := installSkill(srcDir, ScopeRepo(dstDir), "my-skill", false, false, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Installed != 1 {
		t.Errorf("installed = %d, want 1", result.Installed)
	}

	// Verify references are copied
	refPath := filepath.Join(dstDir, ".github", "skills", "my-skill", "references", "data.md")
	if _, err := os.Stat(refPath); err != nil {
		t.Error("reference file not copied")
	}
}

func TestInstallConflictBlocked(t *testing.T) {
	srcDir := t.TempDir()
	agentsDir := filepath.Join(srcDir, ".github", "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "test.agent.md"), []byte("source content"), 0o644)

	dstDir := t.TempDir()
	dstAgents := filepath.Join(dstDir, ".github", "agents")
	os.MkdirAll(dstAgents, 0o755)
	os.WriteFile(filepath.Join(dstAgents, "test.agent.md"), []byte("local modified content"), 0o644)

	result := &installResult{}
	err := installAgent(srcDir, ScopeRepo(dstDir), "test", false, false, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Conflicts != 1 {
		t.Errorf("conflicts = %d, want 1", result.Conflicts)
	}
	if result.Installed != 0 {
		t.Errorf("installed = %d, want 0", result.Installed)
	}
}

func TestInstallConflictForced(t *testing.T) {
	srcDir := t.TempDir()
	agentsDir := filepath.Join(srcDir, ".github", "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "test.agent.md"), []byte("source content"), 0o644)

	dstDir := t.TempDir()
	dstAgents := filepath.Join(dstDir, ".github", "agents")
	os.MkdirAll(dstAgents, 0o755)
	os.WriteFile(filepath.Join(dstAgents, "test.agent.md"), []byte("local modified content"), 0o644)

	result := &installResult{}
	err := installAgent(srcDir, ScopeRepo(dstDir), "test", false, true, result) // force=true
	if err != nil {
		t.Fatal(err)
	}
	if result.Installed != 1 {
		t.Errorf("installed = %d, want 1", result.Installed)
	}

	// Verify content was overwritten
	got, _ := os.ReadFile(filepath.Join(dstAgents, "test.agent.md"))
	if string(got) != "source content" {
		t.Errorf("file not overwritten, got %q", string(got))
	}
}

// ─── Validation tests ───────────────────────────────────────────────────────

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid-name", false},
		{"my_skill", false},
		{"kotlin-backend", false},
		{"", true},
		{"..", true},
		{"../etc/passwd", true},
		{"foo/bar", true},
		{"foo\\bar", true},
		{"..sneaky", true},
		{"a/../b", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestInstallAgent_PathTraversal(t *testing.T) {
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, ".github", "agents"), 0o755)
	dstDir := t.TempDir()
	result := &installResult{}

	err := installAgent(srcDir, ScopeRepo(dstDir), "../../../etc/passwd", false, false, result)
	if err == nil {
		t.Fatal("expected error for path traversal attempt")
	}
	if !strings.Contains(err.Error(), "invalid agent name") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── Instruction and prompt tests ───────────────────────────────────────────

func TestInstallInstruction(t *testing.T) {
	srcDir := t.TempDir()
	instrDir := filepath.Join(srcDir, ".github", "instructions")
	os.MkdirAll(instrDir, 0o755)
	os.WriteFile(filepath.Join(instrDir, "my-instr.instructions.md"), []byte("# Instruction"), 0o644)

	dstDir := t.TempDir()
	result := &installResult{}

	err := installInstruction(srcDir, ScopeRepo(dstDir), "my-instr", false, false, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Installed != 1 {
		t.Errorf("installed = %d, want 1", result.Installed)
	}

	dstPath := filepath.Join(dstDir, ".github", "instructions", "my-instr.instructions.md")
	if _, err := os.Stat(dstPath); err != nil {
		t.Error("instruction file not created")
	}
}

func TestInstallInstruction_NotFound(t *testing.T) {
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, ".github", "instructions"), 0o755)
	dstDir := t.TempDir()
	result := &installResult{}

	err := installInstruction(srcDir, ScopeRepo(dstDir), "nonexistent", false, false, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", result.Skipped)
	}
}

func TestInstallPrompt_FlatFile(t *testing.T) {
	srcDir := t.TempDir()
	promptsDir := filepath.Join(srcDir, ".github", "prompts")
	os.MkdirAll(promptsDir, 0o755)
	os.WriteFile(filepath.Join(promptsDir, "my-prompt.prompt.md"), []byte("# Prompt"), 0o644)

	dstDir := t.TempDir()
	result := &installResult{}

	err := installPrompt(srcDir, ScopeRepo(dstDir), "my-prompt", false, false, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Installed != 1 {
		t.Errorf("installed = %d, want 1", result.Installed)
	}
}

func TestInstallPrompt_Directory(t *testing.T) {
	srcDir := t.TempDir()
	promptDir := filepath.Join(srcDir, ".github", "prompts", "my-prompt")
	os.MkdirAll(promptDir, 0o755)
	os.WriteFile(filepath.Join(promptDir, "prompt.md"), []byte("# Prompt"), 0o644)

	dstDir := t.TempDir()
	result := &installResult{}

	err := installPrompt(srcDir, ScopeRepo(dstDir), "my-prompt", false, false, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Installed != 1 {
		t.Errorf("installed = %d, want 1", result.Installed)
	}

	dstPath := filepath.Join(dstDir, ".github", "prompts", "my-prompt", "prompt.md")
	if _, err := os.Stat(dstPath); err != nil {
		t.Error("prompt directory file not copied")
	}
}

func TestInstallPrompt_NotFound(t *testing.T) {
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, ".github", "prompts"), 0o755)
	dstDir := t.TempDir()
	result := &installResult{}

	err := installPrompt(srcDir, ScopeRepo(dstDir), "nonexistent", false, false, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", result.Skipped)
	}
}

// ─── Uninstall tests ────────────────────────────────────────────────────────

func TestCmdUninstall(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(dir, ".github", "agents", "test.agent.md"), []byte("# Agent"), 0o644)

	state := &StateFile{
		Collection: "test",
		Version:    "1.0",
		Files: []InstalledFile{
			{Path: ".github/agents/test.agent.md", Hash: "abc123"},
		},
	}
	writeState(dir, state)

	err := cmdUninstall(ScopeRepo(dir), false)
	if err != nil {
		t.Fatal(err)
	}

	// File should be removed
	if _, err := os.Stat(filepath.Join(dir, ".github", "agents", "test.agent.md")); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}

	// State file should be removed
	if _, err := os.Stat(filepath.Join(dir, stateFilePath)); !os.IsNotExist(err) {
		t.Error("state file should have been removed")
	}
}

func TestCmdUninstall_NoState(t *testing.T) {
	dir := t.TempDir()
	err := cmdUninstall(ScopeRepo(dir), false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCmdUninstall_DryRun(t *testing.T) {
	dir := t.TempDir()
	agentPath := filepath.Join(dir, ".github", "agents", "test.agent.md")
	os.MkdirAll(filepath.Dir(agentPath), 0o755)
	os.WriteFile(agentPath, []byte("# Agent"), 0o644)

	state := &StateFile{
		Collection: "test",
		Version:    "1.0",
		Files: []InstalledFile{
			{Path: ".github/agents/test.agent.md", Hash: "abc123"},
		},
	}
	writeState(dir, state)

	err := cmdUninstall(ScopeRepo(dir), true)
	if err != nil {
		t.Fatal(err)
	}

	// File should still exist in dry-run
	if _, err := os.Stat(agentPath); err != nil {
		t.Error("file should still exist in dry-run mode")
	}
}

// ─── Status tests ───────────────────────────────────────────────────────────

func TestCmdStatus_NoState(t *testing.T) {
	dir := t.TempDir()
	err := cmdStatus(ScopeRepo(dir))
	if err != nil {
		t.Fatal(err)
	}
}

func TestCmdStatus_WithState(t *testing.T) {
	dir := t.TempDir()
	agentPath := filepath.Join(dir, ".github", "agents", "test.agent.md")
	os.MkdirAll(filepath.Dir(agentPath), 0o755)
	os.WriteFile(agentPath, []byte("# Agent"), 0o644)

	hash, _ := fileHash(agentPath)
	state := &StateFile{
		Collection:  "test",
		Version:     "1.0",
		SourceSHA:   "abc1234",
		InstalledAt: "2025-07-01T12:00:00Z",
		Files: []InstalledFile{
			{Path: ".github/agents/test.agent.md", Hash: hash},
		},
	}
	writeState(dir, state)

	err := cmdStatus(ScopeRepo(dir))
	if err != nil {
		t.Fatal(err)
	}
}

// ─── DirHash tests ──────────────────────────────────────────────────────────

func TestDirHash(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("world"), 0o644)

	h1, err := dirHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(h1) != 16 {
		t.Errorf("hash length = %d, want 16", len(h1))
	}

	// Same content = same hash
	dir2 := t.TempDir()
	os.MkdirAll(filepath.Join(dir2, "sub"), 0o755)
	os.WriteFile(filepath.Join(dir2, "a.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(dir2, "sub", "b.txt"), []byte("world"), 0o644)
	h2, _ := dirHash(dir2)
	if h1 != h2 {
		t.Error("identical directories should have identical hashes")
	}

	// Different content = different hash
	os.WriteFile(filepath.Join(dir2, "a.txt"), []byte("changed"), 0o644)
	h3, _ := dirHash(dir2)
	if h1 == h3 {
		t.Error("different directories should have different hashes")
	}
}

// ─── InstallItems integration test ──────────────────────────────────────────

func TestInstallItems(t *testing.T) {
	srcDir := t.TempDir()
	// Create agent
	os.MkdirAll(filepath.Join(srcDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(srcDir, ".github", "agents", "a.agent.md"), []byte("# A"), 0o644)
	// Create skill
	skillDir := filepath.Join(srcDir, ".github", "skills", "s")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# S"), 0o644)

	manifest := &Manifest{
		Name:   "test",
		Agents: []string{"a"},
		Skills: []string{"s"},
	}

	dstDir := t.TempDir()
	result, err := installItems(srcDir, ScopeRepo(dstDir), manifest, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Installed != 2 {
		t.Errorf("installed = %d, want 2", result.Installed)
	}
	if len(result.Files) != 2 {
		t.Errorf("files = %d, want 2", len(result.Files))
	}
}

// ─── Security tests (B1, B2) ───────────────────────────────────────────────

func TestValidateStatePath(t *testing.T) {
	repo := ScopeRepo("/tmp")
	tests := []struct {
		path    string
		wantErr bool
	}{
		{".github/agents/foo.agent.md", false},
		{".github/skills/bar/", false},
		{".github/instructions/baz.instructions.md", false},
		{".github/copilot-instructions.md", false},
		// Malicious paths
		{"../../etc/passwd", true},
		{"/etc/passwd", true},
		{"etc/passwd", true},
		{".github/../../../etc/passwd", true},
		{"agents/foo.agent.md", true},         // not under .github/
		{".githu/agents/foo.agent.md", true},  // typo, not .github/
		{"", true},                            // empty path
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			err := repo.ValidateStatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestReadState_MaliciousPath(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".github"), 0o755)

	// Write a state file with a malicious path
	state := `{
		"collection": "evil",
		"files": [{"path": "../../etc/passwd", "hash": "abc123"}]
	}`
	os.WriteFile(filepath.Join(dir, ".github", ".nav-pilot-state.json"), []byte(state), 0o644)

	_, err := readState(dir)
	if err == nil {
		t.Fatal("expected error for malicious state path")
	}
	if !strings.Contains(err.Error(), "unsafe state file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCopyFile_RefusesSymlink(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source.txt")
	os.WriteFile(srcFile, []byte("source content"), 0o644)

	dstDir := t.TempDir()
	outsideFile := filepath.Join(t.TempDir(), "outside.txt")
	os.WriteFile(outsideFile, []byte("should not change"), 0o644)

	// Create a symlink at the destination pointing outside
	symlink := filepath.Join(dstDir, "target.txt")
	os.Symlink(outsideFile, symlink)

	err := copyFile(srcFile, symlink)
	if err == nil {
		t.Fatal("expected error when destination is a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify the outside file was not modified
	got, _ := os.ReadFile(outsideFile)
	if string(got) != "should not change" {
		t.Error("symlink target was modified despite protection")
	}
}

func TestCopyFile_AtomicWrite(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source.txt")
	os.WriteFile(srcFile, []byte("new content"), 0o644)

	dstDir := t.TempDir()
	dstFile := filepath.Join(dstDir, "target.txt")
	os.WriteFile(dstFile, []byte("old content"), 0o644)

	err := copyFile(srcFile, dstFile)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(dstFile)
	if string(got) != "new content" {
		t.Errorf("expected 'new content', got %q", string(got))
	}
}

func TestCopyFile_RefusesSymlinkedParentDir(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source.txt")
	os.WriteFile(srcFile, []byte("payload"), 0o644)

	outsideDir := t.TempDir()
	os.WriteFile(filepath.Join(outsideDir, "target.txt"), []byte("original"), 0o644)

	// Create a symlinked parent directory
	repoDir := t.TempDir()
	symlinkedParent := filepath.Join(repoDir, "agents")
	os.Symlink(outsideDir, symlinkedParent)

	dstFile := filepath.Join(symlinkedParent, "target.txt")
	err := copyFile(srcFile, dstFile)
	if err == nil {
		t.Fatal("expected error when parent directory is a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify outside file was not modified
	got, _ := os.ReadFile(filepath.Join(outsideDir, "target.txt"))
	if string(got) != "original" {
		t.Error("file behind symlinked parent was modified")
	}
}

func TestWriteState_RefusesSymlink(t *testing.T) {
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "stolen.json")
	os.WriteFile(outsideFile, []byte("original"), 0o644)

	repoDir := t.TempDir()
	stateDir := filepath.Join(repoDir, ".github")
	os.MkdirAll(stateDir, 0o755)

	// Create symlink at state file location pointing outside
	os.Symlink(outsideFile, filepath.Join(stateDir, ".nav-pilot-state.json"))

	state := &StateFile{Collection: "evil", Files: []InstalledFile{
		{Path: ".github/agents/test.agent.md", Hash: "abc"},
	}}

	err := writeState(repoDir, state)
	if err == nil {
		t.Fatal("expected error when state file is a symlink")
	}

	got, _ := os.ReadFile(outsideFile)
	if string(got) != "original" {
		t.Error("symlink target was modified despite protection")
	}
}

func TestUpdateStateHashes_OnlyUpdatesApplied(t *testing.T) {
	dir := t.TempDir()

	// Create two files
	relA := filepath.Join(".github", "agents", "a.agent.md")
	relB := filepath.Join(".github", "agents", "b.agent.md")
	os.MkdirAll(filepath.Join(dir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(dir, relA), []byte("updated-a"), 0o644)
	os.WriteFile(filepath.Join(dir, relB), []byte("old-b"), 0o644)

	// State with old hashes for both
	state := &StateFile{
		Collection: "test",
		SourceSHA:  "old-sha",
		Files: []InstalledFile{
			{Path: relA, Hash: "oldhash-a"},
			{Path: relB, Hash: "oldhash-b"},
		},
	}
	writeState(dir, state)

	// Only update A (simulating partial apply where B failed)
	hashA, _ := fileHash(filepath.Join(dir, relA))
	appliedUpdates := []syncUpdate{
		{Path: relA, CurrentHash: "oldhash-a", SourceHash: hashA},
	}

	if err := updateStateHashes(dir, appliedUpdates); err != nil {
		t.Fatal(err)
	}

	got, _ := readState(dir)
	// A should have new hash
	if got.Files[0].Hash != hashA {
		t.Errorf("file A hash not updated: got %q, want %q", got.Files[0].Hash, hashA)
	}
	// B should keep old hash (was not in appliedUpdates)
	if got.Files[1].Hash != "oldhash-b" {
		t.Errorf("file B hash should be unchanged: got %q, want 'oldhash-b'", got.Files[1].Hash)
	}
}

func TestListAvailableItems_PromptDirectories(t *testing.T) {
	source := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// Create a prompt directory (not just flat file)
	promptDir := filepath.Join(ghDir, "prompts", "my-prompt")
	os.MkdirAll(promptDir, 0o755)
	os.WriteFile(filepath.Join(promptDir, "my-prompt.prompt.md"), []byte("# Prompt"), 0o644)

	// Also create a flat prompt file
	os.WriteFile(filepath.Join(ghDir, "prompts", "flat.prompt.md"), []byte("# Flat"), 0o644)

	// Capture stdout to verify both are listed
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := listAvailableItems(source)

	w.Close()
	os.Stdout = oldStdout
	captured, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("listAvailableItems: %v", err)
	}

	output := string(captured)
	if !strings.Contains(output, "my-prompt") {
		t.Error("prompt directory 'my-prompt' not listed")
	}
	if !strings.Contains(output, "flat") {
		t.Error("flat prompt 'flat' not listed")
	}
}
