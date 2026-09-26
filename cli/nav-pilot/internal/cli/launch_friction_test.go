package cli

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"
)

// One Ctrl-C in the first-run wizard ends the run: "Cancelled. Nothing was
// written." once, exit 130, and none of the prompts that used to follow it
// (it took three presses, with "Setup skipped" and "Config setup failed" in
// between).
func TestFirstRunCtrlCEndsTheRun(t *testing.T) {
	isolatedConfig(t)
	t.Chdir(t.TempDir())
	forceInteractive(t)
	orig := runConfigSetupFn
	runConfigSetupFn = func(string) error { return setupSkipped(huh.ErrUserAborted) }
	t.Cleanup(func() { runConfigSetupFn = orig })

	var err error
	stdout, stderr := captureRun(t, func() { err = run(nil) })
	var code int
	_, stderr2 := captureRun(t, func() { code = exitCodeFor(err) })
	if code != 130 {
		t.Errorf("exit code %d, want 130 (err %v)", code, err)
	}
	if got := strings.Count(stderr2, "Cancelled. Nothing was written."); got != 1 {
		t.Errorf("cancel message printed %d times:\n%s", got, stderr2)
	}
	for _, not := range []string{"Setup skipped", "Config setup failed", "rtk", "Where to install"} {
		if strings.Contains(stdout+stderr, not) {
			t.Errorf("the run went on after Ctrl-C (%q):\n%s%s", not, stdout, stderr)
		}
	}
	if _, statErr := os.Stat(configPath()); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a config was written: %v", statErr)
	}
}

// A prompt that cannot run is not Ctrl-C: the wizard steps aside with one line.
func TestSetupSkippedOnAPromptThatCannotRun(t *testing.T) {
	var err error
	out, _ := captureRun(t, func() { err = setupSkipped(errors.New("no tty")) })
	if err != nil || strings.Count(out, "Setup skipped") != 1 {
		t.Errorf("setupSkipped = %v, printed:\n%s", err, out)
	}
}

// After a first-run install from $HOME the launch would be refused by cplt as
// too broad. The run says the install worked and where to launch from, exits
// 0, and starts nothing.
func TestOfferLaunchAfterInstallFromHome(t *testing.T) {
	cfgPath := isolatedConfig(t)
	if err := os.WriteFile(cfgPath, []byte("version = 1\nclient = \"copilot\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := stubClient(t, "cplt")
	home, _ := os.UserHomeDir()
	t.Chdir(home)
	forceInteractive(t)

	var err error
	stdout, _ := captureRun(t, func() { err = offerLaunchAfterInstall(ResolvedConfig{Client: "copilot", AutoLaunch: true}) })
	if err != nil {
		t.Fatalf("offerLaunchAfterInstall = %v", err)
	}
	if !strings.Contains(stdout, "Installed. cd into a project and run nav-pilot, or pass --project-dir <dir>.") {
		t.Errorf("stdout:\n%s", stdout)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("launched from $HOME")
	}
}

// The auto_launch = false command always names cplt, never the bare client,
// and survives a path with a space when pasted into a shell.
func TestStartCommandIsSandboxedAndQuoted(t *testing.T) {
	got := startCommand(ResolvedConfig{Client: "opencode", ProjectDir: "/tmp/my project"})
	if !strings.Contains(got, "cplt --project-dir '/tmp/my project' --agent opencode") {
		t.Errorf("startCommand = %q", got)
	}
	if q := shellQuote("it's"); q != `'it'\''s'` {
		t.Errorf("shellQuote(it's) = %s", q)
	}
	if q := shellQuote("/plain/path"); q != "/plain/path" {
		t.Errorf("shellQuote(/plain/path) = %s", q)
	}
}
