package e2e

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rogpeppe/go-internal/testscript"
)

// update rewrites the expected files in a script (the `-- stdout.golden --`
// sections a `cmp stdout stdout.golden` compares against) from what the binary
// printed. Review the diff before committing it: a rewrite records whatever
// the binary does now, including a regression.
//
//	go test ./e2e -run TestScripts -update
var update = flag.Bool("update", false, "rewrite cmp targets in e2e/testdata/script from actual output")

// fakeMLXEnv makes the test binary serve as a fake mlx-lm server instead of
// running tests. The fake is a separate process on purpose: nav-pilot only
// talks to a server whose recorded pid is the one listening on the port, and
// `restart` signals that pid, which must never be the test run itself.
const fakeMLXEnv = "NAV_PILOT_E2E_FAKE_MLX"

func TestMain(m *testing.M) {
	if os.Getenv(fakeMLXEnv) == "1" {
		serveFakeMLX()
		return
	}
	if filepath.Base(os.Args[0]) == ptyRunName {
		os.Exit(ptyRun(os.Args[1:]))
	}
	os.Exit(m.Run())
}

// TestScripts runs every journey in testdata/script against the real binary.
// Each script gets its own $WORK with HOME, the nav-pilot config and the
// Hugging Face cache inside it, so a journey sees only the files its archive
// lays down. See README.md for how to add one.
func TestScripts(t *testing.T) {
	bin := binary(t)
	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/script",
		UpdateScripts:       *update,
		RequireExplicitExec: true,
		// [pty]: ttyin needs /dev/ptmx. Scripts skip on it rather than fail
		// where a container or sandbox withholds one.
		Condition: func(cond string) (bool, error) {
			if cond != "pty" {
				return false, fmt.Errorf("unknown condition %q", cond)
			}
			f, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
			if err != nil {
				return false, nil
			}
			return true, f.Close()
		},
		Setup: func(e *testscript.Env) error {
			home := filepath.Join(e.WorkDir, "home")
			e.Setenv("HOME", home)
			e.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			e.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
			e.Setenv("NAV_PILOT_CONFIG", filepath.Join(home, "config.toml"))
			e.Setenv("HF_HOME", filepath.Join(home, ".cache", "huggingface"))
			e.Setenv("NO_COLOR", "1")
			e.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "false")
			e.Setenv("DO_NOT_TRACK", "1")
			// Bench-only overrides (internal/local/bench.go): a developer's
			// shell must not change what any other journey sees.
			e.Setenv("NAV_PILOT_BENCH_MANIFEST", "")
			e.Setenv("NAV_PILOT_BENCH_ALLOW_ORGS", "")
			// A proxy that refuses every connection. Go's HTTP client sends
			// all non-loopback requests through it, so the manifest fetch,
			// update checks and anything else that would reach a real service
			// fail at once instead of leaving the machine; the fakes on
			// 127.0.0.1 are exempt and still answer.
			e.Setenv("HTTP_PROXY", "http://127.0.0.1:9")
			e.Setenv("HTTPS_PROXY", "http://127.0.0.1:9")
			// Set, not inherited: a bypass list naming a real host would let
			// that host through. Go never proxies loopback anyway.
			e.Setenv("NO_PROXY", "127.0.0.1,localhost")
			e.Setenv("no_proxy", "127.0.0.1,localhost")
			e.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+e.Getenv("PATH"))
			return os.MkdirAll(home, 0o755)
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"exits":     cmdExits,
			"validjson": cmdValidJSON,
			"fake-mlx":  cmdFakeMLX,
			"fake-bin":  cmdFakeBin,
			"fake-gh":   cmdFakeGH,
		},
	})
}

// exits [-within DUR] CODE PROG [ARGS...] runs PROG like exec and asserts its
// exact exit code, which exec cannot: it knows only zero and non-zero, and
// decide gives 1 and 2 different meanings. With -within, it also fails when
// the run took longer than DUR, for journeys that must fail fast.
func cmdExits(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("unsupported: ! exits (name the code instead)")
	}
	var within time.Duration
	if len(args) > 1 && args[0] == "-within" {
		d, err := time.ParseDuration(args[1])
		ts.Check(err)
		within, args = d, args[2:]
	}
	if len(args) < 2 {
		ts.Fatalf("usage: exits [-within DUR] CODE PROG [ARGS...]")
	}
	want, err := strconv.Atoi(args[0])
	ts.Check(err)
	start := time.Now()
	err = ts.Exec(args[1], args[2:]...)
	took := time.Since(start)
	got := 0
	var ee *exec.ExitError
	switch {
	case errors.As(err, &ee):
		got = ee.ExitCode()
	case err != nil:
		ts.Fatalf("%v", err)
	}
	if got != want {
		ts.Fatalf("exit code %d, want %d", got, want)
	}
	if within > 0 && took > within {
		ts.Fatalf("took %s, want under %s", took.Round(time.Millisecond), within)
	}
}

// fake-bin NAME... replaces PATH with a directory holding a recording fake for
// each NAME (cplt, copilot, opencode, pi), git, and nav-pilot, and nothing
// else, so a real client on the developer's PATH can never start. Each fake
// appends its arguments, one per line and then "---", to $WORK/fake/NAME.log.
// A trailing --version prints a version; anything else exits 0 without doing
// anything. A client left out of NAME is missing, which is how a journey tests
// a machine without cplt.
func cmdFakeBin(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) == 0 {
		ts.Fatalf("usage: fake-bin NAME...")
	}
	dir := ts.MkAbs("fake")
	ts.Check(os.MkdirAll(dir, 0o755))
	for _, name := range args {
		version := "GitHub Copilot CLI 1.0.40"
		if name == "cplt" {
			version = "cplt 2026.09.24-192459-38642b4"
		}
		script := "#!/bin/sh\n" +
			"printf '%s\\n' \"$@\" --- >> '" + filepath.Join(dir, name+".log") + "'\n" +
			"for a in \"$@\"; do last=$a; done\n" +
			"[ \"$last\" = --version ] && echo '" + version + "'\n" +
			"exit 0\n"
		ts.Check(os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))
	}
	git, err := exec.LookPath("git")
	ts.Check(err)
	_ = os.Remove(filepath.Join(dir, "git"))
	ts.Check(os.Symlink(git, filepath.Join(dir, "git")))
	ts.Setenv("PATH", dir+string(os.PathListSeparator)+filepath.Dir(binPath))
}

// validjson FILE asserts that FILE (or stdout/stderr) is exactly one JSON value.
func cmdValidJSON(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: validjson FILE")
	}
	dec := json.NewDecoder(strings.NewReader(ts.ReadFile(args[0])))
	var v any
	err := dec.Decode(&v)
	if err == nil {
		// More() is no end-of-input check at the top level (it is false
		// before a stray "}"), so ask for a second value and want EOF.
		if err = dec.Decode(&v); err == io.EOF {
			err = nil
		} else if err == nil {
			err = errors.New("more than one JSON value")
		}
	}
	if neg != (err != nil) {
		ts.Fatalf("%s: valid JSON = %v, want %v (%v)", args[0], err == nil, !neg, err)
	}
}

// fake-mlx [MODEL] starts a fake mlx-lm server in its own process and records
// it in $HOME/.nav-pilot/local/server.json the way `alpha local start` would,
// so status, use and decide treat it as nav-pilot's running server. It exports
// FAKE_MLX_URL. MODEL defaults to the manifest's default model.
func cmdFakeMLX(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) > 1 {
		ts.Fatalf("usage: fake-mlx [MODEL]")
	}
	model := "mlx-community/Qwen3.6-35B-A3B-OptiQ-4bit"
	if len(args) == 1 {
		model = args[0]
	}
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), fakeMLXEnv+"=1")
	// Its own process group: nav-pilot signals the recorded server's group,
	// and that must not reach go test.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out, err := cmd.StdoutPipe()
	ts.Check(err)
	ts.Check(cmd.Start())
	ts.Defer(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	var port int
	if _, err := fmt.Fscan(out, &port); err != nil {
		ts.Fatalf("fake mlx-lm did not report a port: %v", err)
	}
	lstart, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(cmd.Process.Pid)).Output()
	ts.Check(err)
	state, _ := json.Marshal(map[string]any{
		"pid":     cmd.Process.Pid,
		"model":   model,
		"started": time.Now(),
		"port":    port,
		"lstart":  strings.Join(strings.Fields(string(lstart)), " "),
	})
	dir := filepath.Join(ts.Getenv("HOME"), ".nav-pilot", "local")
	ts.Check(os.MkdirAll(dir, 0o755))
	ts.Check(os.WriteFile(filepath.Join(dir, "server.json"), state, 0o644))
	ts.Setenv("FAKE_MLX_URL", fmt.Sprintf("http://127.0.0.1:%d", port))
}

// serveFakeMLX answers /v1/chat/completions the way mlx-lm does for decide's
// one-token logprobs request: most of the mass on "A", the rest on "B". It
// mirrors fakeDecideServer in internal/cli, which lives in a test file of
// another package and cannot be imported. The port goes to stdout for
// fake-mlx to read.
func serveFakeMLX() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(ln.Addr().(*net.TCPAddr).Port)
	_ = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		top := []map[string]any{{"token": "A", "logprob": -0.1}, {"token": "B", "logprob": -2.4}}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]any{"role": "assistant", "content": "A"},
				"finish_reason": "length",
				"logprobs":      map[string]any{"content": []any{map[string]any{"token": "A", "logprob": -0.1, "top_logprobs": top}}},
			}},
			"usage": map[string]any{"prompt_tokens": 50, "completion_tokens": 1},
		})
	}))
}
