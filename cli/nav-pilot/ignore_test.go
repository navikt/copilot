package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── cmdIgnore tests ─────────────────────────────────────────────────────────

func TestCmdIgnore_InvalidType(t *testing.T) {
	scope, _ := tempUserScope(t)
	err := cmdIgnore("prompt", "something", scope, false)
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("expected 'unknown type' error, got %v", err)
	}
}

func TestCmdIgnore_InvalidName(t *testing.T) {
	scope, _ := tempUserScope(t)
	err := cmdIgnore("instruction", "../evil", scope, false)
	if err == nil || !strings.Contains(err.Error(), "invalid name") {
		t.Errorf("expected 'invalid name' error, got %v", err)
	}
}

func TestCmdIgnore_NonUserScope(t *testing.T) {
	dir := t.TempDir()
	scope := ScopeRepo(dir)
	err := cmdIgnore("instruction", "nextjs-aksel", scope, false)
	if err == nil || !strings.Contains(err.Error(), "user-scope") {
		t.Errorf("expected user-scope error, got %v", err)
	}
}

func TestCmdIgnore_NoState(t *testing.T) {
	scope, _ := tempUserScope(t)
	err := cmdIgnore("instruction", "nextjs-aksel", scope, false)
	if err == nil || !strings.Contains(err.Error(), "no installation found") {
		t.Errorf("expected 'no installation found' error, got %v", err)
	}
}

func TestCmdIgnore_AddsNewIgnoredEntry(t *testing.T) {
	scope, target := tempUserScope(t)
	writeAllState(t, scope, []InstalledFile{
		{Path: "agents/nais.agent.md", Hash: "abc"},
	})

	if err := cmdIgnore("instruction", "nextjs-aksel", scope, false); err != nil {
		t.Fatalf("cmdIgnore: %v", err)
	}

	state := readTestState(t, target)
	wantPath := ".github/instructions/nextjs-aksel.instructions.md"
	found := false
	for _, f := range state.Files {
		if f.Path == wantPath {
			found = true
			if f.Status != fileStatusIgnored {
				t.Errorf("status = %q, want %q", f.Status, fileStatusIgnored)
			}
			if f.Hash != "" {
				t.Errorf("hash = %q, want empty", f.Hash)
			}
		}
	}
	if !found {
		t.Errorf("ignored entry not found in state (want path %q)", wantPath)
	}
}

func TestCmdIgnore_AddsAgentEntry(t *testing.T) {
	scope, target := tempUserScope(t)
	writeAllState(t, scope, nil)

	if err := cmdIgnore("agent", "security-champion", scope, false); err != nil {
		t.Fatalf("cmdIgnore: %v", err)
	}

	state := readTestState(t, target)
	wantPath := "agents/security-champion.agent.md"
	found := false
	for _, f := range state.Files {
		if f.Path == wantPath {
			found = true
			if f.Status != fileStatusIgnored {
				t.Errorf("status = %q, want %q", f.Status, fileStatusIgnored)
			}
		}
	}
	if !found {
		t.Errorf("ignored agent entry not found in state (want %q)", wantPath)
	}
}

func TestCmdIgnore_AddsSkillEntry(t *testing.T) {
	scope, target := tempUserScope(t)
	writeAllState(t, scope, nil)

	if err := cmdIgnore("skill", "kotlin-app-config", scope, false); err != nil {
		t.Fatalf("cmdIgnore: %v", err)
	}

	state := readTestState(t, target)
	wantPath := "skills/kotlin-app-config/"
	found := false
	for _, f := range state.Files {
		if f.Path == wantPath {
			found = true
			if f.Status != fileStatusIgnored {
				t.Errorf("status = %q, want %q", f.Status, fileStatusIgnored)
			}
		}
	}
	if !found {
		t.Errorf("ignored skill entry not found in state (want %q)", wantPath)
	}
}

func TestCmdIgnore_ActiveEntryMarkedIgnored(t *testing.T) {
	scope, target := tempUserScope(t)
	writeAllState(t, scope, []InstalledFile{
		{Path: ".github/instructions/nextjs-aksel.instructions.md", Hash: "deadbeef"},
	})

	if err := cmdIgnore("instruction", "nextjs-aksel", scope, false); err != nil {
		t.Fatalf("cmdIgnore: %v", err)
	}

	state := readTestState(t, target)
	for _, f := range state.Files {
		if f.Path == ".github/instructions/nextjs-aksel.instructions.md" {
			if f.Status != fileStatusIgnored {
				t.Errorf("status = %q, want %q", f.Status, fileStatusIgnored)
			}
			return
		}
	}
	t.Error("entry not found in state after ignore")
}

func TestCmdIgnore_AlreadyIgnoredIsIdempotent(t *testing.T) {
	scope, _ := tempUserScope(t)
	writeAllState(t, scope, []InstalledFile{
		{Path: ".github/instructions/nextjs-aksel.instructions.md", Hash: "", Status: fileStatusIgnored},
	})

	if err := cmdIgnore("instruction", "nextjs-aksel", scope, false); err != nil {
		t.Fatalf("cmdIgnore: %v", err)
	}
	// Call again — should still succeed (idempotent).
	if err := cmdIgnore("instruction", "nextjs-aksel", scope, false); err != nil {
		t.Fatalf("second cmdIgnore: %v", err)
	}
}

func TestCmdIgnore_JSONOutput(t *testing.T) {
	scope, _ := tempUserScope(t)
	writeAllState(t, scope, nil)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := cmdIgnore("instruction", "nextjs-aksel", scope, true)
	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("cmdIgnore: %v", err)
	}

	var out ignoreResult
	if decErr := json.NewDecoder(r).Decode(&out); decErr != nil {
		t.Fatalf("json decode: %v", decErr)
	}
	if out.Type != "instruction" {
		t.Errorf("type = %q, want instruction", out.Type)
	}
	if out.Name != "nextjs-aksel" {
		t.Errorf("name = %q, want nextjs-aksel", out.Name)
	}
	if out.Status != "ignored" {
		t.Errorf("status = %q, want ignored", out.Status)
	}
}

// TestDetectNewItems_IgnoredEntryNotReported ensures that items explicitly
// ignored via cmdIgnore do not reappear in new-item reminders.
func TestDetectNewItems_IgnoredEntryNotReported(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	ghDir := filepath.Join(source, ".github")

	// Source has one agent and one instruction.
	agentsDir := filepath.Join(ghDir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "nais.agent.md"), []byte("# Nais"), 0o644)

	instrDir := filepath.Join(ghDir, "instructions")
	os.MkdirAll(instrDir, 0o755)
	os.WriteFile(filepath.Join(instrDir, "nextjs-aksel.instructions.md"), []byte("# Aksel"), 0o644)

	// State: agent installed, instruction explicitly ignored (empty hash).
	scope := &InstallScope{Name: "user", RootDir: target, StateFile: ".nav-pilot-state.json", SupportedTypes: []string{"agent", "skill", "instruction"}}
	state := &StateFile{
		Collection: CollectionAll,
		Scope:      "user",
		Version:    "dev",
		Files: []InstalledFile{
			{Path: "agents/nais.agent.md", Hash: "abc"},
			{Path: ".github/instructions/nextjs-aksel.instructions.md", Hash: "", Status: fileStatusIgnored},
		},
	}
	writeScopedState(scope, state)

	newItems := detectNewItems(scope, source)
	if len(newItems) != 0 {
		t.Errorf("expected no new items (instruction is ignored), got %v", newItems)
	}
}

// TestRun_IgnoreCommand tests CLI dispatch for "ignore".
func TestRun_IgnoreCommand(t *testing.T) {
	// Missing args should return a helpful error.
	err := run([]string{"ignore"})
	if err == nil || !strings.Contains(err.Error(), "ignore requires") {
		t.Errorf("expected 'ignore requires' error, got %v", err)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func tempUserScope(t *testing.T) (*InstallScope, string) {
	t.Helper()
	target := t.TempDir()
	scope := &InstallScope{
		Name:           "user",
		RootDir:        target,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	return scope, target
}

func writeAllState(t *testing.T, scope *InstallScope, files []InstalledFile) {
	t.Helper()
	state := &StateFile{
		Collection: CollectionAll,
		Scope:      "user",
		Version:    "dev",
		Files:      files,
	}
	if err := writeScopedState(scope, state); err != nil {
		t.Fatalf("writeAllState: %v", err)
	}
}

func readTestState(t *testing.T, dir string) *StateFile {
	t.Helper()
	scope := &InstallScope{
		Name:           "user",
		RootDir:        dir,
		StateFile:      ".nav-pilot-state.json",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}
	state, err := readScopedState(scope)
	if err != nil {
		t.Fatalf("readScopedState: %v", err)
	}
	if state == nil {
		t.Fatal("state is nil")
	}
	return state
}
