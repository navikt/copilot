package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	// Override HOME so ScopeUser() finds no existing user-home installs
	t.Setenv("HOME", dir)

	err := cmdInteractive()
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("unexpected error: %v", err)
	}
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
