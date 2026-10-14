package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

func TestOpenCodeInstallHint(t *testing.T) {
	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origInteractive })

	for _, tt := range []struct {
		name string
		bins []string
		want string
	}{
		{"brew first", []string{"npm", "pnpm", "brew"}, opencodeBrewInstall},
		{"pnpm before npm", []string{"npm", "pnpm"}, "pnpm add -g opencode-ai@" + providerpkg.OpenCodeInstallVersion},
		{"npm", []string{"npm"}, "npm i -g opencode-ai@" + providerpkg.OpenCodeInstallVersion},
		{"no package manager", nil, opencodeScriptInstall},
	} {
		t.Run(tt.name, func(t *testing.T) {
			isolatedConfig(t)
			bin := t.TempDir()
			for _, name := range append(tt.bins, "cplt") {
				if err := testhome.WriteExec(filepath.Join(bin, name), "#!/bin/sh\nexit 1\n"); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			if got := opencodeInstallCommand(); got != tt.want {
				t.Fatalf("opencodeInstallCommand() = %q, want %q", got, tt.want)
			}
			var launchErr error
			_, stderr := captureRun(t, func() {
				launchErr = offerLaunch(ResolvedConfig{Client: "opencode", AutoLaunch: true}, false)
			})
			if launchErr != nil {
				t.Fatal(launchErr)
			}
			if !strings.Contains(stderr, "opencode is not installed, so nothing was launched. Install it: "+tt.want) {
				t.Errorf("interactive hint = %q, want %q", stderr, tt.want)
			}
		})
	}
}

func TestOpenCodeInstallHintSkipsNonExecutableBrew(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "brew"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := testhome.WriteExec(filepath.Join(bin, "npm"), "#!/bin/sh\nexit 1\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	want := "npm i -g opencode-ai@" + providerpkg.OpenCodeInstallVersion
	if got := opencodeInstallCommand(); got != want {
		t.Fatalf("opencodeInstallCommand() = %q, want %q", got, want)
	}
	if got := clientInstallCommand("cplt"); got != cpltInstallHint {
		t.Fatalf("clientInstallCommand(cplt) = %q, want %q", got, cpltInstallHint)
	}
}
