package cli

import (
	"errors"
	"os"
	"os/exec"
	"testing"
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

// The detached sender is not started inside cplt's sandbox, whose audit flags
// a setsid escapee, nor when telemetry is off.
func TestTelemetrySenderAllowed(t *testing.T) {
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "true")
	t.Setenv(cpltSandboxEnvVar, "")
	os.Unsetenv(cpltSandboxEnvVar)
	if !telemetrySenderAllowed() {
		t.Fatal("not allowed with telemetry on, outside the sandbox")
	}
	t.Setenv(cpltSandboxEnvVar, "1")
	if telemetrySenderAllowed() {
		t.Error("allowed inside cplt's sandbox")
	}
	os.Unsetenv(cpltSandboxEnvVar)
	t.Setenv("DO_NOT_TRACK", "1")
	if telemetrySenderAllowed() {
		t.Error("allowed with DO_NOT_TRACK")
	}
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "false")
	if telemetrySenderAllowed() {
		t.Error("allowed with telemetry disabled")
	}
}
