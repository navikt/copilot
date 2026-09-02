package local

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
// Fakes. Nothing in this file touches the network or writes outside a temp dir
// — the same rule local_test.go's stubFetch keeps, and the reason every
// external call in runtime.go is a package-level func var.
//
// One exception spawns a process: [TestStartProcessSendsTheServerOutputToTheLog]
// runs /bin/sh, because what it pins is that a real child's real stdout and
// stderr land in a real file, and the seam that would let it fake that is the
// exact line the test exists to protect.
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

	// The fake tarball is not the real release, so the pin has to describe the
	// bytes this fake writes. The real sums are pinned by
	// TestUVChecksumsArePinnedForEveryAsset.
	origSums := uvSHA256
	uvSHA256 = map[string]string{asset: fmt.Sprintf("%x", sha256.Sum256([]byte("tarball")))}
	t.Cleanup(func() { uvSHA256 = origSums })

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

// stubPortInUse answers the pre-flight bind check without touching a real
// port, so the lifecycle tests do not depend on what happens to be listening
// on the machine running them — including, as it happens, the stale server
// that made the check necessary.
func stubPortInUse(t *testing.T, inUse bool) {
	t.Helper()
	orig := portInUse
	portInUse = func(int) bool { return inUse }
	t.Cleanup(func() { portInUse = orig })
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
		Key:     "qwen",
		Name:    "A Model",
		Model:   okModel,
		Backend: "mlx-lm",
		// Parse refuses a manifest without exactly one default, so a fixture
		// without one is a shape production never sees. It went unnoticed while
		// selection was Models[0] and every fixture had exactly one model.
		Default:      true,
		WeightsGB:    25,
		MinRAMGB:     48,
		WiredLimitGB: 36,
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

// EnsureEnv on a machine with no uv, no Python and no packages has to end with
// all three, at the pinned versions, and must not need Homebrew or a system
// Python to get there.
func TestEnsureEnvProvisionsAMissingEnvironment(t *testing.T) {
	stubDirs(t)
	calls := fakeUVEnv(t, "3.12.8")

	if err := EnsureEnv(context.Background()); err != nil {
		t.Fatalf("EnsureEnv() errored: %v", err)
	}
	// The pins are the point of the whole step, so they are asserted on the
	// argument vectors and not only on the outcome. The install refuses anything
	// whose bytes are not in the lockfile: without --require-hashes, mlx-lm and
	// mlx are pinned and everything they pull is not.
	for _, want := range []string{"--python " + pythonVersion, "--require-hashes", "requirements.txt"} {
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
	stubPortInUse(t, false)
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
	// The port is whatever the kernel handed out, not 8080: that is the port a
	// developer's own Ktor or Spring service binds, and the two cannot both have
	// it. What matters is that the number the child was given is the number the
	// server reports, so every later reader looks in the right place.
	if s.Port == 0 {
		t.Fatal("Start() left Port unset; nothing downstream can find the server")
	}
	if s.Port == DefaultPort {
		t.Errorf("Start() chose %d, the port a local web service binds", DefaultPort)
	}
	if !slices.Contains((*starts)[0].args, strconv.Itoa(s.Port)) {
		t.Errorf("Start() args = %v, want the chosen port %d", (*starts)[0].args, s.Port)
	}
	if want := fmt.Sprintf("http://127.0.0.1:%d", s.Port); s.URL() != want {
		t.Errorf("URL() = %q, want %q", s.URL(), want)
	}
}

// TestStartRecordsThePortForOtherProcesses: the guard, the ownership check and
// `status` all run in processes that did not start the server, so the port has to
// survive in the state file or they look for it on 8080 and refuse a healthy
// server.
func TestStartRecordsThePortForOtherProcesses(t *testing.T) {
	stubDirs(t)
	stubStart(t, newFakeProc())
	stubCompletion(t, func(context.Context, int) (int, error) { return 1, nil })

	s := &Server{}
	if err := s.Start(context.Background(), testModel()); err != nil {
		t.Fatal(err)
	}
	// Start does not write the state file; the command that owns the lifecycle
	// does. What is pinned here is that the port survives the round trip and
	// that ServerURL, which every other process calls, reports it.
	if err := SaveState(State{PID: s.Status().PID, Model: testModel().Model, Port: s.Port}); err != nil {
		t.Fatal(err)
	}
	st, ok, err := LoadState()
	if err != nil || !ok {
		t.Fatalf("LoadState() = (%v, %v), want a recorded server", ok, err)
	}
	if st.ServerPort() != s.Port {
		t.Errorf("recorded port %d, server is on %d", st.ServerPort(), s.Port)
	}
	if want := fmt.Sprintf("http://127.0.0.1:%d", s.Port); ServerURL() != want {
		t.Errorf("ServerURL() = %q, want %q", ServerURL(), want)
	}
	// A file written before ports were recorded still means 8080.
	if (State{}).ServerPort() != DefaultPort {
		t.Errorf("an unrecorded port must mean %d, for a server started by an older nav-pilot", DefaultPort)
	}
}

func TestStartTimesOutWithoutACompletion(t *testing.T) {
	stubDirs(t)
	fastTimers(t)
	p := newFakeProc()
	stubPortInUse(t, false)
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
	stubPortInUse(t, false)
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

// The failure this whole guard exists for, from the outside: a server nav-pilot
// did not start already holds the port. It was found on a real machine, where a
// stale mlx_lm.server from earlier work still held 8080 — the child could not
// bind and exited in milliseconds, and the stranger answered the readiness
// probe.
//
// Start must refuse before spawning anything, and must name what is there. A
// matching model id would not change the answer: the wired-memory limit, the
// MLX_* params and the context that server runs under are invisible over HTTP,
// so adopting it on an id alone is a guess dressed as a check.
func TestStartRefusesAPortItDoesNotOwn(t *testing.T) {
	stubDirs(t)
	fastTimers(t)

	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/v1/models") {
			fmt.Fprint(w, `{"data":[{"id":"someone-else/model"}]}`)
			return
		}
		fmt.Fprint(w, `{"usage":{"completion_tokens":1}}`)
	}))
	defer foreign.Close()
	port := foreignPort(t, foreign.URL)

	// The real bind check, against a real listener: the seam is left alone
	// here on purpose, because what is being pinned is that a held port is
	// noticed at all.
	starts := stubStart(t, newFakeProc())
	stubCompletion(t, func(context.Context, int) (int, error) { return 1, nil })

	s := &Server{Port: port}
	err := s.Start(context.Background(), testModel())
	if err == nil {
		t.Fatal("Start() returned nil with a foreign server on the port — it adopted a model nav-pilot did not choose")
	}
	for _, want := range []string{strconv.Itoa(port), "Refusing to start"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Start() error = %q, want it to mention %q", err, want)
		}
	}
	// It must NOT name the model the stranger reports. mlx-lm answers /v1/models
	// with the whole Hugging Face cache rather than the loaded model, so that id
	// is an arbitrary pick from someone's disk; naming it sent a developer
	// chasing a model mismatch that did not exist.
	if strings.Contains(err.Error(), "someone-else/model") {
		t.Errorf("Start() error names a model read from /v1/models, which lists the cache: %q", err)
	}
	if len(*starts) != 0 {
		t.Errorf("Start() spawned %d processes onto a port that was already in use", len(*starts))
	}
	// Nothing came up, so nothing may be recorded: a Status with a pid in it is
	// what `stop` and `status` would later report as a crash.
	if st := s.Status(); st.Health != HealthNotStarted || st.PID != 0 {
		t.Errorf("Status() after a refused Start = %+v, want %q and no pid", st, HealthNotStarted)
	}
}

// The same failure from the inside, for the case the pre-flight check cannot
// see: the child exits during the probe — it lost a bind race, or died on
// launch — and something else answers. A completion is evidence that a server
// is on the port, never that it is ours.
func TestStartDoesNotAcceptReadinessFromAStrangerWhenTheChildIsDead(t *testing.T) {
	stubDirs(t)
	fastTimers(t)
	stubPortInUse(t, false)

	p := newFakeProc()
	p.die(exitInfo{Code: 1}) // could not bind; gone before the first probe
	stubStart(t, p)

	var s Server
	stubCompletion(t, func(context.Context, int) (int, error) {
		// The stranger answers only once the exit has been observed, so this
		// pins the check after the probe rather than racing the one before it.
		waitFor(t, func() bool { return s.exited() != nil })
		return 1, nil
	})

	err := s.Start(context.Background(), testModel())
	if err == nil {
		t.Fatal("Start() reported ready from a probe answered while its own process was already dead")
	}
	if !strings.Contains(err.Error(), "exited before it was ready") {
		t.Errorf("Start() error = %q, want it to say the server exited", err)
	}
	if got := s.Status().Health; got == HealthReady {
		t.Errorf("Status().Health = %q after the process exited", got)
	}
}

// foreignPort is the port of a test server, for a Server that has to be pointed
// at it.
func foreignPort(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parsing %s: %v", rawURL, err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("no port in %s: %v", rawURL, err)
	}
	return port
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
			stubPortInUse(t, false)
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
		// since is how long ago the server was started, stated rather than
		// waited out: Start's own wait has already spent the readiness budget
		// by the time it gives up, and the deadline is what tells a cold start
		// from a hang.
		since time.Duration
		probe func(ctx context.Context) (int, error)
		want  Health
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
			// The cold start that must not read as a hang: mlx-lm binds the
			// port before it maps the weights, so a probe during loading is
			// accepted and never answered. A supervisor that restarts on hung
			// would kill-loop a healthy server on every start.
			name:  "never ready, the probe times out while it loads",
			start: true,
			probe: func(ctx context.Context) (int, error) { <-ctx.Done(); return 0, ctx.Err() },
			want:  HealthStarting,
		},
		{
			// Past the readiness budget the same timeout is a hang: nothing is
			// still loading after that long.
			name:  "never ready, the probe still times out past the deadline",
			start: true,
			since: 2 * time.Minute,
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
			stubPortInUse(t, false)
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
				s.mu.Lock()
				s.started = time.Now().Add(-tc.since)
				s.mu.Unlock()
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
	stubPortInUse(t, false)
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

// The wired limit is a measured number the manifest carries, so an entry
// without one is refused rather than run under a guess: the guess is what
// decides whether the machine keeps its compositor.
func TestCheckWiredLimitRefusesAnUnmeasuredModel(t *testing.T) {
	w, err := CheckWiredLimit(Model{Model: okModel, WeightsGB: 25})
	if err == nil {
		t.Fatalf("CheckWiredLimit() = %+v, want a refusal", w)
	}
	for _, want := range []string{"wired_limit_gb", "measured"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("CheckWiredLimit() error = %q, want it to mention %q", err, want)
		}
	}
}

func TestCheckWiredLimit(t *testing.T) {
	tests := []struct {
		name           string
		wiredLimitGB   int
		ramGB          int
		currentMB      string // "" means the sysctl does not exist
		wantRefused    bool
		wantSufficient bool
		wantCurrentGB  int
	}{
		{
			// 36 GB required on a 64 GB machine leaves 28 GB. Fine, and the
			// cap is already set.
			name: "sufficient", wiredLimitGB: 36, ramGB: 64,
			currentMB: "36864", wantSufficient: true, wantCurrentGB: 36,
		},
		{
			// The same machine with the cap still at the default: not
			// refused, just not raised yet. Raising it needs sudo and is a
			// command's job.
			name: "insufficient but raisable", wiredLimitGB: 36, ramGB: 64,
			currentMB: "", wantSufficient: false, wantCurrentGB: 0,
		},
		{
			// 36 GB on a 48 GB machine leaves 12, exactly the reserve.
			name: "exactly at the reserve", wiredLimitGB: 36, ramGB: 48,
			currentMB: "36864", wantSufficient: true, wantCurrentGB: 36,
		},
		{
			// The advice that wrecks workstations: a ~40 GB cap on a 48 GB
			// machine. Refused here rather than discovered when the
			// compositor dies.
			name: "40 GB on a 48 GB machine is refused", wiredLimitGB: 40, ramGB: 48,
			wantRefused: true,
		},
		{
			name: "a large model on a small machine is refused", wiredLimitGB: 36, ramGB: 32,
			wantRefused: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubRun(t, func(name string, args []string) (string, error) {
				if name != sysctlPath {
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

			w, err := CheckWiredLimit(Model{Model: okModel, WeightsGB: 25, WiredLimitGB: tc.wiredLimitGB})
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
			if w.RequiredGB != tc.wiredLimitGB {
				t.Errorf("CheckWiredLimit().RequiredGB = %d, want the manifest's measured %d", w.RequiredGB, tc.wiredLimitGB)
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

// TestUVChecksumsArePinnedForEveryAsset keeps the pin and the asset list from
// drifting apart: a new target in uvAsset with no sum here would fail at
// install time on exactly the machine that has no other way to get uv.
func TestUVChecksumsArePinnedForEveryAsset(t *testing.T) {
	for _, target := range []string{"darwin/arm64", "darwin/amd64", "linux/arm64", "linux/amd64"} {
		if _, ok := uvSHA256[uvAssetFor(target)]; !ok {
			t.Errorf("no pinned sha256 for the uv asset on %s", target)
		}
	}
	for _, sum := range uvSHA256 {
		if len(sum) != 64 {
			t.Errorf("pinned sha256 %q is not 64 hex characters", sum)
		}
	}
}

// TestEnsureUVRefusesATamperedDownload is the point of the pin: a tarball that
// is not the release must never reach tar, chmod +x and exec.
func TestEnsureUVRefusesATamperedDownload(t *testing.T) {
	stubDirs(t)
	origDownload := downloadFile
	downloadFile = func(_ context.Context, _, dest string) error {
		return os.WriteFile(dest, []byte("not the release"), 0o644)
	}
	t.Cleanup(func() { downloadFile = origDownload })
	calls := stubRun(t, func(string, []string) (string, error) { return "", errors.New("nope") })

	err := ensureUV(context.Background())
	if err == nil {
		t.Fatal("ensureUV accepted a tarball that does not match the pinned checksum")
	}
	if !strings.Contains(err.Error(), "pinned checksum") {
		t.Errorf("error does not name the checksum mismatch: %v", err)
	}
	if ranWith(*calls, "tar") {
		t.Error("ensureUV unpacked a tarball it had not verified")
	}
	if _, statErr := os.Stat(uvPath()); statErr == nil {
		t.Error("ensureUV installed a uv binary from an unverified tarball")
	}

}

// TestVerifyUVDownloadRefusesAnUnpinnedAsset: an asset with no entry is
// unverifiable, which is refused rather than waved through.
func TestVerifyUVDownloadRefusesAnUnpinnedAsset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "uv.tar.gz")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := verifyUVDownload(path, "uv-some-future-target")
	if err == nil || !strings.Contains(err.Error(), "no pinned sha256") {
		t.Errorf("verifyUVDownload on an unpinned asset = %v, want a refusal", err)
	}
}

// ---------------------------------------------------------------------------
// The server log
// ---------------------------------------------------------------------------

// TestStartProcessSendsTheServerOutputToTheLog is the ship blocker: without it
// mlx_lm.server's tracebacks, bind errors and OOM messages went nowhere, so
// every crash a maintainer inherited said "crashed" and nothing else.
func TestStartProcessSendsTheServerOutputToTheLog(t *testing.T) {
	stubDirs(t)

	p, err := startProcess(context.Background(), "/bin/sh", []string{"-c", "echo on-stdout; echo on-stderr >&2"}, os.Environ())
	if err != nil {
		t.Fatalf("startProcess: %v", err)
	}
	p.Wait()

	data, err := os.ReadFile(LogPath())
	if err != nil {
		t.Fatalf("reading %s: %v", LogPath(), err)
	}
	for _, want := range []string{"on-stdout", "on-stderr"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("the server log has no %q in it (%q); a crash report built from this has nothing to read", want, data)
		}
	}
}

// TestServerLogKeepsTheLastRunUntilItIsTooBig: a restart must not throw away
// what the previous run printed on its way out — that is usually the whole
// diagnosis — but the file cannot grow without bound either.
func TestServerLogKeepsTheLastRunUntilItIsTooBig(t *testing.T) {
	data, _ := stubDirs(t)
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LogPath(), []byte("what the crashed run printed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := openLog()
	if f == nil {
		t.Fatal("openLog() = nil with a writable data directory")
	}
	f.Close()
	if got, _ := os.ReadFile(LogPath()); !strings.Contains(string(got), "what the crashed run printed") {
		t.Errorf("a restart threw away the previous run's output: %q", got)
	}

	if err := os.WriteFile(LogPath(), bytes.Repeat([]byte("x"), maxLogBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if f := openLog(); f != nil {
		f.Close()
	}
	if fi, err := os.Stat(LogPath()); err != nil || fi.Size() != 0 {
		t.Errorf("the log was %d bytes after a start past the %d-byte cap, want it started over", fi.Size(), maxLogBytes)
	}
}

// TestExitedBeforeReadyNamesTheLog: the message a failed start hands a
// developer has to say where the process's own words are, or the supported way
// to diagnose a crash is a macOS .ips file.
func TestExitedBeforeReadyNamesTheLog(t *testing.T) {
	stubDirs(t)
	err := exitedBeforeReady("mlx-community/x", exitInfo{Signal: syscall.SIGSEGV})
	if !strings.Contains(err.Error(), LogPath()) {
		t.Errorf("exitedBeforeReady() = %q, want it to name %s", err, LogPath())
	}
}

// TestStartStopsTheChildWhenInterrupted: a cancelled context must not leave a
// server behind.
//
// A cold start takes minutes, so Ctrl-C during one is the normal case rather than
// the unlucky one. The child runs in its own process group and does not receive
// the signal, so before `alpha local start` handled it, an interrupt killed
// nav-pilot and left 21 GB of resident memory loading a model nobody was waiting
// for, holding a port, with no state file written for `stop` to find.
func TestStartStopsTheChildWhenInterrupted(t *testing.T) {
	stubDirs(t)
	proc := newFakeProc()
	starts := stubStart(t, proc)
	// Never ready: readiness is what Start waits on, so this is the window an
	// interrupt actually lands in.
	stubCompletion(t, func(context.Context, int) (int, error) {
		return 0, errors.New("not ready yet")
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := &Server{}
	if err := s.Start(ctx, testModel()); err == nil {
		t.Fatal("Start() with a cancelled context returned nil")
	}
	proc.mu.Lock()
	signalled := len(proc.signals)
	proc.mu.Unlock()
	if len(*starts) > 0 && signalled == 0 {
		t.Error("Start() left the child running after the context was cancelled")
	}
}

// seedWeights makes WeightsPresent true for a model, the way a machine that has
// run init looks.
func seedWeights(t *testing.T, model string) {
	t.Helper()
	// Both files: WeightsPresent wants a snapshot carrying weights and a config,
	// because a metadata-only snapshot is what an interrupted download leaves.
	snap := filepath.Join(modelCacheDir(model), "snapshots", "abc123")
	if err := os.MkdirAll(snap, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"model.safetensors": "weights", "config.json": "{}"} {
		if err := os.WriteFile(filepath.Join(snap, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestLockfilePinsTheVersionsTheCodeNames: the lockfile and the constants must
// agree, or the stamp records a version the environment does not have.
//
// They drift silently. mlxLMVersion and mlxVersion decide what `Installed()`
// compares against, and the lockfile decides what is actually installed; nothing
// but this test connects them, and the recipe for regenerating one lives in the
// other's header.
func TestLockfilePinsTheVersionsTheCodeNames(t *testing.T) {
	got := string(requirementsTxt)
	for _, want := range []string{"mlx-lm==" + mlxLMVersion, "mlx==" + mlxVersion} {
		if !strings.Contains(got, want) {
			t.Errorf("requirements.txt does not pin %q; regenerate it with the recipe in its header", want)
		}
	}
	if !strings.Contains(got, "--hash=sha256:") {
		t.Error("requirements.txt carries no hashes, so --require-hashes buys nothing")
	}
}

// TestServerFlagsCarryTheTunedKnobs pins the fix for a silent production bug.
//
// The manifest's params were passed to mlx_lm.server as environment variables,
// and mlx-lm reads exactly one variable in its whole package. So every profile
// shipped {"enable_thinking": false} and a tuned sampler, and the server ran
// with thinking on and mlx-lm's own defaults — including --temp 0.0, greedy
// decoding, which is what makes a local model repeat a tool call forever. The
// benchmarks that produced those values ran through a harness that translates
// the same variables into flags, so the numbers described a configuration no
// developer was running.
func TestServerFlagsCarryTheTunedKnobs(t *testing.T) {
	got := serverFlags(map[string]string{
		"MLX_MODEL":              "org/Model",
		"MLX_SERVER_TYPE":        "mlx-lm",
		"MLX_TEMP":               "0.6",
		"MLX_TOP_K":              "20",
		"MLX_TOP_P":              "0.95",
		"MLX_MAX_TOKENS":         "32768",
		"MLX_CACHE_SIZE":         "3",
		"MLX_CHAT_TEMPLATE_ARGS": `{"enable_thinking": false}`,
		"MLX_OPENCODE_CONTEXT":   "65536",
	})
	// Exact equality rather than substring matching. serverFlags is
	// deterministic, so the order is part of the contract, and a substring
	// check would pass on a duplicated flag — which is how a doubled
	// device_id attribute reached a commit earlier the same week.
	want := []string{
		"--temp", "0.6",
		"--top-p", "0.95",
		"--top-k", "20",
		"--max-tokens", "32768",
		"--prompt-cache-size", "3",
		"--chat-template-args", `{"enable_thinking": false}`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("serverFlags =\n  %v\nwant\n  %v", got, want)
	}

	joined := strings.Join(got, " ")

	// Keys the server does not take must not become flags. Params come from a
	// file fetched over the network, so a generic key-to-flag loop would let
	// that file pass arbitrary arguments to a process on the developer's
	// machine. Failing closed means an unknown key stays inert.
	for _, unwanted := range []string{"MLX_MODEL", "MLX_SERVER_TYPE", "MLX_OPENCODE_CONTEXT", "org/Model"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("serverFlags leaked %q into the command line: %v", unwanted, got)
		}
	}

	if len(serverFlags(map[string]string{})) != 0 {
		t.Error("no params should mean no flags, so a manifest can still say nothing")
	}
}

// A model's declared floor decides which machines may run it. The wired limit
// alone does not: a 27 GB cap fits a 36 GB machine arithmetically, while the
// entry was measured on 48 and says so.
func TestCheckWiredLimitRefusesBelowDeclaredMinimum(t *testing.T) {
	for _, tc := range []struct {
		name        string
		minRAMGB    int
		machineGB   int
		wantRefusal bool
	}{
		{name: "below the floor", minRAMGB: 48, machineGB: 36, wantRefusal: true},
		{name: "exactly at the floor", minRAMGB: 48, machineGB: 48},
		{name: "above the floor", minRAMGB: 48, machineGB: 64},
		{name: "no floor declared", minRAMGB: 0, machineGB: 36},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubRun(t, func(name string, args []string) (string, error) {
				if name != sysctlPath {
					return "", errors.New("unexpected command " + name)
				}
				switch args[len(args)-1] {
				case "hw.memsize":
					return memsizeBytes(tc.machineGB), nil
				case "iogpu.wired_limit_mb":
					return "24576\n", nil
				}
				return "", errors.New("unexpected sysctl")
			})

			_, err := CheckWiredLimit(Model{Model: okModel, WeightsGB: 16, WiredLimitGB: 24, MinRAMGB: tc.minRAMGB})
			if tc.wantRefusal {
				if err == nil {
					t.Fatal("CheckWiredLimit() accepted a model the manifest says needs a bigger machine")
				}
				// The refusal has to name both numbers, or the developer
				// cannot tell whether it is their machine or the entry.
				for _, want := range []string{"48 GB", "36 GB"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("CheckWiredLimit() error = %q, want it to mention %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("CheckWiredLimit() errored: %v", err)
			}
		})
	}
}
