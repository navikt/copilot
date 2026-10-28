package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

func TestOpenCode2InstallHint(t *testing.T) {
	npm := "npm i -g @opencode/cli@" + OpenCode2InstallVersion
	for _, tc := range []struct {
		name, target string
		owner        domain.PkgManager
		want         string
	}{
		{"npm opencode-ai", "lib/node_modules/opencode-ai/bin/opencode", domain.PkgNone, "npm uninstall -g opencode-ai && " + npm},
		{"script", "home/.opencode/bin/opencode", domain.PkgNone, "rm ~/.opencode/bin/opencode && " + npm},
		{"brew opencode", "Cellar/opencode/1.19.4/bin/opencode", domain.PkgBrew, "brew uninstall opencode && brew install anomalyco/tap/opencode-v2"},
		{"other", "opt/opencode", domain.PkgNone, npm},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeOpenCodeOnPath(t, tc.target, tc.owner)
			if got := OpenCode2InstallHint(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// fakeOpenCodeOnPath puts a bin/opencode symlink to target (relative to a
// temp root) alone on PATH, with owner as its package manager.
func fakeOpenCodeOnPath(t *testing.T, rel string, owner domain.PkgManager) {
	t.Helper()
	root := t.TempDir()
	target := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(bin, "opencode")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	orig := domain.PkgOwner
	domain.PkgOwner = func(string) domain.PkgManager { return owner }
	t.Cleanup(func() { domain.PkgOwner = orig })
}
