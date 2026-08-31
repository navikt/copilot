package main

import "testing"

func TestLogSafe(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"ordinary name", "list_agents", "list_agents"},
		{"forged entry", "x\nlevel=ERROR msg=\"breach\"", "x?level=ERROR msg=\"breach\""},
		{"carriage return", "a\rb", "a?b"},
		{"terminal escape", "a\x1b[2Kb", "a?[2Kb"},
		{"tab is a control char too", "a\tb", "a?b"},
	} {
		if got := logSafe(tc.in); got != tc.want {
			t.Errorf("%s: logSafe(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	if got := logSafe(string(long)); len([]rune(got)) != 129 {
		t.Errorf("a 300 byte name produced %d runes, want 129 (128 plus the ellipsis)", len([]rune(got)))
	}
}
