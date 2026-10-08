package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

func TestOpenCode1InstallHint(t *testing.T) {
	cases := []struct {
		name, target string
		owner        domain.PkgManager
		want         string
	}{
		{"npm", "lib/node_modules/@opencode/cli/bin/opencode", domain.PkgNone,
			"npm uninstall -g @opencode/cli && npm i -g opencode-ai@" + OpenCodeInstallVersion},
		{"brew opencode-v2", "Cellar/opencode-v2/2.0.24/bin/opencode", domain.PkgBrew,
			"brew uninstall opencode-v2 && brew install anomalyco/tap/opencode"},
		{"brew opencode", "Cellar/opencode/2.0.24/bin/opencode", domain.PkgBrew,
			"brew uninstall opencode && brew install anomalyco/tap/opencode"},
		{"script", "home/.opencode/bin/opencode", domain.PkgNone, OpenCodeScriptInstall},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, tc.target)
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
			domain.PkgOwner = func(string) domain.PkgManager { return tc.owner }
			t.Cleanup(func() { domain.PkgOwner = orig })
			if got := OpenCode1InstallHint(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
