package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubClient puts a fake client binary named name on an otherwise empty PATH.
// The stub records that it ran by creating a marker file, which is how these
// tests tell "launched" from "not launched".
func stubClient(t *testing.T, name string) (markerPath string) {
	t.Helper()
	binDir := t.TempDir()
	markerPath = filepath.Join(t.TempDir(), "launched")
	// The marker path is baked into the script rather than passed through the
	// environment: the copilot launch hands the client a curated env, and this
	// test must not depend on what that env keeps.
	// ": >" is a shell builtin, so the stub works under the curated launch env
	// even when that env carries no PATH.
	script := "#!/bin/sh\n: > \"" + markerPath + "\"\n"
	if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// PATH holds only the stub dir, so no real client (cplt included) can be
	// found or started by accident.
	t.Setenv("PATH", binDir)
	return markerPath
}

// captureRun runs fn with stdout and stderr captured and returns both.
func captureRun(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW
	outDone := make(chan string, 1)
	errDone := make(chan string, 1)
	go func() { b, _ := io.ReadAll(outR); outDone <- string(b) }()
	go func() { b, _ := io.ReadAll(errR); errDone <- string(b) }()
	fn()
	outW.Close()
	errW.Close()
	got := [2]string{<-outDone, <-errDone}
	os.Stdout, os.Stderr = origOut, origErr
	return got[0], got[1]
}

// The --sync path must honour auto_launch = false like every other launch path
// (#472): print the command to run, and start nothing.
func TestSyncFlagHonoursAutoLaunchFalse(t *testing.T) {
	cfgPath := isolatedConfig(t)
	if err := os.WriteFile(cfgPath, []byte("version = 1\nclient = \"pi\"\nauto_launch = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := stubClient(t, "pi")

	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origInteractive })

	var runErr error
	stdout, _ := captureRun(t, func() { runErr = run([]string{"--sync"}) })
	if runErr != nil {
		t.Fatalf("run(--sync) = %v", runErr)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatalf("run(--sync) launched the client despite auto_launch = false")
	}
	if !strings.Contains(stdout, "auto_launch = false") {
		t.Errorf("run(--sync) with auto_launch = false should name the setting and the command to run, got:\n%s", stdout)
	}
}

// The --sync path must warn about an unsandboxed copilot launch (#472): the
// plain copilot CLI without cplt gets the missing-sandbox warning decideLaunch
// produces on every other path, and then launches.
func TestSyncFlagWarnsUnsandboxedLaunch(t *testing.T) {
	cfgPath := isolatedConfig(t)
	if err := os.WriteFile(cfgPath, []byte("version = 1\nclient = \"copilot\"\nauto_launch = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := stubClient(t, "copilot")

	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origInteractive })

	var runErr error
	_, stderr := captureRun(t, func() { runErr = run([]string{"--sync"}) })
	if runErr != nil {
		t.Fatalf("run(--sync) = %v", runErr)
	}
	if !strings.Contains(stderr, "unsandboxed") {
		t.Errorf("run(--sync) with plain copilot should warn about the missing sandbox, got stderr:\n%s", stderr)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("run(--sync) with auto_launch = true should still launch the client: %v\nstderr:\n%s", err, stderr)
	}
}
