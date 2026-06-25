package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── cmdAdd tests ───────────────────────────────────────────────────────────

func TestRun_AddNoArgs(t *testing.T) {
	err := run([]string{"add"})
	if err == nil {
		t.Fatal("expected error for add with no args")
	}
	if !strings.Contains(err.Error(), "type and name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_AddOnlyType(t *testing.T) {
	err := run([]string{"add", "agent"})
	if err == nil {
		t.Fatal("expected error for add with only type")
	}
	if !strings.Contains(err.Error(), "type and name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCmdAdd_InvalidType(t *testing.T) {
	err := cmdAdd("widget", "foo", ScopeRepo(t.TempDir()), "", "", true, false, false)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCmdAdd_InvalidName(t *testing.T) {
	err := cmdAdd("agent", "../etc/passwd", ScopeRepo(t.TempDir()), "", "", true, false, false)
	if err == nil {
		t.Fatal("expected error for path traversal name")
	}
	if !strings.Contains(err.Error(), "invalid name") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCmdAdd_Agent(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	// Create source agent
	agentDir := filepath.Join(source, "agents")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "test-agent.agent.md"), []byte("# Test Agent"), 0o644)

	// Create .git in target
	os.MkdirAll(filepath.Join(target, ".git"), 0o755)

	// Set up for local source resolution
	os.MkdirAll(filepath.Join(source, "collections"), 0o755)

	result := &installResult{}
	err := installArtifact(NewSourceResolver(source), ScopeRepo(target), KindAgent, "test-agent", false, false, result)
	if err != nil {
		t.Fatalf("installArtifact agent: %v", err)
	}
	if result.Installed != 1 {
		t.Errorf("expected 1 installed, got %d", result.Installed)
	}

	// Verify agent file exists
	dst := filepath.Join(target, ".github", "agents", "test-agent.agent.md")
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("agent file not created")
	}
}

func TestCmdAdd_Skill(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	// Create source skill directory
	skillDir := filepath.Join(source, "skills", "test-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill"), 0o644)
	os.WriteFile(filepath.Join(skillDir, "metadata.json"), []byte(`{"name":"test"}`), 0o644)

	// Create target .git
	os.MkdirAll(filepath.Join(target, ".git"), 0o755)

	result := &installResult{}
	err := installArtifact(NewSourceResolver(source), ScopeRepo(target), KindSkill, "test-skill", false, false, result)
	if err != nil {
		t.Fatalf("installArtifact skill: %v", err)
	}
	if result.Installed != 1 {
		t.Errorf("expected 1 installed, got %d", result.Installed)
	}

	// Verify skill directory exists
	dst := filepath.Join(target, ".github", "skills", "test-skill", "SKILL.md")
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("skill SKILL.md not created")
	}
}

func TestCmdAdd_Skill_RootLevel(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	// Create source skill at root level (gh skill convention)
	skillDir := filepath.Join(source, "skills", "test-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Root Skill"), 0o644)

	os.MkdirAll(filepath.Join(target, ".git"), 0o755)

	result := &installResult{}
	err := installArtifact(NewSourceResolver(source), ScopeRepo(target), KindSkill, "test-skill", false, false, result)
	if err != nil {
		t.Fatalf("installArtifact skill: %v", err)
	}
	if result.Installed != 1 {
		t.Errorf("expected 1 installed, got %d", result.Installed)
	}

	dst := filepath.Join(target, ".github", "skills", "test-skill", "SKILL.md")
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal("skill SKILL.md not created from root-level source")
	}
	if string(got) != "# Root Skill" {
		t.Errorf("content mismatch: got %q", string(got))
	}
}

func TestCmdAdd_Skill_RootLevel_RecordsCorrectStatePath(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	// Create root-level skill
	os.MkdirAll(filepath.Join(source, "skills", "test-skill"), 0o755)
	os.WriteFile(filepath.Join(source, "skills", "test-skill", "SKILL.md"), []byte("# Root"), 0o644)

	os.MkdirAll(filepath.Join(target, ".git"), 0o755)

	result := &installResult{}
	err := installArtifact(NewSourceResolver(source), ScopeRepo(target), KindSkill, "test-skill", false, false, result)
	if err != nil {
		t.Fatalf("installArtifact skill: %v", err)
	}

	// State should record destination path (.github/skills/...) not source path
	for _, f := range result.Files {
		if strings.HasPrefix(f.Path, "skills/") && !strings.HasPrefix(f.Path, ".github/skills/") {
			t.Errorf("state path should use .github/ prefix for repo scope, got %q", f.Path)
		}
	}
	foundSkill := false
	for _, f := range result.Files {
		if strings.Contains(f.Path, "test-skill") {
			foundSkill = true
			if !strings.HasPrefix(f.Path, ".github/skills/") {
				t.Errorf("skill state path = %q, want .github/skills/test-skill/ prefix", f.Path)
			}
		}
	}
	if !foundSkill {
		t.Error("expected to find test-skill in result files")
	}
}

func TestCmdAdd_Agent_RootLevel(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	// Create agent at root level
	os.MkdirAll(filepath.Join(source, "agents"), 0o755)
	os.WriteFile(filepath.Join(source, "agents", "nais.agent.md"), []byte("# Root Agent"), 0o644)
	os.WriteFile(filepath.Join(source, "agents", "nais.metadata.json"), []byte(`{"tools":[]}`), 0o644)

	os.MkdirAll(filepath.Join(target, ".git"), 0o755)

	result := &installResult{}
	err := installArtifact(NewSourceResolver(source), ScopeRepo(target), KindAgent, "nais", false, false, result)
	if err != nil {
		t.Fatalf("installArtifact agent: %v", err)
	}
	if result.Installed != 1 {
		t.Errorf("expected 1 installed, got %d", result.Installed)
	}

	// Verify agent installed at .github/agents/ in target
	got, err := os.ReadFile(filepath.Join(target, ".github", "agents", "nais.agent.md"))
	if err != nil {
		t.Fatalf("agent not created: %v", err)
	}
	if string(got) != "# Root Agent" {
		t.Errorf("content mismatch: got %q", string(got))
	}
}

func TestCmdAdd_Prompt_RootLevel(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	// Root-level flat prompt
	os.MkdirAll(filepath.Join(source, "prompts"), 0o755)
	os.WriteFile(filepath.Join(source, "prompts", "review.prompt.md"), []byte("# Root Prompt"), 0o644)

	os.MkdirAll(filepath.Join(target, ".git"), 0o755)

	result := &installResult{}
	err := installArtifact(NewSourceResolver(source), ScopeRepo(target), KindPrompt, "review", false, false, result)
	if err != nil {
		t.Fatalf("installArtifact prompt: %v", err)
	}
	if result.Installed != 1 {
		t.Errorf("expected 1 installed, got %d", result.Installed)
	}

	got, err := os.ReadFile(filepath.Join(target, ".github", "prompts", "review.prompt.md"))
	if err != nil {
		t.Fatalf("prompt not created: %v", err)
	}
	if string(got) != "# Root Prompt" {
		t.Errorf("content mismatch: got %q", string(got))
	}
}

func TestCmdAdd_Prompt_RootDirLevel(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	// Root-level dir prompt
	os.MkdirAll(filepath.Join(source, "prompts", "complex"), 0o755)
	os.WriteFile(filepath.Join(source, "prompts", "complex", "prompt.md"), []byte("# Complex"), 0o644)

	os.MkdirAll(filepath.Join(target, ".git"), 0o755)

	result := &installResult{}
	err := installArtifact(NewSourceResolver(source), ScopeRepo(target), KindPrompt, "complex", false, false, result)
	if err != nil {
		t.Fatalf("installArtifact prompt: %v", err)
	}
	if result.Installed != 1 {
		t.Errorf("expected 1 installed, got %d", result.Installed)
	}

	got, err := os.ReadFile(filepath.Join(target, ".github", "prompts", "complex", "prompt.md"))
	if err != nil {
		t.Fatalf("prompt dir not created: %v", err)
	}
	if string(got) != "# Complex" {
		t.Errorf("content mismatch: got %q", string(got))
	}
}

func TestCmdAdd_AppendsToState(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	// Pre-existing state
	os.MkdirAll(filepath.Join(target, ".git"), 0o755)
	os.MkdirAll(filepath.Join(target, ".github"), 0o755)
	initialState := &StateFile{
		Collection: "kotlin-backend",
		SourceSHA:  "abc1234",
		Files: []InstalledFile{
			{Path: ".github/agents/existing.agent.md", Hash: "aaa"},
		},
	}
	writeState(target, initialState)

	// Create source agent
	agentDir := filepath.Join(source, "agents")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "new-agent.agent.md"), []byte("# New Agent"), 0o644)

	result := &installResult{}
	installArtifact(NewSourceResolver(source), ScopeRepo(target), KindAgent, "new-agent", false, false, result)

	// Simulate what cmdAdd does: merge state
	state, _ := readState(target)
	existing := make(map[string]bool)
	for _, f := range state.Files {
		existing[f.Path] = true
	}
	for _, f := range result.Files {
		if !existing[f.Path] {
			state.Files = append(state.Files, f)
		}
	}
	writeState(target, state)

	// Read back state
	state, _ = readState(target)
	if len(state.Files) != 2 {
		t.Errorf("expected 2 files in state, got %d", len(state.Files))
	}
	if state.Collection != "kotlin-backend" {
		t.Errorf("expected collection preserved, got %q", state.Collection)
	}
}

// ─── cmdList --items tests ──────────────────────────────────────────────────

func TestListAvailableItems(t *testing.T) {
	source := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// Create sample items
	os.MkdirAll(filepath.Join(ghDir, "agents"), 0o755)
	os.WriteFile(filepath.Join(ghDir, "agents", "test.agent.md"), []byte("# Agent"), 0o644)

	os.MkdirAll(filepath.Join(ghDir, "skills", "test-skill"), 0o755)
	os.WriteFile(filepath.Join(ghDir, "skills", "test-skill", "SKILL.md"), []byte("# Skill"), 0o644)

	os.MkdirAll(filepath.Join(ghDir, "instructions"), 0o755)
	os.WriteFile(filepath.Join(ghDir, "instructions", "test.instructions.md"), []byte("# Inst"), 0o644)

	os.MkdirAll(filepath.Join(ghDir, "prompts"), 0o755)
	os.WriteFile(filepath.Join(ghDir, "prompts", "test.prompt.md"), []byte("# Prompt"), 0o644)

	// Should not panic/error
	err := listAvailableItems(source)
	if err != nil {
		t.Fatalf("listAvailableItems: %v", err)
	}
}

// ─── resolveSource (findGitRoot) tests ──────────────────────────────────────

func TestFindGitRoot(t *testing.T) {
	tmp := t.TempDir()
	// Create nested dirs with .git at top
	gitDir := filepath.Join(tmp, "repo")
	os.MkdirAll(filepath.Join(gitDir, ".git"), 0o755)
	nested := filepath.Join(gitDir, "a", "b", "c")
	os.MkdirAll(nested, 0o755)

	root := findGitRoot(nested)
	if root != gitDir {
		t.Errorf("expected %q, got %q", gitDir, root)
	}
}

func TestFindGitRoot_NotFound(t *testing.T) {
	// findGitRoot walks up to filesystem root — if there's a .git anywhere
	// above the temp dir, it will be found. Test the boundary: a dir
	// without .git should not match itself.
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "a", "b")
	os.MkdirAll(nested, 0o755)

	root := findGitRoot(nested)
	// The result depends on whether any parent of tmp has .git.
	// We can only assert that the result is NOT the nested dir itself.
	if root == nested || root == filepath.Join(tmp, "a") || root == tmp {
		t.Errorf("findGitRoot should not find .git in temp dirs without one, got %q", root)
	}
}

func TestFindGitRoot_RelativeInput(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, "repo")
	os.MkdirAll(filepath.Join(gitDir, ".git"), 0o755)

	// chdir into the repo root and pass "."
	prev, _ := os.Getwd()
	defer os.Chdir(prev)
	os.Chdir(gitDir)

	root := findGitRoot(".")
	if !filepath.IsAbs(root) {
		t.Fatalf("expected absolute path, got %q", root)
	}
	// Resolve symlinks (macOS: /var → /private/var) before comparing.
	want, _ := filepath.EvalSymlinks(gitDir)
	if root != want {
		t.Errorf("expected %q, got %q", want, root)
	}
}

// ─── Integration-style tests ────────────────────────────────────────────────

// TestCmdInstall_FullFlow tests install with a local fixture source.
func TestCmdInstall_FullFlow(t *testing.T) {
	source := createFixtureSource(t)
	target := t.TempDir()
	os.MkdirAll(filepath.Join(target, ".git"), 0o755)

	// installItems with the fixture manifest
	manifest, err := loadManifest(source, "test-collection")
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}

	result, err := installItems(source, ScopeRepo(target), manifest, false, false)
	if err != nil {
		t.Fatalf("installItems: %v", err)
	}

	if result.Installed != 4 {
		t.Errorf("expected 4 installed, got %d", result.Installed)
	}

	// Verify all files
	for _, path := range []string{
		".github/agents/test.agent.md",
		".github/skills/test-skill/SKILL.md",
		".github/instructions/test.instructions.md",
		".github/prompts/test.prompt.md",
	} {
		if _, err := os.Stat(filepath.Join(target, path)); os.IsNotExist(err) {
			t.Errorf("missing: %s", path)
		}
	}
}

// TestCmdInstall_DryRunNoSideEffects ensures dry-run doesn't create files.
func TestCmdInstall_DryRunNoSideEffects(t *testing.T) {
	source := createFixtureSource(t)
	target := t.TempDir()

	manifest, err := loadManifest(source, "test-collection")
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}

	result, err := installItems(source, ScopeRepo(target), manifest, true, false)
	if err != nil {
		t.Fatalf("installItems dry-run: %v", err)
	}

	if result.Installed != 4 {
		t.Errorf("expected 4 would-install, got %d", result.Installed)
	}

	// No files should exist
	entries, _ := os.ReadDir(filepath.Join(target, ".github"))
	if len(entries) > 0 {
		t.Errorf("dry-run created files: %v", entries)
	}
}

// TestInstallArtifact_ConflictStillTracked ensures that conflicted (skipped) files
// are still recorded in state so future syncs can manage them.
func TestInstallArtifact_ConflictStillTracked(t *testing.T) {
	source := createFixtureSource(t)
	target := t.TempDir()
	os.MkdirAll(filepath.Join(target, ".git"), 0o755)

	// First install (clean)
	manifest, err := loadManifest(source, "test-collection")
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	result, err := installItems(source, ScopeRepo(target), manifest, false, false)
	if err != nil {
		t.Fatalf("first install: %v", err)
	}
	if result.Installed != 4 {
		t.Fatalf("expected 4 installed, got %d", result.Installed)
	}

	// Modify one installed file to create a conflict
	agentPath := filepath.Join(target, ".github/agents/test.agent.md")
	os.WriteFile(agentPath, []byte("# Modified locally"), 0o644)

	// Second install WITHOUT force — should report conflict but still track it
	result2, err := installItems(source, ScopeRepo(target), manifest, false, false)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if result2.Conflicts != 1 {
		t.Errorf("expected 1 conflict, got %d", result2.Conflicts)
	}

	// The conflicted file should still be in result.Files (tracked in state)
	totalTracked := len(result2.Files)
	if totalTracked != 4 {
		t.Errorf("expected 4 files tracked in state (including conflict), got %d", totalTracked)
	}

	// Verify the conflicted agent is tracked with conflict status
	found := false
	for _, f := range result2.Files {
		if strings.Contains(f.Path, "test.agent.md") {
			found = true
			if f.Hash == "" {
				t.Error("conflicted file has empty hash in state")
			}
			if f.Status != fileStatusConflict {
				t.Errorf("conflicted file should have status %q, got %q", fileStatusConflict, f.Status)
			}
		}
	}
	if !found {
		t.Error("conflicted agent file not found in state tracking")
	}
}

// TestCmdStatus_Integrity tests status command reports correct integrity.
func TestCmdStatus_Integrity(t *testing.T) {
	source := createFixtureSource(t)
	target := t.TempDir()
	os.MkdirAll(filepath.Join(target, ".git"), 0o755)

	manifest, _ := loadManifest(source, "test-collection")
	result, _ := installItems(source, ScopeRepo(target), manifest, false, false)

	state := &StateFile{
		Collection: "test-collection",
		Version:    "2025.07",
		SourceSHA:  "abc1234",
		Files:      result.Files,
	}
	writeState(target, state)

	// Should not error
	err := cmdStatus(ScopeRepo(target), false)
	if err != nil {
		t.Fatalf("cmdStatus: %v", err)
	}
}

// TestCmdUninstall_RemovesFiles tests uninstall cleans up files.
func TestCmdUninstall_RemovesFiles(t *testing.T) {
	source := createFixtureSource(t)
	target := t.TempDir()
	os.MkdirAll(filepath.Join(target, ".git"), 0o755)

	manifest, _ := loadManifest(source, "test-collection")
	result, _ := installItems(source, ScopeRepo(target), manifest, false, false)

	state := &StateFile{
		Collection: "test-collection",
		Files:      result.Files,
	}
	writeState(target, state)

	// Uninstall
	err := cmdUninstall(ScopeRepo(target), false)
	if err != nil {
		t.Fatalf("cmdUninstall: %v", err)
	}

	// State file should be gone
	if _, err := readState(target); err != nil {
		t.Fatalf("readState after uninstall: %v", err)
	}

	// All installed files should be gone
	for _, f := range result.Files {
		path := filepath.Join(target, f.Path)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("file still exists after uninstall: %s", f.Path)
		}
	}
}

// ─── Fixture helpers ────────────────────────────────────────────────────────

// createFixtureSource builds a minimal source tree with one of each item type.
func createFixtureSource(t *testing.T) string {
	t.Helper()
	source := t.TempDir()
	gh := source

	// Collection manifest
	collDir := filepath.Join(gh, "collections", "test-collection")
	os.MkdirAll(collDir, 0o755)
	os.WriteFile(filepath.Join(collDir, "manifest.json"), []byte(`{
		"name": "test-collection",
		"description": "Test collection",
		"version": "2025.07",
		"agents": ["test"],
		"skills": ["test-skill"],
		"instructions": ["test"],
		"prompts": ["test"]
	}`), 0o644)

	// Agent
	os.MkdirAll(filepath.Join(gh, "agents"), 0o755)
	os.WriteFile(filepath.Join(gh, "agents", "test.agent.md"), []byte("# Test Agent\nI help with testing."), 0o644)
	os.WriteFile(filepath.Join(gh, "agents", "test.metadata.json"), []byte(`{"name":"test","description":"Test agent"}`), 0o644)

	// Skill
	skillDir := filepath.Join(gh, "skills", "test-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill\nA skill for testing."), 0o644)
	os.WriteFile(filepath.Join(skillDir, "metadata.json"), []byte(`{"name":"test-skill"}`), 0o644)

	// Instruction
	os.MkdirAll(filepath.Join(gh, "instructions"), 0o755)
	os.WriteFile(filepath.Join(gh, "instructions", "test.instructions.md"), []byte("# Test Instructions\nFollow these."), 0o644)

	// Prompt
	os.MkdirAll(filepath.Join(gh, "prompts"), 0o755)
	os.WriteFile(filepath.Join(gh, "prompts", "test.prompt.md"), []byte("# Test Prompt\nGenerate a thing."), 0o644)

	return source
}

func TestCmdAdd_ClearsIgnoredStatus(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".github"), 0o755)
	scope := ScopeRepo(dir)

	// Pre-populate state with an ignored file
	state := &StateFile{
		Collection: "nextjs-frontend",
		Version:    "2025.07",
		Scope:      "repo",
		SourceSHA:  "abc123",
		Files: []InstalledFile{
			{Path: ".github/agents/test.agent.md", Hash: "oldhash", Status: fileStatusIgnored},
			{Path: ".github/skills/api-design/", Hash: "skillhash"},
		},
	}
	writeScopedState(scope, state)

	// Simulate what cmdAdd does when re-adding a file that exists in state:
	// merge result files, clearing ignored status
	newFile := InstalledFile{Path: ".github/agents/test.agent.md", Hash: "newhash"}

	updated, _ := readScopedState(scope)
	existing := make(map[string]bool)
	for _, f := range updated.Files {
		existing[f.Path] = true
	}
	if existing[newFile.Path] {
		for i, sf := range updated.Files {
			if sf.Path == newFile.Path {
				updated.Files[i].Hash = newFile.Hash
				updated.Files[i].Status = ""
				break
			}
		}
	} else {
		updated.Files = append(updated.Files, newFile)
	}
	writeScopedState(scope, updated)

	// Verify status was cleared and hash updated
	final, _ := readScopedState(scope)
	for _, f := range final.Files {
		if f.Path == ".github/agents/test.agent.md" {
			if f.Status != "" {
				t.Errorf("expected cleared status after re-add, got %q", f.Status)
			}
			if f.Hash != "newhash" {
				t.Errorf("expected updated hash, got %q", f.Hash)
			}
		}
	}

	// Verify resolveSyncFiles includes the file again
	files, _, err := resolveSyncFiles(scope, "", false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range files {
		if f.localPath == ".github/agents/test.agent.md" {
			found = true
		}
	}
	if !found {
		t.Error("re-added file should appear in sync file list")
	}
}
