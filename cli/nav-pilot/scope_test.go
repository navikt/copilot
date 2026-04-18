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
		SupportedTypes: []string{"agent", "skill", "instruction"},
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

	// Instructions in user scope get .github/ prefix (required by COPILOT_CUSTOM_INSTRUCTIONS_DIRS)
	instrDst := scope.DstPath("instructions", "golang.instructions.md")
	instrWant := filepath.Join("/home/dev/.copilot", ".github", "instructions", "golang.instructions.md")
	if instrDst != instrWant {
		t.Errorf("DstPath(instructions) = %q, want %q", instrDst, instrWant)
	}

	instrRel := scope.RelPath("instructions", "golang.instructions.md")
	instrRelWant := filepath.Join(".github", "instructions", "golang.instructions.md")
	if instrRel != instrRelWant {
		t.Errorf("RelPath(instructions) = %q, want %q", instrRel, instrRelWant)
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
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	for _, typ := range []string{"agent", "skill", "instruction"} {
		if !user.SupportsType(typ) {
			t.Errorf("user scope should support %q", typ)
		}
	}
	for _, typ := range []string{"prompt"} {
		if user.SupportsType(typ) {
			t.Errorf("user scope should NOT support %q", typ)
		}
	}
}

func TestScope_ValidateStatePath(t *testing.T) {
	repo := ScopeRepo("/tmp")
	user := &InstallScope{Name: "user", SupportedTypes: []string{"agent", "skill", "instruction"}}

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
		{user, ".github/instructions/foo.instructions.md", false}, // instructions use .github/ in user scope
		{user, ".github/agents/foo.agent.md", true},  // .github/agents not allowed in user
		{user, "/etc/passwd", true},                  // absolute
		{user, "instructions/foo.instructions.md", true}, // bare instructions/ not allowed
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

	scope := &InstallScope{Name: "user", RootDir: tmp, SupportedTypes: []string{"agent", "skill", "instruction"}}
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

func TestCmdAdd_UserScopeAcceptsInstruction(t *testing.T) {
	source := t.TempDir()
	ghDir := filepath.Join(source, ".github", "instructions")
	os.MkdirAll(ghDir, 0o755)
	os.WriteFile(filepath.Join(ghDir, "test.instructions.md"), []byte("# Test"), 0o644)

	scope := &InstallScope{
		Name:           "user",
		RootDir:        t.TempDir(),
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	// Dry run won't need real source resolution, but cmdAdd resolves source.
	// Instead, verify SupportsType directly since cmdAdd needs network.
	if !scope.SupportsType("instruction") {
		t.Error("user scope should support instruction type")
	}
}

func TestCmdAdd_UserScopeRejectsPrompt(t *testing.T) {
	scope := &InstallScope{
		Name:           "user",
		RootDir:        t.TempDir(),
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	err := cmdAdd("prompt", "test", scope, "", "", true, false, false)
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
		SupportedTypes: []string{"agent", "skill", "instruction"},
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
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}

	manifest, err := loadManifest(source, "test-collection")
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}

	result, err := installItems(source, scope, manifest, false, false)
	if err != nil {
		t.Fatalf("installItems: %v", err)
	}

	// Agent, skill, and instruction should be installed; only prompt skipped
	if result.Installed != 3 {
		t.Errorf("expected 3 installed (agent + skill + instruction), got %d", result.Installed)
	}
	if len(result.Unsupported) != 1 {
		t.Errorf("expected 1 unsupported (prompt), got %d: %v", len(result.Unsupported), result.Unsupported)
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
		SupportedTypes: []string{"agent", "skill", "instruction"},
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

	user := &InstallScope{Name: "user", SupportedTypes: []string{"agent", "skill", "instruction"}}
	if user.ShouldInstallMetadata() {
		t.Error("user scope should NOT install metadata")
	}
}

func TestResolveSourcePath(t *testing.T) {
	tests := []struct {
		name        string
		localPath   string
		isUserScope bool
		setupRoot   bool // create root-level skills/ in source
		want        string
	}{
		// Repo scope: .github/skills/x/ → skills/x/ when root exists
		{
			name: "repo_skill_root_exists",
			localPath: ".github/skills/api-design/",
			isUserScope: false,
			setupRoot: true,
			want: "skills/api-design/",
		},
		// Repo scope: .github/skills/x/ → stays when no root (legacy)
		{
			name: "repo_skill_legacy_only",
			localPath: ".github/skills/api-design/",
			isUserScope: false,
			setupRoot: false,
			want: ".github/skills/api-design/",
		},
		// Repo scope: .github/agents/x stays (agents not affected)
		{
			name: "repo_agent_unchanged",
			localPath: ".github/agents/nais.agent.md",
			isUserScope: false,
			setupRoot: true,
			want: ".github/agents/nais.agent.md",
		},
		// User scope: skills/x/ stays when root exists
		{
			name: "user_skill_root_exists",
			localPath: "skills/api-design/",
			isUserScope: true,
			setupRoot: true,
			want: "skills/api-design/",
		},
		// User scope: skills/x/ → .github/skills/x/ when no root
		{
			name:        "user_skill_legacy_only",
			localPath:   "skills/api-design/",
			isUserScope: true,
			setupRoot:   false,
			want:        filepath.Join(".github", "skills", "api-design") + "/",
		},
		// User scope: agents/x → .github/agents/x (always, agents stay in .github/)
		{
			name: "user_agent_always_prefixed",
			localPath: "agents/nais.agent.md",
			isUserScope: true,
			setupRoot: false,
			want: filepath.Join(".github", "agents/nais.agent.md"),
		},
		// User scope: .github/instructions/x → stays (already has prefix)
		{
			name: "user_instruction_already_prefixed",
			localPath: ".github/instructions/golang.instructions.md",
			isUserScope: true,
			setupRoot: false,
			want: ".github/instructions/golang.instructions.md",
		},
		// Both dirs exist: root wins for repo scope
		{
			name: "repo_skill_both_exist_root_wins",
			localPath: ".github/skills/api-design/",
			isUserScope: false,
			setupRoot: true, // root also exists, will set up .github/skills too
			want: "skills/api-design/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceDir := t.TempDir()

			// Always create .github/skills/api-design/ as legacy
			os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "api-design"), 0o755)
			os.MkdirAll(filepath.Join(sourceDir, ".github", "agents"), 0o755)
			os.MkdirAll(filepath.Join(sourceDir, ".github", "instructions"), 0o755)
			os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "api-design", "SKILL.md"), []byte("# API Design"), 0o644)
			os.WriteFile(filepath.Join(sourceDir, ".github", "agents", "nais.agent.md"), []byte("# Nais"), 0o644)
			os.WriteFile(filepath.Join(sourceDir, ".github", "instructions", "golang.instructions.md"), []byte("# Go"), 0o644)

			if tt.setupRoot {
				os.MkdirAll(filepath.Join(sourceDir, "skills", "api-design"), 0o755)
				os.WriteFile(filepath.Join(sourceDir, "skills", "api-design", "SKILL.md"), []byte("# API Design"), 0o644)
			}

			got := resolveSourcePath(sourceDir, tt.localPath, tt.isUserScope)
			if got != tt.want {
				t.Errorf("resolveSourcePath(%q, user=%v) = %q, want %q", tt.localPath, tt.isUserScope, got, tt.want)
			}
		})
	}
}

func TestResolveSourcePath_InvalidRootFallsBackToLegacy(t *testing.T) {
	sourceDir := t.TempDir()

	// Root dir exists but has NO SKILL.md — invalid
	os.MkdirAll(filepath.Join(sourceDir, "skills", "broken"), 0o755)

	// Legacy dir has valid SKILL.md
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "broken"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "broken", "SKILL.md"), []byte("# Valid"), 0o644)

	// Repo scope: should NOT pick invalid root, should stay at .github/
	got := resolveSourcePath(sourceDir, ".github/skills/broken/", false)
	if got != ".github/skills/broken/" {
		t.Errorf("repo scope: got %q, want .github/skills/broken/ (invalid root should not win)", got)
	}

	// User scope: should also fall back to .github/
	got = resolveSourcePath(sourceDir, "skills/broken/", true)
	want := filepath.Join(".github", "skills", "broken") + "/"
	if got != want {
		t.Errorf("user scope: got %q, want %q (invalid root should not win)", got, want)
	}
}

func TestResolveSkillDir(t *testing.T) {
	sourceDir := t.TempDir()

	// Root-level valid skill
	os.MkdirAll(filepath.Join(sourceDir, "skills", "root-skill"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "root-skill", "SKILL.md"), []byte("# Root"), 0o644)

	// Legacy valid skill
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "legacy-skill"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "legacy-skill", "SKILL.md"), []byte("# Legacy"), 0o644)

	// Invalid root (dir exists, no SKILL.md) with valid legacy
	os.MkdirAll(filepath.Join(sourceDir, "skills", "invalid-root"), 0o755)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "invalid-root"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "invalid-root", "SKILL.md"), []byte("# Fallback"), 0o644)

	tests := []struct {
		name      string
		skillName string
		wantDir   string
		wantOK    bool
	}{
		{"root skill", "root-skill", filepath.Join(sourceDir, "skills", "root-skill"), true},
		{"legacy skill", "legacy-skill", filepath.Join(sourceDir, ".github", "skills", "legacy-skill"), true},
		{"invalid root falls back", "invalid-root", filepath.Join(sourceDir, ".github", "skills", "invalid-root"), true},
		{"nonexistent", "nope", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, ok := resolveSkillDir(sourceDir, tt.skillName)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if dir != tt.wantDir {
				t.Errorf("dir = %q, want %q", dir, tt.wantDir)
			}
		})
	}
}

func TestScanSkillDirs(t *testing.T) {
	sourceDir := t.TempDir()

	// Root-only skill
	os.MkdirAll(filepath.Join(sourceDir, "skills", "alpha"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "alpha", "SKILL.md"), []byte("# Alpha"), 0o644)

	// Legacy-only skill
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "beta"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "beta", "SKILL.md"), []byte("# Beta"), 0o644)

	// Both locations — root should win (dedup)
	os.MkdirAll(filepath.Join(sourceDir, "skills", "gamma"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "gamma", "SKILL.md"), []byte("# Gamma Root"), 0o644)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "gamma"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "gamma", "SKILL.md"), []byte("# Gamma Legacy"), 0o644)

	// Invalid root — dir exists but no SKILL.md, legacy is valid
	os.MkdirAll(filepath.Join(sourceDir, "skills", "delta"), 0o755)
	os.MkdirAll(filepath.Join(sourceDir, ".github", "skills", "delta"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, ".github", "skills", "delta", "SKILL.md"), []byte("# Delta"), 0o644)

	skills := scanSkillDirs(sourceDir)

	// Should find all 4 unique skills, sorted
	if len(skills) != 4 {
		t.Fatalf("expected 4 skills, got %d: %v", len(skills), skills)
	}
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	expected := []string{"alpha", "beta", "delta", "gamma"}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("skill[%d] = %q, want %q", i, names[i], want)
		}
	}

	// Verify root wins for gamma
	for _, s := range skills {
		if s.Name == "gamma" && strings.Contains(s.Dir, ".github") {
			t.Errorf("gamma should come from root, not .github: %q", s.Dir)
		}
	}

	// Verify delta comes from legacy (invalid root)
	for _, s := range skills {
		if s.Name == "delta" && !strings.Contains(s.Dir, ".github") {
			t.Errorf("delta should come from legacy (invalid root), got %q", s.Dir)
		}
	}
}

func TestScanSkillDirs_RootOnly(t *testing.T) {
	sourceDir := t.TempDir()

	// Only root-level skills, no .github/skills at all
	os.MkdirAll(filepath.Join(sourceDir, "skills", "a"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "a", "SKILL.md"), []byte("# A"), 0o644)
	os.MkdirAll(filepath.Join(sourceDir, "skills", "b"), 0o755)
	os.WriteFile(filepath.Join(sourceDir, "skills", "b", "SKILL.md"), []byte("# B"), 0o644)

	skills := scanSkillDirs(sourceDir)
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	if skills[0].Name != "a" || skills[1].Name != "b" {
		t.Errorf("expected [a, b], got [%s, %s]", skills[0].Name, skills[1].Name)
	}
}
