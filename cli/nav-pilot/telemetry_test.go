package main

import (
	"errors"
	"testing"
)

func TestTelemetryResult(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "success", err: nil, want: "success"},
		{name: "updates available", err: errUpdatesAvailable, want: "updates_available"},
		{name: "wrapped updates available", err: errors.Join(errUpdatesAvailable, errors.New("other")), want: "updates_available"},
		{name: "error", err: errors.New("boom"), want: "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := telemetryResult(tt.err); got != tt.want {
				t.Fatalf("telemetryResult() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTelemetryEnabled(t *testing.T) {
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "1")
	if !telemetryEnabled() {
		t.Fatal("expected telemetryEnabled to return true for 1")
	}

	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "0")
	if telemetryEnabled() {
		t.Fatal("expected telemetryEnabled to return false for 0")
	}
}
