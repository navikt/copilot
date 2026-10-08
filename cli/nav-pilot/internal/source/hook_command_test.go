package source

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/testhome"
)

// A HOME with a space or a quote in it must still run the gate: unquoted, the
// path split, python3 could not open it, and every call was allowed.
func TestHookCommandQuotesThePath(t *testing.T) {
	// HookCommand runs whichever python3 is on PATH; put a working one first.
	bin := t.TempDir()
	wrapper := "#!/bin/sh\nexec '" + testhome.Python3(t) + "' \"$@\"\n"
	if err := testhome.WriteExec(filepath.Join(bin, "python3"), wrapper); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := filepath.Join(t.TempDir(), "Hans Kristian's home")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "gate.py")
	if err := os.WriteFile(script, []byte("import sys\nprint('deny:' + sys.stdin.read())\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", HookCommand(script, 5, false))
	cmd.Stdin = strings.NewReader("payload")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "deny:payload" {
		t.Errorf("gate did not run on %q: stdout %q", script, got)
	}
}

// The watchdog leaves Copilot two seconds after it fires: one was not enough
// under load, and the hook missed the deadline.
func TestHookCommandKeepsTwoSecondMargin(t *testing.T) {
	for timeout, want := range map[int]string{5: "(sleep 3;", 3: "(sleep 1;"} {
		if got := HookCommand("s.py", timeout, false); !strings.Contains(got, want) {
			t.Errorf("HookCommand(_, %d) lacks %q: %s", timeout, want, got)
		}
	}
}

// runHook runs HookCommand for a gate with body, the way Copilot does, with
// mktemp writing under a directory of its own. It returns stdout, stderr and
// that directory.
func runHook(t *testing.T, body string, timeout int, failClosed ...bool) (stdout, stderr, tmp string) {
	t.Helper()
	bin := t.TempDir()
	wrapper := "#!/bin/sh\nexec '" + testhome.Python3(t) + "' \"$@\"\n"
	if err := testhome.WriteExec(filepath.Join(bin, "python3"), wrapper); err != nil {
		t.Fatal(err)
	}
	// macOS mktemp ignores TMPDIR without a template, so a mktemp first on
	// PATH puts the hook's temp files where the test can see them.
	tmp = t.TempDir()
	mktemp, err := exec.LookPath("mktemp")
	if err != nil {
		t.Skip("no mktemp")
	}
	if err := testhome.WriteExec(filepath.Join(bin, "mktemp"), "#!/bin/sh\nexec '"+mktemp+"' '"+tmp+"/hook.XXXXXX'\n"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := filepath.Join(t.TempDir(), "gate.py")
	if err := os.WriteFile(script, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", HookCommand(script, timeout, len(failClosed) > 0 && failClosed[0]))
	cmd.Stdin = strings.NewReader("{}")
	var o, e strings.Builder
	cmd.Stdout, cmd.Stderr = &o, &e
	if err := cmd.Run(); err != nil {
		t.Fatalf("hook exited %v, want 0", err)
	}
	return o.String(), e.String(), tmp
}

// emptyEventually waits for the background rm to empty dir.
func emptyEventually(t *testing.T, dir string) {
	t.Helper()
	for range 300 { // up to 30 s: rm is one more process start
		if entries, _ := os.ReadDir(dir); len(entries) == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	entries, _ := os.ReadDir(dir)
	t.Errorf("temp files left behind in %s: %v", dir, entries)
}

// The script's stderr reaches Copilot when it wrote some, and the temp files
// go either way.
func TestHookCommandForwardsStderr(t *testing.T) {
	out, errOut, tmp := runHook(t, "import sys\nprint('ok')\nsys.stderr.write('warned\\n')\n", 5)
	if strings.TrimSpace(out) != "ok" || strings.TrimSpace(errOut) != "warned" {
		t.Errorf("stdout %q, stderr %q; want ok and warned", out, errOut)
	}
	emptyEventually(t, tmp)

	out, errOut, tmp = runHook(t, "print('ok')\n", 5)
	if strings.TrimSpace(out) != "ok" || errOut != "" {
		t.Errorf("stdout %q, stderr %q; want ok and nothing", out, errOut)
	}
	emptyEventually(t, tmp)

	// A script that fails: its stdout is dropped, its stderr still shown.
	out, errOut, tmp = runHook(t, "import sys\nprint('deny')\nsys.exit('broke')\n", 5)
	if out != "" || strings.TrimSpace(errOut) != "broke" {
		t.Errorf("stdout %q, stderr %q; want nothing and broke", out, errOut)
	}
	emptyEventually(t, tmp)
}

// A killed script prints nothing, allows the call, and its temp files are
// still removed by the rm the hook no longer waits for.
func TestHookCommandCleansUpAfterKill(t *testing.T) {
	out, errOut, tmp := runHook(t, "import sys, time\nprint('deny')\nsys.stderr.write('slow\\n')\nsys.stdout.flush()\nsys.stderr.flush()\ntime.sleep(30)\n", 3)
	if out != "" || errOut != "" {
		t.Errorf("killed gate wrote stdout %q, stderr %q; want nothing", out, errOut)
	}
	emptyEventually(t, tmp)
}

// A gate marked failClosed denies when it is killed or fails, with a reason
// that says which, and still passes on a clean answer, an empty one included.
// Without the flag the same cases allow (TestHookCommandCleansUpAfterKill,
// TestHookCommandForwardsStderr).
func TestHookCommandFailClosed(t *testing.T) {
	deny := func(why string) string {
		return `{"permissionDecision":"deny","permissionDecisionReason":"gate ` + why + `, så kallet er stoppet"}`
	}
	// Only the killed case wants a short deadline; the others get a long one,
	// so a python3 that is slow to start under load is not killed instead.
	for name, c := range map[string]struct {
		body, want string
		timeout    int
	}{
		"killed": {"import time\ntime.sleep(30)\n", deny("svarte ikke innen fristen"), 3},
		"failed": {"import sys\nprint('allow')\nsys.exit(1)\n", deny("feilet"), 20},
		"answer": {"print('ok')\n", "ok", 20},
		"silent": {"pass\n", "", 20},
	} {
		out, _, tmp := runHook(t, c.body, c.timeout, true)
		if got := strings.TrimSpace(out); got != c.want {
			t.Errorf("%s: stdout %q, want %q", name, got, c.want)
		}
		emptyEventually(t, tmp)
	}
	// No python3 at all, or no temp directory: the guards deny too.
	for name, env := range map[string][]string{
		"no python3": {"PATH=/nonexistent"},
		"no mktemp":  {"PATH=" + os.Getenv("PATH"), "TMPDIR=/nonexistent"},
	} {
		cmd := exec.Command("/bin/sh", "-c", HookCommand("gate.py", 3, true))
		cmd.Env = env
		if out, err := cmd.Output(); err != nil || strings.TrimSpace(string(out)) != deny("feilet") {
			t.Errorf("%s: %v, stdout %q", name, err, out)
		}
	}
}

// The gate's name lands in the deny reason, so a name with shell and JSON
// metacharacters must neither run anything nor break the JSON.
func TestHookCommandFailClosedQuotesTheName(t *testing.T) {
	dir := t.TempDir()
	name := "it's \"a\" $(touch pwned) `id`\nx"
	cmd := exec.Command("/bin/sh", "-c", HookCommand(filepath.Join(dir, name+".py"), 3, true))
	cmd.Dir = dir
	cmd.Env = []string{"PATH=/nonexistent"}
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ PermissionDecision, PermissionDecisionReason string }
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("not JSON: %v: %q", err, out)
	}
	if got.PermissionDecision != "deny" || !strings.HasPrefix(got.PermissionDecisionReason, name+" ") {
		t.Errorf("got %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
		t.Error("the name's $(…) ran")
	}
}

func TestLoadHookMetaTimeoutFloor(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "g.hook.json"), []byte(`{"timeoutSec": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadHookMeta(filepath.Join(dir, "g.py")).TimeoutSec; got != 3 {
		t.Errorf("timeoutSec 1 loaded as %d, want 3", got)
	}
}
