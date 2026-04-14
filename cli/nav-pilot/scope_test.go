package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScopeRepo_Paths(t *testing.T) {
	scope := ScopeRepo("/tmp/myrepo")

	dst := scope.DstPath("agents", "nais.agent.md")
	want := filepath.Join("/tmp/myrepo", ".github", "agents", "nais.agent.md")
	if dst != want {
		t.Errorf("DstPath = %q, want %q", dst, want)
	}

	rel := scope.RelPath("agents", "nais.agent.md")
	if rel != filepath.Join(".github", "agents", "nais.agent.md") {
		t.Errorf("RelPath = %q", rel)
	}

	src := scope.SourcePath("/tmp/source", "agents", "nais.agent.md")
	if src != filepath.Join("/tmp/source", ".github", "agents", "nais.agent.md") {
		t.Errorf("SourcePath = %q", src)
	}

	if scope.IsUser() {
		t.Error("repo scope should not be user")
	}
	if scope.Label() != "/tmp/myrepo" {
		t.Errorf("Label = %q", scope.Label())
	}
}

func TestScopeUser_Paths(t *testing.T) {
	scope := &InstallScope{
		Name:           "user",
		RootDir:        "/home/dev/.copilot",
		StateFile:      ".nav-pilot-state.json",
		PathPrefix:     "",
		SupportedTypes: []string{"agent", "skill"},
	}

	dst := scope.DstPath("agents", "nais.agent.md")
	want := filepath.Join("/home/dev/.copilot", "agents", "nais.agent.md")
	if dst != want {
		t.Errorf("DstPath = %q, want %q", dst, want)
	}

	rel := scope.RelPath("agents", "nais.agent.md")
	if rel != filepath.Join("agents", "nais.agent.md") {
		t.Errorf("RelPath = %q", rel)
	}

	if !scope.IsUser() {
		t.Error("user scope should be user")
	}
	if scope.Label() != "~/.copilot (user-wide)" {
		t.Errorf("Label = %q", scope.Label())
	}
}

func TestScope_SupportsType(t *testing.T) {
	repo := ScopeRepo("/tmp")
	for _, typ := range []string{"agent", "skill", "instruction", "prompt"} {
		if !repo.SupportsType(typ) {
			t.Errorf("repo scope should support %q", typ)
		}
	}

	user := &InstallScope{
		Name:           "user",
		SupportedTypes: []string{"agent", "skill"},
	}
	for _, typ := range []string{"agent", "skill"} {
		if !user.SupportsType(typ) {
			t.Errorf("user scope should support %q", typ)
		}
	}
	for _, typ := range []string{"instruction", "prompt"} {
		if user.SupportsType(typ) {
			t.Errorf("user scope should NOT support %q", typ)
		}
	}
}

func TestScope_ValidateStatePath(t *testing.T) {
	repo := ScopeRepo("/tmp")
	user := &InstallScope{Name: "user", SupportedTypes: []string{"agent", "skill"}}

	tests := []struct {
		scope   *InstallScope
		path    string
		wantErr bool
	}{
		{repo, ".github/agents/foo.agent.md", false},
		{repo, ".github/skills/bar/", false},
		{repo, "agents/foo.agent.md", true},         // missing .github/ prefix
		{repo, "/etc/passwd", true},                  // absolute
		{repo, ".github/../../../etc/passwd", true},  // traversal
		{user, "agents/foo.agent.md", false},
		{user, "skills/bar/", false},
		{user, ".github/agents/foo.agent.md", true},  // .github not allowed in user
		{user, "/etc/passwd", true},                  // absolute
		{user, "instructions/foo.instructions.md", true}, // not supported
	}

	for _, tt := range tests {
		err := tt.scope.ValidateStatePath(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateStatePath(%q, scope=%s) err=%v, wantErr=%v", tt.path, tt.scope.Name, err, tt.wantErr)
		}
	}
}

func TestScope_CleanupDirs(t *testing.T) {
	tmp := t.TempDir()

	// Create empty dirs
	for _, sub := range []string{"agents", "skills"} {
		os.MkdirAll(filepath.Join(tmp, sub), 0o755)
	}

	scope := &InstallScope{Name: "user", RootDir: tmp, SupportedTypes: []string{"agent", "skill"}}
	scope.CleanupDirs()

	for _, sub := range []string{"agents", "skills"} {
		if _, err := os.Stat(filepath.Join(tmp, sub)); !os.IsNotExist(err) {
			t.Errorf("directory %q should have been removed", sub)
		}
	}
}

func TestUserAndTargetMutuallyExclusive(t *testing.T) {
	err := run([]string{"install", "--user", "--target", "/tmp/foo", "fullstack"})
	if err == nil {
		t.Fatal("expected error for --user + --target")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUserAndTargetDotMutuallyExclusive(t *testing.T) {
	err := run([]string{"install", "--user", "--target", ".", "fullstack"})
	if err == nil {
		t.Fatal("expected error for --user + --target . (explicit dot)")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCmdAdd_UserScopeRejectsInstruction(t *testing.T) {
	scope := &InstallScope{
		Name:           "user",
		RootDir:        t.TempDir(),
		SupportedTypes: []string{"agent", "skill"},
	}
	err := cmdAdd("instruction", "test", scope, "", "", true, false)
	if err == nil {
		t.Fatal("expected error for instruction in user scope")
	}
	if !strings.Contains(err.Error(), "not supported in user scope") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCmdAdd_UserScopeRejectsPrompt(t *testing.T) {
	scope := &InstallScope{
		Name:           "user",
		RootDir:        t.TempDir(),
		SupportedTypes: []string{"agent", "skill"},
	}
	err := cmdAdd("prompt", "test", scope, "", "", true, false)
	if err == nil {
		t.Fatal("expected error for prompt in user scope")
	}
	if !strings.Contains(err.Error(), "not supported in user scope") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallAgent_UserScope(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()

	// Create source agent
	agentDir := filepath.Join(source, ".github", "agents")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "test.agent.md"), []byte("# Test Agent"), 0o644)
	os.WriteFile(filepath.Join(agentDir, "test.metadata.json"), []byte(`{"name":"test"}`), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		PathPrefix:     "",
		SupportedTypes: []string{"agent", "skill"},
	}

	result := &installResult{}
	err := installAgent(source, scope, "test", false, false, result)
	if err != nil {
		t.Fatalf("installAgent user scope: %v", err)
	}
	if result.Installed != 1 {
		t.Errorf("expected 1 installed, got %d", result.Installed)
	}

	// Verify agent file is at agents/test.agent.md (no .github prefix)
	dst := filepath.Join(target, "agents", "test.agent.md")
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("agent file not created at user-scope path")
	}

	// Metadata should NOT be installed for user scope
	dstMeta := filepath.Join(target, "agents", "test.metadata.json")
	if _, err := os.Stat(dstMeta); !os.IsNotExist(err) {
		t.Error("metadata should not be installed in user scope")
	}

	// State should have "agents/test.agent.md" path (no .github prefix)
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
	if result.Files[0].Path != filepath.Join("agents", "test.agent.md") {
		t.Errorf("expected agents/test.agent.md, got %q", result.Files[0].Path)
	}
}

func TestInstallItems_UserScope_SkipsUnsupported(t *testing.T) {
	source := createFixtureSource(t)
	target := t.TempDir()

	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		PathPrefix:     "",
		SupportedTypes: []string{"agent", "skill"},
	}

	manifest, err := loadManifest(source, "test-collection")
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}

	result, err := installItems(source, scope, manifest, false, false)
	if err != nil {
		t.Fatalf("installItems: %v", err)
	}

	// Only agent and skill should be installed, instruction and prompt skipped
	if result.Installed != 2 {
		t.Errorf("expected 2 installed (agent + skill), got %d", result.Installed)
	}
	if len(result.Unsupported) != 2 {
		t.Errorf("expected 2 unsupported, got %d: %v", len(result.Unsupported), result.Unsupported)
	}
}

func TestInstalledAgents_UserScope(t *testing.T) {
	state := &StateFile{
		Files: []InstalledFile{
			{Path: "agents/nais.agent.md"},
			{Path: "agents/security-champion.agent.md"},
			{Path: "skills/postgresql-review/"},
		},
	}
	agents := installedAgents(state)
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d: %v", len(agents), agents)
	}
	if agents[0] != "nais" || agents[1] != "security-champion" {
		t.Errorf("unexpected agents: %v", agents)
	}
}

func TestReadScopedState_RejectsRepoStateInUserScope(t *testing.T) {
	tmp := t.TempDir()
	state := &StateFile{
		Collection: "test",
		Scope:      "repo",
		Files: []InstalledFile{
			{Path: ".github/agents/test.agent.md", Hash: "abc"},
		},
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	statePath := filepath.Join(tmp, ".nav-pilot-state.json")
	os.WriteFile(statePath, data, 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        tmp,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill"},
	}

	_, err := readScopedState(scope)
	if err == nil {
		t.Fatal("expected scope mismatch error")
	}
	if !strings.Contains(err.Error(), "scope mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadScopedState_RejectsUserStateInRepoScope(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".github"), 0o755)
	state := &StateFile{
		Collection: "test",
		Scope:      "user",
		Files: []InstalledFile{
			{Path: "agents/test.agent.md", Hash: "abc"},
		},
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	statePath := filepath.Join(tmp, ".github", ".nav-pilot-state.json")
	os.WriteFile(statePath, data, 0o644)

	scope := ScopeRepo(tmp)

	_, err := readScopedState(scope)
	if err == nil {
		t.Fatal("expected scope mismatch error")
	}
	if !strings.Contains(err.Error(), "scope mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadScopedState_AcceptsEmptyScopeAsRepo(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".github"), 0o755)
	// Old state file without scope field (backwards compat)
	state := &StateFile{
		Collection: "test",
		Files: []InstalledFile{
			{Path: ".github/agents/test.agent.md", Hash: "abc"},
		},
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	statePath := filepath.Join(tmp, ".github", ".nav-pilot-state.json")
	os.WriteFile(statePath, data, 0o644)

	scope := ScopeRepo(tmp)

	got, err := readScopedState(scope)
	if err != nil {
		t.Fatalf("expected success for empty scope (backwards compat), got: %v", err)
	}
	if got.Collection != "test" {
		t.Errorf("expected collection 'test', got %q", got.Collection)
	}
}

func TestScope_ShouldInstallMetadata(t *testing.T) {
	repo := ScopeRepo("/tmp")
	if !repo.ShouldInstallMetadata() {
		t.Error("repo scope should install metadata")
	}

	user := &InstallScope{Name: "user", SupportedTypes: []string{"agent", "skill"}}
	if user.ShouldInstallMetadata() {
		t.Error("user scope should NOT install metadata")
	}
}
