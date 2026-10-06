package cli

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

// plainCopilot puts a copilot in dir that only answers --version: cplt starts
// the copilot on PATH, and nav-pilot does not hand off to cplt without one
// (#1064).
func plainCopilot(t *testing.T, dir string) {
	t.Helper()
	script := "#!/bin/sh\n[ \"$1\" = --version ] && echo 'GitHub Copilot CLI 1.0.40'\nexit 0\n"
	if err := testhome.WriteExec(filepath.Join(dir, "copilot"), script); err != nil {
		t.Fatal(err)
	}
}

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
	// A --version probe (FindCopilotCLI asks a plain copilot whether it is
	// cplt) is not a launch.
	script := "#!/bin/sh\n[ \"$1\" = --version ] && exit 0\n: > \"" + markerPath + "\"\n"
	if err := testhome.WriteExec(filepath.Join(binDir, name), script); err != nil {
		t.Fatal(err)
	}
	if name == "cplt" {
		plainCopilot(t, binDir)
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
	// pi launches only inside cplt, so without it pi is not available at all.
	if err := testhome.WriteExec(filepath.Join(os.Getenv("PATH"), "cplt"), "#!/bin/sh\n"); err != nil {
		t.Fatal(err)
	}

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

// The --sync path never launches a client without cplt (#472): a plain
// copilot CLI is not started, and the install command is named.
func TestSyncFlagNeverLaunchesWithoutCplt(t *testing.T) {
	cfgPath := isolatedConfig(t)
	if err := os.WriteFile(cfgPath, []byte("version = 1\nclient = \"copilot\"\nauto_launch = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := stubClient(t, "copilot")
	t.Chdir(t.TempDir())

	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origInteractive })

	var runErr error
	_, stderr := captureRun(t, func() { runErr = run([]string{"--sync"}) })
	if runErr != nil {
		t.Fatalf("run(--sync) = %v", runErr)
	}
	if !strings.Contains(stderr, "brew install navikt/tap/cplt (or sudo apt install cplt)") {
		t.Errorf("no install command on stderr:\n%s", stderr)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("copilot launched without cplt")
	}
}

// --no-sandbox is gone, but still parsed: it exits non-zero and says why,
// instead of an unknown-flag error or being silently ignored.
func TestNoSandboxFlagRefused(t *testing.T) {
	isolatedConfig(t)
	marker := stubClient(t, "cplt")
	err := run([]string{"--no-sandbox", "--", "-p", "hi"})
	if err == nil || !strings.Contains(err.Error(), "always runs clients inside cplt") {
		t.Fatalf("run(--no-sandbox) = %v, want the removal message", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("--no-sandbox launched a client")
	}
}

func TestSyncFlagRemovesUnusableRtkHook(t *testing.T) {
	cfgPath := isolatedConfig(t)
	if err := os.WriteFile(cfgPath, []byte("version = 1\nclient = \"copilot\"\nauto_launch = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := stubClient(t, "cplt")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(home, ".copilot", "hooks", "rtk-rewrite.json")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origInteractive })

	var runErr error
	_, stderr := captureRun(t, func() { runErr = run([]string{"--sync"}) })
	if runErr != nil {
		t.Fatalf("run(--sync) = %v\nstderr:\n%s", runErr, stderr)
	}
	if _, err := os.Stat(hook); !os.IsNotExist(err) {
		t.Fatalf("run(--sync) left unusable RTK hook behind: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("run(--sync) did not launch Copilot after removing the hook: %v", err)
	}
}

// --project-dir must reach cplt from the command line, made absolute: the
// provider tests set ResolvedConfig.ProjectDir directly, so they cannot see
// the flag being parsed or carried into the resolved config.
func TestProjectDirFlagReachesCplt(t *testing.T) {
	cfgPath := isolatedConfig(t)
	if err := os.WriteFile(cfgPath, []byte("version = 1\nclient = \"copilot\"\nauto_launch = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"" + argsFile + "\"\n"
	if err := testhome.WriteExec(filepath.Join(binDir, "cplt"), script); err != nil {
		t.Fatal(err)
	}
	plainCopilot(t, binDir)
	t.Setenv("PATH", binDir)

	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	origInteractive := isInteractive
	isInteractive = func() bool { return true }
	t.Cleanup(func() { isInteractive = origInteractive })

	var runErr error
	_, stderr := captureRun(t, func() { runErr = run([]string{"--sync", "--project-dir", ".."}) })
	if runErr != nil {
		t.Fatalf("run(--sync --project-dir ..) = %v\nstderr:\n%s", runErr, stderr)
	}
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("cplt was not launched: %v\nstderr:\n%s", err, stderr)
	}
	args := strings.Split(strings.TrimSpace(string(raw)), "\n")
	i := slices.Index(args, "--project-dir")
	if i < 0 || i+1 >= len(args) {
		t.Fatalf("cplt vector %q has no --project-dir operand", args)
	}
	got, _ := filepath.EvalSymlinks(args[i+1])
	want, _ := filepath.EvalSymlinks(root)
	if !filepath.IsAbs(args[i+1]) || got != want {
		t.Errorf("--project-dir = %q, want the absolute path of %q", args[i+1], root)
	}
}
