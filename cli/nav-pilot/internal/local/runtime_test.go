package local

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fakes. Nothing in this file touches the network, spawns a process, or writes
// outside a temp dir — the same rule local_test.go's stubFetch keeps, and the
// reason every external call in runtime.go is a package-level func var.
// ---------------------------------------------------------------------------

// stubDirs points the data directory and the Hugging Face cache at temp dirs.
func stubDirs(t *testing.T) (data, hf string) {
	t.Helper()
	data, hf = t.TempDir(), t.TempDir()
	origData, origHF := dataDir, hfHome
	dataDir = func() string { return data }
	hfHome = func() string { return hf }
	t.Cleanup(func() { dataDir, hfHome = origData, origHF })
	return data, hf
}

// runCall is one command a test's fake runCommand was asked to run.
type runCall struct {
	name string
	args []string
	env  []string
}

// stubRun replaces the command seam and records every call.
func stubRun(t *testing.T, fn func(name string, args []string) (string, error)) *[]runCall {
	t.Helper()
	var mu sync.Mutex
	calls := []runCall{}
	orig := runCommand
	runCommand = func(_ context.Context, name string, args, env []string) (string, error) {
		mu.Lock()
		calls = append(calls, runCall{name: name, args: slices.Clone(args), env: env})
		mu.Unlock()
		return fn(name, args)
	}
	t.Cleanup(func() { runCommand = orig })
	return &calls
}

// ranWith reports whether any recorded call carried the given substring in its
// argument vector.
func ranWith(calls []runCall, want string) bool {
	for _, c := range calls {
		if strings.Contains(c.name+" "+strings.Join(c.args, " "), want) {
			return true
		}
	}
	return false
}

// fakeUVEnv is the state-driven fake for the provisioning path: the fake
// commands create the files a real uv would create, so the version probes that
// follow answer from the filesystem instead of from a scripted sequence. A
// scripted sequence would pass even if EnsureEnv probed in the wrong order.
func fakeUVEnv(t *testing.T, pythonReports string) *[]runCall {
	t.Helper()
	asset, err := uvAsset()
	if err != nil {
		t.Fatalf("uvAsset() on this platform: %v", err)
	}
	origDownload := downloadFile
	downloadFile = func(_ context.Context, _, dest string) error {
		return os.WriteFile(dest, []byte("tarball"), 0o644)
	}
	t.Cleanup(func() { downloadFile = origDownload })

	return stubRun(t, func(name string, args []string) (string, error) {
		exists := func(p string) bool { _, err := os.Stat(p); return err == nil }
		switch {
		case name == "tar":
			// -xzf <tarball> -C <dir>: lay down <dir>/<asset>/uv.
			dir := args[len(args)-1]
			if err := os.MkdirAll(filepath.Join(dir, asset), 0o755); err != nil {
				return "", err
			}
			return "", os.WriteFile(filepath.Join(dir, asset, "uv"), []byte("#!uv"), 0o755)
		case name == venvBin("python"):
			if !exists(name) {
				return "", errors.New("no such file or directory")
			}
			return "Python " + pythonReports + "\n", nil
		case name == uvPath() && len(args) > 0 && args[0] == "--version":
			if !exists(name) {
				return "", errors.New("no such file or directory")
			}
			return "uv " + uvVersion + "\n", nil
		case name == uvPath() && len(args) > 0 && args[0] == "venv":
			if err := os.MkdirAll(filepath.Join(venvPath(), "bin"), 0o755); err != nil {
				return "", err
			}
			return "", os.WriteFile(venvBin("python"), []byte("#!python"), 0o755)
		case name == uvPath() && len(args) > 0 && args[0] == "pip":
			return "", nil
		}
		return "", errors.New("unexpected command " + name)
	})
}

// fakeProc is a supervised process a test can end on demand, including with a
// signal — which is the whole point, since os.ProcessState cannot be
// constructed to report one.
type fakeProc struct {
	pid  int
	done chan struct{}
	once sync.Once

	mu      sync.Mutex
	info    exitInfo
	signals []os.Signal
}

func newFakeProc() *fakeProc { return &fakeProc{pid: 4242, done: make(chan struct{})} }

func (p *fakeProc) PID() int { return p.pid }

func (p *fakeProc) Signal(sig os.Signal) error {
	p.mu.Lock()
	p.signals = append(p.signals, sig)
	p.mu.Unlock()
	return nil
}

func (p *fakeProc) Wait() exitInfo {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.info
}

func (p *fakeProc) die(info exitInfo) {
	p.once.Do(func() {
		p.mu.Lock()
		p.info = info
		p.mu.Unlock()
		close(p.done)
	})
}

func (p *fakeProc) sentSignals() []os.Signal {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.signals)
}

// stubStart hands Start a fake process instead of spawning one, and records
// the environment it would have been given.
func stubStart(t *testing.T, p proc) *[]runCall {
	t.Helper()
	var mu sync.Mutex
	calls := []runCall{}
	orig := startProcess
	startProcess = func(_ context.Context, name string, args, env []string) (proc, error) {
		mu.Lock()
		calls = append(calls, runCall{name: name, args: slices.Clone(args), env: slices.Clone(env)})
		mu.Unlock()
		if p == nil {
			return nil, errors.New("spawn refused")
		}
		return p, nil
	}
	t.Cleanup(func() { startProcess = orig })
	return &calls
}

// stubCompletion replaces the readiness/health probe. n counts the calls so a
// test can assert readiness kept asking rather than accepting the first
// answer.
func stubCompletion(t *testing.T, fn func(ctx context.Context, n int) (int, error)) *int {
	t.Helper()
	var mu sync.Mutex
	n := 0
	orig := probeCompletion
	probeCompletion = func(ctx context.Context, _, _ string) (int, error) {
		mu.Lock()
		n++
		count := n
		mu.Unlock()
		return fn(ctx, count)
	}
	t.Cleanup(func() { probeCompletion = orig })
	return &n
}

// fastTimers shrinks every wall-clock budget so the lifecycle tests take
// milliseconds.
func fastTimers(t *testing.T) {
	t.Helper()
	origReady, origPoll, origHealth, origStop := readyTimeout, readyPollInterval, healthTimeout, stopGrace
	readyTimeout = 500 * time.Millisecond
	readyPollInterval = time.Millisecond
	healthTimeout = 50 * time.Millisecond
	stopGrace = 20 * time.Millisecond
	t.Cleanup(func() {
		readyTimeout, readyPollInterval, healthTimeout, stopGrace = origReady, origPoll, origHealth, origStop
	})
}

// testModel is a manifest entry shaped like the real one, with the params the
// wired-limit and launch paths read.
func testModel() Model {
	return Model{
		Key:       "qwen",
		Name:      "A Model",
		Model:     okModel,
		Backend:   "mlx-lm",
		WeightsGB: 25,
		MinRAMGB:  48,
		Params: map[string]string{
			"MLX_MODEL":            okModel,
			"MLX_OPENCODE_CONTEXT": "65536",
			"MLX_MAX_TOKENS":       "32768",
		},
	}
}

// ---------------------------------------------------------------------------
// Environment
// ---------------------------------------------------------------------------

func TestEnvStatus(t *testing.T) {
	tests := []struct {
		name string
		// seed lays down whatever the case wants present before the probe.
		seed        func(t *testing.T)
		uv, python  string
		wantReady   bool
		wantMissing []string // substrings, one per expected Missing entry
	}{
		{
			name:        "nothing is installed",
			wantReady:   false,
			wantMissing: []string{"uv", "virtual environment", "mlx-lm"},
		},
		{
			// A uv that runs but is not the pinned one is a worse failure than
			// a missing one, so it has to be named as missing.
			name:        "a uv at the wrong version counts as missing",
			uv:          "0.4.0",
			wantMissing: []string{"uv", "virtual environment", "mlx-lm"},
		},
		{
			name:        "a Python 3.13 environment counts as missing",
			uv:          uvVersion,
			python:      "3.13.1",
			wantMissing: []string{"virtual environment", "mlx-lm"},
		},
		{
			name:      "a fully provisioned environment is ready",
			uv:        uvVersion,
			python:    "3.12.8",
			seed:      func(t *testing.T) { mustWriteStamp(t) },
			wantReady: true,
		},
		{
			// The interpreter is the gate, not the package stamp: a stamp
			// written against a 3.12 environment says nothing once the
			// environment is a 3.13 one.
			name:        "packages installed but the interpreter moved",
			uv:          uvVersion,
			python:      "3.13.1",
			seed:        func(t *testing.T) { mustWriteStamp(t) },
			wantMissing: []string{"virtual environment"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubDirs(t)
			stubRun(t, func(name string, args []string) (string, error) {
				switch name {
				case uvPath():
					if tc.uv == "" {
						return "", errors.New("no such file")
					}
					return "uv " + tc.uv + "\n", nil
				case venvBin("python"):
					if tc.python == "" {
						return "", errors.New("no such file")
					}
					return "Python " + tc.python + "\n", nil
				}
				return "", errors.New("unexpected command " + name)
			})
			if tc.seed != nil {
				tc.seed(t)
			}

			env := EnvStatus()
			if env.Ready != tc.wantReady {
				t.Errorf("EnvStatus().Ready = %t, want %t (missing: %v)", env.Ready, tc.wantReady, env.Missing)
			}
			if len(env.Missing) != len(tc.wantMissing) {
				t.Fatalf("EnvStatus().Missing = %v, want %d entries %v", env.Missing, len(tc.wantMissing), tc.wantMissing)
			}
			for i, want := range tc.wantMissing {
				if !strings.Contains(env.Missing[i], want) {
					t.Errorf("EnvStatus().Missing[%d] = %q, want it to mention %q", i, env.Missing[i], want)
				}
			}
		})
	}
}

func mustWriteStamp(t *testing.T) {
	t.Helper()
	if err := writeStamp(); err != nil {
		t.Fatalf("seeding the environment stamp: %v", err)
	}
}

// EnsureEnv on a machine with no uv, no Python and no packages has to end with
// all three, at the pinned versions, and must not need Homebrew or a system
// Python to get there.
func TestEnsureEnvProvisionsAMissingEnvironment(t *testing.T) {
	stubDirs(t)
	calls := fakeUVEnv(t, "3.12.8")

	if got := EnvStatus(); got.Ready {
		t.Fatal("EnvStatus() was ready before EnsureEnv ran")
	}
	if err := EnsureEnv(context.Background()); err != nil {
		t.Fatalf("EnsureEnv() errored: %v", err)
	}
	if got := EnvStatus(); !got.Ready {
		t.Errorf("EnvStatus() after EnsureEnv = %+v, want ready", got)
	}
	// The pins are the point of the whole step, so they are asserted on the
	// argument vectors and not only on the outcome.
	for _, want := range []string{"--python " + pythonVersion, "mlx-lm==" + mlxLMVersion, "mlx==" + mlxVersion} {
		if !ranWith(*calls, want) {
			t.Errorf("EnsureEnv() ran %v, want a command carrying %q", *calls, want)
		}
	}

	// Called again on a provisioned machine it must install nothing: this runs
	// on every launch.
	before := len(*calls)
	if err := EnsureEnv(context.Background()); err != nil {
		t.Fatalf("second EnsureEnv() errored: %v", err)
	}
	for _, c := range (*calls)[before:] {
		if len(c.args) > 0 && c.args[0] != "--version" {
			t.Errorf("second EnsureEnv() ran %s %v, want version probes only", c.name, c.args)
		}
	}
}

// The pin exists because mlx publishes no cp313 wheel and its source build
// fails. An environment on the wrong interpreter must be refused before
// anything is installed into it, and the refusal has to say why — an
// unexplained "use 3.12" is an instruction people route around.
func TestEnsureEnvRefusesAWrongInterpreter(t *testing.T) {
	tests := []struct {
		name   string
		python string
		isErr  bool
	}{
		{name: "the pinned interpreter is accepted", python: "3.12.8"},
		{name: "a patch release of the pin is accepted", python: "3.12.0"},
		{name: "3.13 is refused", python: "3.13.1", isErr: true},
		{name: "3.9 is refused", python: "3.9.18", isErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := stubDirs(t)
			// A venv already exists, built by hand or by an older nav-pilot.
			if err := os.MkdirAll(filepath.Join(data, "venv", "bin"), 0o755); err != nil {
				t.Fatalf("seeding a venv: %v", err)
			}
			if err := os.WriteFile(venvBin("python"), []byte("#!python"), 0o755); err != nil {
				t.Fatalf("seeding a venv: %v", err)
			}
			calls := fakeUVEnv(t, tc.python)

			err := EnsureEnv(context.Background())
			if !tc.isErr {
				if err != nil {
					t.Fatalf("EnsureEnv() errored: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("EnsureEnv() accepted the environment, want a refusal")
			}
			for _, want := range []string{tc.python, pythonVersion, "cp312"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("EnsureEnv() error = %q, want it to mention %q", err, want)
				}
			}
			// Refused before installing: a wheel set inside a broken
			// environment is minutes of download for nothing.
			if ranWith(*calls, "pip") {
				t.Errorf("EnsureEnv() installed packages into a refused environment: %v", *calls)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Weights
// ---------------------------------------------------------------------------

func TestWeightsPresent(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string // path under the hub dir -> contents
		want  bool
	}{
		{name: "nothing in the cache", want: false},
		{
			name: "a complete snapshot",
			files: map[string]string{
				"snapshots/abc123/config.json":       "{}",
				"snapshots/abc123/model.safetensors": "weights",
			},
			want: true,
		},
		{
			// The case a directory-exists check gets wrong: the snapshot is
			// there, the blobs are not, and mlx-lm discovers it minutes later.
			name: "an interrupted download",
			files: map[string]string{
				"snapshots/abc123/config.json":       "{}",
				"snapshots/abc123/model.safetensors": "weights",
				"blobs/deadbeef.incomplete":          "partial",
			},
			want: false,
		},
		{
			name:  "metadata without weights",
			files: map[string]string{"snapshots/abc123/config.json": "{}"},
			want:  false,
		},
		{
			name:  "an empty snapshot directory",
			files: map[string]string{"snapshots/abc123/.keep": ""},
			want:  false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubDirs(t)
			dir := modelCacheDir(okModel)
			for rel, contents := range tc.files {
				path := filepath.Join(dir, rel)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("seeding the cache: %v", err)
				}
				if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
					t.Fatalf("seeding the cache: %v", err)
				}
			}
			got, err := WeightsPresent(okModel)
			if err != nil {
				t.Fatalf("WeightsPresent() errored: %v", err)
			}
			if got != tc.want {
				t.Errorf("WeightsPresent() = %t, want %t", got, tc.want)
			}
		})
	}
}

// The three-host requirement is not folklore: behind a TLS-inspecting proxy
// huggingface.co answers and the two xethub hosts hang, so the failure a
// developer sees carries no hint at all unless the error names them.
func TestDownloadWeightsErrorNamesTheHostsItNeeds(t *testing.T) {
	stubDirs(t)
	orig := runStreaming
	runStreaming = func(context.Context, string, []string, []string, func(string)) error {
		return errors.New("timed out")
	}
	t.Cleanup(func() { runStreaming = orig })

	err := DownloadWeights(context.Background(), okModel, nil)
	if err == nil {
		t.Fatal("DownloadWeights() returned no error")
	}
	for _, host := range []string{"huggingface.co", "cas-server.xethub.hf.co", "transfer.xethub.hf.co"} {
		if !strings.Contains(err.Error(), host) {
			t.Errorf("DownloadWeights() error = %q, want it to name %q", err, host)
		}
	}
}

func TestDownloadWeightsReportsProgressToTheCallback(t *testing.T) {
	stubDirs(t)
	orig := runStreaming
	runStreaming = func(_ context.Context, _ string, _ []string, _ []string, onLine func(string)) error {
		onLine("Fetching 12 files:  50%")
		onLine("Fetching 12 files: 100%")
		return nil
	}
	t.Cleanup(func() { runStreaming = orig })

	var lines []string
	if err := DownloadWeights(context.Background(), okModel, func(s string) { lines = append(lines, s) }); err != nil {
		t.Fatalf("DownloadWeights() errored: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("progress callback got %v, want both lines", lines)
	}
}

// ---------------------------------------------------------------------------
// Server lifecycle
// ---------------------------------------------------------------------------

// Readiness is a completion, not a bind. A server that has bound the port has
// not necessarily mapped the weights, and a benchmark that starts on the bind
// charges model loading to its first request.
func TestStartWaitsForARealCompletion(t *testing.T) {
	stubDirs(t)
	fastTimers(t)
	p := newFakeProc()
	starts := stubStart(t, p)
	// The port is open from the first probe, but nothing generates until the
	// third: bind, then load, then answer.
	probes := stubCompletion(t, func(_ context.Context, n int) (int, error) {
		if n < 3 {
			return 0, errors.New("connection refused")
		}
		return 1, nil
	})

	var s Server
	if err := s.Start(context.Background(), testModel()); err != nil {
		t.Fatalf("Start() errored: %v", err)
	}
	t.Cleanup(func() { p.die(exitInfo{}); s.Stop() })

	if *probes < 3 {
		t.Errorf("Start() returned after %d probes, want it to keep asking until a completion answered", *probes)
	}
	if got := s.Status().Health; got != HealthReady {
		t.Errorf("Status().Health = %q, want %q", got, HealthReady)
	}
	// The manifest params reach the server as environment, unchanged.
	if len(*starts) != 1 {
		t.Fatalf("Start() spawned %d processes, want 1", len(*starts))
	}
	if !slices.Contains((*starts)[0].env, "MLX_OPENCODE_CONTEXT=65536") {
		t.Error("Start() did not pass the manifest MLX_* params as environment")
	}
	if !slices.Contains((*starts)[0].args, "8080") {
		t.Errorf("Start() args = %v, want the default port", (*starts)[0].args)
	}
}

func TestStartTimesOutWithoutACompletion(t *testing.T) {
	stubDirs(t)
	fastTimers(t)
	p := newFakeProc()
	stubStart(t, p)
	stubCompletion(t, func(context.Context, int) (int, error) {
		return 0, errors.New("connection refused")
	})

	var s Server
	err := s.Start(context.Background(), testModel())
	t.Cleanup(func() { p.die(exitInfo{}); s.Stop() })
	if err == nil {
		t.Fatal("Start() returned nil, want a readiness timeout")
	}
	if !strings.Contains(err.Error(), "completion") {
		t.Errorf("Start() error = %q, want it to say no completion answered", err)
	}
	// The process is alive and unready, which is starting — not crashed.
	if got := s.Status().Health; got != HealthStarting {
		t.Errorf("Status().Health = %q, want %q", got, HealthStarting)
	}
}

// An HTTP 200 with completion_tokens 0 is the leading indicator of the crash
// below. It must not be accepted as readiness, and it must be counted.
func TestZeroCompletionTokensAreNotReadiness(t *testing.T) {
	stubDirs(t)
	fastTimers(t)
	p := newFakeProc()
	stubStart(t, p)
	stubCompletion(t, func(context.Context, int) (int, error) { return 0, nil })

	var s Server
	err := s.Start(context.Background(), testModel())
	t.Cleanup(func() { p.die(exitInfo{}); s.Stop() })
	if err == nil {
		t.Fatal("Start() accepted a 200 with completion_tokens 0 as ready")
	}
	if !strings.Contains(err.Error(), "completion_tokens 0") {
		t.Errorf("Start() error = %q, want it to name the zero-token answer", err)
	}
	st := s.Status()
	if st.ZeroTokenReplies == 0 {
		t.Error("Status().ZeroTokenReplies = 0, want the zero-token answers counted")
	}
	if st.Health == HealthReady {
		t.Error("Status().Health = ready after only zero-token answers")
	}
}

// A signal death is unambiguous — the process is gone, restart it — and is
// counted on its own, because MLX's recursive graph walk can run off a stack
// guard page and take the listening socket with it. Watching the port cannot
// tell that from "not up yet".
func TestCrashIsDetectedAsASignalDeath(t *testing.T) {
	tests := []struct {
		name         string
		exit         exitInfo
		wantSignals  int
		wantSignal   syscall.Signal
		wantExitCode int
	}{
		{
			name:        "EXC_BAD_ACCESS arrives as SIGSEGV",
			exit:        exitInfo{Signal: syscall.SIGSEGV, Code: -1},
			wantSignals: 1,
			wantSignal:  syscall.SIGSEGV,
		},
		{
			name:        "SIGBUS on the stack guard page",
			exit:        exitInfo{Signal: syscall.SIGBUS, Code: -1},
			wantSignals: 1,
			wantSignal:  syscall.SIGBUS,
		},
		{
			// A clean non-zero exit is a crash too, but not a signal death:
			// it is usually the server refusing the model, which a restart
			// will not fix, so it must not inflate the restart counter.
			name:         "a refusal exits with a status, not a signal",
			exit:         exitInfo{Code: 1},
			wantSignals:  0,
			wantExitCode: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubDirs(t)
			fastTimers(t)
			p := newFakeProc()
			stubStart(t, p)
			stubCompletion(t, func(context.Context, int) (int, error) {
				return 0, errors.New("connection refused")
			})
			p.die(tc.exit)

			var s Server
			err := s.Start(context.Background(), testModel())
			if err == nil {
				t.Fatal("Start() returned nil for a server that died")
			}
			if !strings.Contains(err.Error(), "exited before it was ready") {
				t.Errorf("Start() error = %q, want it to say the process died", err)
			}
			st := s.Status()
			if st.Health != HealthCrashed {
				t.Errorf("Status().Health = %q, want %q", st.Health, HealthCrashed)
			}
			if st.SignalDeaths != tc.wantSignals {
				t.Errorf("Status().SignalDeaths = %d, want %d", st.SignalDeaths, tc.wantSignals)
			}
			if st.Signal != tc.wantSignal {
				t.Errorf("Status().Signal = %v, want %v", st.Signal, tc.wantSignal)
			}
			if tc.wantSignals == 0 && st.ExitCode != tc.wantExitCode {
				t.Errorf("Status().ExitCode = %d, want %d", st.ExitCode, tc.wantExitCode)
			}
			if got := s.Health(context.Background()); got != HealthCrashed {
				t.Errorf("Health() = %q, want %q", got, HealthCrashed)
			}
		})
	}
}

// The five states, and in particular the two that look identical from outside:
// a hung server accepts connections and never answers, a crashed one is gone.
// Restarting is right for both, but only one of them is visible on the port.
func TestHealth(t *testing.T) {
	tests := []struct {
		name string
		// start false means Health is asked before anything was started.
		start bool
		// ready runs a successful probe first, so the server has been ready
		// once — which changes what a later failure means.
		ready bool
		// dieWith ends the process before the health probe.
		dieWith *exitInfo
		probe   func(ctx context.Context) (int, error)
		want    Health
	}{
		{
			name: "nothing started",
			want: HealthNotStarted,
		},
		{
			name:  "alive, port not open yet",
			start: true,
			probe: func(context.Context) (int, error) { return 0, errors.New("connection refused") },
			want:  HealthStarting,
		},
		{
			name:  "alive and answering",
			start: true,
			probe: func(context.Context) (int, error) { return 7, nil },
			want:  HealthReady,
		},
		{
			// An exact prompt-cache hit can kill the generation thread while
			// the process stays alive and the socket stays open. Nothing about
			// the port says so.
			name:  "alive, accepting, never answers",
			start: true,
			ready: true,
			probe: func(ctx context.Context) (int, error) { <-ctx.Done(); return 0, ctx.Err() },
			want:  HealthHung,
		},
		{
			name:    "the process is gone",
			start:   true,
			ready:   true,
			dieWith: &exitInfo{Signal: syscall.SIGSEGV, Code: -1},
			probe:   func(context.Context) (int, error) { return 0, errors.New("connection refused") },
			want:    HealthCrashed,
		},
		{
			// Zero tokens from a server that has never answered is still
			// loading as far as anyone can tell; it is counted either way.
			name:  "zero tokens before the first completion",
			start: true,
			probe: func(context.Context) (int, error) { return 0, nil },
			want:  HealthStarting,
		},
		{
			// Once it has been ready, a zero-token answer is degradation, not
			// startup. It stays ready and the counter carries the warning.
			name:  "zero tokens after it has been ready",
			start: true,
			ready: true,
			probe: func(context.Context) (int, error) { return 0, nil },
			want:  HealthReady,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubDirs(t)
			fastTimers(t)
			p := newFakeProc()
			stubStart(t, p)

			// Readiness first, then the case's own probe.
			var s Server
			if tc.start {
				stubCompletion(t, func(context.Context, int) (int, error) {
					if tc.ready {
						return 1, nil
					}
					return 0, errors.New("connection refused")
				})
				err := s.Start(context.Background(), testModel())
				if tc.ready && err != nil {
					t.Fatalf("Start() errored: %v", err)
				}
			}
			if tc.dieWith != nil {
				p.die(*tc.dieWith)
				// The supervisor records the exit asynchronously; wait for it
				// rather than racing it.
				waitFor(t, func() bool { return s.exited() != nil })
			}
			if tc.probe != nil {
				stubCompletion(t, func(ctx context.Context, _ int) (int, error) { return tc.probe(ctx) })
			}
			t.Cleanup(func() { p.die(exitInfo{}); s.Stop() })

			if got := s.Health(context.Background()); got != tc.want {
				t.Errorf("Health() = %q, want %q", got, tc.want)
			}
			if tc.want == HealthReady && tc.probe != nil {
				// The zero-token warning is only useful if it is counted.
				if tokens, _ := tc.probe(context.Background()); tokens == 0 && s.Status().ZeroTokenReplies == 0 {
					t.Error("Status().ZeroTokenReplies = 0 after a zero-token answer")
				}
			}
		})
	}
}

func TestStopSignalsAndIsIdempotent(t *testing.T) {
	stubDirs(t)
	fastTimers(t)
	p := newFakeProc()
	stubStart(t, p)
	stubCompletion(t, func(context.Context, int) (int, error) { return 1, nil })

	var s Server
	if err := s.Start(context.Background(), testModel()); err != nil {
		t.Fatalf("Start() errored: %v", err)
	}
	// The process exits on the term, as a healthy one does.
	go func() { <-time.After(2 * time.Millisecond); p.die(exitInfo{}) }()
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() errored: %v", err)
	}
	if sigs := p.sentSignals(); len(sigs) == 0 || sigs[0] != syscall.SIGTERM {
		t.Errorf("Stop() sent %v, want SIGTERM first", sigs)
	}
	// A deferred Stop after a failed Start must not be a second kill.
	if err := s.Stop(); err != nil {
		t.Errorf("second Stop() errored: %v", err)
	}
	if got := s.Status().Health; got != HealthNotStarted {
		t.Errorf("Status().Health after Stop = %q, want %q", got, HealthNotStarted)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition never became true")
		}
		time.Sleep(time.Millisecond)
	}
}

// ---------------------------------------------------------------------------
// Memory cap
// ---------------------------------------------------------------------------

func TestRequiredWiredLimitGB(t *testing.T) {
	tests := []struct {
		name    string
		weights int
		params  map[string]string
		want    int
	}{
		{
			// The manifest's own tuned entry: 25 GB of weights at 64k context.
			name:    "the manifest entry",
			weights: 25,
			params:  map[string]string{"MLX_OPENCODE_CONTEXT": "65536"},
			want:    36,
		},
		{
			// Same weights, a quarter of the context: the KV budget is what
			// moves, which is the reason this is computed and not a constant.
			name:    "the same weights at 16k context",
			weights: 25,
			params:  map[string]string{"MLX_OPENCODE_CONTEXT": "16384"},
			want:    30,
		},
		{
			name:    "no context param falls back to the token cap",
			weights: 25,
			params:  map[string]string{"MLX_MAX_TOKENS": "32768"},
			want:    32,
		},
		{
			name:    "no params at all still yields a budget",
			weights: 10,
			want:    17,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RequiredWiredLimitGB(Model{WeightsGB: tc.weights, Params: tc.params})
			if got != tc.want {
				t.Errorf("RequiredWiredLimitGB() = %d, want %d", got, tc.want)
			}
		})
	}
}

// The formula is calibrated against a value someone measured. If a manifest
// entry's tuned wired_limit_gb and the computed requirement drift apart, one of
// the two is wrong and this is where it shows up rather than on a workstation.
func TestRequiredWiredLimitMatchesTheManifest(t *testing.T) {
	m, err := Parse(embeddedManifest)
	if err != nil {
		t.Fatalf("Parse(embedded) errored: %v", err)
	}
	for _, model := range m.Models {
		if model.WiredLimitGB == 0 {
			continue
		}
		if got := RequiredWiredLimitGB(model); got != model.WiredLimitGB {
			t.Errorf("RequiredWiredLimitGB(%s) = %d, but the manifest tuned it to %d",
				model.Key, got, model.WiredLimitGB)
		}
	}
}

func TestCheckWiredLimit(t *testing.T) {
	tests := []struct {
		name           string
		weights        int
		context        string
		ramGB          int
		currentMB      string // "" means the sysctl does not exist
		wantRefused    bool
		wantSufficient bool
		wantCurrentGB  int
	}{
		{
			// 36 GB required on a 64 GB machine leaves 28 GB. Fine, and the
			// cap is already set.
			name: "sufficient", weights: 25, context: "65536", ramGB: 64,
			currentMB: "36864", wantSufficient: true, wantCurrentGB: 36,
		},
		{
			// The same machine with the cap still at the default: not
			// refused, just not raised yet. Raising it needs sudo and is a
			// command's job.
			name: "insufficient but raisable", weights: 25, context: "65536", ramGB: 64,
			currentMB: "", wantSufficient: false, wantCurrentGB: 0,
		},
		{
			// 36 GB on a 48 GB machine leaves 12, exactly the reserve.
			name: "exactly at the reserve", weights: 25, context: "65536", ramGB: 48,
			currentMB: "36864", wantSufficient: true, wantCurrentGB: 36,
		},
		{
			// The advice that wrecks workstations: a ~40 GB cap on a 48 GB
			// machine. Refused here rather than discovered when the
			// compositor dies.
			name: "40 GB on a 48 GB machine is refused", weights: 30, context: "65536", ramGB: 48,
			wantRefused: true,
		},
		{
			name: "a large model on a small machine is refused", weights: 25, context: "65536", ramGB: 32,
			wantRefused: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubRun(t, func(name string, args []string) (string, error) {
				if name != "sysctl" {
					return "", errors.New("unexpected command " + name)
				}
				switch args[len(args)-1] {
				case "hw.memsize":
					return memsizeBytes(tc.ramGB), nil
				case "iogpu.wired_limit_mb":
					if tc.currentMB == "" {
						return "", errors.New("unknown oid")
					}
					return tc.currentMB + "\n", nil
				}
				return "", errors.New("unexpected sysctl")
			})

			m := Model{Model: okModel, WeightsGB: tc.weights}
			if tc.context != "" {
				m.Params = map[string]string{"MLX_OPENCODE_CONTEXT": tc.context}
			}
			w, err := CheckWiredLimit(m)
			if tc.wantRefused {
				if err == nil {
					t.Fatalf("CheckWiredLimit() = %+v, want a refusal", w)
				}
				// The refusal has to say what it would have left, or it reads
				// as an arbitrary limit.
				for _, want := range []string{"GB machine", "everything else"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("CheckWiredLimit() error = %q, want it to mention %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("CheckWiredLimit() errored: %v", err)
			}
			if w.Sufficient != tc.wantSufficient {
				t.Errorf("CheckWiredLimit().Sufficient = %t, want %t (%+v)", w.Sufficient, tc.wantSufficient, w)
			}
			if w.CurrentGB != tc.wantCurrentGB {
				t.Errorf("CheckWiredLimit().CurrentGB = %d, want %d", w.CurrentGB, tc.wantCurrentGB)
			}
			if w.MachineRAMGB != tc.ramGB {
				t.Errorf("CheckWiredLimit().MachineRAMGB = %d, want %d", w.MachineRAMGB, tc.ramGB)
			}
			if !strings.Contains(w.Command, "iogpu.wired_limit_mb") {
				t.Errorf("CheckWiredLimit().Command = %q, want the sysctl a developer would run", w.Command)
			}
		})
	}
}

// memsizeBytes renders a GB count as the byte string hw.memsize reports.
func memsizeBytes(gb int) string {
	return strconv.FormatInt(int64(gb)*gib, 10) + "\n"
}
