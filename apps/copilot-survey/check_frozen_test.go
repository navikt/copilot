package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckFrozenSurveys runs check-frozen-surveys.sh against a base commit in
// a scratch repository, one pull request per case.
func TestCheckFrozenSurveys(t *testing.T) {
	script, err := filepath.Abs("check-frozen-surveys.sh")
	if err != nil {
		t.Fatal(err)
	}
	const (
		started = `{"id":"w","title":"T","active":true,"starts":"2000-01-01","ends":"2000-02-01","questions":[{"id":"a","version":1}]}`
		paused  = `{"id":"w","title":"T","active":false,"starts":"2000-01-01","ends":"2000-02-01","questions":[{"id":"a","version":1}]}`
		future  = `{"id":"w","title":"T","active":true,"starts":"2999-01-01","ends":"2999-02-01","questions":[{"id":"a","version":1}]}`
	)
	cases := []struct {
		name      string
		base, pr  string // "" means no file
		wantError bool
	}{
		{"started, questions changed", started, `{"id":"w","title":"T","active":true,"starts":"2000-01-01","ends":"2000-02-01","questions":[{"id":"a","version":2}]}`, true},
		// The second of two pull requests: the first only set active: false.
		{"started and paused, questions changed", paused, `{"id":"w","title":"T","active":true,"starts":"2000-01-01","ends":"2000-02-01","questions":[{"id":"a","version":2}]}`, true},
		{"started, starts moved later", started, `{"id":"w","title":"T","active":true,"starts":"2999-01-01","ends":"2999-02-01","questions":[{"id":"a","version":1}]}`, true},
		{"started, id changed", started, `{"id":"x","title":"T","active":true,"starts":"2000-01-01","ends":"2000-02-01","questions":[{"id":"a","version":1}]}`, true},
		{"started, file deleted", started, "", true},
		{"started, paused and retitled", started, `{"id":"w","title":"U","active":false,"starts":"2000-01-01","ends":"2000-03-01","questions":[{"id":"a","version":1}]}`, false},
		{"not started, questions changed", future, `{"id":"w","title":"T","active":true,"starts":"2999-01-01","ends":"2999-02-01","questions":[{"id":"a","version":2}]}`, false},
		{"new file", "", started, false},
		// jq keeps the last "questions", encoding/json merges min into it.
		{"started, duplicate questions key", started, `{"id":"w","title":"T","active":true,"starts":"2000-01-01","ends":"2000-02-01","questions":[{"min":2}],"questions":[{"id":"a","version":1}]}`, true},
		{"started, key differing in case", started, `{"id":"w","title":"T","active":true,"starts":"2000-01-01","ends":"2000-02-01","Questions":[{"min":2}],"questions":[{"id":"a","version":1}]}`, true},
		{"new file, duplicate key", "", `{"id":"w","id":"w","title":"T","starts":"2999-01-01","ends":"2999-02-01","questions":[]}`, true},
		{"dev-only, started, questions changed", `{"id":"w","title":"T","environments":["dev-gcp"],"active":false,"starts":"2000-01-01","ends":"2000-02-01","questions":[{"id":"a","version":1}]}`, `{"id":"w","title":"T","environments":["dev-gcp"],"active":false,"starts":"2000-01-01","ends":"2000-02-01","questions":[{"id":"a","version":2}]}`, false},
		{"prod-only, started, questions changed", `{"id":"w","title":"T","environments":["prod-gcp"],"active":false,"starts":"2000-01-01","ends":"2000-02-01","questions":[{"id":"a","version":1}]}`, `{"id":"w","title":"T","environments":["prod-gcp"],"active":false,"starts":"2000-01-01","ends":"2000-02-01","questions":[{"id":"a","version":2}]}`, true},
		// The first of two pull requests that would make a prod survey dev-only.
		{"started, made dev-only", started, `{"id":"w","title":"T","environments":["dev-gcp"],"active":true,"starts":"2000-01-01","ends":"2000-02-01","questions":[{"id":"a","version":1}]}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := t.TempDir()
			dir := filepath.Join(repo, "apps", "copilot-survey")
			file := filepath.Join(dir, "surveys", "w.json")
			if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
				t.Fatal(err)
			}
			git := func(args ...string) string {
				t.Helper()
				cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...) //nolint:gosec // fixed git arguments from this test
				cmd.Dir = repo
				cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "HOME="+repo)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
				return string(out)
			}
			write := func(content string) {
				t.Helper()
				if content == "" {
					if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
						t.Fatal(err)
					}
					return
				}
				if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			git("init", "-q")
			write(c.base)
			git("add", "-A")
			git("commit", "-q", "--allow-empty", "-m", "base")
			base := git("rev-parse", "HEAD")
			write(c.pr)
			git("add", "-A")
			git("commit", "-q", "--allow-empty", "-m", "pr")

			cmd := exec.Command("bash", script, strings.TrimSpace(base)) //nolint:gosec // the script under test and a sha from git
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if gotError := err != nil; gotError != c.wantError {
				t.Fatalf("error = %v, want error %v\n%s", err, c.wantError, out)
			}
		})
	}
}
