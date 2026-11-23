package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeOpenCode puts an opencode on PATH that runs script with LOG set.
func fakeOpenCode(t *testing.T, script string) (log string) {
	t.Helper()
	bin := t.TempDir()
	log = filepath.Join(t.TempDir(), "log")
	if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte("#!/bin/sh\nLOG="+log+"\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	versionCache.Delete("opencode")
	t.Cleanup(func() { versionCache.Delete("opencode") })
	return log
}

// #2789: every host call starts in an empty directory, never the repo, with
// stdin closed and no inherited config pointer. The list still sees the
// repo's project through the .git file.
func TestOpenCodeHostCallsDoNotStartInRepo(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".opencode", "plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo) // nav-pilot runs in the repo
	real := realDir(repo)
	t.Setenv("OPENCODE_CONFIG_DIR", filepath.Join(repo, ".opencode"))
	log := fakeOpenCode(t, `{
  echo "cwd=$(pwd -P)"
  echo "files=$(ls -A | tr '\n' ' ')"
  echo "git=$(cat .git)"
  if read -r x; then echo stdin=open; else echo stdin=eof; fi
  echo "cfgdir=${OPENCODE_CONFIG_DIR:-unset} disable=${OPENCODE_DISABLE_PROJECT_CONFIG:-unset}"
} >> "$LOG"
case "$*" in
  --version) echo 2.3.2 ;;
  *list*) echo '[{"id":"s1","updated":2000,"directory":"`+real+`"}]' ;;
  *) echo '{"messages":[]}' ;;
esac
`)
	readOpenCodeUsage(repo, time.UnixMilli(1000))
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if n := strings.Count(got, "cwd="); n != 3 {
		t.Fatalf("want 3 calls (--version, list, export), got %d:\n%s", n, got)
	}
	for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
		switch {
		case strings.HasPrefix(line, "cwd="):
			if cwd := strings.TrimPrefix(line, "cwd="); strings.HasPrefix(cwd, real) {
				t.Errorf("opencode started in the repo: %s", cwd)
			}
		case strings.HasPrefix(line, "files="):
			if line != "files=.git " && line != "files=" {
				t.Errorf("cwd not empty: %q", line)
			}
		case strings.HasPrefix(line, "git="):
			if line != "git=gitdir: "+filepath.Join(repo, ".git") && line != "git=" {
				t.Errorf(".git file = %q", line)
			}
		case strings.HasPrefix(line, "stdin="):
			if line != "stdin=eof" {
				t.Errorf("stdin not closed: %q", line)
			}
		case strings.HasPrefix(line, "cfgdir="):
			if !strings.HasSuffix(line, "disable=1") && line != "cfgdir="+filepath.Join(repo, ".opencode")+" disable=unset" {
				t.Errorf("env = %q", line)
			}
		}
	}
	if left, _ := filepath.Glob(filepath.Join(os.TempDir(), "nav-pilot-opencode-cwd-*")); len(left) != 0 {
		t.Errorf("temp cwd left behind: %v", left)
	}
}

// The deadline holds even when opencode leaves a child behind that keeps
// running.
func TestOpenCodeCommandTimeoutWithLingeringChild(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	fakeOpenCode(t, "sleep 30 &\nsleep 30\n")
	old := openCodeHostTimeout
	t.Cleanup(func() { openCodeHostTimeout = old })
	openCodeHostTimeout = 200 * time.Millisecond
	start := time.Now()
	if _, err := openCodeCommand(context.Background(), t.TempDir(), "session", "list"); err == nil {
		t.Fatal("want an error from the timeout")
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("returned after %s", d)
	}
}

func TestFindGitDirLinkedWorktree(t *testing.T) {
	d := t.TempDir()
	sub := filepath.Join(d, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, ".git"), []byte("gitdir: ../main/.git/worktrees/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := findGitDir(sub), filepath.Join(d, "../main/.git/worktrees/x"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := findGitDir(""); got != "" {
		t.Fatalf("empty dir: %q", got)
	}
}
