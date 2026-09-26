package provider

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

// fakeCpltArgs puts a cplt on PATH that records its argument vector, one
// argument per line, and returns the file it records to.
func fakeCpltArgs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "args.txt")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + out + "\n"
	if err := os.WriteFile(filepath.Join(dir, "cplt"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return out
}

// projectDirArg returns the operand of --project-dir in the recorded vector,
// and fails the test when there is none or it is not ahead of the "--".
func projectDirArg(t *testing.T, out string) string {
	t.Helper()
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the fake cplt recorded nothing: %v", err)
	}
	args := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	i := slices.Index(args, "--project-dir")
	sep := slices.Index(args, "--")
	if i < 0 || i+1 >= len(args) || (sep >= 0 && i > sep) {
		t.Fatalf("cplt vector %q has no --project-dir operand before --", args)
	}
	return args[i+1]
}

func sameDir(t *testing.T, got, want string) {
	t.Helper()
	g, err1 := filepath.EvalSymlinks(got)
	w, err2 := filepath.EvalSymlinks(want)
	if err1 != nil || err2 != nil || g != w {
		t.Errorf("--project-dir = %q, want %q", got, want)
	}
}

// TestLaunchViaCpltPassesProjectDir pins the sandbox scope to the working
// directory. Without an explicit --project-dir, cplt widens a directory inside
// a git repository to the repository root, so a session started from a
// subfolder that is not a repository itself could edit its siblings.
func TestLaunchViaCpltPassesProjectDir(t *testing.T) {
	isolateHome(t)
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "workspaces", "one")
	mustMkdir(t, sub)
	t.Chdir(sub)

	t.Run("working directory by default", func(t *testing.T) {
		out := fakeCpltArgs(t)
		if err := launchViaCplt(cpltLaunch{agent: "opencode", displayName: "opencode"}); err != nil {
			t.Fatalf("launchViaCplt: %v", err)
		}
		sameDir(t, projectDirArg(t, out), sub)
	})

	t.Run("explicit --project-dir wins, made absolute", func(t *testing.T) {
		out := fakeCpltArgs(t)
		if err := launchViaCplt(cpltLaunch{agent: "opencode", displayName: "opencode", projectDir: "../.."}); err != nil {
			t.Fatalf("launchViaCplt: %v", err)
		}
		got := projectDirArg(t, out)
		if !filepath.IsAbs(got) {
			t.Errorf("--project-dir %q is not absolute", got)
		}
		sameDir(t, got, repo)
	})

	t.Run("legacy copilot seam, explicit --project-dir wins", func(t *testing.T) {
		out := fakeCpltArgs(t)
		if err := LaunchCopilotResolved(domain.ResolvedConfig{Client: "copilot", AskUser: true, ProjectDir: repo}); err != nil {
			t.Fatalf("LaunchCopilotResolved: %v", err)
		}
		sameDir(t, projectDirArg(t, out), repo)
	})

	t.Run("legacy copilot seam, working directory by default", func(t *testing.T) {
		out := fakeCpltArgs(t)
		if err := LaunchCopilotResolved(domain.ResolvedConfig{Client: "copilot", AskUser: true}); err != nil {
			t.Fatalf("LaunchCopilotResolved: %v", err)
		}
		sameDir(t, projectDirArg(t, out), sub)
	})
}

// TestLaunchViaCpltHandsHomeToCplt: nav-pilot does not duplicate cplt's own
// refusal of $HOME and / as too broad ("cplt refuses to sandbox '<dir>', it
// is too broad"). It passes the directory through unchanged and lets cplt say
// no.
func TestLaunchViaCpltHandsHomeToCplt(t *testing.T) {
	home := isolateHome(t)
	t.Chdir(home)
	out := fakeCpltArgs(t)
	if err := launchViaCplt(cpltLaunch{agent: "opencode", displayName: "opencode"}); err != nil {
		t.Fatalf("launchViaCplt: %v", err)
	}
	sameDir(t, projectDirArg(t, out), home)
}
