package local

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
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

func TestStateRoundTripsAndForgets(t *testing.T) {
	stubDirs(t)

	if _, ok, err := LoadState(); ok || err != nil {
		t.Fatalf("LoadState() with nothing recorded = (%v, %v), want (false, nil)", ok, err)
	}

	want := State{PID: 4242, Model: "mlx-community/x", Port: 8080, Started: time.Now().Truncate(time.Second)}
	if err := SaveState(want); err != nil {
		t.Fatalf("SaveState() errored: %v", err)
	}
	got, ok, err := LoadState()
	if err != nil || !ok {
		t.Fatalf("LoadState() = (%v, %v), want a recorded server", ok, err)
	}
	if got.PID != want.PID || got.Model != want.Model || got.Port != want.Port || !got.Started.Equal(want.Started) {
		t.Errorf("LoadState() = %+v, want %+v", got, want)
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
	if _, ok, err := LoadState(); err == nil || ok {
		t.Errorf("LoadState() on a corrupt record = (%v, %v), want an error", ok, err)
	}
}

// TestAttachToADeadProcessIsCrashedImmediately is the state that matters most:
// the server died since it was started, and a developer running status must be
// told "crashed", not "still starting".
func TestAttachToADeadProcessIsCrashedImmediately(t *testing.T) {
	stubDirs(t)
	stubAlive(t, func(int) bool { return false })

	// A probe that would answer "starting" if it were consulted at all — the
	// classification must not depend on it.
	orig := probeCompletion
	probeCompletion = func(context.Context, string, string) (int, error) {
		return 0, errors.New("connection refused")
	}
	t.Cleanup(func() { probeCompletion = orig })

	s := Attach(State{PID: 4242, Model: "m", Port: 8080, Started: time.Now()})
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
	stubAlive(t, func(int) bool { return true })

	orig := probeCompletion
	t.Cleanup(func() { probeCompletion = orig })

	probeCompletion = func(context.Context, string, string) (int, error) { return 3, nil }
	s := Attach(State{PID: 4242, Model: "m", Port: 8080, Started: time.Now()})
	if got := s.Health(context.Background()); got != HealthReady {
		t.Errorf("Health() for a server answering completions = %q, want %q", got, HealthReady)
	}

	probeCompletion = func(context.Context, string, string) (int, error) {
		return 0, errors.New("connection refused")
	}
	cold := Attach(State{PID: 4242, Model: "m", Port: 8080, Started: time.Now()})
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

	p := &attachedProc{pid: 4242, done: make(chan struct{})}
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
