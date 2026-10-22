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
		{"unconfigured pnpm alone", []string{"pnpm"}, false, ""},
		{"npm", []string{"npm"}, false, "npm i -g @opencode/cli@" + providerpkg.OpenCode2InstallVersion},
		{"no package manager", nil, false, ""},
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
			want := "opencode is not installed, so nothing was launched. Install it: " + tt.want
			if tt.want == "" {
				want = "opencode is not installed, so nothing was launched. Install Homebrew (https://brew.sh) or Node.js (https://nodejs.org), then run " + opencodeBrewInstall + " or " + opencodeNpmInstall + "."
			}
			if !strings.Contains(stderr, want) {
				t.Errorf("interactive hint = %q, want %q", stderr, want)
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

// TestInstallOpenCodeFailures: an npm that fails, and one that succeeds
// without putting opencode on PATH, are both errors.
func TestInstallOpenCodeFailures(t *testing.T) {
	isolatedConfig(t)
	for _, tc := range []struct{ name, script, want string }{
		{"npm fails", "#!/bin/sh\nexit 1\n", "exit status 1"},
		{"no opencode afterwards", "#!/bin/sh\nexit 0\n", "still not on PATH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := t.TempDir()
			if err := testhome.WriteExec(filepath.Join(bin, "npm"), tc.script); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			if err := installOpenCode(opencodeNpmInstall); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

// TestOpenCodeInstallCommandOtherOS: opencode 2 runs only on macOS and Linux,
// so elsewhere nothing is installed, even with npm on PATH.
func TestOpenCodeInstallCommandOtherOS(t *testing.T) {
	bin := t.TempDir()
	if err := testhome.WriteExec(filepath.Join(bin, "npm"), "#!/bin/sh\nexit 1\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	prev := installGOOS
	t.Cleanup(func() { installGOOS = prev })
	installGOOS = "windows"
	if got := opencodeInstallCommand(); got != "" {
		t.Errorf("windows: %q, want \"\"", got)
	}
	installGOOS = "linux"
	if got := opencodeInstallCommand(); got != opencodeNpmInstall {
		t.Errorf("linux: %q, want %q", got, opencodeNpmInstall)
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
	if want := "install Homebrew (https://brew.sh) or Node.js (https://nodejs.org), run " + opencodeBrewInstall + " or " + opencodeNpmInstall + ", then nav-pilot config set client opencode"; !strings.Contains(out, want) {
		t.Errorf("output %q lacks %q", out, want)
	}
	for _, want := range []string{"https://brew.sh", "https://nodejs.org", opencodeBrewInstall, opencodeNpmInstall} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q lacks %q", out, want)
		}
	}
	if strings.Contains(out, "opencode.ai/install") || strings.Contains(out, "opencode-ai") {
		t.Errorf("output offers opencode 1: %q", out)
	}
}
