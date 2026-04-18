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
	sourceDir := t.TempDir()

	// Create source with legacy layout
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "agents", "nais.agent.md"), []byte("# Nais"), 0o644)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "api-design"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "api-design", "SKILL.md"), []byte("# API"), 0o644)

	state := &StateFile{
		Collection: "kotlin-backend",
		Version:    "2025.07",
		Files: []InstalledFile{
			{Path: ".github/agents/nais.agent.md", Hash: "abc123"},
			{Path: ".github/skills/api-design/", Hash: "def456"},
		},
	}
	writeState(dir, state)

	files, collection, err := resolveSyncFiles(ScopeRepo(dir), sourceDir)
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

func TestResolveSyncFiles_AutoDetect_RootLevelSource(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	// Target has skill at .github/skills/api-design/
	os.MkdirAll(filepath.Join(targetDir, ".github", "skills", "api-design"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "skills", "api-design", "SKILL.md"), []byte("old"), 0o644)

	// Source has skill at ROOT skills/api-design/ (post-migration)
	os.MkdirAll(filepath.Join(sourceDir, "skills", "api-design"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "api-design", "SKILL.md"), []byte("new"), 0o644)

	files, _, err := resolveSyncFiles(ScopeRepo(targetDir), sourceDir)
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, f := range files {
		if f.localPath == filepath.Join(".github", "skills", "api-design")+"/" {
			found = true
			wantSource := filepath.Join("skills", "api-design") + "/"
			if f.sourcePath != wantSource {
				t.Errorf("sourcePath = %q, want %q (should point to root-level source)", f.sourcePath, wantSource)
			}
		}
	}
	if !found {
		t.Error("should find api-design skill with root-level source")
	}
}

func TestCheckSyncFile_CarriesSourcePath(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	// Target has skill at .github/skills/s/ — source at root skills/s/
	os.MkdirAll(filepath.Join(targetDir, ".github", "skills", "s"), 0o755)
	os.MkdirAll(filepath.Join(sourceDir, "skills", "s"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "skills", "s", "SKILL.md"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(sourceDir, "skills", "s", "SKILL.md"), []byte("new"), 0o644)

	sf := syncFile{
		localPath:  filepath.Join(".github", "skills", "s") + "/",
		sourcePath: filepath.Join("skills", "s") + "/",
		isDir:      true,
	}
	u, err := checkSyncFile(targetDir, sourceDir, sf)
	if err != nil {
		t.Fatal(err)
	}
	if u == nil {
		t.Fatal("expected update")
	}
	if u.SourcePath != sf.sourcePath {
		t.Errorf("SourcePath = %q, want %q (should carry through from syncFile)", u.SourcePath, sf.sourcePath)
	}
}

func TestApplySyncUpdate_RootLevelSource(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	// Source skill at root level
	os.MkdirAll(filepath.Join(sourceDir, "skills", "s"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "s", "SKILL.md"), []byte("root content"), 0o644)

	// Target skill at .github/skills/s/
	os.MkdirAll(filepath.Join(targetDir, ".github", "skills", "s"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "skills", "s", "SKILL.md"), []byte("old"), 0o644)

	u := syncUpdate{
		Path:        filepath.Join(".github", "skills", "s") + "/",
		SourcePath:  filepath.Join("skills", "s") + "/",
		CurrentHash: "a",
		SourceHash:  "b",
	}
	if err := applySyncUpdate(ScopeRepo(targetDir), sourceDir, u); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(filepath.Join(targetDir, ".github", "skills", "s", "SKILL.md"))
	if string(got) != "root content" {
		t.Errorf("skill not updated from root source, got %q", string(got))
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

	u := syncUpdate{Path: rel, SourcePath: rel, CurrentHash: "a", SourceHash: "b"}
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

	u := syncUpdate{Path: rel, SourcePath: rel, CurrentHash: "a", SourceHash: "b"}
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
	sourceDir := t.TempDir()
	scope := &InstallScope{
		Name:           "user",
		RootDir:        homeDir,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}

	// Create source with legacy layout
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "agents", "nais.agent.md"), []byte("# Nais"), 0o644)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "api-design"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "api-design", "SKILL.md"), []byte("# API"), 0o644)

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

	files, collection, err := resolveSyncFiles(scope, sourceDir)
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
	expectedSkillSource := filepath.Join(".github", "skills", "api-design") + "/"
	if skill.sourcePath != expectedSkillSource {
		t.Errorf("sourcePath = %q, want %q", skill.sourcePath, expectedSkillSource)
	}
}

func TestResolveSyncFiles_UserScope_InstructionPathNotDoubled(t *testing.T) {
	// Regression: instructions store paths as ".github/instructions/x.instructions.md"
	// which already has .github/ prefix. resolveSyncFiles must NOT double it.
	homeDir := t.TempDir()
	sourceDir := t.TempDir()
	scope := &InstallScope{
		Name:           "user",
		RootDir:        homeDir,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}

	// Create source layout
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "agents", "nais.agent.md"), []byte("# Nais"), 0o644)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "instructions"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "instructions", "go-nais.instructions.md"), []byte("# Go"), 0o644)

	state := &StateFile{
		Collection: CollectionAll,
		Version:    "dev",
		Scope:      "user",
		Files: []InstalledFile{
			{Path: "agents/nais.agent.md", Hash: "abc"},
			{Path: ".github/instructions/go-nais.instructions.md", Hash: "def"},
		},
	}
	writeScopedState(scope, state)

	files, _, err := resolveSyncFiles(scope, sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files count = %d, want 2", len(files))
	}

	// Agent: local="agents/nais.agent.md" → source=".github/agents/nais.agent.md"
	if files[0].sourcePath != filepath.Join(".github", "agents", "nais.agent.md") {
		t.Errorf("agent sourcePath = %q, want .github/agents/nais.agent.md", files[0].sourcePath)
	}

	// Instruction: already has .github/ prefix — source should NOT be ".github/.github/..."
	expectedInstrSource := filepath.Join(".github", "instructions", "go-nais.instructions.md")
	if files[1].sourcePath != expectedInstrSource {
		t.Errorf("instruction sourcePath = %q, want %q (was double-prefixed?)", files[1].sourcePath, expectedInstrSource)
	}
}

func TestApplySyncUpdate_UserScope_PathRemapping(t *testing.T) {
	homeDir := t.TempDir()
	sourceDir := t.TempDir()

	scope := &InstallScope{
		Name:           "user",
		RootDir:        homeDir,
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}

	// Source has the file at .github/agents/x.agent.md
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "agents", "x.agent.md"), []byte("new content"), 0o644)

	// Target (user home) has the file at agents/x.agent.md
	os.MkdirAll(filepath.Join(homeDir, "agents"), 0o755)
	os.WriteFile(filepath.Join(homeDir, "agents", "x.agent.md"), []byte("old content"), 0o644)

	u := syncUpdate{Path: "agents/x.agent.md", SourcePath: filepath.Join(".github", "agents", "x.agent.md"), CurrentHash: "a", SourceHash: "b"}
	if err := applySyncUpdate(scope, sourceDir, u); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(filepath.Join(homeDir, "agents", "x.agent.md"))
	if string(got) != "new content" {
		t.Errorf("file not updated, got %q", string(got))
	}
}

func TestApplySyncUpdate_UserScope_InstructionNotDoubled(t *testing.T) {
	homeDir := t.TempDir()
	sourceDir := t.TempDir()

	scope := &InstallScope{
		Name:           "user",
		RootDir:        homeDir,
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}

	// Source has instruction at .github/instructions/
	instrRel := filepath.Join(".github", "instructions", "go-nais.instructions.md")
	os.MkdirAll(filepath.Join(sourceDir, ".github", "instructions"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, instrRel), []byte("new instruction"), 0o644)

	// Target (user home) also has .github/instructions/ prefix
	os.MkdirAll(filepath.Join(homeDir, ".github", "instructions"), 0o755)
	os.WriteFile(filepath.Join(homeDir, instrRel), []byte("old instruction"), 0o644)

	u := syncUpdate{Path: instrRel, SourcePath: instrRel, CurrentHash: "a", SourceHash: "b"}
	if err := applySyncUpdate(scope, sourceDir, u); err != nil {
		t.Fatalf("applySyncUpdate failed (double .github/ prefix?): %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(homeDir, instrRel))
	if string(got) != "new instruction" {
		t.Errorf("instruction not updated, got %q", string(got))
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
		SupportedTypes: []string{"agent", "skill", "instruction"},
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

// ─── Override tests ─────────────────────────────────────────────────────────

func TestOverrideSet_FiltersMatchingFiles(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	// Create two files in both target and source with different content
	agentRel := filepath.Join(".github", "agents", "nais.agent.md")
	instrRel := filepath.Join(".github", "instructions", "security.instructions.md")
	for _, dir := range []string{targetDir, sourceDir} {
		os.MkdirAll(filepath.Join(dir, ".github", "agents"), 0o755)
		os.MkdirAll(filepath.Join(dir, ".github", "instructions"), 0o755)
	}
	os.WriteFile(filepath.Join(targetDir, agentRel), []byte("local agent"), 0o644)
	os.WriteFile(filepath.Join(sourceDir, agentRel), []byte("source agent"), 0o644)
	os.WriteFile(filepath.Join(targetDir, instrRel), []byte("local instr"), 0o644)
	os.WriteFile(filepath.Join(sourceDir, instrRel), []byte("source instr"), 0o644)

	// Mark agent as overridden
	cfg := &SyncConfig{Overrides: []string{agentRel}}
	overrides := overrideSet(cfg)

	files := []syncFile{
		{localPath: agentRel, sourcePath: agentRel},
		{localPath: instrRel, sourcePath: instrRel},
	}

	var filtered []syncFile
	for _, sf := range files {
		if !overrides[sf.localPath] {
			filtered = append(filtered, sf)
		}
	}

	if len(filtered) != 1 {
		t.Fatalf("filtered count = %d, want 1", len(filtered))
	}
	if filtered[0].localPath != instrRel {
		t.Errorf("expected instruction file, got %q", filtered[0].localPath)
	}
}

func TestOverrideSet_DirPath(t *testing.T) {
	cfg := &SyncConfig{Overrides: []string{".github/skills/api-design/"}}
	overrides := overrideSet(cfg)

	files := []syncFile{
		{localPath: ".github/skills/api-design/", sourcePath: ".github/skills/api-design/", isDir: true},
		{localPath: ".github/skills/other/", sourcePath: ".github/skills/other/", isDir: true},
	}

	var filtered []syncFile
	for _, sf := range files {
		if !overrides[sf.localPath] {
			filtered = append(filtered, sf)
		}
	}

	if len(filtered) != 1 {
		t.Fatalf("filtered count = %d, want 1", len(filtered))
	}
	if filtered[0].localPath != ".github/skills/other/" {
		t.Errorf("expected other skill, got %q", filtered[0].localPath)
	}
}

func TestSyncResultJSON_WithOverrides(t *testing.T) {
	result := syncResult{
		UpToDate:  true,
		Source:    "abc1234",
		Overrides: []string{".github/agents/custom.agent.md"},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var got syncResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Overrides) != 1 {
		t.Errorf("overrides count = %d, want 1", len(got.Overrides))
	}
	if got.Overrides[0] != ".github/agents/custom.agent.md" {
		t.Errorf("overrides[0] = %q", got.Overrides[0])
	}
}

// ─── Formatting-tolerant comparison tests ───────────────────────────────────

func TestCheckSyncFile_FormattingTolerant_MD(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	rel := filepath.Join(".github", "agents", "x.agent.md")
	os.MkdirAll(filepath.Join(targetDir, ".github", "agents"), 0o755)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)

	// Same content but different formatting (trailing whitespace, CRLF)
	os.WriteFile(filepath.Join(targetDir, rel), []byte("# Agent\nDo stuff\n"), 0o644)
	os.WriteFile(filepath.Join(sourceDir, rel), []byte("# Agent  \r\nDo stuff   \r\n"), 0o644)

	sf := syncFile{localPath: rel, sourcePath: rel}
	u, err := checkSyncFile(targetDir, sourceDir, sf)
	if err != nil {
		t.Fatal(err)
	}
	if u != nil {
		t.Error("expected no update for formatting-only difference in .md file")
	}
}

func TestCheckSyncFile_RealDiff_MD(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	rel := filepath.Join(".github", "agents", "x.agent.md")
	os.MkdirAll(filepath.Join(targetDir, ".github", "agents"), 0o755)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)

	// Actual content difference
	os.WriteFile(filepath.Join(targetDir, rel), []byte("# Agent v1\nOld content\n"), 0o644)
	os.WriteFile(filepath.Join(sourceDir, rel), []byte("# Agent v2\nNew content\n"), 0o644)

	sf := syncFile{localPath: rel, sourcePath: rel}
	u, err := checkSyncFile(targetDir, sourceDir, sf)
	if err != nil {
		t.Fatal(err)
	}
	if u == nil {
		t.Error("expected update for real content difference")
	}
}

func TestCheckSyncFile_JSON_ByteExact(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	rel := filepath.Join(".github", "agents", "x.metadata.json")
	os.MkdirAll(filepath.Join(targetDir, ".github", "agents"), 0o755)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)

	// JSON with whitespace difference — should still trigger update (byte-exact)
	os.WriteFile(filepath.Join(targetDir, rel), []byte(`{"key":"value"}`), 0o644)
	os.WriteFile(filepath.Join(sourceDir, rel), []byte(`{"key": "value"}`), 0o644)

	sf := syncFile{localPath: rel, sourcePath: rel}
	u, err := checkSyncFile(targetDir, sourceDir, sf)
	if err != nil {
		t.Fatal(err)
	}
	if u == nil {
		t.Error("expected update for JSON whitespace difference (byte-exact)")
	}
}

func TestResolveSyncFiles_SkipsIgnoredFiles(t *testing.T) {
	dir := t.TempDir()
	sourceDir := t.TempDir()

	// Create source with legacy layout
	os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "agents", "nais.agent.md"), []byte("# Nais"), 0o644)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "instructions"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "instructions", "nextjs-aksel.instructions.md"), []byte("# NJS"), 0o644)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "api-design"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "api-design", "SKILL.md"), []byte("# API"), 0o644)

	state := &StateFile{
		Collection: "nextjs-frontend",
		Version:    "2025.07",
		Scope:      "repo",
		Files: []InstalledFile{
			{Path: ".github/agents/nais.agent.md", Hash: "abc123"},
			{Path: ".github/instructions/nextjs-aksel.instructions.md", Hash: "def456", Status: fileStatusIgnored},
			{Path: ".github/skills/api-design/", Hash: "ghi789"},
		},
	}
	writeState(dir, state)

	files, _, err := resolveSyncFiles(ScopeRepo(dir), sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files count = %d, want 2 (ignored file should be excluded)", len(files))
	}
	for _, f := range files {
		if f.localPath == ".github/instructions/nextjs-aksel.instructions.md" {
			t.Error("ignored file should not appear in sync file list")
		}
	}
}

func TestMarkFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	scope := ScopeRepo(dir)

	state := &StateFile{
		Collection: "nextjs-frontend",
		Version:    "2025.07",
		Scope:      "repo",
		Files: []InstalledFile{
			{Path: ".github/agents/nais.agent.md", Hash: "abc123"},
			{Path: ".github/instructions/nextjs-aksel.instructions.md", Hash: "def456"},
			{Path: ".github/skills/api-design/", Hash: "ghi789"},
		},
	}
	writeScopedState(scope, state)

	err := markFilesIgnored(scope, []string{".github/instructions/nextjs-aksel.instructions.md"})
	if err != nil {
		t.Fatal(err)
	}

	// Re-read state and verify
	updated, err := readScopedState(scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range updated.Files {
		switch f.Path {
		case ".github/instructions/nextjs-aksel.instructions.md":
			if f.Status != fileStatusIgnored {
				t.Errorf("expected status 'ignored', got %q", f.Status)
			}
		default:
			if f.Status != "" {
				t.Errorf("unexpected status %q for %s", f.Status, f.Path)
			}
		}
	}
}

func TestStateFile_BackwardsCompat_NoStatusField(t *testing.T) {
	// Simulate a state file written by an older version of nav-pilot (no status field)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".github"), 0o755)
	stateJSON := `{
		"collection": "kotlin-backend",
		"version": "2025.07",
		"scope": "repo",
		"source_sha": "abc123",
		"installed_at": "2025-07-01T00:00:00Z",
		"files": [
			{"path": ".github/agents/nais.agent.md", "hash": "abc123"},
			{"path": ".github/skills/api-design/", "hash": "def456"}
		]
	}`
	os.WriteFile(filepath.Join(dir, stateFilePath), []byte(stateJSON), 0o644)

	state, err := readScopedState(ScopeRepo(dir))
	if err != nil {
		t.Fatal(err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	// All files should have empty Status (active)
	for _, f := range state.Files {
		if f.Status != "" {
			t.Errorf("expected empty status for %s, got %q", f.Path, f.Status)
		}
	}

	// resolveSyncFiles should include all files (none ignored)
	files, _, err := resolveSyncFiles(ScopeRepo(dir), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files count = %d, want 2", len(files))
	}
}

func TestCountFileIntegrity_IgnoredFiles(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".github"), 0o755)
	f1 := filepath.Join(tmp, ".github", "a.md")
	os.WriteFile(f1, []byte("hello"), 0o644)
	hash1, _ := fileHash(f1)

	state := &StateFile{
		Files: []InstalledFile{
			{Path: ".github/a.md", Hash: hash1},                                          // ok
			{Path: ".github/ignored.md", Hash: "x", Status: fileStatusIgnored},            // ignored
			{Path: ".github/missing.md", Hash: "x"},                                      // missing
		},
	}

	ok, modified, missing, ignored, _ := countFileIntegrity(tmp, state)
	if ok != 1 {
		t.Errorf("ok = %d, want 1", ok)
	}
	if modified != 0 {
		t.Errorf("modified = %d, want 0", modified)
	}
	if missing != 1 {
		t.Errorf("missing = %d, want 1", missing)
	}
	if ignored != 1 {
		t.Errorf("ignored = %d, want 1", ignored)
	}
}

func TestResolveSyncFiles_AutoDetect_InvalidRootFallsBack(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	// Target has skill installed
	os.MkdirAll(filepath.Join(targetDir, ".github", "skills", "my-skill"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "skills", "my-skill", "SKILL.md"), []byte("old"), 0o644)

	// Source has root dir but NO SKILL.md (invalid)
	os.MkdirAll(filepath.Join(sourceDir, "skills", "my-skill"), 0o755)

	// Source has valid legacy location
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "my-skill"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "my-skill", "SKILL.md"), []byte("new-legacy"), 0o644)

	files, _, err := resolveSyncFiles(ScopeRepo(targetDir), sourceDir)
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, f := range files {
		if strings.Contains(f.localPath, "my-skill") {
			found = true
			// Source path should point to legacy (invalid root must not win)
			if !strings.Contains(f.sourcePath, ".github") {
				t.Errorf("sourcePath = %q, should point to .github/ (invalid root)", f.sourcePath)
			}
		}
	}
	if !found {
		t.Error("should find my-skill in sync files")
	}
}
