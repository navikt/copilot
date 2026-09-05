package provider

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

func TestLaunchViaCplt_CpltNotFound(t *testing.T) {
	// Empty PATH so neither cplt nor copilot is resolvable.
	t.Setenv("PATH", t.TempDir())

	err := launchViaCplt(cpltLaunch{
		agent:       "opencode",
		agentArgs:   []string{"--model", "anthropic/claude-sonnet-4-5"},
		displayName: "opencode",
	})
	if err == nil {
		t.Fatal("launchViaCplt must return an error when cplt is not on PATH")
	}
	if !strings.Contains(err.Error(), "cplt") {
		t.Errorf("error should mention cplt, got: %v", err)
	}
	if !strings.Contains(err.Error(), "opencode") {
		t.Errorf("error should mention the client display name, got: %v", err)
	}
}

func TestLaunchOpenCode_RequiresCplt(t *testing.T) {
	// Make opencode resolvable but cplt absent: a temp dir on PATH containing
	// only an executable named "opencode".
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "opencode"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("writing fake opencode: %v", err)
	}
	t.Setenv("PATH", dir)
	// Avoid writing Nav context into the real ~/.config/opencode.
	NavContextDirOverride = t.TempDir()
	t.Cleanup(func() { NavContextDirOverride = "" })
	// The pre-seed of <config dir>/.gitignore must not land in the real
	// ~/.config/opencode (#565).
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	err := LaunchOpenCode(domain.ResolvedConfig{Client: "opencode", Mode: "default"})
	if err == nil {
		t.Fatal("LaunchOpenCode must return an error when cplt is not on PATH")
	}
	if !strings.Contains(err.Error(), "cplt") {
		t.Errorf("expected cplt-not-found error, got: %v", err)
	}
}

// TestClassifyLaunchErrorSeparatesQuittingFromFailing pins the distinction the
// instrument exists for.
//
// cmd.Run returns an *exec.ExitError whenever the launched client exits
// non-zero, Ctrl-C included, and every one of those used to be recorded as
// "launch_failed". A counter described as "client launch failures" was
// therefore mostly a count of normal session endings: the more people used the
// tool, the worse the panel looked.
func TestClassifyLaunchErrorSeparatesQuittingFromFailing(t *testing.T) {
	run := func(t *testing.T, script string) error {
		t.Helper()
		return exec.Command("/bin/sh", "-c", script).Run()
	}

	tests := []struct {
		name   string
		err    error
		want   string
		reason string
	}{
		{"nothing went wrong", nil, "", "no error is not an event"},
		{"binary is not on PATH", exec.ErrNotFound, "client_not_found",
			"the real launch failure: nothing ever ran"},
		{"the client exited non-zero", run(t, "exit 3"), "client_exit",
			"it started and then failed, which is not a launch failure"},
		{"the developer pressed Ctrl-C", run(t, "kill -INT $$"), "",
			"quitting is not worth a data point"},
		{"a shell reported a signalled child", run(t, "exit 130"), "",
			"128+SIGINT, how a shell passes its child's status through"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyLaunchError(tt.err); got != tt.want {
				t.Errorf("classifyLaunchError(%v) = %q, want %q — %s", tt.err, got, tt.want, tt.reason)
			}
		})
	}
}
