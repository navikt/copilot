package cli

import "testing"

func TestVersionTimestamp(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2026.04.14-202800-a25f6c3", "2026.04.14-202800"},
		{"2026.04.14-120650-71dcb83", "2026.04.14-120650"},
		{"2026.01.01-080000-old1234", "2026.01.01-080000"},
		{"dev", "dev"},
		{"", ""},
		{"no-hyphens", "no-hyphens"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := versionTimestamp(tt.input)
			if got != tt.want {
				t.Errorf("versionTimestamp(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestVersionNewer(t *testing.T) {
	tests := []struct {
		name      string
		candidate string
		current   string
		want      bool
	}{
		{"newer same day", "2026.04.14-202800-a25f6c3", "2026.04.14-120650-71dcb83", true},
		{"older same day", "2026.04.14-120650-71dcb83", "2026.04.14-202800-a25f6c3", false},
		{"same version", "2026.04.14-202800-a25f6c3", "2026.04.14-202800-a25f6c3", false},
		{"newer different day", "2026.04.15-080000-abc1234", "2026.04.14-202800-a25f6c3", true},
		{"older different day", "2026.04.13-170138-abc1234", "2026.04.14-120650-71dcb83", false},
		{"same timestamp different commit", "2026.04.14-120650-aaaaaaa", "2026.04.14-120650-bbbbbbb", false},
		{"dev vs real version", "dev", "2026.04.14-120650-71dcb83", false},
		{"real version vs dev", "2026.04.14-120650-71dcb83", "dev", false},
		{"both dev", "dev", "dev", false},
		{"empty vs real", "", "2026.04.14-120650-71dcb83", false},
		{"real vs empty", "2026.04.14-120650-71dcb83", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := versionNewer(tt.candidate, tt.current)
			if got != tt.want {
				t.Errorf("versionNewer(%q, %q) = %v, want %v",
					tt.candidate, tt.current, got, tt.want)
			}
		})
	}
}
