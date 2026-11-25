package metrics

import (
	"errors"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"ab", 1, "a..."},
		{"exactly10!", 10, "exactly10!"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestIsDecodeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "decode error message",
			err:      errors.New("failed to decode report response (body: ...): unexpected EOF"),
			expected: true,
		},
		{
			name:     "generic error",
			err:      errors.New("connection timeout"),
			expected: false,
		},
		{
			name:     "API status error",
			err:      errors.New("API returned status 500: internal error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDecodeError(tt.err)
			if got != tt.expected {
				t.Errorf("isDecodeError(%q) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}
