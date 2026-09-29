package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

// doctor reports the copilot binary, which cplt needs on PATH (#1051). Missing
// it fails the check only for a user whose client is copilot.
func TestReportCopilotCLI(t *testing.T) {
	for _, c := range []struct {
		name, config string
		script       string // the fake copilot on PATH; "" for none
		ok           bool
		want, reject string
	}{
		{"found", "", "#!/bin/sh\necho 'GitHub Copilot CLI 1.0.89'\n", true, "Binary found", "Solution"},
		// cplt installed under the name copilot is not the Copilot CLI.
		{"alias to cplt", "", "#!/bin/sh\necho 'cplt 2026.09.24-192459-38642b4'\n", false, providerpkg.CopilotInstallCommand, "Binary found"},
		{"missing, default client", "", "", false, providerpkg.CopilotInstallCommand, "optional"},
		{"missing, client copilot", "client = \"copilot\"\n", "", false, providerpkg.CopilotInstallCommand, "optional"},
		{"missing, client opencode", "client = \"opencode\"\n", "", true, "(optional)", "Solution"},
		{"missing, client pi", "client = \"pi\"\n", "", true, "(optional)", "Solution"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg := isolatedConfig(t)
			if c.config != "" {
				mustWrite(t, cfg, c.config)
			}
			bin := t.TempDir()
			if c.script != "" {
				if err := testhome.WriteExec(filepath.Join(bin, "copilot"), c.script); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin)
			var ok bool
			out := captureStdoutFor(t, func() { ok = reportCopilotCLI() })
			if ok != c.ok || !strings.Contains(out, c.want) || strings.Contains(out, c.reject) {
				t.Errorf("ok = %v (want %v)\n%s", ok, c.ok, out)
			}
		})
	}
}

// Doctor reads .cplt.toml presence from `cplt config show`, which finds it at
// the repo root from any subdirectory. It suggests `cplt init` only inside a
// repo that has none.
func TestReportCpltProjectConfig(t *testing.T) {
	const withRepoConfig = "[cplt] ── Repo Config (.cplt.toml) ────\n[cplt]  Path:   /repo/.cplt.toml\n"
	for _, c := range []struct {
		name, cfgOut string
		gitRepo      bool
		want, reject string
	}{
		{"subdirectory of a repo with .cplt.toml", withRepoConfig, true, "rules are trusted", "cplt init"},
		{"repo without .cplt.toml", "", true, "cplt init", "trusted"},
		{"not a repo", "", false, "Not in a git repository", "cplt init"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if c.gitRepo {
				if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
				dir = filepath.Join(dir, "sub")
				if err := os.Mkdir(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(dir)

			out := captureStdout(func() {
				if reportCpltProjectConfig(c.cfgOut, nil) {
					t.Error("reported a problem")
				}
			})
			if !strings.Contains(out, c.want) || strings.Contains(out, c.reject) {
				t.Errorf("want %q and no %q in:\n%s", c.want, c.reject, out)
			}
		})
	}
}
