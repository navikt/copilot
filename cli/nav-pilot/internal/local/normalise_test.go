package local

import "testing"

func TestNormaliseResult(t *testing.T) {
	tests := []struct {
		name       string
		a, b       string
		wantSameAs bool
	}{
		{"ISO timestamps", "2026-09-24T10:00:01Z queued", "2026-09-24T10:03:59.123+02:00 queued", true},
		{"log timestamps with a space", "2026-09-24 10:00:01 ok", "2026-09-25 11:11:11 ok", true},
		{"durations in different units", "done in 12.3s", "done in 900ms", true},
		{"compound durations", "elapsed 1m30s", "elapsed 2h5m", true},
		{"spelled-out durations", "took 3 seconds", "took 41 seconds", true},
		{"uuids", "request 123e4567-e89b-12d3-a456-426614174000 failed", "request 9f0c1a2b-0000-4abc-8def-0123456789ab failed", true},
		{"git shas", "HEAD is now at 4e1f9c2", "HEAD is now at a0b1c2d3e4f5", true},
		{"bare counters", "<shellId: 3 completed with exit code 0>", "<shellId: 17 completed with exit code 0>", true},
		{"different words stay different", "status: queued", "status: done", false},
		{"a-f words are words, not ids", "defaced", "effaced", false},
		{"an exit code change is still a change in words", "exit code 1: failed", "exit code 0: ok", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			na, nb := NormaliseResult(tt.a), NormaliseResult(tt.b)
			if (na == nb) != tt.wantSameAs {
				t.Errorf("NormaliseResult(%q) = %q, NormaliseResult(%q) = %q; same = %v, want %v",
					tt.a, na, tt.b, nb, na == nb, tt.wantSameAs)
			}
		})
	}
}
