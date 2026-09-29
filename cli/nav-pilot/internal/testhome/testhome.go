// Package testhome keeps a test binary out of the developer's real home.
//
// Every test package in nav-pilot calls Run from its TestMain. Run points HOME,
// the XDG directories and NAV_PILOT_CONFIG at a fresh temporary directory for
// the whole binary, so a test that forgets isolatedConfig(t) writes there and
// not in ~/.nav-pilot, ~/.copilot or ~/.config (#627, #1063). This is only
// the floor: a test that sets HOME itself must set NAV_PILOT_CONFIG too, or it
// reads and writes the config Run chose; isolatedConfig(t) does both.
//
// Run also guards the real home, in two ways. It fails the run if this test
// binary opened or statted anything in the watched trees under the real home:
// Go's test log (-test.testlogfile) records every path the process itself
// touches through package os, so that check cannot be tripped by another
// process. And it compares the watched trees before and after: a change this
// binary did not touch is most likely a real nav-pilot, Copilot session or
// sync running alongside the tests (#1317). Locally that is a warning; in CI,
// where nothing else writes to the home, it fails the run.
package testhome

import (
	"bufio"
	"context"
	"flag"
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
// What the test log does not see locally: a child process started with the
// real HOME (only testhome.OriginalEnv gives one), and an os.Remove, Rename,
// Mkdir or Chtimes into the real home with no open or stat there first. Those
// still show up in the before-and-after comparison: a warning locally, a
// failure in CI. NAV_PILOT_TESTHOME_GUARD=0 turns both checks off; the
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

	logFile := testLogFile(tmp)
	wd, _ := os.Getwd()
	code := m.Run()

	if os.Getenv("NAV_PILOT_TESTHOME_GUARD") == "0" {
		return code
	}
	if p := touched(logFile, realHome, wd); p != "" {
		fmt.Fprintf(os.Stderr, "testhome: a test in this package opened %s in the real home.\n"+
			"Give the test its own HOME and NAV_PILOT_CONFIG (in package cli: isolatedConfig(t)).\n", p)
		return 1
	}
	p := diff(before, snapshot(realHome))
	if p == "" {
		return code
	}
	window := fmt.Sprintf("between %s and %s", start.Format(time.TimeOnly), time.Now().Format(time.TimeOnly))
	if os.Getenv("CI") == "" && logFile != "" {
		fmt.Fprintf(os.Stderr, "testhome: warning: %s in the real home changed %s.\n"+
			"This test binary did not open it, so it was most likely another process, such as a real nav-pilot.\n"+
			"CI fails on it, since nothing else writes to the home there.\n", p, window)
		return code
	}
	fmt.Fprintf(os.Stderr, "testhome: a test in this package, or a process it started, wrote %s\n"+
		"in the real home %s.\n"+
		"Give the test its own HOME and NAV_PILOT_CONFIG (in package cli: isolatedConfig(t)).\n",
		p, window)
	return 1
}

// testLogFile makes sure the testing package writes its test log, the list of
// files this process opens and stats, and returns where. `go test` passes
// -test.testlogfile itself when the result can be cached; otherwise the log
// goes to dir. "" when the flag is missing, and the guard falls back to the
// before-and-after comparison alone.
func testLogFile(dir string) string {
	if !flag.Parsed() {
		flag.Parse()
	}
	f := flag.Lookup("test.testlogfile")
	if f == nil {
		return ""
	}
	if f.Value.String() == "" {
		if err := f.Value.Set(filepath.Join(dir, "testlog.txt")); err != nil {
			return ""
		}
	}
	return f.Value.String()
}

// touched returns a path in a watched tree under home that the test log shows
// this process opened, statted or entered, or "". The log holds each path as
// the caller passed it, so it is cleaned, and a relative one is resolved
// against the directory of the last chdir line, or wd before any.
func touched(logFile, home, wd string) string {
	if logFile == "" || home == "" {
		return ""
	}
	f, err := os.Open(logFile)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		op, p, _ := strings.Cut(sc.Text(), " ")
		if op != "open" && op != "stat" && op != "chdir" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(wd, p)
		}
		p = filepath.Clean(p)
		if op == "chdir" {
			wd = p
		}
		for _, rel := range watched {
			root := filepath.Join(home, rel)
			if p == root || strings.HasPrefix(p, root+string(filepath.Separator)) {
				return p
			}
		}
	}
	return ""
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

// WriteExec writes an executable script for a test to run, already past the
// slow first exec. On macOS the first exec of a new file waits for a security
// check (XProtect or an endpoint agent; which one was not pinned down, the
// laptop measured runs CrowdStrike Falcon). The checks queue: 40 new scripts
// started at once took 2.1s median and up to 4.1s to start, against about
// 0.13s one at a time, and under a full parallel test run that went past the
// 2s deadlines nav-pilot puts on version probes and cplt calls. So the file is
// first written as a script that does nothing and run once, then overwritten
// in place with the real one. The check is not repeated for the rewritten
// file (the same 40 then started in about 0.12s), and the real script never
// runs outside the test. hack/probes/first-exec.go measures this.
func WriteExec(path, script string) error {
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		return err
	}
	if err := exec.Command(path).Run(); err != nil {
		return fmt.Errorf("warming %s: %w", path, err)
	}
	return os.WriteFile(path, []byte(script), 0o755)
}
