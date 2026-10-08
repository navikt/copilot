package cli

import (
	"errors"
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
