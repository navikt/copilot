package cli

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

type mockNetError struct{}

func (mockNetError) Error() string   { return "timeout" }
func (mockNetError) Timeout() bool   { return true }
func (mockNetError) Temporary() bool { return true }

func TestClassifyError(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{exec.ErrNotFound, "client_not_found"},
		{&exec.ExitError{}, "launch_failed"},
		{mockNetError{}, "network_error"},
		{errors.New("HTTP 401 Unauthorized"), "auth_error"},
		{errors.New("HTTP 403 Forbidden"), "auth_error"},
		{errSyncFailed, "sync_failed"},
		{errUpdatesAvailable, ""},
		{errors.New("some unknown error"), "unknown"},
	}

	for _, tt := range tests {
		if got := classifyError(tt.err); got != tt.want {
			t.Errorf("classifyError(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

// blockingTelemetry is an exporter stuck on an unreachable host that ignores
// its context.
type blockingTelemetry struct{ noopTelemetry }

func (blockingTelemetry) Shutdown(context.Context) error { select {} }

// Exit must not wait on an unreachable telemetry host (#1101).
func TestFlushTelemetryIsBounded(t *testing.T) {
	defer func(c string) { sessionClient = c }(sessionClient)
	sessionClient = ""
	start := time.Now()
	flushTelemetry(blockingTelemetry{}, 50*time.Millisecond)
	if took := time.Since(start); took > time.Second {
		t.Fatalf("flush took %s with a 50ms budget", took)
	}
	if got := flushBudget(); got != telemetryFlushBudget {
		t.Errorf("flushBudget = %s, want %s", got, telemetryFlushBudget)
	}
	// After a session the user waits for the shell.
	sessionClient = "copilot"
	if got := flushBudget(); got != telemetrySessionFlushBudget {
		t.Errorf("flushBudget after a session = %s, want %s", got, telemetrySessionFlushBudget)
	}
}
