package source

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A HOME with a space or a quote in it must still run the gate: unquoted, the
// path split, python3 could not open it, and every call was allowed.
func TestHookCommandQuotesThePath(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("needs python3")
	}
	dir := filepath.Join(t.TempDir(), "Hans Kristian's home")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "gate.py")
	if err := os.WriteFile(script, []byte("import sys\nprint('deny:' + sys.stdin.read())\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", HookCommand(script, 5))
	cmd.Stdin = strings.NewReader("payload")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "deny:payload" {
		t.Errorf("gate did not run on %q: stdout %q", script, got)
	}
}

func TestLoadHookMetaTimeoutFloor(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "g.hook.json"), []byte(`{"timeoutSec": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadHookMeta(filepath.Join(dir, "g.py")).TimeoutSec; got != 2 {
		t.Errorf("timeoutSec 1 loaded as %d, want 2", got)
	}
}
