package source

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// --- ValidateName ---

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"nav-pilot", false},
		{"auth-agent", false},
		{"", true},
		{"dot..dot", true},
		{"a/b", true},
		{"a\\b", true},
	}
	for _, tt := range tests {
		err := ValidateName(tt.name)
		if tt.wantErr && err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", tt.name)
		}
		if !tt.wantErr && err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", tt.name, err)
		}
	}
}

// --- ValidateManifest ---

func TestValidateManifest(t *testing.T) {
	m := &Manifest{
		Name:   "all",
		Agents: []string{"nav-pilot", "auth"},
		Skills: []string{"kafka"},
	}
	if err := ValidateManifest(m); err != nil {
		t.Errorf("ValidateManifest(valid) = %v, want nil", err)
	}

	empty := &Manifest{}
	if err := ValidateManifest(empty); err == nil {
		t.Error("ValidateManifest(empty name) = nil, want error")
	}

	dup := &Manifest{
		Name:   "all",
		Agents: []string{"nav-pilot", "nav-pilot"},
	}
	if err := ValidateManifest(dup); err == nil {
		t.Error("ValidateManifest(duplicate) = nil, want error")
	}

	bad := &Manifest{
		Name:   "all",
		Agents: []string{""},
	}
	if err := ValidateManifest(bad); err == nil {
		t.Error("ValidateManifest(invalid name) = nil, want error")
	}
}

// --- LoadManifest ---

func TestLoadManifest_Success(t *testing.T) {
	tmp := t.TempDir()
	collDir := filepath.Join(tmp, "collections", "all")
	if err := os.MkdirAll(collDir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := Manifest{Name: "all", Agents: []string{"nav-pilot"}}
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(collDir, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadManifest(tmp, "all")
	if err != nil {
		t.Fatalf("LoadManifest = %v", err)
	}
	if got.Name != "all" {
		t.Errorf("Manifest.Name = %q, want all", got.Name)
	}
}

func TestLoadManifest_NotFound(t *testing.T) {
	tmp := t.TempDir()
	_, err := LoadManifest(tmp, "nonexistent")
	if err == nil {
		t.Error("LoadManifest(nonexistent) = nil, want error")
	}
}

// --- ListCollectionDirs ---

func TestListCollectionDirs_WithCollections(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"all", "kotlin-backend"} {
		dir := filepath.Join(tmp, "collections", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		m := Manifest{Name: name}
		data, _ := json.Marshal(m)
		os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644)
	}
	// A dir without manifest.json should be excluded
	os.MkdirAll(filepath.Join(tmp, "collections", "noManifest"), 0o755)

	names, err := ListCollectionDirs(tmp)
	if err != nil {
		t.Fatalf("ListCollectionDirs = %v", err)
	}
	if len(names) != 2 {
		t.Errorf("len = %d, want 2: %v", len(names), names)
	}
}

func TestListCollectionDirs_NoDir(t *testing.T) {
	tmp := t.TempDir()
	_, err := ListCollectionDirs(tmp)
	if err == nil {
		t.Error("ListCollectionDirs(no dir) = nil, want error")
	}
}

// --- CollectAllItems ---

func TestCollectAllItems(t *testing.T) {
	tmp := setupSourceDir(t)
	m, err := CollectAllItems(tmp)
	if err != nil {
		t.Fatalf("CollectAllItems = %v", err)
	}
	if m.Name != "(all)" {
		t.Errorf("Name = %q, want (all)", m.Name)
	}
	if len(m.Agents) == 0 {
		t.Error("expected at least one agent")
	}
}

// --- CountDirFiles ---

func TestCountDirFiles(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	n := CountDirFiles(tmp)
	if n != 2 {
		t.Errorf("CountDirFiles = %d, want 2", n)
	}
}

// --- CopyArtifact ---

func TestCopyArtifact_File(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.md")
	dst := filepath.Join(tmp, "dst.md")
	if err := os.WriteFile(src, []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyArtifact(src, dst, tmp, false); err != nil {
		t.Fatalf("CopyArtifact(file) = %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("dst not created: %v", err)
	}
}

func TestCopyArtifact_Dir(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "srcdir")
	dst := filepath.Join(tmp, "dstdir")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyArtifact(src, dst, tmp, true); err != nil {
		t.Fatalf("CopyArtifact(dir) = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not copied: %v", err)
	}
}

// --- ComparableArtifactHash ---

func TestComparableArtifactHash_File(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.md")
	if err := os.WriteFile(path, []byte("# hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := ComparableArtifactHash(path, false)
	if err != nil {
		t.Fatalf("ComparableArtifactHash = %v", err)
	}
	if len(h) != 16 {
		t.Errorf("hash len = %d, want 16", len(h))
	}
}

func TestComparableArtifactHash_Dir(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.md"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := ComparableArtifactHash(dir, true)
	if err != nil {
		t.Fatalf("ComparableArtifactHash(dir) = %v", err)
	}
	if len(h) != 16 {
		t.Errorf("hash len = %d, want 16", len(h))
	}
}

// --- RelPathForName ---

func TestRelPathForName_Repo(t *testing.T) {
	scope := domain.ScopeRepo("/repo")
	rel := KindAgent.RelPathForName(scope, "nav-pilot")
	// repo scope: .github/agents/nav-pilot.agent.md
	if rel == "" {
		t.Error("RelPathForName returned empty string")
	}
}

func TestRelPathForName_User(t *testing.T) {
	scope := &domain.InstallScope{Name: "user", RootDir: "/home/user/.copilot"}
	rel := KindAgent.RelPathForName(scope, "nav-pilot")
	// user scope: agents/nav-pilot.agent.md
	if rel == "" {
		t.Error("RelPathForName returned empty string")
	}
}

// setupSourceDir creates a minimal source dir for testing.
func setupSourceDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	agentsDir := filepath.Join(tmp, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "nav-pilot.agent.md"), []byte(`---
name: nav-pilot
description: Test agent
---
Body.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return tmp
}
