package provider

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
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

// #2789: every host call starts in an empty temp directory, never the repo.
// Each call site is checked on its own: list and export must also get the
// .git pointer, the disable flag and no inherited config pointers. The
// OPENCODE_CONFIG_DIR set here proves the strip matters: opencode 2.3.2 still
// reads that directory with the disable flag alone.
func TestOpenCodeHostCallsDoNotStartInRepo(t *testing.T) {
	// Private TMPDIR: the cleanup check below must only see this test's directories.
	t.Setenv("TMPDIR", t.TempDir())
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".opencode", "plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo) // nav-pilot runs in the repo
	real := realDir(repo)
	tmp := realDir(os.TempDir())
	t.Setenv("OPENCODE_CONFIG_DIR", filepath.Join(repo, ".opencode"))
	t.Setenv("OPENCODE_CONFIG", filepath.Join(repo, "opencode.json"))
	log := fakeOpenCode(t, `case "$*" in
  --version) site=version ;;
  *list*) site=list ;;
  *) site=export ;;
esac
{
  echo "$site cwd=$(pwd -P)"
  echo "$site git=$(cat .git 2>/dev/null)"
  if read -r x; then echo "$site stdin=open"; else echo "$site stdin=eof"; fi
  echo "$site cfgdir=${OPENCODE_CONFIG_DIR:-unset}"
  echo "$site cfg=${OPENCODE_CONFIG:-unset}"
  echo "$site disable=${OPENCODE_DISABLE_PROJECT_CONFIG:-unset}"
} >> "$LOG"
case $site in
  version) echo 2.3.2 ;;
  list) echo '[{"id":"s1","updated":2000,"directory":"`+real+`"}]' ;;
  *) echo '{"messages":[]}' ;;
esac
`)
	readOpenCodeUsage(repo, time.UnixMilli(1000))
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		k, v, _ := strings.Cut(line, "=")
		got[k] = v
	}
	inTemp := func(site string) {
		cwd := got[site+" cwd"]
		if cwd == "" || strings.HasPrefix(cwd, real) || !strings.HasPrefix(cwd, tmp) {
			t.Errorf("%s: cwd %q is not a temp dir outside the repo", site, cwd)
		}
	}
	inTemp("version")
	for _, site := range []string{"list", "export"} {
		inTemp(site)
		want := map[string]string{
			" git":     "gitdir: " + filepath.Join(real, ".git"),
			" stdin":   "eof",
			" cfgdir":  "unset",
			" cfg":     "unset",
			" disable": "1",
		}
		for k, v := range want {
			if got[site+k] != v {
				t.Errorf("%s%s = %q, want %q", site, k, got[site+k], v)
			}
		}
	}
	if left, _ := filepath.Glob(filepath.Join(os.TempDir(), "nav-pilot-opencode-cwd-*")); len(left) != 0 {
		t.Errorf("temp cwd left behind: %v", left)
	}
}

// No temp dir means no probe at all, not a probe in the repo.
func TestStagedProbeFailsClosedWithoutTempDir(t *testing.T) {
	log := fakeOpenCode(t, `pwd >> "$LOG"; echo 2.3.2`)
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if _, err := runStagedProbe(5*time.Second, "opencode", "--version"); err == nil {
		t.Fatal("want an error when no temp dir can be made")
	}
	if _, err := os.Stat(log); err == nil {
		t.Fatal("probe ran without an isolated cwd")
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
	d := realDir(t.TempDir())
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

// Outside a git repo no host call is made at all (#2790 review).
func TestReadOpenCodeUsageSkipsNonGitDir(t *testing.T) {
	log := fakeOpenCode(t, `echo "$*" >> "$LOG"`)
	u, sessions := readOpenCodeUsage(t.TempDir(), time.UnixMilli(1000))
	if _, err := os.Stat(log); err == nil {
		b, _ := os.ReadFile(log)
		t.Fatalf("opencode ran for a non-git dir: %s", b)
	}
	if len(u.Models) != 0 || sessions != nil {
		t.Fatalf("usage = %+v, sessions = %+v", u, sessions)
	}
}

func TestFindGitDirSymlinkedSubdir(t *testing.T) {
	repo := realDir(t.TempDir())
	sub := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(sub, link); err != nil {
		t.Fatal(err)
	}
	if got, want := findGitDir(link), filepath.Join(repo, ".git"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A symlink to a linked worktree: the relative gitdir resolves against the
// real worktree, not the alias.
func TestFindGitDirSymlinkedLinkedWorktree(t *testing.T) {
	root := realDir(t.TempDir())
	wt := filepath.Join(root, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: ../main/.git/worktrees/wt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(wt, link); err != nil {
		t.Fatal(err)
	}
	if got, want := findGitDir(link), filepath.Join(root, "main", ".git", "worktrees", "wt"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A hostile .git is not git: no hang, no unbounded read, no opencode call.
func TestReadOpenCodeUsageHostileGitFile(t *testing.T) {
	cases := map[string]func(t *testing.T, p string){
		"fifo": func(t *testing.T, p string) {
			if runtime.GOOS == "windows" {
				t.Skip("no FIFOs")
			}
			if err := syscall.Mkfifo(p, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"symlink": func(t *testing.T, p string) {
			if err := os.Symlink("/dev/zero", p); err != nil {
				t.Fatal(err)
			}
		},
		"oversized": func(t *testing.T, p string) {
			if err := os.WriteFile(p, []byte("gitdir: "+strings.Repeat("a", 8<<10)), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			mk(t, filepath.Join(dir, ".git"))
			log := fakeOpenCode(t, `echo "$*" >> "$LOG"`)
			done := make(chan struct{})
			go func() { readOpenCodeUsage(dir, time.UnixMilli(1000)); close(done) }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("readOpenCodeUsage hung on the .git file")
			}
			if _, err := os.Stat(log); err == nil {
				t.Fatal("opencode ran for a hostile .git")
			}
		})
	}
}
