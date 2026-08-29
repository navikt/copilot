package local

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// stubAlive replaces the liveness check so a test can retire a pid without a
// process — the same rule the rest of this package keeps: nothing spawns.
func stubAlive(t *testing.T, live func(int) bool) {
	t.Helper()
	orig := alive
	alive = live
	t.Cleanup(func() { alive = orig })
}

// aStartTime is what `ps -o lstart=` prints for the recorded server.
const aStartTime = "Fri Aug 21 07:11:03 2026"

// stubProcessStart replaces the process-identity read, the other half of
// liveness.
func stubProcessStart(t *testing.T, fn func(int) string) {
	t.Helper()
	orig := processStart
	processStart = fn
	t.Cleanup(func() { processStart = orig })
}

// stubOurProcess makes a pid both alive and the process that was recorded,
// which together are what the rest of this package calls running.
func stubOurProcess(t *testing.T) {
	t.Helper()
	stubAlive(t, func(int) bool { return true })
	stubProcessStart(t, func(int) string { return aStartTime })
}

// aRunningState is the record start leaves behind for a server stubOurProcess
// vouches for.
func aRunningState() State {
	return State{PID: 4242, Model: "m", Started: time.Now(), Lstart: aStartTime}
}

func TestStateRoundTripsAndForgets(t *testing.T) {
	stubDirs(t)

	if _, ok, err := LoadState(); ok || err != nil {
		t.Fatalf("LoadState() with nothing recorded = (%v, %v), want (false, nil)", ok, err)
	}

	stubProcessStart(t, func(int) string { return aStartTime })

	want := State{PID: 4242, Model: "mlx-community/x", Started: time.Now().Truncate(time.Second)}
	if err := SaveState(want); err != nil {
		t.Fatalf("SaveState() errored: %v", err)
	}
	got, ok, err := LoadState()
	if err != nil || !ok {
		t.Fatalf("LoadState() = (%v, %v), want a recorded server", ok, err)
	}
	if got.PID != want.PID || got.Model != want.Model || !got.Started.Equal(want.Started) {
		t.Errorf("LoadState() = %+v, want %+v", got, want)
	}
	// Recorded by SaveState itself, not by the caller: a pid without the start
	// time that identifies it is a pid stop must refuse to signal.
	if got.Lstart != aStartTime {
		t.Errorf("SaveState() recorded lstart %q, want the process start time %q", got.Lstart, aStartTime)
	}

	if err := ClearState(); err != nil {
		t.Fatalf("ClearState() errored: %v", err)
	}
	if _, ok, _ := LoadState(); ok {
		t.Error("ClearState() left the server recorded")
	}
	// Idempotent: stop runs it whether or not there was anything to clear.
	if err := ClearState(); err != nil {
		t.Errorf("ClearState() on nothing errored: %v", err)
	}
}

// TestLoadStateReportsAnUnreadableRecord: a state file that cannot be parsed is
// an error, not "nothing running". The process may well be running, and
// starting a second server on the same port is the worse answer.
func TestLoadStateReportsAnUnreadableRecord(t *testing.T) {
	stubDirs(t)
	if err := os.MkdirAll(dataDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath(), []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok, err := LoadState()
	if err == nil || ok {
		t.Fatalf("LoadState() on a corrupt record = (%v, %v), want an error", ok, err)
	}
	// This error is what start, stop and status all fail with, so it has to
	// carry the way out. Without it the three commands are bricked and nothing
	// on screen says the file is nav-pilot's own record and safe to delete.
	for _, want := range []string{statePath(), "rm " + statePath()} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("LoadState() on a corrupt record = %q, want it to name %q", err, want)
		}
	}
}

// TestAttachToADeadProcessIsCrashedImmediately is the state that matters most:
// the server died since it was started, and a developer running status must be
// told "crashed", not "still starting".
func TestAttachToADeadProcessIsCrashedImmediately(t *testing.T) {
	stubDirs(t)
	stubAlive(t, func(int) bool { return false })
	stubProcessStart(t, func(int) string { return aStartTime })

	// A probe that would answer "starting" if it were consulted at all — the
	// classification must not depend on it.
	orig := probeCompletion
	probeCompletion = func(context.Context, string, string) (int, error) {
		return 0, errors.New("connection refused")
	}
	t.Cleanup(func() { probeCompletion = orig })

	s := Attach(aRunningState())
	if got := s.Status().Health; got != HealthCrashed {
		t.Errorf("Status().Health for a pid that is gone = %q, want %q", got, HealthCrashed)
	}
	if got := s.Health(context.Background()); got != HealthCrashed {
		t.Errorf("Health() for a pid that is gone = %q, want %q", got, HealthCrashed)
	}
}

// TestAttachClassifiesALiveServer: an adopted server goes through the same
// five-state classification as one this process started.
func TestAttachClassifiesALiveServer(t *testing.T) {
	stubDirs(t)
	stubOurProcess(t)

	orig := probeCompletion
	t.Cleanup(func() { probeCompletion = orig })

	probeCompletion = func(context.Context, string, string) (int, error) { return 3, nil }
	s := Attach(aRunningState())
	if got := s.Health(context.Background()); got != HealthReady {
		t.Errorf("Health() for a server answering completions = %q, want %q", got, HealthReady)
	}

	probeCompletion = func(context.Context, string, string) (int, error) {
		return 0, errors.New("connection refused")
	}
	cold := Attach(aRunningState())
	if got := cold.Health(context.Background()); got != HealthStarting {
		t.Errorf("Health() for a server still loading weights = %q, want %q", got, HealthStarting)
	}
}

// TestAttachedProcNoticesADeathByPolling: an adopted process cannot be waited
// on — waitpid belongs to the parent, and the parent exited — so Stop's grace
// period depends on the poll noticing.
func TestAttachedProcNoticesADeathByPolling(t *testing.T) {
	origPoll := attachPollInterval
	attachPollInterval = time.Millisecond
	t.Cleanup(func() { attachPollInterval = origPoll })

	var live atomic.Bool
	live.Store(true)
	stubAlive(t, func(int) bool { return live.Load() })
	stubProcessStart(t, func(int) string { return aStartTime })

	p := &attachedProc{pid: 4242, lstart: aStartTime, done: make(chan struct{})}
	done := make(chan exitInfo, 1)
	go func() { done <- p.Wait() }()

	select {
	case <-done:
		t.Fatal("Wait() returned while the process was still alive")
	case <-time.After(10 * time.Millisecond):
	}
	live.Store(false)
	select {
	case info := <-done:
		if info.Err == nil {
			t.Error("the exit of an adopted process says nothing about how it ended")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait() never noticed the process was gone")
	}
}

func TestResidentMemoryMB(t *testing.T) {
	stubDirs(t)
	stubRun(t, func(name string, args []string) (string, error) {
		if name != "ps" {
			return "", errors.New("unexpected command " + name)
		}
		return " 26843545\n", nil
	})
	if got := ResidentMemoryMB(context.Background(), 4242); got != 26214 {
		t.Errorf("ResidentMemoryMB() = %d, want 26214 (26843545 KiB)", got)
	}
}

func TestResidentMemoryMBIsZeroWhenItCannotBeRead(t *testing.T) {
	stubDirs(t)
	stubRun(t, func(string, []string) (string, error) { return "", errors.New("no such process") })
	if got := ResidentMemoryMB(context.Background(), 4242); got != 0 {
		t.Errorf("ResidentMemoryMB() on a dead pid = %d, want 0", got)
	}
}

// TestInstalledFollowsThePins: an environment provisioned against older pins is
// not installed, because it is not the environment this binary would run.
func TestInstalledFollowsThePins(t *testing.T) {
	stubDirs(t)
	if Installed() {
		t.Error("Installed() with no environment on disk = true")
	}
	if err := os.MkdirAll(dataDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeStamp(); err != nil {
		t.Fatal(err)
	}
	if !Installed() {
		t.Error("Installed() after provisioning = false")
	}
	if err := os.WriteFile(stampPath(), []byte(`{"mlx_lm":"0.0.1","mlx":"0.0.1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if Installed() {
		t.Error("Installed() with an environment on older pins = true")
	}
}

// TestARecordedPidIsNotAnIdentity is the reboot, which server.json survives and
// the process it names does not.
//
// `kill(pid, 0)` answers "something has this number", never "this is still the
// server". After a reboot the kernel hands 4242 to whatever asks for it next,
// and stop signalled the *negative* pid first — the whole process group of a
// stranger's terminal, editor or build. The recorded start time is what tells
// the two apart, and every command routes through Attach to get it.
func TestARecordedPidIsNotAnIdentity(t *testing.T) {
	stubDirs(t)
	// Alive, and answering to the recorded number — but started later, so it
	// is not the process that was recorded.
	stubAlive(t, func(int) bool { return true })
	stubProcessStart(t, func(int) string { return "Mon Aug 24 09:00:00 2026" })

	srv := Attach(aRunningState())
	if got := srv.Status().Health; got != HealthCrashed {
		t.Errorf("Status().Health for a reused pid = %q, want %q — stop and status would act on a stranger's process", got, HealthCrashed)
	}
	// And Stop, having been told the process is gone, sends nothing at all.
	if err := srv.Stop(); err != nil {
		t.Errorf("Stop() on a reused pid = %v, want nil and no signal", err)
	}
}

// TestSignalRefusesAPidThatIsNotTheRecordedProcess pins the refusal at the one
// place the signal is actually sent, so a caller that reaches an attached
// process without going through Attach's classification still cannot kill a
// stranger's process group.
func TestSignalRefusesAPidThatIsNotTheRecordedProcess(t *testing.T) {
	stubAlive(t, func(int) bool { return true })
	stubProcessStart(t, func(int) string { return "Mon Aug 24 09:00:00 2026" })

	p := &attachedProc{pid: 4242, lstart: aStartTime, done: make(chan struct{})}
	err := p.Signal(syscall.SIGTERM)
	if err == nil || !strings.Contains(err.Error(), "not the process nav-pilot recorded") {
		t.Errorf("Signal() to a reused pid = %v, want a refusal naming the mismatch", err)
	}
}

// TestSignalRefusesAPidWithNoRecordedStartTime: a record written before
// nav-pilot recorded start times is exactly the record that may predate a
// reboot, so "cannot tell" is a refusal, not a pass.
func TestSignalRefusesAPidWithNoRecordedStartTime(t *testing.T) {
	stubOurProcess(t)

	p := &attachedProc{pid: 4242, done: make(chan struct{})}
	if err := p.Signal(syscall.SIGTERM); err == nil {
		t.Error("Signal() to a pid with no recorded start time succeeded; identity cannot be proved, so it must be refused")
	}
}

// TestEnsureOwnServerRefusesAForeignServerOnThePort is the launch-side half of
// the same rule Server.Start keeps at the other end of the day.
//
// The loop guard proxies to a fixed 127.0.0.1:8080. If the recorded server
// crashed and any other tool bound that port, the launch used to start the
// guard anyway and forward every prompt of the session to it, with nothing on
// screen to say so.
func TestEnsureOwnServerRefusesAForeignServerOnThePort(t *testing.T) {
	stubDirs(t)
	stubOurProcess(t)
	// The refusal names what is on the port; nothing here goes near a socket.
	origServed := servedModel
	servedModel = func(context.Context, string) string { return "" }
	t.Cleanup(func() { servedModel = origServed })

	if err := EnsureOwnServer(); err == nil || !strings.Contains(err.Error(), "alpha local start") {
		t.Errorf("EnsureOwnServer() with nothing recorded = %v, want a refusal naming start", err)
	}

	if err := SaveState(aRunningState()); err != nil {
		t.Fatal(err)
	}

	// Something else holds the port.
	stubPortListeners(t, func(int) []int { return []int{9999} })
	err := EnsureOwnServer()
	if err == nil || !strings.Contains(err.Error(), "not what is listening") {
		t.Errorf("EnsureOwnServer() with a stranger on the port = %v, want a refusal", err)
	}

	// Nothing at all holds it — no proof of ownership is still a refusal.
	stubPortListeners(t, func(int) []int { return nil })
	if err := EnsureOwnServer(); err == nil {
		t.Error("EnsureOwnServer() with nothing on the port succeeded; the guard would forward to whatever binds it next")
	}

	// The recorded process, holding the recorded port.
	stubPortListeners(t, func(int) []int { return []int{4242} })
	if err := EnsureOwnServer(); err != nil {
		t.Errorf("EnsureOwnServer() for nav-pilot's own server = %v, want nil", err)
	}

	// The pid is alive but is not the process that was recorded.
	stubProcessStart(t, func(int) string { return "Mon Aug 24 09:00:00 2026" })
	if err := EnsureOwnServer(); err == nil || !strings.Contains(err.Error(), "not running any more") {
		t.Errorf("EnsureOwnServer() for a reused pid = %v, want a refusal", err)
	}
}

// stubPortListeners replaces the lsof call, so the suite does not depend on
// which ports happen to be held on the machine running it.
func stubPortListeners(t *testing.T, fn func(int) []int) {
	t.Helper()
	orig := portListeners
	portListeners = fn
	t.Cleanup(func() { portListeners = orig })
}
