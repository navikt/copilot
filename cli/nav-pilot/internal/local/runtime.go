package local

// Runtime lifecycle for local inference: the Python environment the mlx-lm
// server needs, the weights it loads, the process itself, and the memory cap it
// runs under. The manifest half of this package (local.go) says *what* to run;
// this half runs it.
//
// Nothing here is a command. Provisioning, downloading, starting and checking
// are separate calls so a command can sequence them and report between steps,
// and so the one privileged action in the neighbourhood — raising the wired
// memory limit, which needs sudo — stays out of this package entirely:
// [CheckWiredLimit] reports what is needed and what is set, and nothing here
// writes a sysctl.
//
// Every external interaction (a process, a download, an HTTP request, a
// sysctl) is behind a package-level func var, the same seam local.go uses for
// its manifest fetch. Tests run with no network, no uv, no Python, no model,
// and spawn nothing.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
)

// Pinned toolchain. Every one of these is a deliberate, reviewable bump.
const (
	// uvVersion is the uv release nav-pilot installs into its own data
	// directory. uv is a single static binary, so this needs no Homebrew, no
	// admin rights and no system Python: a developer who has never installed
	// Python gets a working environment from a tarball.
	uvVersion = "0.12.6"

	// pythonVersion is load-bearing, not hygiene. mlx ships macOS arm64
	// wheels for cp310 through cp312 only. There is no cp313 wheel, and the
	// source build fails, so an environment created against whatever
	// interpreter happens to be newest on the machine is a broken install on
	// any machine bought after the wheel matrix was written. uv downloads and
	// manages this interpreter itself, so pinning it costs nothing and an
	// unpinned one costs the whole feature.
	pythonVersion = "3.12"

	// mlxLMVersion and mlxVersion pin the server and the array framework
	// under it. Both are named explicitly rather than letting mlx-lm resolve
	// mlx, so a benchmark number is attributable to a pair of versions.
	mlxLMVersion = "0.31.3"
	mlxVersion   = "0.32.0"
)

// DefaultPort is where the local server listens when a caller names no port.
const DefaultPort = 8080

// Budgets. Vars, not consts, so tests can shrink them without waiting on a
// wall clock.
var (
	// probeTimeout bounds a version probe (`uv --version`, `python
	// --version`), the same shape and scale as internal/provider's cplt
	// probe.
	probeTimeout = 8 * time.Second

	// installTimeout bounds the whole provisioning step. It is generous
	// because it covers uv downloading a Python interpreter and resolving and
	// building a wheel set on a cold cache, over whatever link the developer
	// is on.
	installTimeout = 15 * time.Minute

	// downloadTimeout bounds a weights download. Tens of gigabytes over a
	// home line is the normal case, not the pathological one.
	downloadTimeout = 4 * time.Hour

	// readyTimeout bounds [Server.Start] waiting for the first real
	// completion. mlx-lm maps the weights lazily, so this covers a cold page
	// cache on a 25 GB model.
	readyTimeout      = 10 * time.Minute
	readyPollInterval = 500 * time.Millisecond

	// healthTimeout is how long a health probe waits for an answer before it
	// calls the server hung. It is well past a one-token completion on a warm
	// server, so an expiry means the generation thread is gone, not busy.
	healthTimeout = 30 * time.Second

	// stopGrace is how long Stop waits after SIGTERM before SIGKILL.
	stopGrace = 5 * time.Second
)

// dataDir is where nav-pilot keeps the local-inference toolchain: its own uv
// and its own venv, under the same ~/.nav-pilot directory [navPilotPath]
// states. A func var so tests point it at a temp dir.
var dataDir = func() string { return navPilotPath("local") }

// hfHome is the Hugging Face cache root the weights land in. It honours
// HF_HOME, because a developer who moved that cache to an external disk did it
// on purpose and a second copy of a 25 GB model is not a small mistake.
var hfHome = func() string {
	if h := os.Getenv("HF_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "huggingface")
}

// runCommand runs one bounded command and returns its combined output. The
// seam every provisioning step goes through, so a test can answer `uv
// --version` without a uv on the machine.
var runCommand = func(ctx context.Context, name string, args []string, env []string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}

// runStreaming runs a command and hands each output line to onLine as it
// arrives, for the one step long enough that a caller has to show progress.
var runStreaming = func(ctx context.Context, name string, args []string, env []string, onLine func(string)) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = env
	}
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	// The downloader redraws a progress bar with \r, so lines are split on
	// both terminators; anything else reports one line at the very end.
	scanLines(pipe, onLine)
	return cmd.Wait()
}

// downloadFile fetches a URL to a path. Used for exactly one thing — the uv
// tarball — and kept separate from runCommand so a test can hand over a fake
// tarball without a network.
var downloadFile = func(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return f.Close()
}

// scanLines splits on \n and \r so a redrawn progress bar reports progress
// rather than one line at the end.
func scanLines(r io.Reader, onLine func(string)) {
	buf := make([]byte, 4096)
	var line strings.Builder
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			if b == '\n' || b == '\r' {
				if s := strings.TrimSpace(line.String()); s != "" && onLine != nil {
					onLine(s)
				}
				line.Reset()
				continue
			}
			line.WriteByte(b)
		}
		if err != nil {
			if s := strings.TrimSpace(line.String()); s != "" && onLine != nil {
				onLine(s)
			}
			return
		}
	}
}

// ---------------------------------------------------------------------------
// 1. Environment
// ---------------------------------------------------------------------------

// envStamp records what provisioning installed, so a version check can answer
// without importing mlx into a subprocess.
type envStamp struct {
	MLXLM string `json:"mlx_lm"`
	MLX   string `json:"mlx"`
}

func uvPath() string   { return filepath.Join(dataDir(), "bin", "uv") }
func venvPath() string { return filepath.Join(dataDir(), "venv") }

// venvBin is the path to an executable inside the managed venv.
func venvBin(name string) string { return filepath.Join(venvPath(), "bin", name) }

func stampPath() string { return filepath.Join(dataDir(), "env.json") }

// LogPath is where the server's stdout and stderr go. Without it mlx_lm.server
// writes its tracebacks, bind errors and OOM messages to a pipe nobody reads,
// and every failure report a maintainer inherits says only "crashed" — which is
// the state the diagnosis has to start from, not end at.
func LogPath() string {
	// "" rather than a bare relative name: with no home directory
	// [navPilotPath] gives "", and joining onto that would put the log in
	// whatever directory nav-pilot happened to be run from.
	if dataDir() == "" {
		return ""
	}
	return filepath.Join(dataDir(), "server.log")
}

// maxLogBytes is when the log is started over rather than appended to. A server
// left up for a day writes a request line per completion, and the interesting
// bytes are always the most recent ones — so the cheapest bound that keeps them
// is to truncate on start, not to rotate. One stat per launch, no second file,
// nothing to clean up.
const maxLogBytes = 4 << 20

// openLog opens the server log for a launch, appending unless it has grown past
// [maxLogBytes]. nil when it cannot be opened: a log is for diagnosing a
// failure, and refusing to start the server because the log would not open
// makes it the failure.
func openLog() *os.File {
	path := LogPath()
	if path == "" {
		return nil
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxLogBytes {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil
	}
	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return nil
	}
	return f
}

// probeVersion runs `<bin> --version` and returns the version token after the
// expected prefix, or "" for anything it cannot read — missing binary, wrong
// binary, unreadable output. "" is never mistaken for "up to date", the same
// rule internal/provider's cplt parse follows.
func probeVersion(ctx context.Context, bin, prefix string) string {
	out, err := runCommand(ctx, bin, []string{"--version"}, nil)
	if err != nil {
		return ""
	}
	first, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	fields := strings.Fields(first)
	if len(fields) < 2 || !strings.EqualFold(fields[0], prefix) {
		return ""
	}
	return fields[1]
}

// matchesPythonPin reports whether an interpreter version is the pinned
// major.minor. The patch level is free: 3.12.1 and 3.12.8 load the same cp312
// wheel, and 3.13.0 loads none.
func matchesPythonPin(v string) bool {
	return v == pythonVersion || strings.HasPrefix(v, pythonVersion+".")
}

func readStamp() (envStamp, error) {
	var s envStamp
	data, err := os.ReadFile(stampPath())
	if err != nil {
		return s, err
	}
	return s, json.Unmarshal(data, &s)
}

func writeStamp() error {
	data, err := json.Marshal(envStamp{MLXLM: mlxLMVersion, MLX: mlxVersion})
	if err != nil {
		return err
	}
	return os.WriteFile(stampPath(), data, 0o644)
}

// EnsureEnv brings the local-inference environment up to the pins, doing only
// the steps that are not already done. It is safe to call on every launch: a
// provisioned machine spends three version probes and no network.
func EnsureEnv(ctx context.Context) error {
	if dataDir() == "" {
		return errors.New("could not determine a home directory for the local-inference environment")
	}
	if err := os.MkdirAll(dataDir(), 0o755); err != nil {
		return fmt.Errorf("creating the local-inference directory: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()

	if err := ensureUV(ctx); err != nil {
		return err
	}
	if err := ensureVenv(ctx); err != nil {
		return err
	}
	return ensurePackages(ctx)
}

// uvAsset is the release asset for this machine. uv publishes a static binary
// per target, which is what makes "install it ourselves" a tarball and not a
// package manager.
func uvAsset() (string, error) {
	target := runtime.GOOS + "/" + runtime.GOARCH
	if asset := uvAssetFor(target); asset != "" {
		return asset, nil
	}
	return "", fmt.Errorf("no uv release for %s", target)
}

// uvAssetFor maps a GOOS/GOARCH target to its uv release asset, "" for a
// target uv does not publish. Split out so the checksum pin can be tested
// against every target this binary can be built for, not just the one the test
// happens to run on.
func uvAssetFor(target string) string {
	switch target {
	case "darwin/arm64":
		return "uv-aarch64-apple-darwin"
	case "darwin/amd64":
		return "uv-x86_64-apple-darwin"
	case "linux/arm64":
		return "uv-aarch64-unknown-linux-gnu"
	case "linux/amd64":
		return "uv-x86_64-unknown-linux-gnu"
	}
	return ""
}

// uvSHA256 pins the sha256 of every uv release asset for [uvVersion]. This is
// the one thing this package fetches and then executes, and until now it did so
// on the strength of a URL alone.
//
// The sums are written down here rather than fetched from the release: a
// checksum served next to the file it describes is vouched for by whoever
// served the file, so downloading both proves only that they match each other.
// Bumping uvVersion means replacing every entry below, read once from
// https://github.com/astral-sh/uv/releases/download/<version>/<asset>.tar.gz.sha256
// and reviewed in the same diff as the version.
var uvSHA256 = map[string]string{
	"uv-aarch64-apple-darwin":      "14b459d51ea2e71eeba28c45a268c922bdf8607fc6455e3f40b4e082895d160d",
	"uv-x86_64-apple-darwin":       "2a26ea71bbeff1c7e12c2cc40245c96a041deff276bc921e7038e304d5d3e04c",
	"uv-aarch64-unknown-linux-gnu": "d58030acd26159499ac82f32da12d1b3c12a3a1bfc414232d9082070c03e128d",
	"uv-x86_64-unknown-linux-gnu":  "8681d8921e7d520fb368991dcf5f9c1905b80f5bf2a265a0ed085c8d8e342477",
}

// verifyUVDownload refuses a uv tarball whose bytes are not the pinned ones.
// It fails the whole install rather than falling back to an unverified copy:
// the next two steps unpack this file, chmod it executable and run it, so
// "could not verify" and "verified as wrong" have the same correct answer.
func verifyUVDownload(path, asset string) error {
	want, ok := uvSHA256[asset]
	if !ok {
		return fmt.Errorf(
			"no pinned sha256 for uv asset %s at version %s, so the download cannot be verified — refusing to unpack and run it",
			asset, uvVersion)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("verifying the uv %s download: %w", uvVersion, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("verifying the uv %s download: %w", uvVersion, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf(
			"the uv %s download for %s does not match its pinned checksum — refusing to unpack and run it.\n\n  want %s\n  got  %s",
			uvVersion, asset, want, got)
	}
	return nil
}

// ensureUV installs the pinned uv into nav-pilot's own bin directory, or
// leaves an already-correct one alone.
//
// The version is gated, not merely presence-checked, for the reason
// internal/provider gates cplt: an old binary that runs is a worse failure
// than a missing one, because it fails somewhere further in with an error
// about something else.
func ensureUV(ctx context.Context) error {
	if probeVersion(ctx, uvPath(), "uv") == uvVersion {
		return nil
	}
	asset, err := uvAsset()
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "nav-pilot-uv-")
	if err != nil {
		return fmt.Errorf("unpacking uv: %w", err)
	}
	defer os.RemoveAll(tmp)

	url := fmt.Sprintf("https://github.com/astral-sh/uv/releases/download/%s/%s.tar.gz", uvVersion, asset)
	tarball := filepath.Join(tmp, "uv.tar.gz")
	if err := downloadFile(ctx, url, tarball); err != nil {
		return fmt.Errorf("downloading uv %s: %w", uvVersion, err)
	}
	if err := verifyUVDownload(tarball, asset); err != nil {
		return err
	}
	if out, err := runCommand(ctx, "tar", []string{"-xzf", tarball, "-C", tmp}, nil); err != nil {
		return fmt.Errorf("unpacking uv %s: %w: %s", uvVersion, err, strings.TrimSpace(out))
	}
	if err := os.MkdirAll(filepath.Dir(uvPath()), 0o755); err != nil {
		return fmt.Errorf("installing uv: %w", err)
	}
	if err := os.Rename(filepath.Join(tmp, asset, "uv"), uvPath()); err != nil {
		return fmt.Errorf("installing uv: %w", err)
	}
	return os.Chmod(uvPath(), 0o755)
}

// ensureVenv creates the virtual environment against the pinned interpreter,
// which uv downloads and manages itself when the machine has no 3.12.
//
// An existing venv on the wrong interpreter is refused rather than silently
// rebuilt: the directory may hold packages a developer installed, and deleting
// it is a decision for them. The message says the pin is about wheels, because
// "use 3.12" with no reason is the kind of instruction people work around.
func ensureVenv(ctx context.Context) error {
	switch found := probeVersion(ctx, venvBin("python"), "Python"); {
	case matchesPythonPin(found):
		return nil
	case found != "":
		return fmt.Errorf(
			"the local-inference virtual environment at %s runs Python %s, but mlx only publishes macOS arm64 wheels for cp310 to cp312 and its source build fails, so it must be Python %s.\n\n  Remove it and run the command again: %s",
			venvPath(), found, pythonVersion, domain.Bold("rm -rf "+venvPath()))
	}
	out, err := runCommand(ctx, uvPath(), []string{"venv", "--python", pythonVersion, venvPath()}, nil)
	if err != nil {
		return fmt.Errorf("creating the Python %s environment: %w: %s", pythonVersion, err, strings.TrimSpace(out))
	}
	// Believe the created environment, not the command that created it: uv
	// resolving --python to something else is exactly the failure the pin
	// exists to stop, and it must not reach an mlx install to be discovered.
	if found := probeVersion(ctx, venvBin("python"), "Python"); !matchesPythonPin(found) {
		return fmt.Errorf(
			"the local-inference virtual environment was created with Python %q, not the pinned %s that mlx publishes wheels for",
			found, pythonVersion)
	}
	return nil
}

// ensurePackages installs the pinned mlx-lm and mlx into the venv.
func ensurePackages(ctx context.Context) error {
	if stamp, err := readStamp(); err == nil && stamp.MLXLM == mlxLMVersion && stamp.MLX == mlxVersion {
		return nil
	}
	out, err := runCommand(ctx, uvPath(), []string{
		"pip", "install", "--python", venvBin("python"),
		"mlx-lm==" + mlxLMVersion, "mlx==" + mlxVersion,
	}, nil)
	if err != nil {
		return fmt.Errorf("installing mlx-lm %s and mlx %s: %w: %s", mlxLMVersion, mlxVersion, err, strings.TrimSpace(out))
	}
	return writeStamp()
}

// ---------------------------------------------------------------------------
// 2. Weights
// ---------------------------------------------------------------------------

// modelCacheDir is where the Hugging Face cache keeps one model:
// <hub>/models--<owner>--<repo>.
func modelCacheDir(model string) string {
	return filepath.Join(hfHome(), "hub", "models--"+strings.ReplaceAll(model, "/", "--"))
}

// WeightsPresent reports whether the model is already fully in the Hugging
// Face cache.
//
// "Fully" is the point. An interrupted download leaves the snapshot in place
// with .incomplete blobs beside it, so a directory-exists check answers yes for
// a model that cannot load, and the developer gets the failure from mlx-lm
// several minutes later instead of from here.
func WeightsPresent(model string) (bool, error) {
	dir := modelCacheDir(model)
	snapshots, err := os.ReadDir(filepath.Join(dir, "snapshots"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading the model cache at %s: %w", dir, err)
	}
	blobs, err := os.ReadDir(filepath.Join(dir, "blobs"))
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("reading the model cache at %s: %w", dir, err)
	}
	for _, b := range blobs {
		if strings.HasSuffix(b.Name(), ".incomplete") {
			return false, nil
		}
	}
	for _, snap := range snapshots {
		files, err := os.ReadDir(filepath.Join(dir, "snapshots", snap.Name()))
		if err != nil {
			continue
		}
		var weights, config bool
		for _, f := range files {
			weights = weights || strings.HasSuffix(f.Name(), ".safetensors")
			config = config || f.Name() == "config.json"
		}
		if weights && config {
			return true, nil
		}
	}
	return false, nil
}

// DownloadWeights fetches a model into the Hugging Face cache, resuming
// whatever a previous attempt left behind, and reports progress through
// progress rather than printing: this package has no opinion about whether the
// caller is a TUI, a log line or a benchmark.
//
// The download needs three hosts reachable, not one:
//
//	huggingface.co             the API and the file metadata
//	cas-server.xethub.hf.co    the content-addressed chunk index
//	transfer.xethub.hf.co      the chunks themselves
//
// Behind a TLS-inspecting corporate proxy the first typically succeeds and the
// other two do not, and the way that presents is a download that starts,
// prints nothing further and never finishes — a hang, not a refusal. That is
// worth knowing before spending an afternoon on it: if progress stops at 0%,
// the question is whether the two xethub hosts resolve and complete a TLS
// handshake, not whether the model id is right.
func DownloadWeights(ctx context.Context, model string, progress func(string)) error {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	// The venv's own hf CLI, so the downloader is the pinned huggingface_hub
	// that mlx-lm will load the weights with, not whatever is on PATH.
	if err := runStreaming(ctx, venvBin("hf"), []string{"download", model}, nil, progress); err != nil {
		return fmt.Errorf(
			"downloading %s: %w\n\n  The download needs %s, %s and %s reachable; behind a TLS-inspecting proxy the first works and the other two hang",
			model, err, "huggingface.co", "cas-server.xethub.hf.co", "transfer.xethub.hf.co")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 3. Server lifecycle
// ---------------------------------------------------------------------------

// Health is what a supervisor needs to tell apart. The five states are not
// cosmetic: each one has a different correct response, and the two failure
// states in particular look identical from the outside if you only watch the
// port.
type Health string

const (
	// HealthNotStarted: nothing has been started, or Stop was called.
	HealthNotStarted Health = "not started"

	// HealthStarting: the process is alive but has not yet answered a
	// completion. mlx-lm binds the port before it maps the weights, so this
	// state can last minutes on a large model and is not a fault.
	HealthStarting Health = "starting"

	// HealthReady: a real completion came back with tokens in it.
	HealthReady Health = "ready"

	// HealthCrashed: the process is gone. Restart.
	HealthCrashed Health = "crashed"

	// HealthHung: the process is alive and accepting connections but did not
	// answer within healthTimeout. Restart — it will not recover.
	HealthHung Health = "hung"
)

// Status is the supervisor's view of one server, without probing it. Health
// probes; this reports what is already known, so a status line is free.
type Status struct {
	Health           Health
	Model            string
	Port             int
	PID              int
	Signal           syscall.Signal
	ExitCode         int
	SignalDeaths     int
	ZeroTokenReplies int
}

// exitInfo is how a supervised process ended. Signal is non-zero exactly for a
// signal death, which is the distinction the whole supervision design turns
// on.
type exitInfo struct {
	Signal syscall.Signal
	Code   int
	Err    error
}

// proc is the supervised process. An interface only because a test must be
// able to end a process with SIGSEGV without a process existing, which
// os.ProcessState cannot be constructed to say.
type proc interface {
	// Wait blocks until the process ends and reports how.
	Wait() exitInfo
	// Signal delivers a signal to it.
	Signal(os.Signal) error
	// PID is the process id, for a status line.
	PID() int
}

// startProcess spawns the server. The seam that keeps the test suite from
// spawning anything.
var startProcess = func(ctx context.Context, name string, args []string, env []string) (proc, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	// Both streams to one file: mlx-lm splits its own output across them and
	// which half a traceback landed on is not information anyone wants to
	// reconstruct from two files. The parent's copy of the descriptor is closed
	// once the child holds it.
	if log := openLog(); log != nil {
		cmd.Stdout, cmd.Stderr = log, log
		defer log.Close()
	}
	// Its own process group, so Stop reaches the server and anything it
	// spawned rather than orphaning them — the same reasoning as
	// internal/provider's staged probe.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &execProc{cmd: cmd, done: make(chan struct{})}, nil
}

// execProc makes Wait callable more than once — the supervisor goroutine and
// Stop both wait on the same process — which os/exec's Wait is not.
type execProc struct {
	cmd  *exec.Cmd
	once sync.Once
	info exitInfo
	done chan struct{}
}

func (p *execProc) PID() int { return p.cmd.Process.Pid }

func (p *execProc) Signal(sig os.Signal) error {
	// Negative pid: the group, not just the direct child.
	if s, ok := sig.(syscall.Signal); ok {
		if err := syscall.Kill(-p.cmd.Process.Pid, s); err == nil {
			return nil
		}
	}
	return p.cmd.Process.Signal(sig)
}

func (p *execProc) Wait() exitInfo {
	p.once.Do(func() {
		err := p.cmd.Wait()
		info := exitInfo{Err: err}
		if state := p.cmd.ProcessState; state != nil {
			info.Code = state.ExitCode()
			if ws, ok := state.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				info.Signal = ws.Signal()
			}
		}
		p.info = info
		close(p.done)
	})
	<-p.done
	return p.info
}

// probeCompletion asks the server for one token and returns how many it
// produced. The readiness and health check both go through it, because a port
// bind proves nothing: mlx-lm accepts connections before the weights are
// mapped, so a benchmark that starts timing on the bind charges model loading
// to its first request and reports a number that is not about the model.
//
// The prompt carries a nonce. An exact prompt-cache hit can kill the
// generation thread while the process stays alive — the hung state below — so
// a health check that sends the same bytes every time is a health check that
// can cause the failure it is looking for.
var probeCompletion = func(ctx context.Context, baseURL, model string) (int, error) {
	body, err := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": fmt.Sprintf("ping %d", rand.Int64())}},
		"max_tokens": 1,
		"stream":     false,
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", strings.NewReader(string(body)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("POST /v1/chat/completions: %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	var parsed struct {
		Usage struct {
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return 0, fmt.Errorf("POST /v1/chat/completions returned unparsable JSON: %w", err)
	}
	return parsed.Usage.CompletionTokens, nil
}

// portInUse reports whether something already holds 127.0.0.1:port. A bind is
// the only check that answers for every listener, including one owned by
// another user that this process may not signal or inspect.
//
// A func var, like every other external interaction here, so the lifecycle
// tests do not depend on which ports happen to be free on the machine running
// them.
var portInUse = func(port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}

// servedModel asks whatever is on a port which model it serves, so a refusal
// can name it. "" for anything that does not answer as an OpenAI-compatible
// server — which is itself worth printing.
//
// This identifies; it does not authorise. A stranger reporting the model id
// nav-pilot wanted is still a stranger: the wired-memory limit it runs under,
// the MLX_* params it was given and the context it was configured with are all
// invisible over HTTP, so a matching id would only make an adoption look
// justified. Hence [Server.Start] refuses either way and uses this for the
// message alone.
var servedModelCount = func(ctx context.Context, baseURL string) int {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return 0
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || resp.StatusCode != http.StatusOK {
		return 0
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return 0
	}
	return len(parsed.Data)
}

// Server is one supervised mlx-lm process. Zero value is usable; Port 0 means
// [DefaultPort].
type Server struct {
	// Port the server listens on. Read before Start; ignored after.
	Port int

	mu    sync.Mutex
	model string
	proc  proc
	// started is when the current process was launched. Health needs it: a
	// probe that times out means one thing inside the readiness budget and
	// another outside it.
	started          time.Time
	ready            bool
	exit             *exitInfo
	signalDeaths     int
	zeroTokenReplies int
}

// URL is where the server answers.
func (s *Server) URL() string {
	port := s.Port
	if port == 0 {
		port = DefaultPort
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

// Start launches the mlx-lm server for a manifest model and returns when it has
// answered a real completion — not when it has bound the port. A caller that
// gets a nil error has a server that has loaded the model, and one this process
// started.
//
// The manifest entry's MLX_* params go in as environment, unchanged and
// untyped, so a knob the generator adds reaches the server without a nav-pilot
// release.
func (s *Server) Start(ctx context.Context, model Model) error {
	s.mu.Lock()
	if s.proc != nil && s.exit == nil {
		s.mu.Unlock()
		return fmt.Errorf("a local server for %s is already running", s.model)
	}
	port := s.Port
	if port == 0 {
		port = DefaultPort
		s.Port = port
	}
	s.mu.Unlock()

	// Refuse a port this process does not own, before anything is spawned.
	//
	// This is the failure that made the check exist: a stale mlx_lm.server from
	// an earlier session still held the port, the child could not bind and
	// exited within milliseconds, and the readiness probe below was answered by
	// the stranger. Start reported ready, and every prompt for the rest of the
	// day went to a model nav-pilot did not choose, under a memory cap it never
	// checked, with nothing on screen to say so.
	if portInUse(port) {
		return fmt.Errorf(
			"127.0.0.1:%d is in use, and nav-pilot did not start what is listening there%s.\n\n"+
				"  Refusing to start: a server already on this port answers the readiness probe, so nav-pilot would report a model it did not launch and cannot vouch for.\n\n"+
				"  Stop it, then start again:\n\n    %s",
			port, describeForeignServer(ctx, fmt.Sprintf("http://127.0.0.1:%d", port)),
			domain.Bold(fmt.Sprintf("lsof -ti tcp:%d | xargs kill", port)))
	}

	s.mu.Lock()
	s.model, s.ready, s.exit, s.started = model.Model, false, nil, time.Now()
	s.mu.Unlock()

	env := os.Environ()
	for k, v := range model.Params {
		env = append(env, k+"="+v)
	}
	p, err := startProcess(ctx, venvBin("mlx_lm.server"), []string{
		"--model", model.Model,
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
	}, env)
	if err != nil {
		return fmt.Errorf("starting the local %s server: %w", model.Model, err)
	}

	s.mu.Lock()
	s.proc = p
	s.mu.Unlock()

	// Supervise the process, not the port. The server can die of
	// EXC_BAD_ACCESS on a stack guard page inside MLX's recursive graph walk
	// and take the listening socket with it, so watching the port conflates
	// "crashed" with "not up yet"; watching the process gives a signal death,
	// which is unambiguous and is counted on its own.
	go func() {
		info := p.Wait()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.exit = &info
		s.ready = false
		if info.Signal != 0 {
			s.signalDeaths++
		}
	}()

	return s.waitReady(ctx, model.Model)
}

// waitReady polls until a completion comes back with tokens in it, the process
// dies, or the budget runs out.
func (s *Server) waitReady(ctx context.Context, model string) error {
	deadline := time.Now().Add(readyTimeout)
	var last error
	for {
		if info := s.exited(); info != nil {
			return exitedBeforeReady(model, *info)
		}
		probeCtx, cancel := context.WithTimeout(ctx, healthTimeout)
		tokens, err := probeCompletion(probeCtx, s.URL(), model)
		cancel()
		switch {
		case err != nil:
			last = err
		case tokens > 0:
			// An answer proves something is on the port, not that it is ours.
			// The exit is re-checked here rather than only before the probe:
			// a child that could not bind dies in the window the probe spends
			// on the wire, and accepting this answer is how a stranger becomes
			// nav-pilot's server.
			if info := s.exited(); info != nil {
				return exitedBeforeReady(model, *info)
			}
			s.mu.Lock()
			s.ready = true
			s.mu.Unlock()
			return nil
		default:
			// A 200 with zero completion tokens is not readiness. It is the
			// leading indicator of the crash below, so it is counted rather
			// than accepted.
			s.mu.Lock()
			s.zeroTokenReplies++
			s.mu.Unlock()
			last = errors.New("the server answered with completion_tokens 0")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf(
				"the local %s server did not answer a completion within %s: %w\n\n  It may still be loading weights; readiness is a real completion, not a port bind",
				model, readyTimeout, last)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(readyPollInterval):
		}
	}
}

// Health probes the server and classifies it. It is the one call that can tell
// a hung server from a crashed one, which is the distinction a supervisor
// needs and the port cannot make.
func (s *Server) Health(ctx context.Context) Health {
	s.mu.Lock()
	p, exit, wasReady, model, started := s.proc, s.exit, s.ready, s.model, s.started
	s.mu.Unlock()

	if p == nil {
		return HealthNotStarted
	}
	if exit != nil {
		return HealthCrashed
	}
	probeCtx, cancel := context.WithTimeout(ctx, healthTimeout)
	tokens, err := probeCompletion(probeCtx, s.URL(), model)
	timedOut := probeCtx.Err() != nil
	cancel()

	// Re-check the exit first: a process that died mid-probe is crashed, not
	// hung, and the probe failing is a consequence of that, not evidence of
	// anything else.
	if s.exited() != nil {
		return HealthCrashed
	}
	switch {
	case err != nil && timedOut && !wasReady && time.Since(started) < readyTimeout:
		// A cold start, not a hang. mlx-lm binds the port before it maps the
		// weights, so a probe sent while it is loading is accepted and then
		// left unanswered — identical to a hang from the outside, and only the
		// clock tells them apart. Until the readiness budget is spent this is
		// still startup; calling it hung makes a supervisor that restarts on
		// hung kill-loop a healthy server on every cold start.
		return HealthStarting
	case err != nil && timedOut:
		// Alive, accepting connections, not answering. An exact prompt-cache
		// hit can kill the generation thread while the process stays up.
		return HealthHung
	case err != nil && !wasReady:
		return HealthStarting
	case err != nil:
		return HealthHung
	case tokens == 0:
		s.mu.Lock()
		s.zeroTokenReplies++
		s.mu.Unlock()
		if !wasReady {
			return HealthStarting
		}
		// Ready, but degraded. Status().ZeroTokenReplies is the number to
		// watch: it climbs before the signal death, so a supervisor that
		// restarts on it restarts on its own schedule instead of mid-request.
		return HealthReady
	default:
		s.mu.Lock()
		s.ready = true
		s.mu.Unlock()
		return HealthReady
	}
}

// Status reports what is known without probing.
func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{
		Model:            s.model,
		Port:             s.Port,
		SignalDeaths:     s.signalDeaths,
		ZeroTokenReplies: s.zeroTokenReplies,
	}
	if s.proc != nil {
		st.PID = s.proc.PID()
	}
	switch {
	case s.proc == nil:
		st.Health = HealthNotStarted
	case s.exit != nil:
		st.Health = HealthCrashed
		st.Signal, st.ExitCode = s.exit.Signal, s.exit.Code
	case s.ready:
		st.Health = HealthReady
	default:
		st.Health = HealthStarting
	}
	return st
}

// Stop ends the server: SIGTERM, then SIGKILL if it is still there after the
// grace period. Idempotent, so a deferred Stop after a failed Start is safe.
func (s *Server) Stop() error {
	s.mu.Lock()
	p, exit := s.proc, s.exit
	s.proc, s.ready = nil, false
	s.mu.Unlock()

	if p == nil || exit != nil {
		return nil
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("stopping the local server: %w", err)
	}
	done := make(chan struct{})
	go func() { p.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-time.After(stopGrace):
		return p.Signal(syscall.SIGKILL)
	}
}

func (s *Server) exited() *exitInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exit
}

// exitedBeforeReady is the one thing readiness can conclude from a dead child:
// whatever answered, it was not this server.
func exitedBeforeReady(model string, info exitInfo) error {
	return fmt.Errorf(
		"the local %s server exited before it was ready: %s.\n\n  Anything answering on its port now is not nav-pilot's server.\n  What it printed on the way out is in %s",
		model, describeExit(info), LogPath())
}

// describeForeignServer names what is on a port, for a refusal message.
//
// It does not claim which model the stranger serves. mlx-lm answers /v1/models
// with every model in the Hugging Face cache rather than the one it loaded — a
// machine with twelve downloaded gets twelve entries — so the first id is an
// arbitrary pick from someone's disk. Naming it once sent a developer chasing a
// model mismatch that did not exist.
func describeForeignServer(ctx context.Context, baseURL string) string {
	if n := servedModelCount(ctx, baseURL); n > 0 {
		return ""
	}
	return " — it does not answer /v1/models, so it is not an mlx-lm server at all"
}

func describeExit(info exitInfo) string {
	if info.Signal != 0 {
		return fmt.Sprintf("killed by %s", info.Signal)
	}
	return fmt.Sprintf("exit status %d", info.Code)
}

// ---------------------------------------------------------------------------
// 4. Memory cap
// ---------------------------------------------------------------------------

const gib = 1 << 30

// The wired limit is measured, not derived. Weights alone do not predict it:
// we measured a model needing 27.0 GB resident run happily under a 36 GB cap,
// and another at 28.9 GB refused by its own server before it produced a token.
// The difference is the KV cache, which moves with the attention shape and the
// configured context. So the manifest carries the number the generator measured
// per model (wired_limit_gb) and this file reads it — an arithmetic model
// fitted here would only reproduce that measurement for the one model it was
// calibrated against, and be confidently wrong for the next one.
const (
	// minFreeGB is what must be left for the rest of the machine. This is the
	// number that stops the common advice from wrecking a workstation: "set
	// the wired limit to 40 GB on a 48 GB machine" is widely repeated, and it
	// is how a developer running a couple of containers and a browser gets a
	// compositor collapse — the GPU allocator wins the memory and everything
	// drawing the screen loses it. Twelve gigabytes is not generous; it is the
	// difference between a slow machine and one that has to be power-cycled.
	minFreeGB = 12
)

// WiredLimit is what [CheckWiredLimit] found. It reports; it never sets. The
// sysctl needs sudo, and asking for a password belongs to a command a
// developer typed, not to a library call on a launch path.
type WiredLimit struct {
	// RequiredGB is the manifest entry's measured wired_limit_gb.
	RequiredGB int

	// CurrentGB is what iogpu.wired_limit_mb is set to now. Zero means unset,
	// which is the macOS default (roughly 75% of RAM) and not "zero
	// allowed".
	CurrentGB int

	// MachineRAMGB is the machine's physical memory.
	MachineRAMGB int

	// Sufficient is true when the current cap already covers the requirement.
	Sufficient bool

	// Command is what a developer would run to raise the cap. This package
	// does not run it.
	Command string
}

// CheckWiredLimit reports the wired-limit situation for a model, and refuses
// outright a requirement that would not leave the machine enough to keep
// running. A refusal is an error; a cap that is merely too low is not — that is
// a fixable state the caller reports with Command.
//
// A model with no measured limit is refused rather than run under a guessed
// one: the guess decides whether the machine keeps its compositor.
func CheckWiredLimit(m Model) (WiredLimit, error) {
	if m.WiredLimitGB <= 0 {
		return WiredLimit{}, fmt.Errorf(
			"the local-model manifest entry for %s carries no wired_limit_gb, and the wired-memory limit is measured per model, not derived from the weights.\n\n  The manifest must carry a measured limit for a model before nav-pilot will run it",
			m.Model)
	}

	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	w := WiredLimit{RequiredGB: m.WiredLimitGB}
	w.Command = fmt.Sprintf("sudo sysctl -w iogpu.wired_limit_mb=%d", w.RequiredGB*1024)

	ram, err := sysctlInt(ctx, "hw.memsize")
	if err != nil {
		return w, fmt.Errorf("could not read this machine's memory size, which the wired-limit check needs: %w", err)
	}
	w.MachineRAMGB = int(ram / gib)

	// An unset iogpu.wired_limit_mb is not an error: the sysctl only exists
	// once it has been set on some systems, and unset means the OS default.
	if cur, err := sysctlInt(ctx, "iogpu.wired_limit_mb"); err == nil {
		w.CurrentGB = int(cur / 1024)
	}
	w.Sufficient = w.CurrentGB >= w.RequiredGB

	if w.RequiredGB+minFreeGB > w.MachineRAMGB {
		return w, fmt.Errorf(
			"%s needs a %d GB wired-memory limit, which would leave %d GB of this %d GB machine for everything else — below the %d GB the rest of the system needs.\n\n  Pick a smaller model: a cap this close to physical memory is how a machine running containers and a browser loses its compositor and has to be power-cycled",
			m.Model, w.RequiredGB, w.MachineRAMGB-w.RequiredGB, w.MachineRAMGB, minFreeGB)
	}
	return w, nil
}

// sysctlInt reads one integer sysctl. Shelling out rather than using the
// syscall keeps this file free of build tags for a package the rest of the
// binary compiles on every platform.
//
// The absolute path matters. sysctl lives in /usr/sbin, which a login shell has
// on its PATH and a detached or launchd-started process often does not; looked up
// by name it fails there with "executable file not found in $PATH", and the
// wired-limit check it feeds then blocks the whole local setup for a reason that
// has nothing to do with memory.
const sysctlPath = "/usr/sbin/sysctl"

func sysctlInt(ctx context.Context, name string) (int64, error) {
	out, err := runCommand(ctx, sysctlPath, []string{"-n", name}, nil)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(out), 10, 64)
}
