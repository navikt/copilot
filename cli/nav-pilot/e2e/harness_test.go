// Package e2e drives the real nav-pilot binary against real git repositories.
//
// It exists because the unit suite stubs the one seam every command passes
// through: 113 of its files replace CloneRemoteFn, three use git for real, and
// one runs the binary. That suite could not see any of the defects found on
// 2026-09-08 — composition resolving at install and deleting at sync, a pakke
// installed in its own repo rewriting its reuse declaration, a retired hook
// leaving its activation entry behind — because each of them lives in the
// wiring between components rather than inside one.
//
// The rule here is that nothing is stubbed. The binary is built, the sources
// are git repositories, and the assertions read what is on disk afterwards.
// HOME and NAV_PILOT_CONFIG are redirected per test, so a run can never touch
// the developer's own ~/.copilot or ~/.nav-pilot/config.toml (#627).
package e2e

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	buildOnce sync.Once
	binPath   string
	buildOut  string
	buildErr  error
)

// binary builds nav-pilot once per run and returns its path.
func binary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "nav-pilot-e2e-bin")
		if err != nil {
			buildErr = err
			return
		}
		binPath = filepath.Join(dir, "nav-pilot")
		cmd := exec.Command("go", "build", "-o", binPath, ".")
		cmd.Dir = ".."
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = err
			buildOut = string(out)
		}
	})
	if buildErr != nil {
		t.Fatalf("building nav-pilot: %v\n%s", buildErr, buildOut)
	}
	return binPath
}

// env is one isolated world: a HOME nav-pilot may write to, a config file of
// its own, and whatever repositories the test makes.
type env struct {
	t    *testing.T
	home string
	root string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	return &env{t: t, home: home, root: root}
}

// commandEnv is the sandbox half of the environment every run gets, as a map,
// so a test can assert that the redirection is actually in place rather than
// trusting that it is.
func (e *env) commandEnv() map[string]string {
	return map[string]string{
		"HOME":             e.home,
		"XDG_CONFIG_HOME":  filepath.Join(e.home, ".config"),
		"XDG_DATA_HOME":    filepath.Join(e.home, ".local", "share"),
		"NAV_PILOT_CONFIG": filepath.Join(e.home, "config.toml"),
	}
}

// run executes nav-pilot in dir and returns combined output plus exit code.
func (e *env) run(dir string, args ...string) (string, int) {
	e.t.Helper()
	cmd := exec.Command(binary(e.t), args...)
	cmd.Dir = dir
	// XDG_CONFIG_HOME too: opencode honours it (openCodeConfigDir), so a
	// developer or CI runner that has it set would otherwise send writes
	// outside the sandbox even though HOME points inside it.
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "NAV_PILOT_TELEMETRY=off")
	for k, v := range e.commandEnv() {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		e.t.Fatalf("running nav-pilot %v: %v", args, err)
	}
	return stripANSI(string(out)), code
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// gitRepo makes dir a committed git repository.
func (e *env) gitRepo(dir string) {
	e.t.Helper()
	for _, args := range [][]string{
		{"init", "-q", "."},
		{"add", "-A"},
		{"-c", "user.email=e2e@example.test", "-c", "user.name=e2e", "commit", "-qm", "innhold"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			e.t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}
}

func (e *env) write(path, body string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// pakke lays down a conforming Tier 1 agentpakke and commits it.
func (e *env) pakke(name string, agents ...string) string {
	e.t.Helper()
	dir := filepath.Join(e.root, name)
	manifest := `{"contractVersion":"1","name":"` + name + `","description":"` + name +
		`","layout":{"agents":"agents","skills":"skills"},"clients":{"copilot":{"primaryAgents":["` + agents[0] + `"]}}}`
	e.write(filepath.Join(dir, ".nav-pilot", "agentpakke.json"), manifest)
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		e.t.Fatal(err)
	}
	for _, a := range agents {
		e.write(filepath.Join(dir, "agents", a+".agent.md"),
			"---\nname: "+a+"\ndescription: fra "+name+"\n---\n"+name+"\n")
	}
	e.gitRepo(dir)
	return dir
}

// consumer makes an empty git repository to install into.
func (e *env) consumer(name string) string {
	e.t.Helper()
	dir := filepath.Join(e.root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "-q", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		e.t.Fatalf("git init in %s: %v\n%s", dir, err, out)
	}
	return dir
}

// declareReuse writes the reuse declaration into a pakke repo and commits it.
func (e *env) declareReuse(pakkeDir, baseDir string) {
	e.t.Helper()
	e.write(filepath.Join(pakkeDir, ".nav-pilot", "agentpakke.lock.json"),
		`{"contractVersion":"1","source":"`+baseDir+`"}`)
	for _, args := range [][]string{
		{"add", "-A"},
		{"-c", "user.email=e2e@example.test", "-c", "user.name=e2e", "commit", "-qm", "gjenbruk"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = pakkeDir
		if out, err := cmd.CombinedOutput(); err != nil {
			e.t.Fatalf("git %v i %s: %v\n%s", args, pakkeDir, err, out)
		}
	}
}

// The harness must be able to fail. A test that never exercises the binary
// would make every case below vacuous, so this one asserts the plumbing.
func TestHarnessRunsTheRealBinary(t *testing.T) {
	e := newEnv(t)
	out, code := e.run(e.root, "--version")
	if code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("--version ga kode %d og utdata %q", code, out)
	}
	if _, code := e.run(e.root, "ikke-en-kommando"); code == 0 {
		t.Error("en ukjent kommando ga exit 0, da måler ikke harnessen exit-koder")
	}
	if e.exists(filepath.Join(e.home, ".copilot")) {
		t.Error("~/.copilot fantes før noe var installert")
	}
}

// gitBlobHash is the id git would give this content: sha1 over the blob
// header and the bytes. The retired record is keyed by it.
func gitBlobHash(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(data))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// commit records whatever is in dir right now.
func (e *env) commit(dir, msg string) {
	e.t.Helper()
	for _, args := range [][]string{
		{"add", "-A"},
		{"-c", "user.email=e2e@example.test", "-c", "user.name=e2e", "commit", "-qm", msg},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			e.t.Fatalf("git %v i %s: %v\n%s", args, dir, err, out)
		}
	}
}
