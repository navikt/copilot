package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
)

// doctor reports the copilot binary, which cplt needs on PATH (#1051). Missing
// it fails the check only for a user whose client is copilot.
func TestReportCopilotCLI(t *testing.T) {
	for _, c := range []struct {
		name, config string
		onPath, ok   bool
		want, reject string
	}{
		{"found", "", true, true, "Binary found", "Solution"},
		{"missing, default client", "", false, false, providerpkg.CopilotInstallCommand, "optional"},
		{"missing, client copilot", "client = \"copilot\"\n", false, false, providerpkg.CopilotInstallCommand, "optional"},
		{"missing, client opencode", "client = \"opencode\"\n", false, true, "(optional)", "Solution"},
		{"missing, client pi", "client = \"pi\"\n", false, true, "(optional)", "Solution"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfg := isolatedConfig(t)
			if c.config != "" {
				mustWrite(t, cfg, c.config)
			}
			bin := t.TempDir()
			if c.onPath {
				if err := os.WriteFile(filepath.Join(bin, "copilot"), []byte("#!/bin/sh\n"), 0o755); err != nil {
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
