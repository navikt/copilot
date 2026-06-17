package cli

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

func TestConfigModelLabel(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", "unset"},
		{"   ", "unset"},
		// exact canonical id — returned as-is
		{"claude-opus-4.8", "claude-opus-4.8"},
		// uppercase input → canonical curated id (canonicalization)
		{"CLAUDE-OPUS-4.8", "claude-opus-4.8"},
		{"Claude-Opus-4.8", "claude-opus-4.8"},
		{"some/local-model", "custom"},
		{"gpt-bogus", "custom"},
		{"anthropic/claude-sonnet-4-5", "anthropic/claude-sonnet-4-5"},
		// mixed-case → canonical opencode model id
		{"ANTHROPIC/CLAUDE-SONNET-4-5", "anthropic/claude-sonnet-4-5"},
		{"anthropic/claude-opus-4-5", "anthropic/claude-opus-4-5"},
		{"anthropic/claude-3-5-sonnet", "custom"},
	}
	for _, tt := range tests {
		if got := configModelLabel(tt.in); got != tt.want {
			t.Errorf("configModelLabel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
