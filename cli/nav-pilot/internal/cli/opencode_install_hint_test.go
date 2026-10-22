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
		name     string
		bins     []string
		pnpmHome bool
		want     string
	}{
		{"brew first", []string{"npm", "pnpm", "brew"}, true, opencodeBrewInstall},
		{"pnpm before npm", []string{"npm", "pnpm"}, true, "pnpm add -g @opencode/cli@" + providerpkg.OpenCode2InstallVersion},
		{"unconfigured pnpm with npm", []string{"npm", "pnpm"}, false, "npm i -g @opencode/cli@" + providerpkg.OpenCode2InstallVersion},
		{"unconfigured pnpm alone", []string{"pnpm"}, false, opencodeNoInstaller},
		{"npm", []string{"npm"}, false, "npm i -g @opencode/cli@" + providerpkg.OpenCode2InstallVersion},
		{"no package manager", nil, false, opencodeNoInstaller},
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
			t.Setenv("PNPM_HOME", "")
			if tt.pnpmHome {
				t.Setenv("PNPM_HOME", bin)
			}
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

func TestOpenCodeInstallHintRejectsInvalidPnpmHome(t *testing.T) {
	bin := t.TempDir()
	for _, name := range []string{"pnpm", "npm"} {
		if err := testhome.WriteExec(filepath.Join(bin, name), "#!/bin/sh\nexit 1\n"); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	for _, home := range []string{t.TempDir(), filepath.Join(bin, "missing"), filepath.Join(bin, "npm"), "."} {
		t.Setenv("PNPM_HOME", home)
		want := "npm i -g @opencode/cli@" + providerpkg.OpenCode2InstallVersion
		if got := opencodeInstallCommand(); got != want {
			t.Errorf("PNPM_HOME=%q: got %q, want %q", home, got, want)
		}
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
	want := "npm i -g @opencode/cli@" + providerpkg.OpenCode2InstallVersion
	if got := opencodeInstallCommand(); got != want {
		t.Fatalf("opencodeInstallCommand() = %q, want %q", got, want)
	}
	if got := clientInstallCommand("cplt"); got != cpltInstallHint {
		t.Fatalf("clientInstallCommand(cplt) = %q, want %q", got, cpltInstallHint)
	}
}

// TestInstallOpenCodeRunsPackageManager: without Homebrew, the wizard's
// install really runs npm with the pinned opencode 2 package, and succeeds
// once opencode is on PATH.
func TestInstallOpenCodeRunsPackageManager(t *testing.T) {
	isolatedConfig(t)
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "npm.log")
	opencode := filepath.Join(bin, "opencode")
	npm := "#!/bin/sh\necho \"$@\" > " + log + "\nprintf '#!/bin/sh\\necho opencode v2.1.2\\n' > " + opencode + "\n/bin/chmod +x " + opencode + "\n"
	if err := testhome.WriteExec(filepath.Join(bin, "npm"), npm); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("PNPM_HOME", "")
	cmd := opencodeInstallCommand()
	if cmd != opencodeNpmInstall {
		t.Fatalf("command = %q, want %q", cmd, opencodeNpmInstall)
	}
	if err := installOpenCode(cmd); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(log)
	if want := "i -g @opencode/cli@" + providerpkg.OpenCode2InstallVersion + "\n"; string(got) != want {
		t.Errorf("npm args = %q, want %q", got, want)
	}
}

// TestOpenCodeForSetupWithoutInstaller: with neither Homebrew nor npm the
// wizard installs nothing, never offers opencode 1's install script, and says
// how to get opencode 2.
func TestOpenCodeForSetupWithoutInstaller(t *testing.T) {
	isolatedConfig(t)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("PNPM_HOME", "")
	var client string
	var err error
	stdout, stderr := captureRun(t, func() { client, err = opencodeForSetup() })
	if err != nil || client != "copilot" {
		t.Fatalf("opencodeForSetup() = %q, %v; want copilot", client, err)
	}
	out := stdout + stderr
	for _, want := range []string{"https://brew.sh", "https://nodejs.org", opencodeBrewInstall, opencodeNpmInstall} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}
	if strings.Contains(out, "opencode.ai/install") || strings.Contains(out, "opencode-ai") {
		t.Errorf("output offers opencode 1: %q", out)
	}
}
