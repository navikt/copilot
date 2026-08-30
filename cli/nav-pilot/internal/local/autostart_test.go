package local

import (
	"context"
	"sync"
	"testing"
)

// TestAutostartOffRefusesAndSaysHowToTurnItOn: a launch that finds no server must
// not start one unasked, and must not fall through to the hosted model in silence.
// A developer who configured local and got billed for a cloud session has been
// wronged quietly, which is the worst way.
func TestAutostartOffRefusesAndSaysHowToTurnItOn(t *testing.T) {
	stubDirs(t)
	SetAutostart(false)
	t.Cleanup(func() { SetAutostart(false) })

	err := EnsureServerRunning(context.Background(), nil)
	if err == nil {
		t.Fatal("EnsureServerRunning with autostart off returned nil and started nothing")
	}
	for _, want := range []string{"alpha local start", "local_autostart"} {
		if !contains(err.Error(), want) {
			t.Errorf("refusal = %q, want it to mention %q", err, want)
		}
	}
}

// TestConcurrentLaunchesStartOneServer is the requirement the whole design turns
// on: the server is one per machine. It holds 21 GB and a warm prompt cache, so a
// second is not a fallback, it is a machine with no memory left.
//
// Two launches starting at the same moment must converge on one rather than race.
// The loser waits on the lock, then finds the winner's server recorded and
// attaches, so the outcome does not depend on who got there first.
func TestConcurrentLaunchesStartOneServer(t *testing.T) {
	stubDirs(t)
	SetAutostart(true)
	t.Cleanup(func() { SetAutostart(false) })
	SetActive(&Manifest{Models: []Model{testModel()}})

	proc := newFakeProc()
	starts := stubStart(t, proc)
	stubCompletion(t, func(context.Context, int) (int, error) { return 1, nil })
	// The recorded pid has to look alive and hold the port, or every launch
	// re-checks, finds nothing, and starts its own: which is the bug rather than
	// the test.
	stubOurProcess(t)
	stubPortListeners(t, func(int) []int { return []int{proc.PID()} })
	// Autostart enforces the same gates as `alpha local start`, so the fixture
	// has to satisfy them: weights on disk and a wired limit that covers the
	// model. Both are what a developer who ran init already has.
	seedWeights(t, testModel().Model)
	stubRun(t, func(name string, args []string) (string, error) {
		if len(args) > 0 && args[len(args)-1] == "hw.memsize" {
			return "137438953472\n", nil // 128 GB
		}
		return "999999\n", nil // a wired limit far above the requirement
	})

	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = EnsureServerRunning(context.Background(), nil)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("launch %d = %v, want it to attach to the one server", i, err)
		}
	}
	if n := len(*starts); n != 1 {
		t.Errorf("four concurrent launches started %d servers, want 1", n)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
