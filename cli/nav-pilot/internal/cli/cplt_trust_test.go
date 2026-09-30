package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	approved, _ := parseCpltTrust([]byte(`{"command":null,"content_hash":"x","message":"","proposed":[],"state":"approved","version":1}`), nil)
	out = captureStdout(func() {
		if reportCpltTrust(approved) {
			t.Error("approved is reported as a problem")
		}
	})
	if !strings.Contains(out, "rules are trusted") {
		t.Errorf("approved:\n%s", out)
	}
}

// trustNudgeEnv is a repo with a .cplt.toml and a fake cplt that logs every
// call and answers trust show with trustPendingJSON.
func trustNudgeEnv(t *testing.T, interactive bool) (repo, log string, asked *int) {
	t.Helper()
	isolatedConfig(t)
	repo = t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, ".cplt.toml"), "[propose]\nallow_localhost_any = true\n")
	bin := t.TempDir()
	log = filepath.Join(bin, "calls.log")
	// Builtins only: PATH holds nothing but this cplt.
	script := "#!/bin/sh\necho \"$*\" >> " + log + "\n[ \"$1 $2\" = \"trust show\" ] && echo '" + trustPendingJSON + "'\nexit 0\n"
	if err := testhome.WriteExec(filepath.Join(bin, "cplt"), script); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
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
	t.Cleanup(func() { isInteractive, cpltInstalled, askTrustReview = prevI, prevC, prevA })
	sessionPrompted = false
	return repo, log, asked
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

	// A changed file is asked about again; No is recorded like yes.
	mustWrite(t, filepath.Join(repo, ".cplt.toml"), "[propose]\nallow_localhost_any = true\n# changed\n")
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
