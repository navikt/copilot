package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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

	err := installAgent(srcDir, dstDir, "test", false, false, result)
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

	err := installAgent(srcDir, dstDir, "nonexistent", false, false, result)
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

	err := installSkill(srcDir, dstDir, "my-skill", false, false, result)
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
	err := installAgent(srcDir, dstDir, "test", false, false, result)
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
	err := installAgent(srcDir, dstDir, "test", false, true, result) // force=true
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
