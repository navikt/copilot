package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSyncFiles_WithState(t *testing.T) {
	dir := t.TempDir()

	state := &StateFile{
		Collection: "kotlin-backend",
		Version:    "2025.07",
		Files: []InstalledFile{
			{Path: ".github/agents/nais.agent.md", Hash: "abc123"},
			{Path: ".github/skills/api-design/", Hash: "def456"},
		},
	}
	writeState(dir, state)

	files, collection, err := resolveSyncFiles(ScopeRepo(dir), "")
	if err != nil {
		t.Fatal(err)
	}
	if collection != "kotlin-backend" {
		t.Errorf("collection = %q, want %q", collection, "kotlin-backend")
	}
	if len(files) != 2 {
		t.Fatalf("files count = %d, want 2", len(files))
	}
	if !files[1].isDir {
		t.Error("skill path should be detected as dir")
	}
}

func TestResolveSyncFiles_AutoDetect(t *testing.T) {
	// Setup target repo with some customization files
	targetDir := t.TempDir()
	os.MkdirAll(filepath.Join(targetDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "agents", "nais.agent.md"), []byte("# Nais"), 0o644)
	os.WriteFile(filepath.Join(targetDir, ".github", "agents", "auth.agent.md"), []byte("# Auth"), 0o644)

	os.MkdirAll(filepath.Join(targetDir, ".github", "skills", "api-design"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "skills", "api-design", "SKILL.md"), []byte("# API"), 0o644)

	// Setup source repo with matching files (nais exists, auth does not)
	sourceDir := t.TempDir()
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "agents", "nais.agent.md"), []byte("# Nais v2"), 0o644)
	// auth.agent.md intentionally missing in source

	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "api-design"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "api-design", "SKILL.md"), []byte("# API"), 0o644)

	files, collection, err := resolveSyncFiles(ScopeRepo(targetDir), sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if collection != "" {
		t.Errorf("collection should be empty for auto-detect, got %q", collection)
	}

	// Should find nais.agent.md and api-design skill (not auth.agent.md since it's not in source)
	foundNais := false
	foundAuth := false
	foundSkill := false
	for _, f := range files {
		switch {
		case f.localPath == filepath.Join(".github", "agents", "nais.agent.md"):
			foundNais = true
		case f.localPath == filepath.Join(".github", "agents", "auth.agent.md"):
			foundAuth = true
		case f.localPath == filepath.Join(".github", "skills", "api-design")+"/":
			foundSkill = true
		}
	}
	if !foundNais {
		t.Error("should find nais.agent.md")
	}
	if foundAuth {
		t.Error("should NOT find auth.agent.md (not in source)")
	}
	if !foundSkill {
		t.Error("should find api-design skill")
	}
}

func TestCheckSyncFile_UpToDate(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	// Same content in both
	os.MkdirAll(filepath.Join(targetDir, ".github", "agents"), 0o755)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "agents", "x.agent.md"), []byte("same"), 0o644)
	os.WriteFile(filepath.Join(sourceDir, ".github", "agents", "x.agent.md"), []byte("same"), 0o644)

	sf := syncFile{localPath: filepath.Join(".github", "agents", "x.agent.md"), sourcePath: filepath.Join(".github", "agents", "x.agent.md")}
	u, err := checkSyncFile(targetDir, sourceDir, sf)
	if err != nil {
		t.Fatal(err)
	}
	if u != nil {
		t.Error("expected no update for identical files")
	}
}

func TestCheckSyncFile_UpdateAvailable(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	os.MkdirAll(filepath.Join(targetDir, ".github", "agents"), 0o755)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "agents", "x.agent.md"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(sourceDir, ".github", "agents", "x.agent.md"), []byte("new"), 0o644)

	sf := syncFile{localPath: filepath.Join(".github", "agents", "x.agent.md"), sourcePath: filepath.Join(".github", "agents", "x.agent.md")}
	u, err := checkSyncFile(targetDir, sourceDir, sf)
	if err != nil {
		t.Fatal(err)
	}
	if u == nil {
		t.Fatal("expected update for differing files")
	}
	if u.CurrentHash == u.SourceHash {
		t.Error("hashes should differ")
	}
}

func TestCheckSyncFile_Directory(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	// Different content in skill dirs
	os.MkdirAll(filepath.Join(targetDir, ".github", "skills", "s"), 0o755)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "s"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "skills", "s", "SKILL.md"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "s", "SKILL.md"), []byte("new"), 0o644)

	sf := syncFile{
		localPath:  filepath.Join(".github", "skills", "s") + "/",
		sourcePath: filepath.Join(".github", "skills", "s") + "/",
		isDir:      true,
	}
	u, err := checkSyncFile(targetDir, sourceDir, sf)
	if err != nil {
		t.Fatal(err)
	}
	if u == nil {
		t.Fatal("expected update for differing dirs")
	}
}

func TestApplySyncUpdate_File(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	rel := filepath.Join(".github", "agents", "x.agent.md")
	os.MkdirAll(filepath.Join(targetDir, ".github", "agents"), 0o755)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(targetDir, rel), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(sourceDir, rel), []byte("new"), 0o644)

	u := syncUpdate{Path: rel, CurrentHash: "a", SourceHash: "b"}
	if err := applySyncUpdate(ScopeRepo(targetDir), sourceDir, u); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(filepath.Join(targetDir, rel))
	if string(got) != "new" {
		t.Errorf("file not updated, got %q", string(got))
	}
}

func TestApplySyncUpdate_Dir(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	rel := filepath.Join(".github", "skills", "s") + "/"
	os.MkdirAll(filepath.Join(targetDir, ".github", "skills", "s"), 0o755)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "s"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "skills", "s", "SKILL.md"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "s", "SKILL.md"), []byte("new"), 0o644)

	u := syncUpdate{Path: rel, CurrentHash: "a", SourceHash: "b"}
	if err := applySyncUpdate(ScopeRepo(targetDir), sourceDir, u); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(filepath.Join(targetDir, ".github", "skills", "s", "SKILL.md"))
	if string(got) != "new" {
		t.Errorf("skill not updated, got %q", string(got))
	}
}

func TestUpdateStateHashes(t *testing.T) {
	dir := t.TempDir()

	// Create a file
	rel := filepath.Join(".github", "agents", "x.agent.md")
	os.MkdirAll(filepath.Join(dir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(dir, rel), []byte("updated content"), 0o644)

	// Write state with old hash
	state := &StateFile{
		Collection: "test",
		Files: []InstalledFile{
			{Path: rel, Hash: "oldhash"},
		},
	}
	writeState(dir, state)

	newHash, _ := fileHash(filepath.Join(dir, rel))
	updates := []syncUpdate{{Path: rel, CurrentHash: "oldhash", SourceHash: newHash}}

	if err := updateStateHashes(dir, updates); err != nil {
		t.Fatal(err)
	}

	// Read back and verify hash was updated
	got, _ := readState(dir)
	if got.Files[0].Hash != newHash {
		t.Errorf("hash = %q, want %q", got.Files[0].Hash, newHash)
	}
}

func TestUpdateStateHashes_NoState(t *testing.T) {
	dir := t.TempDir()
	// Should not error when no state file exists
	err := updateStateHashes(dir, []syncUpdate{{Path: "x", CurrentHash: "a", SourceHash: "b"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncResultJSON(t *testing.T) {
	result := syncResult{
		UpToDate: false,
		Source:   "abc1234",
		Updates: []syncUpdate{
			{Path: ".github/agents/x.agent.md", CurrentHash: "aaa", SourceHash: "bbb"},
		},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var got syncResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.UpToDate {
		t.Error("expected up_to_date=false")
	}
	if len(got.Updates) != 1 {
		t.Errorf("updates count = %d, want 1", len(got.Updates))
	}
}

// TestSyncJSON_StdoutIsCleanJSON is a regression test for the sync workflow
// failure where git's detached HEAD advice polluted the JSON output.
// It verifies that outputJSON writes only valid JSON to stdout.
func TestSyncJSON_StdoutIsCleanJSON(t *testing.T) {
	result := syncResult{
		UpToDate: false,
		Source:   "abc1234",
		Updates: []syncUpdate{
			{Path: ".github/agents/test.agent.md", CurrentHash: "aaa", SourceHash: "bbb"},
			{Path: ".github/skills/test-skill/", CurrentHash: "ccc", SourceHash: "ddd"},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	if err := outputJSON(result); err != nil {
		w.Close()
		os.Stdout = oldStdout
		t.Fatal(err)
	}

	w.Close()
	os.Stdout = oldStdout
	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	// stdout must be valid JSON — no git advice, no progress messages
	output := strings.TrimSpace(string(captured))
	var got syncResult
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("stdout is not valid JSON:\n---\n%s\n---\nerror: %v", output, err)
	}

	if got.UpToDate {
		t.Error("expected up_to_date=false")
	}
	if len(got.Updates) != 2 {
		t.Errorf("expected 2 updates, got %d", len(got.Updates))
	}
	if got.Source != "abc1234" {
		t.Errorf("expected source abc1234, got %s", got.Source)
	}
}

// ─── User-scope sync tests ──────────────────────────────────────────────────

func TestResolveSyncFiles_UserScope_PathRemapping(t *testing.T) {
	// User-scope state stores paths as "agents/x" but source has ".github/agents/x"
	homeDir := t.TempDir()
	scope := &InstallScope{
		Name:           "user",
		RootDir:        homeDir,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill"},
	}

	state := &StateFile{
		Collection: "fullstack",
		Version:    "dev",
		Scope:      "user",
		Files: []InstalledFile{
			{Path: "agents/nais.agent.md", Hash: "abc"},
			{Path: "skills/api-design/", Hash: "def"},
		},
	}
	writeScopedState(scope, state)

	files, collection, err := resolveSyncFiles(scope, "")
	if err != nil {
		t.Fatal(err)
	}
	if collection != "fullstack" {
		t.Errorf("collection = %q, want %q", collection, "fullstack")
	}
	if len(files) != 2 {
		t.Fatalf("files count = %d, want 2", len(files))
	}

	// Local path stays as "agents/..." but source path should be ".github/agents/..."
	agent := files[0]
	if agent.localPath != "agents/nais.agent.md" {
		t.Errorf("localPath = %q, want %q", agent.localPath, "agents/nais.agent.md")
	}
	expectedSource := filepath.Join(".github", "agents", "nais.agent.md")
	if agent.sourcePath != expectedSource {
		t.Errorf("sourcePath = %q, want %q", agent.sourcePath, expectedSource)
	}

	skill := files[1]
	if skill.localPath != "skills/api-design/" {
		t.Errorf("localPath = %q, want %q", skill.localPath, "skills/api-design/")
	}
	expectedSkillSource := filepath.Join(".github", "skills", "api-design")
	// filepath.Join strips trailing slash; isDir flag controls directory behavior
	if skill.sourcePath != expectedSkillSource {
		t.Errorf("sourcePath = %q, want %q", skill.sourcePath, expectedSkillSource)
	}
}

func TestApplySyncUpdate_UserScope_PathRemapping(t *testing.T) {
	homeDir := t.TempDir()
	sourceDir := t.TempDir()

	scope := &InstallScope{
		Name:           "user",
		RootDir:        homeDir,
		SupportedTypes: []string{"agent", "skill"},
	}

	// Source has the file at .github/agents/x.agent.md
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "agents", "x.agent.md"), []byte("new content"), 0o644)

	// Target (user home) has the file at agents/x.agent.md
	os.MkdirAll(filepath.Join(homeDir, "agents"), 0o755)
	os.WriteFile(filepath.Join(homeDir, "agents", "x.agent.md"), []byte("old content"), 0o644)

	u := syncUpdate{Path: "agents/x.agent.md", CurrentHash: "a", SourceHash: "b"}
	if err := applySyncUpdate(scope, sourceDir, u); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(filepath.Join(homeDir, "agents", "x.agent.md"))
	if string(got) != "new content" {
		t.Errorf("file not updated, got %q", string(got))
	}
}

func TestUpdateScopedStateHashes(t *testing.T) {
	dir := t.TempDir()
	scope := ScopeRepo(dir)

	rel := filepath.Join(".github", "agents", "x.agent.md")
	os.MkdirAll(filepath.Join(dir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(dir, rel), []byte("updated"), 0o644)

	state := &StateFile{
		Collection: "test",
		Scope:      "repo",
		Files:      []InstalledFile{{Path: rel, Hash: "oldhash"}},
	}
	writeScopedState(scope, state)

	newHash, _ := fileHash(filepath.Join(dir, rel))
	updates := []syncUpdate{{Path: rel, CurrentHash: "oldhash", SourceHash: newHash}}

	if err := updateScopedStateHashes(scope, updates); err != nil {
		t.Fatal(err)
	}

	got, _ := readScopedState(scope)
	if got.Files[0].Hash != newHash {
		t.Errorf("hash = %q, want %q", got.Files[0].Hash, newHash)
	}
}

func TestResolveSyncFiles_UserScope_NoState_ReturnsEmpty(t *testing.T) {
	homeDir := t.TempDir()
	scope := &InstallScope{
		Name:           "user",
		RootDir:        homeDir,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill"},
	}

	files, collection, err := resolveSyncFiles(scope, "")
	if err != nil {
		t.Fatal(err)
	}
	if collection != "" {
		t.Errorf("collection should be empty, got %q", collection)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files for user scope without state, got %d", len(files))
	}
}
