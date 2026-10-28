package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// doctor shows the opencode 2 launch refusal (Linux, cplt too old), not green.
func TestDoctorReportsOpenCode2Refusal(t *testing.T) {
	prevMajor, prevLaunch := openCodeMajorCheck, openCodeLaunchCheck
	t.Cleanup(func() { openCodeMajorCheck, openCodeLaunchCheck = prevMajor, prevLaunch })
	openCodeMajorCheck = func() error { return nil }
	openCodeLaunchCheck = func() error { return errors.New("opencode 2 runs under cplt on macOS and Linux only") }
	var ok bool
	out := captureStdout(func() { ok = reportOpenCodeVersion() })
	if ok || !strings.Contains(out, "✗ opencode 2 runs under cplt on macOS and Linux only") {
		t.Errorf("ok = %v, out:\n%s", ok, out)
	}
}

// doctor reads the OpenCode context from the dir the sync writes to
// (~/.config/opencode, also on macOS), not os.UserConfigDir.
func TestDoctorOpenCodeContextUsesSyncDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".nav-pilot-state.json"), []byte(`{"scope":"opencode"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(func() { reportOpenCodeContext() })
	if !strings.Contains(out, "Context securely materialized") {
		t.Errorf("out:\n%s", out)
	}
}
