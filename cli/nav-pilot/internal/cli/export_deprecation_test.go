package cli

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureRunStderr runs run(args) with os.Stderr redirected and returns the
// captured stderr plus the run error.
func captureRunStderr(t *testing.T, args []string) (string, error) {
	t.Helper()
	origStderr := os.Stderr
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe: %v", pipeErr)
	}
	os.Stderr = w

	err := run(args)

	w.Close()
	os.Stderr = origStderr
	var buf strings.Builder
	io.Copy(&buf, r)
	return buf.String(), err
}

func TestExportOpenCodeUserScope_PrintsDeprecationWarning(t *testing.T) {
	origExport := cmdExport
	t.Cleanup(func() { cmdExport = origExport })
	var gotFormat string
	cmdExport = func(format string, _ *InstallScope, _, _ string, _, _ bool, _ bool) error {
		gotFormat = format
		return nil
	}

	stderr, err := captureRunStderr(t, []string{"export", "opencode", "--user"})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if gotFormat != "opencode" {
		t.Fatalf("cmdExport called with format %q, want opencode", gotFormat)
	}
	if !strings.Contains(stderr, "deprecated") {
		t.Errorf("expected deprecation warning on stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "nav-pilot sync") {
		t.Errorf("expected warning to point users to sync, got: %q", stderr)
	}
}

func TestExportOpenCodeRepoScope_NoDeprecationWarning(t *testing.T) {
	origExport := cmdExport
	t.Cleanup(func() { cmdExport = origExport })
	cmdExport = func(_ string, _ *InstallScope, _, _ string, _, _ bool, _ bool) error {
		return nil
	}

	stderr, err := captureRunStderr(t, []string{"export", "opencode"})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if strings.Contains(stderr, "deprecated") {
		t.Errorf("repo-scope export should not warn, got: %q", stderr)
	}
}
