package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportBrokenRtkHook(t *testing.T) {
	tests := []struct {
		name        string
		installRtk  bool
		installHook bool
		wantBroken  bool
	}{
		{name: "nothing installed"},
		{name: "rtk without hook", installRtk: true},
		{name: "hook with rtk", installRtk: true, installHook: true},
		{name: "hook without rtk", installHook: true, wantBroken: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			bin := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("PATH", bin)

			if tt.installRtk {
				path := filepath.Join(bin, "rtk")
				if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if tt.installHook {
				path := filepath.Join(home, ".copilot", "hooks", "rtk-rewrite.json")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			var broken bool
			out := stripANSI(captureStdoutFor(t, func() {
				broken = reportBrokenRtkHook()
			}))
			if broken != tt.wantBroken {
				t.Fatalf("reportBrokenRtkHook = %v, want %v\n%s", broken, tt.wantBroken, out)
			}
			if tt.wantBroken {
				for _, want := range []string{
					"hook installed, but binary not found on PATH",
					"denies every matching Copilot tool call",
					"rm -- ~/.copilot/hooks/rtk-rewrite.json",
				} {
					if !strings.Contains(out, want) {
						t.Errorf("output does not contain %q:\n%s", want, out)
					}
				}
			} else if out != "" {
				t.Errorf("healthy state produced output:\n%s", out)
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

		if err := removeUnusableRtkHook("copilot"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(hook); !os.IsNotExist(err) {
			t.Fatalf("hook still exists after cleanup: %v", err)
		}
	})

	t.Run("keeps hook when rtk is available", func(t *testing.T) {
		home := t.TempDir()
		bin := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("PATH", bin)
		rtk := filepath.Join(bin, "rtk")
		if err := os.WriteFile(rtk, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
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
