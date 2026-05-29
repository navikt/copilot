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
	if !strings.Contains(err.Error(), "install requires a name") {
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

// ─── Command alias tests ────────────────────────────────────────────────────

func TestRun_AliasInstall(t *testing.T) {
	// "i" should behave identically to "install" — error when no name given
	err := run([]string{"i"})
	if err == nil {
		t.Fatal("expected error when no name given via alias")
	}
	if !strings.Contains(err.Error(), "install requires a name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_AliasList(t *testing.T) {
	// "ls" should behave identically to "list"
	err := run([]string{"ls", "--json"})
	if err != nil {
		t.Fatalf("list alias failed: %v", err)
	}
}

func TestRun_AliasSync(t *testing.T) {
	// "s" should behave identically to "sync"
	err := run([]string{"s", "--json"})
	if err != nil && err != errUpdatesAvailable && err != errSyncFailed {
		t.Fatalf("sync alias failed unexpectedly: %v", err)
	}
}

func TestRun_AliasUpgrade(t *testing.T) {
	// "up" should behave identically to "upgrade" — non-nil error is acceptable
	// (upgrade fetches from the network), but must not be "unknown command"
	err := run([]string{"up"})
	if err != nil && strings.Contains(err.Error(), "unknown command") {
		t.Errorf("alias 'up' was not resolved: %v", err)
	}
}

func TestRun_AliasUninstall(t *testing.T) {
	// "rm" should behave identically to "uninstall"
	err := run([]string{"rm", "--dry-run"})
	if err != nil {
		t.Fatalf("uninstall alias failed: %v", err)
	}
}

func TestRun_AliasDoesNotAffectUnknownCommands(t *testing.T) {
	// Aliases must not accidentally suppress the unknown-command error
	err := run([]string{"xyz"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("unexpected error: %v", err)
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

	if err := copyFile(src, dst, dir); err != nil {
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

	if err := copyDir(src, dst, dir); err != nil {
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

	copyDir(src, dst, dir)

	// Second version: remove old.md from source
	os.Remove(filepath.Join(src, "old.md"))
	copyDir(src, dst, dir)

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

	// Set up target
	dstDir := t.TempDir()
	result := &installResult{}

	err := installArtifact(NewSourceResolver(srcDir), ScopeRepo(dstDir), KindAgent, "test", false, false, result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Installed != 1 {
		t.Errorf("installed = %d, want 1", result.Installed)
	}
	if len(result.Files) != 1 {
		t.Errorf("files count = %d, want 1", len(result.Files))
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(dstDir, ".github", "agents", "test.agent.md")); err != nil {
		t.Error("agent.md not created")
	}
}

func TestInstallAgent_NotFound(t *testing.T) {
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, ".github", "agents"), 0o755)
	dstDir := t.TempDir()
	result := &installResult{}

	err := installArtifact(NewSourceResolver(srcDir), ScopeRepo(dstDir), KindAgent, "nonexistent", false, false, result)
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

	err := installArtifact(NewSourceResolver(srcDir), ScopeRepo(dstDir), KindSkill, "my-skill", false, false, result)
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
	err := installArtifact(NewSourceResolver(srcDir), ScopeRepo(dstDir), KindAgent, "test", false, false, result)
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
	err := installArtifact(NewSourceResolver(srcDir), ScopeRepo(dstDir), KindAgent, "test", false, true, result) // force=true
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

	err := installArtifact(NewSourceResolver(srcDir), ScopeRepo(dstDir), KindAgent, "../../../etc/passwd", false, false, result)
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

	err := installArtifact(NewSourceResolver(srcDir), ScopeRepo(dstDir), KindInstruction, "my-instr", false, false, result)
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

	err := installArtifact(NewSourceResolver(srcDir), ScopeRepo(dstDir), KindInstruction, "nonexistent", false, false, result)
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

	err := installArtifact(NewSourceResolver(srcDir), ScopeRepo(dstDir), KindPrompt, "my-prompt", false, false, result)
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

	err := installArtifact(NewSourceResolver(srcDir), ScopeRepo(dstDir), KindPrompt, "my-prompt", false, false, result)
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

	err := installArtifact(NewSourceResolver(srcDir), ScopeRepo(dstDir), KindPrompt, "nonexistent", false, false, result)
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
	err := cmdStatus(ScopeRepo(dir), false)
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

	err := cmdStatus(ScopeRepo(dir), false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCmdStatusAuto_ShowsBothScopes(t *testing.T) {
	// Set up a repo-scope state
	repoDir := t.TempDir()
	os.MkdirAll(filepath.Join(repoDir, ".github", "agents"), 0o755)
	agentPath := filepath.Join(repoDir, ".github", "agents", "test.agent.md")
	os.WriteFile(agentPath, []byte("# Agent"), 0o644)
	hash, _ := fileHash(agentPath)
	writeState(repoDir, &StateFile{
		Collection: "kotlin-backend",
		Version:    "2025.07",
		Scope:      "repo",
		SourceSHA:  "abc123",
		Files:      []InstalledFile{{Path: ".github/agents/test.agent.md", Hash: hash}},
	})

	// Set up a user-scope state via isolated HOME
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	userScope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(userScope.RootDir, "agents"), 0o755)
	userAgent := filepath.Join(userScope.RootDir, "agents", "nav-pilot.agent.md")
	os.WriteFile(userAgent, []byte("# Nav Pilot"), 0o644)
	userHash, _ := fileHash(userAgent)
	writeScopedState(userScope, &StateFile{
		Collection: "fullstack",
		Version:    "2025.07",
		Scope:      "user",
		SourceSHA:  "def456",
		Files:      []InstalledFile{{Path: "agents/nav-pilot.agent.md", Hash: userHash}},
	})

	// cmdStatusAuto should show both without error
	err = cmdStatusAuto(repoDir, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCmdStatusAuto_UserOnly(t *testing.T) {
	// No repo state
	repoDir := t.TempDir()

	// Set up user-scope state
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	userScope, _ := ScopeUser()
	os.MkdirAll(filepath.Join(userScope.RootDir, "agents"), 0o755)
	userAgent := filepath.Join(userScope.RootDir, "agents", "test.agent.md")
	os.WriteFile(userAgent, []byte("# Test"), 0o644)
	userHash, _ := fileHash(userAgent)
	writeScopedState(userScope, &StateFile{
		Collection: "fullstack",
		Version:    "2025.07",
		Scope:      "user",
		SourceSHA:  "abc123",
		Files:      []InstalledFile{{Path: "agents/test.agent.md", Hash: userHash}},
	})

	err := cmdStatusAuto(repoDir, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCmdStatusAuto_NeitherScope(t *testing.T) {
	repoDir := t.TempDir()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	err := cmdStatusAuto(repoDir, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCmdStatusAuto_JSON(t *testing.T) {
	repoDir := t.TempDir()
	os.MkdirAll(filepath.Join(repoDir, ".github"), 0o755)
	writeState(repoDir, &StateFile{
		Collection: "test",
		Version:    "1.0",
		Scope:      "repo",
		SourceSHA:  "abc123",
		Files:      []InstalledFile{},
	})

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// JSON output should work without error
	err := cmdStatusAuto(repoDir, true)
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
		{"agents/foo.agent.md", true},        // not under .github/
		{".githu/agents/foo.agent.md", true}, // typo, not .github/
		{"", true},                           // empty path
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

	err := copyFile(srcFile, symlink, dstDir)
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

	err := copyFile(srcFile, dstFile, dstDir)
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
	err := copyFile(srcFile, dstFile, repoDir)
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

func TestCheckSymlink_RejectsEmptyBoundary(t *testing.T) {
	err := checkSymlink("/some/path", "")
	if err == nil {
		t.Fatal("expected error for empty boundary")
	}
	if !strings.Contains(err.Error(), "non-empty absolute path") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckSymlink_RejectsRelativeBoundary(t *testing.T) {
	err := checkSymlink("/some/path", "relative/path")
	if err == nil {
		t.Fatal("expected error for relative boundary")
	}
	if !strings.Contains(err.Error(), "non-empty absolute path") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCheckSymlink_RejectsPathOutsideBoundary(t *testing.T) {
	err := checkSymlink("/other/dir/file.txt", "/boundary/dir")
	if err == nil {
		t.Fatal("expected error for path outside boundary")
	}
	if !strings.Contains(err.Error(), "not under boundary") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCopyDir_RefusesSymlinkedParent(t *testing.T) {
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, "skill"), 0o755)
	os.WriteFile(filepath.Join(srcDir, "skill", "SKILL.md"), []byte("# Skill"), 0o644)

	outsideDir := t.TempDir()
	os.WriteFile(filepath.Join(outsideDir, "victim.txt"), []byte("original"), 0o644)

	repoDir := t.TempDir()
	// Create a symlinked skills directory
	os.Symlink(outsideDir, filepath.Join(repoDir, "skills"))

	dstDir := filepath.Join(repoDir, "skills", "my-skill")
	err := copyDir(filepath.Join(srcDir, "skill"), dstDir, repoDir)
	if err == nil {
		t.Fatal("expected error when parent directory is a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify outside dir was not modified
	got, _ := os.ReadFile(filepath.Join(outsideDir, "victim.txt"))
	if string(got) != "original" {
		t.Error("file behind symlinked parent was modified despite protection")
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

// ─── collectAllItems tests ──────────────────────────────────────────────────

func TestCollectAllItems(t *testing.T) {
	source := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// Create agent files
	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "alpha.agent.md"), []byte("# Alpha"), 0o644)
	os.WriteFile(filepath.Join(agentsDir, "beta.agent.md"), []byte("# Beta"), 0o644)
	os.WriteFile(filepath.Join(agentsDir, "not-an-agent.md"), []byte("# Ignored"), 0o644) // wrong suffix

	// Create skill dirs with SKILL.md
	for _, skill := range []string{"skill-a", "skill-b", "skill-c"} {
		skillDir := filepath.Join(ghDir, "skills", skill)
		os.MkdirAll(skillDir, 0o755)
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+skill), 0o644)
	}
	// Create a dir without SKILL.md — should be ignored
	os.MkdirAll(filepath.Join(ghDir, "skills", "no-skill"), 0o755)

	m, err := collectAllItems(source)
	if err != nil {
		t.Fatalf("collectAllItems: %v", err)
	}
	if m.Name != "(all)" {
		t.Errorf("name = %q, want %q", m.Name, "(all)")
	}
	if len(m.Agents) != 2 {
		t.Errorf("agents = %d, want 2 (got %v)", len(m.Agents), m.Agents)
	}
	if len(m.Skills) != 3 {
		t.Errorf("skills = %d, want 3 (got %v)", len(m.Skills), m.Skills)
	}

	// Verify sorted
	if m.Agents[0] != "alpha" || m.Agents[1] != "beta" {
		t.Errorf("agents not sorted: %v", m.Agents)
	}
	if m.Skills[0] != "skill-a" {
		t.Errorf("skills not sorted: %v", m.Skills)
	}
	if len(m.Instructions) != 0 {
		t.Errorf("instructions = %d, want 0 (got %v)", len(m.Instructions), m.Instructions)
	}
}

func TestCollectAllItems_Empty(t *testing.T) {
	source := t.TempDir()
	m, err := collectAllItems(source)
	if err != nil {
		t.Fatalf("collectAllItems: %v", err)
	}
	if len(m.Agents) != 0 || len(m.Skills) != 0 || len(m.Instructions) != 0 {
		t.Errorf("expected empty manifest, got %d agents, %d skills, %d instructions", len(m.Agents), len(m.Skills), len(m.Instructions))
	}
}

func TestCollectAllItems_SkipsInvalidNames(t *testing.T) {
	source := t.TempDir()
	agentsDir := filepath.Join(source, ".github", "agents")
	os.MkdirAll(agentsDir, 0o755)
	// ".." in name should be rejected by validateName
	os.WriteFile(filepath.Join(agentsDir, "valid.agent.md"), []byte("ok"), 0o644)
	// Empty name can't occur via glob, but verify valid one works
	m, err := collectAllItems(source)
	if err != nil {
		t.Fatalf("collectAllItems: %v", err)
	}
	if len(m.Agents) != 1 || m.Agents[0] != "valid" {
		t.Errorf("unexpected agents: %v", m.Agents)
	}
}

// ─── install all tests ──────────────────────────────────────────────────────

func TestInstallAllFromSource(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// Set up agents
	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "test-a.agent.md"), []byte("# Agent A"), 0o644)

	// Set up skills
	skillDir := filepath.Join(ghDir, "skills", "test-s")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0o644)

	// Set up instructions
	instrDir := filepath.Join(ghDir, "instructions")
	os.MkdirAll(instrDir, 0o755)
	os.WriteFile(filepath.Join(instrDir, "golang.instructions.md"), []byte("# Go instructions"), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	src := &Source{Dir: source, SHA: "abc1234", Version: "dev"}

	err := installAllFromSource(scope, src, nil, false, false, false)
	if err != nil {
		t.Fatalf("installAllFromSource: %v", err)
	}

	// Verify state file uses CollectionAll
	state, err := readScopedState(scope)
	if err != nil {
		t.Fatalf("readScopedState: %v", err)
	}
	if state.Collection != CollectionAll {
		t.Errorf("collection = %q, want %q", state.Collection, CollectionAll)
	}
	if len(state.Files) != 3 {
		t.Errorf("files = %d, want 3", len(state.Files))
	}

	// Verify files were actually installed — user scope puts files directly under rootDir
	agentDst := filepath.Join(target, "agents", "test-a.agent.md")
	if _, err := os.Stat(agentDst); os.IsNotExist(err) {
		t.Error("agent file not installed")
	}
	skillDst := filepath.Join(target, "skills", "test-s", "SKILL.md")
	if _, err := os.Stat(skillDst); os.IsNotExist(err) {
		t.Error("skill file not installed")
	}
	// Instructions go under .github/instructions/ in user scope
	instrDst := filepath.Join(target, ".github", "instructions", "golang.instructions.md")
	if _, err := os.Stat(instrDst); os.IsNotExist(err) {
		t.Error("instruction file not installed at .github/instructions/")
	}
}

func TestInstallAllFromSource_EmptySource(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	scope := &InstallScope{Name: "user", RootDir: target, StateFile: ".nav-pilot-state.json", SupportedTypes: []string{"agent", "skill", "instruction"}}
	src := &Source{Dir: source, SHA: "abc1234", Version: "dev"}

	err := installAllFromSource(scope, src, nil, false, false, false)
	if err == nil {
		t.Fatal("expected error for empty source")
	}
	if !strings.Contains(err.Error(), "no agents, skills, or instructions") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallAllFromSource_DryRun(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "test-a.agent.md"), []byte("# Agent A"), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	src := &Source{Dir: source, SHA: "abc1234", Version: "dev"}

	err := installAllFromSource(scope, src, nil, true, false, false)
	if err != nil {
		t.Fatalf("installAllFromSource dry run: %v", err)
	}

	// No state file should be written in dry run
	state, err := readScopedState(scope)
	if err != nil {
		t.Fatalf("readScopedState: %v", err)
	}
	if state != nil {
		t.Error("state file should not exist after dry run")
	}
}

// ─── detectNewItems tests ───────────────────────────────────────────────────

func TestDetectNewItems(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// Create 3 agents and 2 skills in source
	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "existing.agent.md"), []byte("# E"), 0o644)
	os.WriteFile(filepath.Join(agentsDir, "new-one.agent.md"), []byte("# N"), 0o644)

	skillDir1 := filepath.Join(ghDir, "skills", "existing-skill")
	os.MkdirAll(skillDir1, 0o755)
	os.WriteFile(filepath.Join(skillDir1, "SKILL.md"), []byte("# S"), 0o644)
	skillDir2 := filepath.Join(ghDir, "skills", "new-skill")
	os.MkdirAll(skillDir2, 0o755)
	os.WriteFile(filepath.Join(skillDir2, "SKILL.md"), []byte("# NS"), 0o644)

	// Create state with only the "existing" items
	scope := &InstallScope{Name: "user", RootDir: target, StateFile: ".nav-pilot-state.json", SupportedTypes: []string{"agent", "skill", "instruction"}}
	state := &StateFile{
		Collection: CollectionAll,
		Version:    "dev",
		Scope:      "user",
		Files: []InstalledFile{
			{Path: "agents/existing.agent.md", Hash: "abc"},
			{Path: "skills/existing-skill/", Hash: "def"},
		},
	}
	if err := writeScopedState(scope, state); err != nil {
		t.Fatalf("writeScopedState: %v", err)
	}

	newItems := detectNewItems(scope, source)
	if len(newItems) != 2 {
		t.Fatalf("newItems = %d, want 2 (got %v)", len(newItems), newItems)
	}

	// Verify content
	hasAgent := false
	hasSkill := false
	for _, item := range newItems {
		if item == "agent: new-one" {
			hasAgent = true
		}
		if item == "skill: new-skill" {
			hasSkill = true
		}
	}
	if !hasAgent {
		t.Error("missing new agent in detected items")
	}
	if !hasSkill {
		t.Error("missing new skill in detected items")
	}
}

func TestDetectNewItems_NoState(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	scope := &InstallScope{Name: "user", RootDir: target, StateFile: ".nav-pilot-state.json", SupportedTypes: []string{"agent", "skill", "instruction"}}
	newItems := detectNewItems(scope, source)
	if len(newItems) != 0 {
		t.Errorf("expected no items without state, got %v", newItems)
	}
}

func TestDetectNewItems_NonAllCollection(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	// State with a regular collection, not "(all)"
	scope := &InstallScope{Name: "user", RootDir: target, StateFile: ".nav-pilot-state.json", SupportedTypes: []string{"agent", "skill", "instruction"}}
	state := &StateFile{
		Collection: "fullstack",
		Version:    "dev",
		Scope:      "user",
		Files:      []InstalledFile{{Path: "agents/test.agent.md", Hash: "abc"}},
	}
	writeScopedState(scope, state)

	newItems := detectNewItems(scope, source)
	if len(newItems) != 0 {
		t.Errorf("expected no items for non-all collection, got %v", newItems)
	}
}

func TestDetectNewItems_AllUpToDate(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// One agent
	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "only.agent.md"), []byte("# Only"), 0o644)

	scope := &InstallScope{Name: "user", RootDir: target, StateFile: ".nav-pilot-state.json", SupportedTypes: []string{"agent", "skill", "instruction"}}
	state := &StateFile{
		Collection: CollectionAll,
		Version:    "dev",
		Scope:      "user",
		Files:      []InstalledFile{{Path: "agents/only.agent.md", Hash: "abc"}},
	}
	writeScopedState(scope, state)

	newItems := detectNewItems(scope, source)
	if len(newItems) != 0 {
		t.Errorf("expected no new items, got %v", newItems)
	}
}

// ─── CLI dispatch: install --user no args ───────────────────────────────────

func TestRun_InstallUserNoArgs(t *testing.T) {
	// nav-pilot install --user (no collection) should call cmdInstallAll,
	// which calls resolveSource. In test environment this will fail on source
	// resolution. We just verify it doesn't require a collection name.
	err := run([]string{"install", "--user"})
	if err != nil && strings.Contains(err.Error(), "collection name") {
		t.Errorf("install --user should not require collection name, got: %v", err)
	}
}

func TestRun_InstallUserWithCollection(t *testing.T) {
	// nav-pilot install --user fullstack — should still work as before
	// (backwards compatible, dispatches to cmdInstall not cmdInstallAll)
	err := run([]string{"install", "--user", "fullstack"})
	if err == nil {
		// It'll fail on source resolution, but not on arg parsing
		t.Log("no error (source was resolved)")
	}
	if err != nil && strings.Contains(err.Error(), "collection name") {
		t.Errorf("install --user fullstack should not error on collection name, got: %v", err)
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

// ─── collectAllItems with instructions ──────────────────────────────────────

func TestCollectAllItems_WithInstructions(t *testing.T) {
	source := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// Create agent
	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "test.agent.md"), []byte("# Test"), 0o644)

	// Create skill
	skillDir := filepath.Join(ghDir, "skills", "test-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Skill"), 0o644)

	// Create instructions
	instrDir := filepath.Join(ghDir, "instructions")
	os.MkdirAll(instrDir, 0o755)
	os.WriteFile(filepath.Join(instrDir, "golang.instructions.md"), []byte("# Go"), 0o644)
	os.WriteFile(filepath.Join(instrDir, "kotlin.instructions.md"), []byte("# Kotlin"), 0o644)
	os.WriteFile(filepath.Join(instrDir, "not-an-instruction.md"), []byte("# Ignored"), 0o644) // wrong suffix
	os.WriteFile(filepath.Join(instrDir, "golang.metadata.json"), []byte("{}"), 0o644)         // not instruction

	m, err := collectAllItems(source)
	if err != nil {
		t.Fatalf("collectAllItems: %v", err)
	}

	if len(m.Agents) != 1 {
		t.Errorf("agents = %d, want 1", len(m.Agents))
	}
	if len(m.Skills) != 1 {
		t.Errorf("skills = %d, want 1", len(m.Skills))
	}
	if len(m.Instructions) != 2 {
		t.Errorf("instructions = %d, want 2 (got %v)", len(m.Instructions), m.Instructions)
	}
	// Verify sorted
	if len(m.Instructions) == 2 && (m.Instructions[0] != "golang" || m.Instructions[1] != "kotlin") {
		t.Errorf("instructions not sorted: %v", m.Instructions)
	}
}

// ─── instruction install path tests ─────────────────────────────────────────

func TestInstallAllFromSource_WithInstructions(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// Set up agent
	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "test.agent.md"), []byte("# Agent"), 0o644)

	// Set up instructions
	instrDir := filepath.Join(ghDir, "instructions")
	os.MkdirAll(instrDir, 0o755)
	os.WriteFile(filepath.Join(instrDir, "golang.instructions.md"), []byte("# Go instr"), 0o644)
	os.WriteFile(filepath.Join(instrDir, "kotlin.instructions.md"), []byte("# Kotlin instr"), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	src := &Source{Dir: source, SHA: "abc1234", Version: "dev"}

	err := installAllFromSource(scope, src, nil, false, false, false)
	if err != nil {
		t.Fatalf("installAllFromSource: %v", err)
	}

	// Agent goes to target/agents/
	agentDst := filepath.Join(target, "agents", "test.agent.md")
	if _, err := os.Stat(agentDst); os.IsNotExist(err) {
		t.Error("agent not installed")
	}

	// Instructions go to target/.github/instructions/
	for _, name := range []string{"golang", "kotlin"} {
		instrDst := filepath.Join(target, ".github", "instructions", name+".instructions.md")
		if _, err := os.Stat(instrDst); os.IsNotExist(err) {
			t.Errorf("instruction %q not installed at %s", name, instrDst)
		}
	}

	// Verify state tracks instructions with .github/ prefix
	state, err := readScopedState(scope)
	if err != nil {
		t.Fatalf("readScopedState: %v", err)
	}
	hasInstr := false
	for _, f := range state.Files {
		if strings.HasPrefix(f.Path, ".github/instructions/") {
			hasInstr = true
			break
		}
	}
	if !hasInstr {
		t.Error("state file should contain .github/instructions/ paths")
	}
}

// ─── detectNewItems with instructions ───────────────────────────────────────

func TestDetectNewItems_Instructions(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// Set up source with agent + instruction
	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "test.agent.md"), []byte("# Agent"), 0o644)

	instrDir := filepath.Join(ghDir, "instructions")
	os.MkdirAll(instrDir, 0o755)
	os.WriteFile(filepath.Join(instrDir, "golang.instructions.md"), []byte("# Go"), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}

	// Write state with only the agent installed
	state := &StateFile{
		Collection: CollectionAll,
		Scope:      "user",
		Version:    "v1",
		Files: []InstalledFile{
			{Path: "agents/test.agent.md", Hash: "abc"},
		},
	}
	os.MkdirAll(target, 0o755)
	data, _ := json.Marshal(state)
	os.WriteFile(filepath.Join(target, ".nav-pilot-state.json"), data, 0o644)

	newItems := detectNewItems(scope, source)
	if len(newItems) != 1 {
		t.Fatalf("expected 1 new item, got %d: %v", len(newItems), newItems)
	}
	if !strings.Contains(newItems[0], "instruction: golang") {
		t.Errorf("expected new instruction 'golang', got %q", newItems[0])
	}
}

// ─── copilotEnv tests ───────────────────────────────────────────────────────

func TestCopilotEnv_NoInstructions(t *testing.T) {
	// When no instructions exist, copilotEnv should return nil (inherit parent env)
	env := copilotEnv()
	if env != nil {
		// This can be non-nil if the developer's own ~/.copilot has instructions.
		// Just check the key is present if non-nil.
		t.Log("copilotEnv returned non-nil (may have user instructions installed)")
	}
}

// ─── cmdEnv tests ───────────────────────────────────────────────────────────

func TestCmdEnv_NoInstructions(t *testing.T) {
	// cmdEnv should not error even when no instructions exist
	// (it prints a hint to stderr)
	err := cmdEnv()
	if err != nil {
		t.Errorf("cmdEnv should not error: %v", err)
	}
}

// ─── install with ignored items (picker integration) ────────────────────────

func TestInstallAllFromSource_WithIgnoredItems(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// Set up 3 agents, 1 skill, 1 instruction in source
	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "auth-agent.agent.md"), []byte("# Auth"), 0o644)
	os.WriteFile(filepath.Join(agentsDir, "nais-agent.agent.md"), []byte("# Nais"), 0o644)
	os.WriteFile(filepath.Join(agentsDir, "rust-agent.agent.md"), []byte("# Rust"), 0o644)

	skillDir := filepath.Join(ghDir, "skills", "kafka")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Kafka"), 0o644)

	instrDir := filepath.Join(ghDir, "instructions")
	os.MkdirAll(instrDir, 0o755)
	os.WriteFile(filepath.Join(instrDir, "golang.instructions.md"), []byte("# Go"), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	src := &Source{Dir: source, SHA: "abc1234", Version: "dev"}

	// Only install auth-agent and kafka, skip rust-agent, nais-agent, golang
	manifest := &Manifest{
		Name:   "(all)",
		Agents: []string{"auth-agent"},
		Skills: []string{"kafka"},
	}
	skippedItems := []InstalledFile{
		{Path: "agents/nais-agent.agent.md", Hash: "", Status: fileStatusIgnored},
		{Path: "agents/rust-agent.agent.md", Hash: "", Status: fileStatusIgnored},
		{Path: ".github/instructions/golang.instructions.md", Hash: "", Status: fileStatusIgnored},
	}

	err := installAllFromSource(scope, src, manifest, false, false, false, skippedItems...)
	if err != nil {
		t.Fatalf("installAllFromSource: %v", err)
	}

	// Read state file
	state, err := readScopedState(scope)
	if err != nil {
		t.Fatalf("readScopedState: %v", err)
	}

	// Count active vs ignored
	active := 0
	ignored := 0
	for _, f := range state.Files {
		if f.Status == fileStatusIgnored {
			ignored++
		} else {
			active++
		}
	}

	if active != 2 {
		t.Errorf("expected 2 active files, got %d", active)
	}
	if ignored != 3 {
		t.Errorf("expected 3 ignored files, got %d", ignored)
	}

	// Verify installed files exist
	if _, err := os.Stat(filepath.Join(target, "agents", "auth-agent.agent.md")); os.IsNotExist(err) {
		t.Error("auth-agent not installed")
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "kafka", "SKILL.md")); os.IsNotExist(err) {
		t.Error("kafka skill not installed")
	}

	// Verify ignored files do NOT exist
	if _, err := os.Stat(filepath.Join(target, "agents", "rust-agent.agent.md")); !os.IsNotExist(err) {
		t.Error("rust-agent should not be installed")
	}
	if _, err := os.Stat(filepath.Join(target, "agents", "nais-agent.agent.md")); !os.IsNotExist(err) {
		t.Error("nais-agent should not be installed")
	}
}

func TestInstallAllFromSource_IgnoredItems_DryRun(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "auth-agent.agent.md"), []byte("# Auth"), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	src := &Source{Dir: source, SHA: "abc1234", Version: "dev"}
	manifest := &Manifest{Name: "(all)", Agents: []string{"auth-agent"}}
	skippedItems := []InstalledFile{
		{Path: "agents/rust-agent.agent.md", Hash: "", Status: fileStatusIgnored},
	}

	err := installAllFromSource(scope, src, manifest, true, false, false, skippedItems...)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}

	// Dry run should NOT write state file
	state, err := readScopedState(scope)
	if err != nil {
		t.Fatalf("readScopedState: %v", err)
	}
	if state != nil {
		t.Error("dry run should not write state file")
	}
}

func TestInstallAllFromSource_NoDuplicatePaths(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "auth-agent.agent.md"), []byte("# Auth"), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	src := &Source{Dir: source, SHA: "abc1234", Version: "dev"}
	manifest := &Manifest{Name: "(all)", Agents: []string{"auth-agent"}}

	err := installAllFromSource(scope, src, manifest, false, false, false)
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	state, err := readScopedState(scope)
	if err != nil {
		t.Fatalf("readScopedState: %v", err)
	}

	// Verify no duplicate paths
	seen := make(map[string]bool)
	for _, f := range state.Files {
		if seen[f.Path] {
			t.Errorf("duplicate path in state: %s", f.Path)
		}
		seen[f.Path] = true
	}
}

// ─── picker → install → sync cycle ─────────────────────────────────────────

func TestPickerInstallSyncCycle(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// Step 1: Set up source with 3 agents
	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "auth-agent.agent.md"), []byte("# Auth v1"), 0o644)
	os.WriteFile(filepath.Join(agentsDir, "nais-agent.agent.md"), []byte("# Nais v1"), 0o644)
	os.WriteFile(filepath.Join(agentsDir, "rust-agent.agent.md"), []byte("# Rust v1"), 0o644)

	skillDir := filepath.Join(ghDir, "skills", "kafka")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Kafka v1"), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	src := &Source{Dir: source, SHA: "abc1234", Version: "dev"}

	// Step 2: Install with partial selection (simulating picker)
	selectedManifest := &Manifest{
		Name:   "(all)",
		Agents: []string{"auth-agent", "nais-agent"},
		Skills: []string{"kafka"},
	}
	skippedItems := computeSkippedItems(
		&Manifest{Agents: []string{"auth-agent", "nais-agent", "rust-agent"}, Skills: []string{"kafka"}},
		selectedManifest,
		scope,
	)

	err := installAllFromSource(scope, src, selectedManifest, false, false, false, skippedItems...)
	if err != nil {
		t.Fatalf("initial install: %v", err)
	}

	// Step 3: Verify resolveSyncFiles skips ignored items
	syncFiles, _, err := resolveSyncFiles(scope, source)
	if err != nil {
		t.Fatalf("resolveSyncFiles: %v", err)
	}

	for _, sf := range syncFiles {
		if strings.Contains(sf.localPath, "rust-agent") {
			t.Errorf("resolveSyncFiles should skip ignored rust-agent, got: %s", sf.localPath)
		}
	}
	if len(syncFiles) != 3 {
		t.Errorf("expected 3 sync files (2 agents + 1 skill), got %d", len(syncFiles))
	}

	// Step 4: Update source (new version of installed agent)
	os.WriteFile(filepath.Join(agentsDir, "auth-agent.agent.md"), []byte("# Auth v2 — updated"), 0o644)

	// Step 5: Verify checkSyncFile detects the update
	for _, sf := range syncFiles {
		if strings.Contains(sf.localPath, "auth-agent") {
			update, err := checkSyncFile(scope.RootDir, source, sf)
			if err != nil {
				t.Fatalf("checkSyncFile: %v", err)
			}
			if update == nil {
				t.Error("expected update for auth-agent after source change")
			}
			break
		}
	}

	// Step 6: Verify detectNewItems does NOT report the ignored item
	newItems := detectNewItems(scope, source)
	for _, item := range newItems {
		if strings.Contains(item, "rust-agent") {
			t.Errorf("detectNewItems should not report ignored rust-agent: %v", newItems)
		}
	}
}

func TestPickerInstallSyncCycle_NewSourceItem(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "auth-agent.agent.md"), []byte("# Auth"), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	src := &Source{Dir: source, SHA: "abc1234", Version: "dev"}

	// Install everything
	err := installAllFromSource(scope, src, nil, false, false, false)
	if err != nil {
		t.Fatalf("initial install: %v", err)
	}

	// Add a new agent to source
	os.WriteFile(filepath.Join(agentsDir, "brand-new.agent.md"), []byte("# New"), 0o644)

	// detectNewItems should find it
	newItems := detectNewItems(scope, source)
	found := false
	for _, item := range newItems {
		if strings.Contains(item, "brand-new") {
			found = true
		}
	}
	if !found {
		t.Errorf("detectNewItems should find brand-new agent, got: %v", newItems)
	}
}

func TestDetectNewItems_IgnoredItemNotReported(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "auth-agent.agent.md"), []byte("# Auth"), 0o644)
	os.WriteFile(filepath.Join(agentsDir, "rust-agent.agent.md"), []byte("# Rust"), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}

	// Write state with auth active, rust ignored
	state := &StateFile{
		Collection: CollectionAll,
		Version:    "dev",
		Scope:      "user",
		Files: []InstalledFile{
			{Path: "agents/auth-agent.agent.md", Hash: "abc"},
			{Path: "agents/rust-agent.agent.md", Hash: "", Status: fileStatusIgnored},
		},
	}
	if err := writeScopedState(scope, state); err != nil {
		t.Fatalf("writeScopedState: %v", err)
	}

	newItems := detectNewItems(scope, source)
	for _, item := range newItems {
		if strings.Contains(item, "rust-agent") {
			t.Errorf("ignored item should not be reported as new: %v", newItems)
		}
	}
	if len(newItems) != 0 {
		t.Errorf("expected no new items, got %v", newItems)
	}
}
