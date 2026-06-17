package artifacts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSyncConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".github"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, SyncConfigPath), []byte(`{
		"overrides": [
			".github/agents/nais.agent.md",
			".github/instructions/security.instructions.md"
		]
	}`), 0o644)

	cfg, err := ReadSyncConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Overrides) != 2 {
		t.Fatalf("overrides count = %d, want 2", len(cfg.Overrides))
	}
	if cfg.Overrides[0] != ".github/agents/nais.agent.md" {
		t.Errorf("overrides[0] = %q", cfg.Overrides[0])
	}
}

func TestReadSyncConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := ReadSyncConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		t.Error("expected nil config for missing file")
	}
}

func TestReadSyncConfig_EmptyOverrides(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".github"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, SyncConfigPath), []byte(`{"overrides": []}`), 0o644)

	cfg, err := ReadSyncConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Overrides) != 0 {
		t.Errorf("overrides count = %d, want 0", len(cfg.Overrides))
	}
}

func TestReadSyncConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".github"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, SyncConfigPath), []byte(`{invalid`), 0o644)

	if _, err := ReadSyncConfig(dir); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestOverrideSet_Nil(t *testing.T) {
	m := OverrideSet(nil)
	if m != nil {
		t.Error("expected nil for nil config")
	}
}

func TestOverrideSet_WithEntries(t *testing.T) {
	cfg := &SyncConfig{
		Overrides: []string{".github/agents/a.agent.md", ".github/skills/s/"},
	}
	m := OverrideSet(cfg)
	if !m[".github/agents/a.agent.md"] {
		t.Error("expected agent in set")
	}
	if !m[".github/skills/s/"] {
		t.Error("expected skill dir (with slash) in set")
	}
	if !m[".github/skills/s"] {
		t.Error("expected skill dir (without slash) in set — canonicalization should add both")
	}
	if m[".github/agents/other.agent.md"] {
		t.Error("unexpected entry in set")
	}
}

func TestOverrideSet_CanonicalizesPath(t *testing.T) {
	cfg := &SyncConfig{
		Overrides: []string{"./.github/agents/a.agent.md"},
	}
	m := OverrideSet(cfg)
	if !m[".github/agents/a.agent.md"] {
		t.Error("expected canonicalized path in set (without ./)")
	}
}
