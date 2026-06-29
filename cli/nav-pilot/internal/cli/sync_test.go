package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

func TestResolveSyncFiles_WithState(t *testing.T) {
	dir := t.TempDir()
	sourceDir := t.TempDir()

	// Create source with legacy layout
	os.MkdirAll(filepath.Join(sourceDir, "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "agents", "nais.agent.md"), []byte("# Nais"), 0o644)
	os.MkdirAll(filepath.Join(sourceDir, "skills", "api-design"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "api-design", "SKILL.md"), []byte("# API"), 0o644)

	state := &StateFile{
		Collection: "kotlin-backend",
		Version:    "2025.07",
		Files: []InstalledFile{
			{Path: ".github/agents/nais.agent.md", Hash: "abc123"},
			{Path: ".github/skills/api-design/", Hash: "def456"},
		},
	}

	writeState(dir, state)

	files, collection, err := resolveSyncFiles(ScopeRepo(dir), sourceDir, false)
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

func TestResolveSyncFiles_ConflictHandling(t *testing.T) {
	dir := t.TempDir()
	sourceDir := t.TempDir()

	os.MkdirAll(filepath.Join(sourceDir, "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "agents", "nav-pilot.agent.md"), []byte("# New"), 0o644)

	state := &StateFile{
		Collection: "fullstack",
		Version:    "2025.07",
		Files: []InstalledFile{
			{Path: ".github/agents/nav-pilot.agent.md", Hash: "abc123", Status: fileStatusConflict},
		},
	}
	writeState(dir, state)

	// Default sync check should skip conflicts
	files, _, err := resolveSyncFiles(ScopeRepo(dir), sourceDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("files count (exclude conflicts) = %d, want 0", len(files))
	}

	// Apply mode should include conflicts so they can be overwritten
	files, _, err = resolveSyncFiles(ScopeRepo(dir), sourceDir, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("files count (include conflicts) = %d, want 1", len(files))
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
	os.MkdirAll(filepath.Join(sourceDir, "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "agents", "nais.agent.md"), []byte("# Nais v2"), 0o644)
	// auth.agent.md intentionally missing in source

	os.MkdirAll(filepath.Join(sourceDir, "skills", "api-design"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "api-design", "SKILL.md"), []byte("# API"), 0o644)

	files, collection, err := resolveSyncFiles(ScopeRepo(targetDir), sourceDir, false)
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

	files, _, err := resolveSyncFiles(ScopeRepo(targetDir), sourceDir, false)
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

func TestUpdateStateHashes_ClearsConflictStatus(t *testing.T) {
	dir := t.TempDir()

	rel := filepath.Join(".github", "agents", "x.agent.md")
	os.MkdirAll(filepath.Join(dir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(dir, rel), []byte("updated content"), 0o644)

	state := &StateFile{
		Collection: "test",
		Files: []InstalledFile{
			{Path: rel, Hash: "oldhash", Status: fileStatusConflict},
		},
	}
	writeState(dir, state)

	newHash, _ := fileHash(filepath.Join(dir, rel))
	updates := []syncUpdate{{Path: rel, CurrentHash: "oldhash", SourceHash: newHash}}

	if err := updateStateHashes(dir, updates); err != nil {
		t.Fatal(err)
	}

	got, _ := readState(dir)
	if got.Files[0].Status != "" {
		t.Errorf("status = %q, want empty", got.Files[0].Status)
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

	files, collection, err := resolveSyncFiles(scope, "", false)
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

func TestResolveSyncFiles_SkipsIgnoredFiles(t *testing.T) {
	dir := t.TempDir()
	sourceDir := t.TempDir()

	// Create source with legacy layout
	os.MkdirAll(filepath.Join(sourceDir, "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "agents", "nais.agent.md"), []byte("# Nais"), 0o644)
	os.MkdirAll(filepath.Join(sourceDir, "instructions"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "instructions", "nextjs-aksel.instructions.md"), []byte("# NJS"), 0o644)
	os.MkdirAll(filepath.Join(sourceDir, "skills", "api-design"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "api-design", "SKILL.md"), []byte("# API"), 0o644)

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

	files, _, err := resolveSyncFiles(ScopeRepo(dir), sourceDir, false)
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

func TestResolveSyncFiles_SkipsConflictedFiles(t *testing.T) {
	dir := t.TempDir()
	sourceDir := t.TempDir()

	// Create source
	os.MkdirAll(filepath.Join(sourceDir, "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "agents", "nais.agent.md"), []byte("# Nais"), 0o644)
	os.WriteFile(filepath.Join(sourceDir, "agents", "auth.agent.md"), []byte("# Auth"), 0o644)

	state := &StateFile{
		Collection: "(all)",
		Version:    "2025.07",
		Scope:      "repo",
		Files: []InstalledFile{
			{Path: ".github/agents/nais.agent.md", Hash: "abc123"},
			{Path: ".github/agents/auth.agent.md", Hash: "def456", Status: fileStatusConflict},
		},
	}
	writeState(dir, state)

	files, _, err := resolveSyncFiles(ScopeRepo(dir), sourceDir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("files count = %d, want 1 (conflicted file should be excluded)", len(files))
	}
	if files[0].localPath != ".github/agents/nais.agent.md" {
		t.Errorf("expected nais agent, got %s", files[0].localPath)
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
	files, _, err := resolveSyncFiles(ScopeRepo(dir), "", false)
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
			{Path: ".github/a.md", Hash: hash1},                                // ok
			{Path: ".github/ignored.md", Hash: "x", Status: fileStatusIgnored}, // ignored
			{Path: ".github/missing.md", Hash: "x"},                            // missing
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

func TestResolveSyncFiles_AutoDetect_RootLevelAgents(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	// Target has agent at .github/agents/
	os.MkdirAll(filepath.Join(targetDir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "agents", "nais.agent.md"), []byte("old"), 0o644)

	// Source has agent at ROOT agents/ (post-migration)
	os.MkdirAll(filepath.Join(sourceDir, "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "agents", "nais.agent.md"), []byte("new"), 0o644)

	files, _, err := resolveSyncFiles(ScopeRepo(targetDir), sourceDir, false)
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, f := range files {
		if f.localPath == filepath.Join(".github", "agents", "nais.agent.md") {
			found = true
			wantSource := filepath.Join("agents", "nais.agent.md")
			if f.sourcePath != wantSource {
				t.Errorf("sourcePath = %q, want %q (should point to root-level source)", f.sourcePath, wantSource)
			}
		}
	}
	if !found {
		t.Error("should find nais.agent.md with root-level source")
	}
}

func TestResolveSyncFiles_AutoDetect_RootLevelInstructions(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	// Target has instruction at .github/instructions/
	os.MkdirAll(filepath.Join(targetDir, ".github", "instructions"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "instructions", "go.instructions.md"), []byte("old"), 0o644)

	// Source has instruction at ROOT instructions/ (post-migration)
	os.MkdirAll(filepath.Join(sourceDir, "instructions"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "instructions", "go.instructions.md"), []byte("new"), 0o644)

	files, _, err := resolveSyncFiles(ScopeRepo(targetDir), sourceDir, false)
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, f := range files {
		if f.localPath == filepath.Join(".github", "instructions", "go.instructions.md") {
			found = true
			wantSource := filepath.Join("instructions", "go.instructions.md")
			if f.sourcePath != wantSource {
				t.Errorf("sourcePath = %q, want %q (should point to root-level source)", f.sourcePath, wantSource)
			}
		}
	}
	if !found {
		t.Error("should find go.instructions.md with root-level source")
	}
}

func TestResolveSyncFiles_AutoDetect_RootLevelPromptDir(t *testing.T) {
	targetDir := t.TempDir()
	sourceDir := t.TempDir()

	// Target has prompt dir at .github/prompts/review/
	os.MkdirAll(filepath.Join(targetDir, ".github", "prompts", "review"), 0o755)
	os.WriteFile(filepath.Join(targetDir, ".github", "prompts", "review", "prompt.md"), []byte("old"), 0o644)

	// Source has prompt dir at ROOT prompts/review/
	os.MkdirAll(filepath.Join(sourceDir, "prompts", "review"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "prompts", "review", "prompt.md"), []byte("new"), 0o644)

	files, _, err := resolveSyncFiles(ScopeRepo(targetDir), sourceDir, false)
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, f := range files {
		if f.localPath == filepath.Join(".github", "prompts", "review")+"/" {
			found = true
			wantSource := filepath.Join("prompts", "review") + "/"
			if f.sourcePath != wantSource {
				t.Errorf("sourcePath = %q, want %q (should point to root-level source)", f.sourcePath, wantSource)
			}
		}
	}
	if !found {
		t.Error("should find review prompt dir with root-level source")
	}
}

func TestCmdSyncAuto_BothScopes(t *testing.T) {
	// Isolate HOME so user-scope state is not found
	t.Setenv("HOME", t.TempDir())

	// cmdSyncAuto with no installed scopes should report nothing
	emptyDir := t.TempDir()
	os.MkdirAll(filepath.Join(emptyDir, ".git"), 0o755)
	err := cmdSyncAuto(emptyDir, "", "", false, false)
	if err != nil {
		t.Fatalf("expected nil for empty scopes, got: %v", err)
	}
}

func TestCmdSyncAuto_NoInstall(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)

	// Isolate HOME so user-scope state is not found
	t.Setenv("HOME", t.TempDir())

	// Capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmdSyncAuto(dir, "", "", false, false)

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "No nav-pilot collection installed") {
		t.Errorf("expected 'no collection' message, got: %s", out)
	}
}

func TestCmdSyncAuto_BothScopes_PrintsScopeFeedback(t *testing.T) {
	repoDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Repo scope state
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755)
	repoScope := ScopeRepo(repoDir)
	if err := writeScopedState(repoScope, &StateFile{Collection: "fullstack", Scope: "repo"}); err != nil {
		t.Fatal(err)
	}

	// User scope state
	userScope, err := ScopeUser()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeScopedState(userScope, &StateFile{Collection: CollectionAll, Scope: "user"}); err != nil {
		t.Fatal(err)
	}

	origCmdSyncFn := cmdSyncFn
	t.Cleanup(func() { cmdSyncFn = origCmdSyncFn })
	cmdSyncFn = func(scope *InstallScope, ref, sourceRepo string, apply, jsonOutput bool) error {
		return nil
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = cmdSyncAuto(repoDir, "", "", false, false)

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := string(out)
	if !strings.Contains(output, "Repo scope synced") {
		t.Errorf("expected repo synced feedback, got: %s", output)
	}
	if !strings.Contains(output, "User scope synced") {
		t.Errorf("expected user synced feedback, got: %s", output)
	}
}

func TestSync_RemoteDeletions(t *testing.T) {
	dir := t.TempDir()
	sourceDir := t.TempDir()

	// 1. Setup target directory (with files installed and tracked in state)
	targetScope := ScopeRepo(dir)
	os.MkdirAll(filepath.Join(dir, ".github", "agents"), 0o755)
	os.WriteFile(filepath.Join(dir, ".github", "agents", "nais.agent.md"), []byte("# Nais"), 0o644)
	os.WriteFile(filepath.Join(dir, ".github", "agents", "deprecated.agent.md"), []byte("# Deprecated"), 0o644)

	state := &StateFile{
		Collection: "kotlin-backend",
		Version:    "2026.06",
		SourceRepo: "my-custom/repo",
		Files: []InstalledFile{
			{Path: ".github/agents/nais.agent.md", Hash: "nais-hash"},
			{Path: ".github/agents/deprecated.agent.md", Hash: "deprecated-hash"},
		},
	}
	writeState(dir, state)

	// 2. Setup source directory (only has nais, deprecated.agent.md is DELETED)
	os.MkdirAll(filepath.Join(sourceDir, "agents"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "agents", "nais.agent.md"), []byte("# Nais"), 0o644)

	// Override resolveSourceForSync to point to our sourceDir
	origResolveSourceForSync := resolveSourceForSync
	t.Cleanup(func() { resolveSourceForSync = origResolveSourceForSync })
	resolveSourceForSync = func(ref, sourceRepo string) (*source.Source, error) {
		if sourceRepo != "my-custom/repo" {
			t.Errorf("expected sourceRepo %q, got %q", "my-custom/repo", sourceRepo)
		}
		return &source.Source{Dir: sourceDir, SHA: "new-sha", Version: "2026.07", Repo: sourceRepo}, nil
	}

	// 3. Sync check (dry run): should report deletion and return errUpdatesAvailable
	err := cmdSync(targetScope, "", "", false, false)
	if err != errUpdatesAvailable {
		t.Fatalf("expected errUpdatesAvailable, got %v", err)
	}

	// Check that target files still exist
	if _, err := os.Stat(filepath.Join(dir, ".github", "agents", "deprecated.agent.md")); os.IsNotExist(err) {
		t.Error("deprecated file should not be removed in dry run")
	}

	// 4. Sync apply: should delete the deprecated file and update the state
	err = cmdSync(targetScope, "", "", true, false)
	if err != nil {
		t.Fatalf("sync apply failed: %v", err)
	}

	// Check that target file is removed
	if _, err := os.Stat(filepath.Join(dir, ".github", "agents", "deprecated.agent.md")); !os.IsNotExist(err) {
		t.Error("deprecated file should have been removed in apply run")
	}

	// Check that state has been updated (deprecated file removed, sourceRepo preserved)
	newState, err := readScopedState(targetScope)
	if err != nil {
		t.Fatal(err)
	}
	if newState.SourceRepo != "my-custom/repo" {
		t.Errorf("expected SourceRepo to be %q, got %q", "my-custom/repo", newState.SourceRepo)
	}
	if len(newState.Files) != 1 || newState.Files[0].Path != ".github/agents/nais.agent.md" {
		t.Errorf("state files = %v, expected only nais.agent.md", newState.Files)
	}
}

func TestSync_DirectoryPromptAndLocalSource(t *testing.T) {
	dir := t.TempDir()
	sourceDir := t.TempDir()

	// 1. Setup local source repo
	os.MkdirAll(filepath.Join(sourceDir, "prompts", "my-prompt"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "prompts", "my-prompt", "custom.md"), []byte("# Prompt"), 0o644)

	// Resolve local source (simulating dev mode)
	src, err := source.ResolveSource("", sourceDir, "2026.07")
	if err != nil {
		t.Fatal(err)
	}
	if src.Repo != sourceDir {
		t.Errorf("expected Repo to be sourceDir path %q, got %q", sourceDir, src.Repo)
	}

	// 2. Install directory prompt to target
	targetScope := ScopeRepo(dir)
	err = cmdAddFromSource("prompt", "my-prompt", src, targetScope, false, false, false)
	if err != nil {
		t.Fatal(err)
	}

	// Check that prompt dir exists locally in target
	targetPromptDir := filepath.Join(dir, ".github", "prompts", "my-prompt")
	if info, err := os.Stat(targetPromptDir); err != nil || !info.IsDir() {
		t.Error("prompt should have been installed as directory")
	}

	// Verify that state path includes trailing slash for prompt dir
	state, err := readScopedState(targetScope)
	if err != nil {
		t.Fatal(err)
	}
	foundSlash := false
	for _, f := range state.Files {
		if f.Path == ".github/prompts/my-prompt/" {
			foundSlash = true
		}
	}
	if !foundSlash {
		t.Errorf("state files did not contain directory prompt path with trailing slash: %v", state.Files)
	}

	// 3. Test that sync works correctly using the tracked local source path
	err = cmdSync(targetScope, "", "", false, false)
	if err != nil && err != errUpdatesAvailable {
		t.Fatalf("sync check failed: %v", err)
	}
}
