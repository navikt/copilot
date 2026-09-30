package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

const trustPendingJSON = `{"command":"cplt trust accept","content_hash":"3e92","message":"Untrusted .cplt.toml. This repo wants to relax sandbox permissions.","project_dir":"/r","proposed":[{"approved":false,"detail":"true","effect":"reaches any local listener","key":"allow_localhost_any"},{"approved":true,"detail":"[5432]","effect":"outbound ports","key":"allow.ports"}],"state":"pending","version":1}`

func TestParseCpltTrust(t *testing.T) {
	for _, c := range []struct {
		name string
		out  string
		err  error
		ok   bool
	}{
		{"version 1", trustPendingJSON, nil, true},
		{"unknown version", strings.Replace(trustPendingJSON, `"version":1`, `"version":2`, 1), nil, false},
		{"older cplt, text", "Trust status for /r\n  pending\n", nil, false},
		{"non-zero exit", trustPendingJSON, errors.New("exit status 2"), false},
		{"no state", `{"version":1}`, nil, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			tr, ok := parseCpltTrust([]byte(c.out), c.err)
			if ok != c.ok {
				t.Fatalf("ok = %v, want %v", ok, c.ok)
			}
			if ok && (tr.State != "pending" || tr.Command == nil || len(tr.Proposed) != 2) {
				t.Errorf("parsed %+v", tr)
			}
		})
	}
}

// Doctor prints cplt's message, the cost of each unapproved key and cplt's
// command; without a command, no Run line.
func TestReportCpltTrust(t *testing.T) {
	pending, _ := parseCpltTrust([]byte(trustPendingJSON), nil)
	out := captureStdout(func() {
		if !reportCpltTrust(pending) {
			t.Error("pending is not reported as a problem")
		}
	})
	for _, want := range []string{"Untrusted .cplt.toml", "allow_localhost_any = true: reaches any local listener", "Run cplt trust accept"} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "outbound ports") {
		t.Errorf("an approved key is listed:\n%s", out)
	}

	invalid, _ := parseCpltTrust([]byte(`{"command":null,"content_hash":null,"message":"Failed to load .cplt.toml","proposed":[],"state":"invalid","version":1}`), nil)
	out = captureStdout(func() { reportCpltTrust(invalid) })
	if !strings.Contains(out, "Failed to load .cplt.toml") || strings.Contains(out, "Run ") {
		t.Errorf("invalid:\n%s", out)
	}

	approved, _ := parseCpltTrust([]byte(`{"command":null,"content_hash":"x","message":"","proposed":[],"state":"approved","version":1}`+"\n"), nil)
	out = captureStdout(func() {
		if reportCpltTrust(approved) {
			t.Error("approved is reported as a problem")
		}
	})
	if !strings.Contains(out, "rules are trusted") {
		t.Errorf("approved:\n%s", out)
	}
}

// trustNudgeEnv is a git repo with a committed .cplt.toml and a fake cplt
// that logs every call and answers trust show with the contents of show
// (trustPendingJSON to start with).
func trustNudgeEnv(t *testing.T, interactive bool) (repo, log string, asked *int) {
	t.Helper()
	isolatedConfig(t)
	t.Setenv("__CPLT_TRUST_LOCKED", "")
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("no git")
	}
	repo = t.TempDir()
	bin := t.TempDir()
	log = filepath.Join(bin, "calls.log")
	mustWrite(t, filepath.Join(bin, "show"), trustPendingJSON+"\n")
	// Builtins only: PATH holds nothing but this cplt and git.
	script := "#!/bin/sh\necho \"$*\" >> " + log + "\n[ \"$1 $2\" = \"trust show\" ] && read -r l < " + filepath.Join(bin, "show") + " && echo \"$l\"\nexit 0\n"
	if err := testhome.WriteExec(filepath.Join(bin, "cplt"), script); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(git, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	gitIn(t, repo, "init", "-q")
	mustWrite(t, filepath.Join(repo, ".cplt.toml"), "[propose]\nallow_localhost_any = true\n")
	gitIn(t, repo, "add", ".cplt.toml")
	gitIn(t, repo, "commit", "-qm", "a")
	prevI, prevC, prevA := isInteractive, cpltInstalled, askTrustReview
	isInteractive = func() bool { return interactive }
	cpltInstalled = func() bool { return true }
	asked = new(int)
	askTrustReview = func(prompt string) bool {
		*asked++
		if !strings.Contains(prompt, "(allow_localhost_any). Review now? [y/N]") {
			t.Errorf("prompt %q", prompt)
		}
		return true
	}
	t.Cleanup(func() { isInteractive, cpltInstalled, askTrustReview, sessionPrompted = prevI, prevC, prevA, false })
	sessionPrompted = false
	return repo, log, asked
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func calls(t *testing.T, log string) []string {
	t.Helper()
	raw, _ := os.ReadFile(log)
	_ = os.Remove(log)
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

// Yes hands off to plain `cplt trust accept`; the same file is never asked
// about again, and a launch with it unchanged spawns nothing at all.
func TestTrustNudgeOncePerHash(t *testing.T) {
	repo, log, asked := trustNudgeEnv(t, true)
	maybeTrustNudge(repo)
	if got := calls(t, log); len(got) != 2 || got[0] != "trust show --json" || got[1] != "trust accept" {
		t.Fatalf("calls = %q, want trust show --json then exactly trust accept", got)
	}
	if *asked != 1 {
		t.Fatalf("asked %d times", *asked)
	}

	maybeTrustNudge(repo)
	if _, err := os.Stat(log); err == nil {
		t.Errorf("an unchanged .cplt.toml spawned %q", calls(t, log))
	}
	if *asked != 1 {
		t.Errorf("asked again for the same file")
	}

	// A changed file is asked about again at the next launch; No is
	// recorded like yes.
	mustWrite(t, filepath.Join(repo, ".cplt.toml"), "[propose]\nallow_localhost_any = true\n# changed\n")
	gitIn(t, repo, "commit", "-qam", "changed")
	sessionPrompted = false
	askTrustReview = func(string) bool { *asked++; return false }
	maybeTrustNudge(repo)
	if got := calls(t, log); len(got) != 1 || got[0] != "trust show --json" {
		t.Errorf("after No, calls = %q", got)
	}
	maybeTrustNudge(repo)
	if _, err := os.Stat(log); err == nil || *asked != 2 {
		t.Errorf("a declined file was asked again (asked %d)", *asked)
	}
}

// Without a terminal: nothing spawned, nothing asked, nothing recorded.
func TestTrustNudgeTTYOnly(t *testing.T) {
	repo, log, asked := trustNudgeEnv(t, false)
	maybeTrustNudge(repo)
	if _, err := os.Stat(log); err == nil || *asked != 0 {
		t.Errorf("non-interactive launch spawned or asked (asked %d)", *asked)
	}
	if len(readTrustSeen()) != 0 {
		t.Error("non-interactive launch recorded the file")
	}
}

// cplt's verdict is on HEAD. An approved HEAD with an edit in the working
// tree must not record the edit, or committing it is never asked about.
func TestTrustNudgeWorkingTreeDrift(t *testing.T) {
	repo, log, asked := trustNudgeEnv(t, true)
	bin := filepath.Dir(log)
	mustWrite(t, filepath.Join(bin, "show"), `{"command":null,"content_hash":"a","message":"","proposed":[],"state":"approved","version":1}`+"\n")
	mustWrite(t, filepath.Join(repo, ".cplt.toml"), "[propose]\nallow_localhost_any = true\nallow_ports = [1]\n")
	maybeTrustNudge(repo)
	if len(readTrustSeen()) != 0 {
		t.Fatal("recorded working-tree bytes on a verdict about HEAD")
	}
	gitIn(t, repo, "commit", "-qam", "b")
	mustWrite(t, filepath.Join(bin, "show"), trustPendingJSON+"\n")
	askTrustReview = func(string) bool { *asked++; return false }
	maybeTrustNudge(repo)
	if *asked != 1 {
		t.Errorf("the committed edit was asked about %d times, want 1", *asked)
	}
	_ = calls(t, log)
}

// A cplt killed at the deadline records nothing: the next launch asks it
// again. An older cplt that fails fast records, so it is not re-spawned.
func TestTrustNudgeTimeoutAndOldCplt(t *testing.T) {
	repo, log, asked := trustNudgeEnv(t, true)
	bin := filepath.Dir(log)
	prev := cpltCommandTimeout
	t.Cleanup(func() { cpltCommandTimeout = prev })
	cpltCommandTimeout = 200 * time.Millisecond
	if err := testhome.WriteExec(filepath.Join(bin, "cplt"), "#!/bin/sh\nread -r l < /dev/zero\n"); err != nil {
		t.Fatal(err)
	}
	maybeTrustNudge(repo)
	if len(readTrustSeen()) != 0 || *asked != 0 {
		t.Fatal("a timed-out cplt recorded the file or asked")
	}
	if err := testhome.WriteExec(filepath.Join(bin, "cplt"), "#!/bin/sh\necho 'unknown flag' >&2\nexit 2\n"); err != nil {
		t.Fatal(err)
	}
	maybeTrustNudge(repo)
	if len(readTrustSeen()) != 1 || *asked != 0 {
		t.Errorf("an older cplt: seen %v, asked %d", readTrustSeen(), *asked)
	}
}

// A launch nested inside cplt neither spawns nor asks.
func TestTrustNudgeNestedLaunch(t *testing.T) {
	repo, log, asked := trustNudgeEnv(t, true)
	t.Setenv("__CPLT_TRUST_LOCKED", "1")
	maybeTrustNudge(repo)
	if _, err := os.Stat(log); err == nil || *asked != 0 || len(readTrustSeen()) != 0 {
		t.Errorf("nested launch spawned, asked (%d) or recorded", *asked)
	}
}

// One question per launch: the trust review takes it, strict waits.
func TestTrustNudgeTakesTheSessionPrompt(t *testing.T) {
	repo, _, asked := trustNudgeEnv(t, true)
	maybeTrustNudge(repo)
	if *asked != 1 || !sessionPrompted {
		t.Fatalf("asked %d, sessionPrompted %v", *asked, sessionPrompted)
	}
	prevAsk := askLeaveStrict
	t.Cleanup(func() { askLeaveStrict = prevAsk })
	strict := 0
	askLeaveStrict = func(*bool) error { strict++; return nil }
	cfg := filepath.Join(t.TempDir(), "config.toml")
	mustWrite(t, cfg, "[sandbox]\npreset = \"strict\"\n")
	t.Setenv("CPLT_CONFIG", cfg)
	offerLeaveStrict()
	if strict != 0 {
		t.Error("the strict offer asked in the same launch as the trust review")
	}
	sessionPrompted = false // the next launch
	offerLeaveStrict()
	if strict != 1 || !sessionPrompted {
		t.Errorf("the strict offer asked %d times at the next launch (want 1), sessionPrompted %v", strict, sessionPrompted)
	}
}

// A launch whose one question is already taken asks nothing and records
// nothing: the review is offered at the next launch.
func TestTrustNudgeYieldsATakenPrompt(t *testing.T) {
	repo, _, asked := trustNudgeEnv(t, true)
	sessionPrompted = true
	maybeTrustNudge(repo)
	if *asked != 0 || len(readTrustSeen()) != 0 {
		t.Errorf("asked %d, recorded %v", *asked, readTrustSeen())
	}
}

// An untracked .cplt.toml diffs clean against HEAD, but cplt calls it
// uncommitted: not recorded, so it is asked about once committed.
func TestTrustNudgeUncommitted(t *testing.T) {
	repo, log, asked := trustNudgeEnv(t, true)
	gitIn(t, repo, "rm", "-q", "--cached", ".cplt.toml")
	gitIn(t, repo, "commit", "-qm", "untrack")
	mustWrite(t, filepath.Join(filepath.Dir(log), "show"), `{"command":null,"content_hash":"a","message":"","proposed":[],"state":"uncommitted","version":1}`+"\n")
	maybeTrustNudge(repo)
	if len(readTrustSeen()) != 0 || *asked != 0 {
		t.Errorf("an uncommitted file was recorded or asked about (asked %d)", *asked)
	}
}
