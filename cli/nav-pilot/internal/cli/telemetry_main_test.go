package cli

import (
	"errors"
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
