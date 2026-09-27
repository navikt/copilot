// Package testhome keeps a test binary out of the developer's real home.
//
// Every test package in nav-pilot calls Run from its TestMain. Run points HOME,
// the XDG directories and NAV_PILOT_CONFIG at a fresh temporary directory for
// the whole binary, so a test that forgets isolatedConfig(t) writes there and
// not in ~/.nav-pilot, ~/.copilot or ~/.config (#627, #1063). This is only
// the floor: a test that sets HOME itself must set NAV_PILOT_CONFIG too, or it
// reads and writes the config Run chose; isolatedConfig(t) does both.
//
// Run also guards the real home: it records the files nav-pilot writes there
// before the tests and fails the run, naming the path, if any of them changed.
package testhome

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// watched are the trees under the real home that a test run must leave alone,
// walked file by file so an in-place rewrite shows up too. skipped are
// subtrees not worth the walk: the local model venv, npm's node_modules, and
// Copilot CLI's session-state, which holds hundreds of thousands of files that
// every running Copilot session rewrites.
var (
	watched = []string{
		".nav-pilot",
		".config/opencode",
		".local/share/nav-pilot",
		".copilot/.nav-pilot-state.json",
		".copilot/hooks",
		".copilot/skills",
		".copilot/agents",
		".copilot/.github",
	}
	skipped = map[string]bool{".nav-pilot/local": true, ".config/opencode/node_modules": true}
)

var (
	realHome string
	origEnv  []string
)

// RealHome is the developer's home as it was before Run redirected HOME.
func RealHome() string { return realHome }

// Run isolates the home, runs the tests and returns the exit code for os.Exit.
//
// A real nav-pilot running at the same time as the tests (a sync, a launch
// that writes ~/.config/opencode/.nav-pilot-state.json) trips the guard too:
// it cannot tell the two writers apart. The message says so and gives the time
// window. A false alarm costs a rerun, a missed write costs the developer's
// setup. NAV_PILOT_TESTHOME_GUARD=0 turns the check off for such a run; the
// redirection stays on.
func Run(m *testing.M) int {
	realHome, _ = os.UserHomeDir()
	start := time.Now()
	before := snapshot(realHome)

	tmp, err := os.MkdirTemp("", "nav-pilot-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testhome:", err)
		return 1
	}
	defer os.RemoveAll(tmp)
	pinMise(realHome)
	origEnv = os.Environ()
	os.Setenv("HOME", tmp)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))
	os.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, ".cache"))
	os.Setenv("XDG_DATA_HOME", filepath.Join(tmp, ".local", "share"))
	os.Setenv("XDG_STATE_HOME", filepath.Join(tmp, ".local", "state"))
	os.Setenv("NAV_PILOT_CONFIG", filepath.Join(tmp, ".nav-pilot", "config.toml"))

	code := m.Run()

	if os.Getenv("NAV_PILOT_TESTHOME_GUARD") == "0" {
		return code
	}
	if p := diff(before, snapshot(realHome)); p != "" {
		fmt.Fprintf(os.Stderr, "testhome: a test in this package, or another process, wrote %s\n"+
			"in the real home between %s and %s.\n"+
			"If it was a test, isolate it with isolatedConfig(t), or set both HOME and NAV_PILOT_CONFIG.\n"+
			"If a real nav-pilot ran at the same time, rerun, or set NAV_PILOT_TESTHOME_GUARD=0.\n",
			p, start.Format(time.TimeOnly), time.Now().Format(time.TimeOnly))
		return 1
	}
	return code
}

// pinMise keeps mise shims (python3, node, ...) on PATH working once HOME has
// moved: a shim finds its tools, config and trust list through HOME and the
// XDG directories, so each mise directory is pinned to where it was.
func pinMise(home string) {
	if home == "" {
		return
	}
	xdg := func(env string, def ...string) string {
		if v := os.Getenv(env); v != "" {
			return filepath.Join(v, "mise")
		}
		return filepath.Join(append([]string{home}, append(def, "mise")...)...)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = filepath.Join(home, ".cache")
	}
	for k, v := range map[string]string{
		"MISE_DATA_DIR":   xdg("XDG_DATA_HOME", ".local", "share"),
		"MISE_CONFIG_DIR": xdg("XDG_CONFIG_HOME", ".config"),
		"MISE_STATE_DIR":  xdg("XDG_STATE_HOME", ".local", "state"),
		"MISE_CACHE_DIR":  filepath.Join(cache, "mise"),
	} {
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(k, v)
		}
	}
}

// OriginalEnv is the environment as it was before Run redirected it (mise
// directories pinned), for a child such as `go build` that needs the
// developer's own caches.
func OriginalEnv() []string { return slices.Clone(origEnv) }

// snapshot maps every file and directory under the watched trees to its mtime
// and size. A path missing from one side of diff was created or removed.
func snapshot(home string) map[string]string {
	s := map[string]string{}
	if home == "" {
		return s
	}
	for _, rel := range watched {
		filepath.WalkDir(filepath.Join(home, rel), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if r, _ := filepath.Rel(home, p); skipped[r] {
				return filepath.SkipDir
			}
			if fi, err := d.Info(); err == nil {
				s[p] = fi.ModTime().Format(time.RFC3339Nano) + fmt.Sprintf(" %d", fi.Size())
			}
			return nil
		})
	}
	return s
}

// diff returns a path that changed, appeared or disappeared, or "".
func diff(before, after map[string]string) string {
	for p, b := range before {
		if after[p] != b {
			return p
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			return p
		}
	}
	return ""
}

// Python3 returns a python3 that actually runs, or skips the test. A mise
// shim first on PATH finds its config through HOME, so under a temporary HOME
// it fails; the rest of PATH, then `mise which python3` in the original
// environment, usually hold the real interpreter behind it.
func Python3(t testing.TB) string {
	t.Helper()
	var candidates []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir != "" {
			candidates = append(candidates, filepath.Join(dir, "python3"))
		}
	}
	mise := exec.Command("mise", "which", "python3")
	mise.Env = OriginalEnv()
	if out, err := mise.Output(); err == nil {
		candidates = append(candidates, strings.TrimSpace(string(out)))
	}
	// Trust-free: mise's installs themselves, found through the data dir
	// pinMise set, when every shim and `mise which` refuse.
	if d := os.Getenv("MISE_DATA_DIR"); d != "" {
		found, _ := filepath.Glob(filepath.Join(d, "installs", "python", "*", "bin", "python3"))
		candidates = append(candidates, found...)
	}
	for _, py := range candidates {
		// macOS ships /usr/bin/python3 as a stub that opens the Xcode
		// command line tools installer unless they are there.
		if py == "/usr/bin/python3" && runtime.GOOS == "darwin" && exec.Command("xcode-select", "-p").Run() != nil {
			continue
		}
		if fi, err := os.Stat(py); err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := exec.CommandContext(ctx, py, "-c", "pass").Run()
		cancel()
		if err == nil {
			return py
		}
	}
	t.Skip("no working python3: none on PATH runs, and `mise which python3` found none")
	return ""
}
