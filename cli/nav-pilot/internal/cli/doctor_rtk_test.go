package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

func TestReportRtkLeftovers(t *testing.T) {
	hookRm := "rm -- ~/.copilot/hooks/rtk-rewrite.json"
	pluginRm := "rm -- ~/.config/opencode/plugins/rtk.ts"
	tests := []struct {
		name          string
		installRtk    bool
		installHook   bool
		installPlugin bool
		listPlugin    bool
		want          []string // empty: no output, nothing found
		dontWant      []string
	}{
		{name: "nothing installed"},
		{name: "rtk without hook or plugin", installRtk: true},
		{name: "hook with rtk", installRtk: true, installHook: true,
			want:     []string{"Copilot hook found", "no longer sets up rtk", "If you did not install rtk yourself", hookRm},
			dontWant: []string{"binary not found"}},
		{name: "hook without rtk", installHook: true,
			want: []string{"hook installed, but binary not found on PATH", "denies every matching Copilot tool call", hookRm}},
		{name: "opencode plugin", installPlugin: true,
			want:     []string{"opencode plugin found", "no longer sets up rtk", pluginRm},
			dontWant: []string{"opencode.json"}},
		{name: "opencode plugin listed in opencode.json", installPlugin: true, listPlugin: true,
			want: []string{pluginRm, `delete its entry from "plugin" in ~/.config/opencode/opencode.json`}},
		{name: "opencode.json lists a plugin that is gone", listPlugin: true,
			want:     []string{"~/.config/opencode/opencode.json still lists plugins/rtk.ts, which is gone", `Delete its entry from "plugin"`},
			dontWant: []string{"rm --"}},
	}

	write := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			bin := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			t.Setenv("PATH", bin)

			if tt.installRtk {
				if err := testhome.WriteExec(filepath.Join(bin, "rtk"), "#!/bin/sh\nexit 0\n"); err != nil {
					t.Fatal(err)
				}
			}
			var planted []string
			if tt.installHook {
				p := filepath.Join(home, ".copilot", "hooks", "rtk-rewrite.json")
				write(t, p, "{}\n")
				planted = append(planted, p)
			}
			ocDir := filepath.Join(home, ".config", "opencode")
			if tt.installPlugin {
				p := filepath.Join(ocDir, "plugins", "rtk.ts")
				write(t, p, "export default {}\n")
				planted = append(planted, p)
			}
			if tt.listPlugin {
				p := filepath.Join(ocDir, "opencode.json")
				write(t, p, `{"plugin": ["`+filepath.ToSlash(filepath.Join(ocDir, "plugins", "rtk.ts"))+`"]}`)
				planted = append(planted, p)
			}

			var found bool
			out := stripANSI(captureStdoutFor(t, func() {
				found = reportRtkLeftovers()
			}))
			if found != (len(tt.want) > 0) {
				t.Fatalf("reportRtkLeftovers = %v, want %v\n%s", found, len(tt.want) > 0, out)
			}
			if len(tt.want) == 0 && out != "" {
				t.Errorf("nothing to report, but got output:\n%s", out)
			}
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("output does not contain %q:\n%s", want, out)
				}
			}
			for _, not := range tt.dontWant {
				if strings.Contains(out, not) {
					t.Errorf("output contains %q:\n%s", not, out)
				}
			}
			// doctor only reports: it deletes nothing.
			for _, p := range planted {
				if _, err := os.Stat(p); err != nil {
					t.Errorf("doctor removed %s: %v", p, err)
				}
			}
		})
	}
}

func TestRemoveUnusableRtkHook(t *testing.T) {
	t.Run("removes Copilot hook when rtk is unavailable", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("PATH", t.TempDir())
		hook := filepath.Join(home, ".copilot", "hooks", "rtk-rewrite.json")
		if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(hook, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		var err error
		errOut := captureStderrFor(t, func() { err = removeUnusableRtkHook("copilot") })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(hook); !os.IsNotExist(err) {
			t.Fatalf("hook still exists after cleanup: %v", err)
		}
		// A trimmed PATH can hide an rtk the user installed: say what went.
		want := "Removed ~/.copilot/hooks/rtk-rewrite.json: rtk is not on PATH and the hook would deny every Copilot tool call."
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr = %q, want it to contain %q", errOut, want)
		}
		// Nothing left to remove: nothing to say.
		if again := captureStderrFor(t, func() { err = removeUnusableRtkHook("copilot") }); err != nil || again != "" {
			t.Errorf("second run: err %v, stderr %q; want neither", err, again)
		}
	})

	t.Run("keeps hook when rtk is available", func(t *testing.T) {
		home := t.TempDir()
		bin := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("PATH", bin)
		rtk := filepath.Join(bin, "rtk")
		if err := testhome.WriteExec(rtk, "#!/bin/sh\nexit 0\n"); err != nil {
			t.Fatal(err)
		}
		hook := filepath.Join(home, ".copilot", "hooks", "rtk-rewrite.json")
		if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(hook, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := removeUnusableRtkHook("copilot"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(hook); err != nil {
			t.Fatalf("valid hook was removed: %v", err)
		}
	})

	t.Run("does not touch another client", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("PATH", t.TempDir())
		hook := filepath.Join(home, ".copilot", "hooks", "rtk-rewrite.json")
		if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(hook, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := removeUnusableRtkHook("opencode"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(hook); err != nil {
			t.Fatalf("Copilot hook was removed during OpenCode setup: %v", err)
		}
	})
}
