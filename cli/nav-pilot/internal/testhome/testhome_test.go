package testhome

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) { os.Exit(Run(m)) }

func TestHomeIsRedirected(t *testing.T) {
	home := os.Getenv("HOME")
	if !strings.Contains(filepath.Base(home), "nav-pilot-test-home-") {
		t.Fatalf("HOME = %q, want the temporary test home", home)
	}
	if RealHome() == home {
		t.Fatalf("RealHome() = %q, the temporary home", home)
	}
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "NAV_PILOT_CONFIG"} {
		if !strings.HasPrefix(os.Getenv(k), home) {
			t.Errorf("%s = %q, not under the test home %s", k, os.Getenv(k), home)
		}
	}
}

func TestGuardSeesAWrite(t *testing.T) {
	home := t.TempDir()
	cache := filepath.Join(home, ".nav-pilot", "cache.json")
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(home)
	if p := diff(before, snapshot(home)); p != "" {
		t.Fatalf("nothing written, yet diff = %q", p)
	}

	// An in-place rewrite leaves the directory's mtime alone.
	later := time.Now().Add(time.Second)
	if err := os.Chtimes(cache, later, later); err != nil {
		t.Fatal(err)
	}
	if got := diff(before, snapshot(home)); got != cache {
		t.Fatalf("rewritten cache.json: diff = %q, want %q", got, cache)
	}

	// A new file in a watched tree.
	before = snapshot(home)
	hook := filepath.Join(home, ".copilot", "hooks", "gate.json")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if diff(before, snapshot(home)) == "" {
		t.Fatal("a new ~/.copilot/hooks/gate.json went unnoticed")
	}
}

// The controls for Run's guard. Each runs this test binary again, with a
// temporary directory as its real home, and TestGuardChild doing what the
// case needs: a leak must fail the run, a write from another process must not,
// except in CI.
func TestGuardControls(t *testing.T) {
	cases := []struct {
		name, mode string
		env        []string
		args       []string
		wantFail   bool
		wantOut    string
	}{
		{"leak fails", "leak", nil, nil, true, "a test in this package opened"},
		{"leak fails with go test's own test log", "leak", nil, []string{"-test.testlogfile=" + filepath.Join(t.TempDir(), "log")}, true, "a test in this package opened"},
		{"leak passes with the guard off", "leak", []string{"NAV_PILOT_TESTHOME_GUARD=0"}, nil, false, ""},
		{"another process's write warns", "wait", nil, nil, false, "testhome: warning:"},
		{"another process's write fails in CI", "wait", []string{"CI=true"}, nil, true, "or a process it started, wrote"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home, sync := t.TempDir(), t.TempDir()
			spool := filepath.Join(home, ".nav-pilot", "telemetry-spool")
			if err := os.MkdirAll(spool, 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestGuardChild$"}, c.args...)...)
			for _, kv := range os.Environ() {
				if k, _, _ := strings.Cut(kv, "="); k != "CI" && k != "NAV_PILOT_TESTHOME_GUARD" {
					cmd.Env = append(cmd.Env, kv)
				}
			}
			cmd.Env = append(cmd.Env, append(c.env, "HOME="+home, "TESTHOME_CHILD="+c.mode, "TESTHOME_SYNC="+sync)...)
			var out strings.Builder
			cmd.Stdout, cmd.Stderr = &out, &out
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			if c.mode == "wait" {
				// This process is the other writer: the child did not open
				// the spool, this test did, while the child's tests ran.
				waitFor(t, filepath.Join(sync, "ready"))
				if err := os.WriteFile(filepath.Join(spool, "1-2-3.pb"), []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(sync, "done"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := cmd.Wait()
			if failed := err != nil; failed != c.wantFail {
				t.Fatalf("run failed = %v, want %v; output:\n%s", failed, c.wantFail, out.String())
			}
			if !strings.Contains(out.String(), c.wantOut) {
				t.Fatalf("output lacks %q:\n%s", c.wantOut, out.String())
			}
		})
	}
}

// TestGuardChild is the test binary's side of TestGuardControls.
func TestGuardChild(t *testing.T) {
	switch os.Getenv("TESTHOME_CHILD") {
	case "":
		t.Skip("run by TestGuardControls")
	case "leak":
		// A leak as it happens: code that got hold of the real home.
		if err := os.WriteFile(filepath.Join(RealHome(), ".nav-pilot", "cache.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	case "wait":
		sync := os.Getenv("TESTHOME_SYNC")
		if err := os.WriteFile(filepath.Join(sync, "ready"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		waitFor(t, filepath.Join(sync, "done"))
	}
}

func waitFor(t *testing.T, path string) {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(path); err == nil {
			return
		}
	}
	t.Fatalf("%s did not appear", path)
}

// Every test package must call Run from its TestMain, or its tests run
// against the real home.
func TestEveryTestPackageCallsRun(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	has := map[string]bool{}  // dir has a _test.go file
	runs := map[string]bool{} // dir calls Run
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, "_test.go") {
			return nil
		}
		dir := filepath.Dir(p)
		has[dir] = true
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), "testhome.Run(") || strings.Contains(string(body), "os.Exit(Run(m))") {
			runs[dir] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for dir := range has {
		if !runs[dir] {
			rel, _ := filepath.Rel(root, dir)
			t.Errorf("%s has tests but no TestMain calling testhome.Run(m); see internal/testhome", rel)
		}
	}
}
