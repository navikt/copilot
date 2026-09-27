// Package testhome keeps a test binary out of the developer's real home.
//
// Every test package in nav-pilot calls Run from its TestMain. Run points HOME,
// the XDG directories and NAV_PILOT_CONFIG at a fresh temporary directory for
// the whole binary, so a test that forgets isolatedConfig(t) writes there and
// not in ~/.nav-pilot, ~/.copilot or ~/.config (#627, #1063). A test that sets
// its own HOME with t.Setenv still does so; this is only the floor.
//
// Run also guards the real home: it records the files nav-pilot owns there
// before the tests and fails the run, naming the path, if any of them changed.
package testhome

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Vars are the variables Run redirects. HOME is the one that matters, since
// nav-pilot resolves everything through os.UserHomeDir; the rest cover the
// opencode paths and os.UserConfigDir/UserCacheDir on Linux.
var Vars = []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "NAV_PILOT_CONFIG"}

// watched are the paths under the real home that a test run must leave alone.
// They are nav-pilot's own files, not ~/.copilot as a whole: Copilot CLI
// rewrites its own files there all day, so watching the directory would fail
// every run made while an agent is working.
var watched = []string{
	".nav-pilot",
	".nav-pilot/cache.json",
	".nav-pilot/config.toml",
	".nav-pilot/device-id",
	".nav-pilot/tier-cache.json",
	".copilot/.nav-pilot-state.json",
	".config/opencode",
	".config/opencode/.nav-pilot-state.json",
}

// original holds each Var's value before Run replaced it; ok is false when it
// was unset.
var original = map[string]struct {
	val string
	ok  bool
}{}

// Run isolates the home, runs the tests and returns the exit code for os.Exit.
//
// A real nav-pilot running at the same time as the tests can write
// ~/.nav-pilot/cache.json or config.toml and trip the guard. That is rare and
// the message says what to do; a false alarm costs a rerun, a missed write
// costs the developer's setup. NAV_PILOT_TESTHOME_GUARD=0 turns the check
// off for such a run; the redirection stays on.
func Run(m *testing.M) int {
	realHome, _ := os.UserHomeDir()
	before := snapshot(realHome)

	tmp, err := os.MkdirTemp("", "nav-pilot-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testhome:", err)
		return 1
	}
	defer os.RemoveAll(tmp)
	pinMise(realHome)
	for _, k := range Vars {
		v, ok := os.LookupEnv(k)
		original[k] = struct {
			val string
			ok  bool
		}{v, ok}
	}
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
		fmt.Fprintf(os.Stderr, "testhome: this test run changed %s in the real home.\n"+
			"A test wrote outside its temporary HOME; isolate it with t.Setenv(\"HOME\", t.TempDir()).\n"+
			"If a real nav-pilot ran at the same time, rerun, or set NAV_PILOT_TESTHOME_GUARD=0.\n", p)
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

// OriginalEnv is os.Environ with the redirected variables put back, for a
// child such as `go build` that needs the developer's own caches.
func OriginalEnv() []string {
	env := []string{}
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if _, redirected := original[k]; !redirected {
			env = append(env, kv)
		}
	}
	for k, o := range original {
		if o.ok {
			env = append(env, k+"="+o.val)
		}
	}
	return env
}

// snapshot maps each watched path to its mtime and size, or "absent".
func snapshot(home string) map[string]string {
	s := map[string]string{}
	if home == "" {
		return s
	}
	for _, rel := range watched {
		p := filepath.Join(home, rel)
		fi, err := os.Stat(p)
		if err != nil {
			s[p] = "absent"
			continue
		}
		s[p] = fi.ModTime().Format(time.RFC3339Nano) + fmt.Sprintf(" %d", fi.Size())
	}
	return s
}

// diff returns a watched path whose state changed, or "".
func diff(before, after map[string]string) string {
	for p, b := range before {
		if after[p] != b {
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
	for _, py := range candidates {
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
