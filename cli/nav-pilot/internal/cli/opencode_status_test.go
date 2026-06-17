package cli

import (
	"os"
	"path/filepath"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

func TestCmdStatusAutoIncludesOpenCode(t *testing.T) {
	old := providerpkg.NavContextDirOverride
	ocDir := t.TempDir()
	providerpkg.NavContextDirOverride = ocDir
	defer func() { providerpkg.NavContextDirOverride = old }()

	state := &StateFile{
		Collection: openCodeCollection,
		Version:    "2026.06.16-120000",
		Scope:      openCodeScopeName,
		SourceSHA:  "abc",
		Files: []InstalledFile{
			{Path: "AGENTS.md", Hash: "deadbeef"},
		},
	}
	if err := writeOpenCodeState(ocDir, state); err != nil {
		t.Fatalf("writeOpenCodeState: %v", err)
	}

	if err := os.WriteFile(filepath.Join(ocDir, "AGENTS.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("writing AGENTS.md: %v", err)
	}

	if err := cmdStatusAuto(t.TempDir(), false); err != nil {
		t.Errorf("cmdStatusAuto error: %v", err)
	}
}
