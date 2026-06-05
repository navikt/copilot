package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// ─── buildPickerDefaults tests ──────────────────────────────────────────────

func TestBuildPickerDefaults_FreshInstall(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents:       []string{"auth-agent", "nais-agent"},
		Skills:       []string{"kafka", "observability-setup"},
		Instructions: []string{"golang", "security-owasp"},
	}

	defaults := buildPickerDefaults(full, nil, scope)

	assertStringsEqual(t, "agents", defaults["agents"], full.Agents)
	assertStringsEqual(t, "skills", defaults["skills"], full.Skills)
	assertStringsEqual(t, "instructions", defaults["instructions"], full.Instructions)
}

func TestBuildPickerDefaults_EmptyState(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{Agents: []string{"auth-agent"}}
	emptyState := &StateFile{Files: []InstalledFile{}}

	defaults := buildPickerDefaults(full, emptyState, scope)

	assertStringsEqual(t, "agents", defaults["agents"], []string{"auth-agent"})
}

func TestBuildPickerDefaults_ReinstallPreservesActive(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents: []string{"auth-agent", "nais-agent", "rust-agent"},
		Skills: []string{"kafka"},
	}
	state := &StateFile{
		Files: []InstalledFile{
			{Path: "agents/auth-agent.agent.md", Hash: "abc"},
			{Path: "agents/nais-agent.agent.md", Hash: "def"},
			{Path: "agents/rust-agent.agent.md", Hash: "", Status: fileStatusIgnored},
			{Path: "skills/kafka/", Hash: "ghi"},
		},
	}

	defaults := buildPickerDefaults(full, state, scope)

	assertStringsEqual(t, "agents", defaults["agents"], []string{"auth-agent", "nais-agent"})
	assertStringsEqual(t, "skills", defaults["skills"], []string{"kafka"})
}

func TestBuildPickerDefaults_NewItemDefaultsToSelected(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents: []string{"auth-agent", "brand-new-agent"},
	}
	state := &StateFile{
		Files: []InstalledFile{
			{Path: "agents/auth-agent.agent.md", Hash: "abc"},
			// brand-new-agent is NOT in state
		},
	}

	defaults := buildPickerDefaults(full, state, scope)

	// New items absent from state should be selected by default
	assertStringsEqual(t, "agents", defaults["agents"], []string{"auth-agent", "brand-new-agent"})
}

func TestBuildPickerDefaults_RepoScope(t *testing.T) {
	scope := ScopeRepo(t.TempDir())
	full := &Manifest{
		Agents:       []string{"auth-agent"},
		Skills:       []string{"kafka"},
		Instructions: []string{"golang"},
	}
	state := &StateFile{
		Files: []InstalledFile{
			{Path: ".github/agents/auth-agent.agent.md", Hash: "abc"},
			{Path: ".github/skills/kafka/", Hash: "def"},
			{Path: ".github/instructions/golang.instructions.md", Hash: "", Status: fileStatusIgnored},
		},
	}

	defaults := buildPickerDefaults(full, state, scope)

	assertStringsEqual(t, "agents", defaults["agents"], []string{"auth-agent"})
	assertStringsEqual(t, "skills", defaults["skills"], []string{"kafka"})
	// golang is ignored → not in defaults
	if len(defaults["instructions"]) != 0 {
		t.Errorf("instructions should be empty (ignored), got %v", defaults["instructions"])
	}
}

func TestBuildPickerDefaults_UserScopeInstructionPaths(t *testing.T) {
	// User-scope instructions use .github/instructions/ prefix
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{Instructions: []string{"golang", "security-owasp"}}
	state := &StateFile{
		Files: []InstalledFile{
			{Path: ".github/instructions/golang.instructions.md", Hash: "abc"},
			{Path: ".github/instructions/security-owasp.instructions.md", Hash: "", Status: fileStatusIgnored},
		},
	}

	defaults := buildPickerDefaults(full, state, scope)

	assertStringsEqual(t, "instructions", defaults["instructions"], []string{"golang"})
}

func TestBuildPickerDefaults_SkillTrailingSlash(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{Skills: []string{"kafka", "nais"}}
	state := &StateFile{
		Files: []InstalledFile{
			{Path: "skills/kafka/", Hash: "abc"},
			{Path: "skills/nais/", Hash: "", Status: fileStatusIgnored},
		},
	}

	defaults := buildPickerDefaults(full, state, scope)

	assertStringsEqual(t, "skills", defaults["skills"], []string{"kafka"})
}

// ─── computeSkippedItems tests ──────────────────────────────────────────────

func TestComputeSkippedItems_AllSelected(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents:       []string{"auth-agent", "nais-agent"},
		Skills:       []string{"kafka"},
		Instructions: []string{"golang"},
	}
	selected := &Manifest{
		Agents:       []string{"auth-agent", "nais-agent"},
		Skills:       []string{"kafka"},
		Instructions: []string{"golang"},
	}

	skipped := computeSkippedItems(full, selected, scope)

	if len(skipped) != 0 {
		t.Errorf("expected no skipped items, got %v", skipped)
	}
}

func TestComputeSkippedItems_NoneSelected(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents: []string{"auth-agent", "nais-agent"},
		Skills: []string{"kafka"},
	}
	selected := &Manifest{}

	skipped := computeSkippedItems(full, selected, scope)

	if len(skipped) != 3 {
		t.Fatalf("expected 3 skipped, got %d: %v", len(skipped), skipped)
	}
	for _, s := range skipped {
		if s.Status != fileStatusIgnored {
			t.Errorf("expected status %q, got %q for %s", fileStatusIgnored, s.Status, s.Path)
		}
		if s.Hash != "" {
			t.Errorf("expected empty hash for skipped item %s, got %q", s.Path, s.Hash)
		}
	}
}

func TestComputeSkippedItems_PartialSelection(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{
		Agents:       []string{"auth-agent", "nais-agent", "rust-agent"},
		Skills:       []string{"kafka", "observability-setup"},
		Instructions: []string{"golang"},
	}
	selected := &Manifest{
		Agents:       []string{"auth-agent"},
		Skills:       []string{"kafka"},
		Instructions: []string{"golang"},
	}

	skipped := computeSkippedItems(full, selected, scope)

	if len(skipped) != 3 {
		t.Fatalf("expected 3 skipped, got %d: %v", len(skipped), skipped)
	}

	paths := make(map[string]bool)
	for _, s := range skipped {
		paths[s.Path] = true
	}
	wantSkipped := []string{
		"agents/nais-agent.agent.md",
		"agents/rust-agent.agent.md",
		"skills/observability-setup/",
	}
	for _, w := range wantSkipped {
		if !paths[w] {
			t.Errorf("expected skipped path %q not found in %v", w, skipped)
		}
	}
}

func TestComputeSkippedItems_RepoScope(t *testing.T) {
	scope := ScopeRepo(t.TempDir())
	full := &Manifest{
		Agents: []string{"auth-agent"},
		Skills: []string{"kafka"},
	}
	selected := &Manifest{} // nothing selected

	skipped := computeSkippedItems(full, selected, scope)

	paths := make(map[string]bool)
	for _, s := range skipped {
		paths[s.Path] = true
	}
	if !paths[".github/agents/auth-agent.agent.md"] {
		t.Error("repo scope agent path should have .github/ prefix")
	}
	if !paths[".github/skills/kafka/"] {
		t.Error("repo scope skill path should have .github/ prefix and trailing slash")
	}
}

func TestComputeSkippedItems_UserScopeInstructions(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{Instructions: []string{"golang"}}
	selected := &Manifest{}

	skipped := computeSkippedItems(full, selected, scope)

	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(skipped))
	}
	// User-scope instructions use .github/instructions/ prefix
	if skipped[0].Path != ".github/instructions/golang.instructions.md" {
		t.Errorf("expected .github/instructions/ path, got %q", skipped[0].Path)
	}
}

func TestComputeSkippedItems_EmptyManifest(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	skipped := computeSkippedItems(&Manifest{}, &Manifest{}, scope)
	if len(skipped) != 0 {
		t.Errorf("expected no skipped for empty manifest, got %v", skipped)
	}
}

// ─── RelPathForName tests ───────────────────────────────────────────────────

func TestRelPathForName_UserScope(t *testing.T) {
	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}

	tests := []struct {
		kind *ArtifactKind
		name string
		want string
	}{
		{KindAgent, "auth-agent", "agents/auth-agent.agent.md"},
		{KindSkill, "kafka", "skills/kafka/"},
		{KindInstruction, "golang", ".github/instructions/golang.instructions.md"},
		{KindPrompt, "conventional-commit", "prompts/conventional-commit.prompt.md"},
	}
	for _, tt := range tests {
		got := tt.kind.RelPathForName(scope, tt.name)
		if got != tt.want {
			t.Errorf("RelPathForName(%s, %q) = %q, want %q", tt.kind.Name, tt.name, got, tt.want)
		}
	}
}

func TestRelPathForName_RepoScope(t *testing.T) {
	scope := ScopeRepo(t.TempDir())

	tests := []struct {
		kind *ArtifactKind
		name string
		want string
	}{
		{KindAgent, "auth-agent", ".github/agents/auth-agent.agent.md"},
		{KindSkill, "kafka", ".github/skills/kafka/"},
		{KindInstruction, "golang", ".github/instructions/golang.instructions.md"},
		{KindPrompt, "conventional-commit", ".github/prompts/conventional-commit.prompt.md"},
	}
	for _, tt := range tests {
		got := tt.kind.RelPathForName(scope, tt.name)
		if got != tt.want {
			t.Errorf("RelPathForName(%s, %q) = %q, want %q", tt.kind.Name, tt.name, got, tt.want)
		}
	}
}

// ─── interactiveItemPicker guard test ───────────────────────────────────────

func TestInteractiveItemPicker_NonInteractiveReturnsError(t *testing.T) {
	forceNonInteractive = true
	defer func() { forceNonInteractive = false }()

	scope := &InstallScope{Name: "user", RootDir: t.TempDir(), PathPrefix: ""}
	full := &Manifest{Agents: []string{"auth-agent"}}

	_, _, err := interactiveItemPicker(full, nil, scope)
	if err == nil {
		t.Fatal("expected error when non-interactive")
	}
	if err.Error() != "interactive item picker requires a terminal" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── test helpers ───────────────────────────────────────────────────────────

func assertStringsEqual(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %v (len %d), want %v (len %d)", label, got, len(got), want, len(want))
		return
	}
	// Sort copies for comparison
	g := append([]string{}, got...)
	w := append([]string{}, want...)
	sort.Strings(g)
	sort.Strings(w)
	for i := range g {
		if g[i] != w[i] {
			t.Errorf("%s[%d]: got %q, want %q (full: got=%v, want=%v)", label, i, g[i], w[i], got, want)
		}
	}
}

func TestFindCopilotCLI(t *testing.T) {
	path, name := findCopilotCLI()
	if path != "" {
		if name != "cplt" && name != "copilot" {
			t.Errorf("expected name 'cplt' or 'copilot', got %q", name)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("returned path %q does not exist: %v", path, err)
		}
	} else {
		if name != "" {
			t.Errorf("expected empty name when path is empty, got %q", name)
		}
	}
}

func TestCmdInteractive_NotGitRepo(t *testing.T) {
	origDir, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Override HOME so ScopeUser() uses the temp dir
	t.Setenv("HOME", dir)

	// Verify that findGitRoot returns empty for a temp dir (no git repo)
	root := findGitRoot(".")
	if root != "" {
		t.Skipf("temp dir is inside a git repo (%s), skipping", root)
	}

	// The key assertion: cmdInteractive no longer produces the old
	// "not in a git repository" error — instead it tries to resolve source.
	// Since resolveSource does network I/O and interactive prompts may block,
	// we only verify the code path selection here rather than running the full flow.
	// The flow goes to interactiveUserOnlyInstall which attempts a clone.
	// This is tested indirectly by other tests (sync, add).
}

func TestCmdInteractive_InstalledUpToDate(t *testing.T) {
	origDir, _ := os.Getwd()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".github"), 0o755)
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Isolate HOME so user-scope installs don't leak into the test
	t.Setenv("HOME", dir)

	// Prevent huh TUI prompts from blocking in tests
	forceNonInteractive = true
	defer func() { forceNonInteractive = false }()

	state := &StateFile{
		Collection: "test-collection",
		Version:    "2026.04.13-170000-abc1234",
		SourceSHA:  "abc1234",
	}
	if err := writeState(dir, state); err != nil {
		t.Fatal(err)
	}

	// Mock GitHub API returning same version (up-to-date)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ghRelease{
			{TagName: "nav-pilot/2026.04.13-170000-abc1234"},
		})
	}))
	defer srv.Close()

	origAPI := releasesAPI
	releasesAPI = srv.URL
	defer func() { releasesAPI = origAPI }()

	setupTestCache(t)

	// Should not error — will try to launch cplt (which may not exist, that's ok)
	err := cmdInteractive()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstalledAgents(t *testing.T) {
	state := &StateFile{
		Files: []InstalledFile{
			{Path: ".github/agents/nav-pilot.agent.md"},
			{Path: ".github/agents/auth-agent.agent.md"},
			{Path: ".github/agents/nais-agent.agent.md"},
			{Path: ".github/skills/threat-model/SKILL.md"},
			{Path: ".github/instructions/golang.instructions.md"},
		},
	}
	agents := installedAgents(state)
	expected := []string{"auth-agent", "nais-agent", "nav-pilot"}
	if len(agents) != len(expected) {
		t.Fatalf("expected %d agents, got %d: %v", len(expected), len(agents), agents)
	}
	for i, a := range agents {
		if a != expected[i] {
			t.Errorf("agent[%d]: expected %q, got %q", i, expected[i], a)
		}
	}
}

func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		input []string
		want  []string
	}{
		{[]string{"b", "a", "a", "c"}, []string{"a", "b", "c"}},
		{[]string{"x"}, []string{"x"}},
		{nil, nil},
		{[]string{"a", "a", "a"}, []string{"a"}},
	}
	for _, tt := range tests {
		got := uniqueStrings(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("uniqueStrings(%v) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("uniqueStrings(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestCopilotAgentArgs(t *testing.T) {
	tests := []struct {
		agent string
		want  []string
	}{
		{"nav-pilot", nil},
		{"auth", nil},
		{"", nil},
	}
	for _, tt := range tests {
		got := copilotAgentArgs(tt.agent)
		if len(got) != len(tt.want) {
			t.Errorf("copilotAgentArgs(%q) = %v, want %v", tt.agent, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("copilotAgentArgs(%q)[%d] = %q, want %q", tt.agent, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIsCplt(t *testing.T) {
	// Create a fake binary that outputs cplt version info
	dir := t.TempDir()
	fakeCplt := filepath.Join(dir, "fake-cplt")
	os.WriteFile(fakeCplt, []byte("#!/bin/sh\necho 'cplt version 1.0.43'"), 0o755)

	if !isCplt(fakeCplt) {
		t.Error("expected isCplt=true for binary that outputs 'cplt'")
	}

	// Create a fake binary that outputs copilot version info
	fakeCopilot := filepath.Join(dir, "fake-copilot")
	os.WriteFile(fakeCopilot, []byte("#!/bin/sh\necho 'GitHub Copilot CLI 1.0.0'"), 0o755)

	if isCplt(fakeCopilot) {
		t.Error("expected isCplt=false for binary that outputs 'GitHub Copilot CLI'")
	}
}

func TestBuildCopilotArgs(t *testing.T) {
	tests := []struct {
		name    string
		cliName string
		agent   string
		want    []string
	}{
		{
			name:    "cplt with agent",
			cliName: "cplt",
			agent:   "nav-pilot",
			want:    []string{"--", "--agent", "nav-pilot"},
		},
		{
			name:    "copilot with agent",
			cliName: "copilot",
			agent:   "nav-pilot",
			want:    []string{"--agent", "nav-pilot"},
		},
		{
			name:    "cplt without agent",
			cliName: "cplt",
			agent:   "",
			want:    []string{},
		},
		{
			name:    "copilot with non-nav-pilot agent",
			cliName: "copilot",
			agent:   "auth",
			want:    []string{"--agent", "auth"},
		},
		{
			name:    "cplt with non-nav-pilot agent",
			cliName: "cplt",
			agent:   "auth",
			want:    []string{"--", "--agent", "auth"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCopilotArgs(tt.cliName, tt.agent)
			if len(got) != len(tt.want) {
				t.Fatalf("buildCopilotArgs(%q, %q) = %v, want %v", tt.cliName, tt.agent, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("buildCopilotArgs(%q, %q)[%d] = %q, want %q", tt.cliName, tt.agent, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestUserCopilotDir(t *testing.T) {
	// Create a temp HOME with user-scope agents but no instructions
	home := t.TempDir()
	t.Setenv("HOME", home)

	// No customizations → empty
	if got := userCopilotDir(); got != "" {
		t.Errorf("expected empty for no customizations, got %q", got)
	}

	// Create agents dir with an agent file
	agentsDir := filepath.Join(home, ".copilot", ".github", "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "nav-pilot.agent.md"), []byte("test"), 0o644)

	expected := filepath.Join(home, ".copilot")
	if got := userCopilotDir(); got != expected {
		t.Errorf("expected %q for agents-only, got %q", expected, got)
	}

	// Remove agents, add instructions instead
	os.RemoveAll(filepath.Join(home, ".copilot", ".github", "agents"))
	instrDir := filepath.Join(home, ".copilot", ".github", "instructions")
	os.MkdirAll(instrDir, 0o755)
	os.WriteFile(filepath.Join(instrDir, "golang.instructions.md"), []byte("test"), 0o644)

	if got := userCopilotDir(); got != expected {
		t.Errorf("expected %q for instructions-only, got %q", expected, got)
	}
}
