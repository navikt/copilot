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

// A repo without .cplt.toml gets pointed at `cplt init`, which only previews.
// One with the file does not.
func TestReportCpltProjectConfigSuggestsInit(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	out := captureStdout(func() {
		if reportCpltProjectConfig("", nil) {
			t.Error("a repo without .cplt.toml is not a problem")
		}
	})
	if !strings.Contains(out, "cplt init") {
		t.Errorf("no .cplt.toml, but no cplt init hint:\n%s", out)
	}

	if err := os.WriteFile(filepath.Join(dir, ".cplt.toml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	out = captureStdout(func() { reportCpltProjectConfig("", nil) })
	if strings.Contains(out, "cplt init") {
		t.Errorf(".cplt.toml exists, but still suggests cplt init:\n%s", out)
	}
}
