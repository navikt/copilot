package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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

	files, collection, err := resolveSyncFiles(dir, "")
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

	files, collection, err := resolveSyncFiles(targetDir, sourceDir)
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
	if err := applySyncUpdate(targetDir, sourceDir, u); err != nil {
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
	if err := applySyncUpdate(targetDir, sourceDir, u); err != nil {
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
