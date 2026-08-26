package cli

import "testing"

func TestCpltPresetFromConfigGet(t *testing.T) {
	tests := []struct {
		name, out, want string
	}{
		{"value with annotation", "standard\n[cplt] (default — not set in config file)\n", "standard"},
		{"strict", "strict\n[cplt] (set in config file)\n", "strict"},
		{"bare value", "permissive\n", "permissive"},
		{"padded", "  full-trust  \n", "full-trust"},
		{"empty (command failed, nothing on stdout)", "", ""},
		{"only whitespace", "\n\n", ""},
		{"unknown value", "paranoid\n", ""},
		{"error text", "[cplt] unknown config key 'sandbox.preset'\n", ""},
	}
	for _, tc := range tests {
		if got := cpltPresetFromConfigGet(tc.out); got != tc.want {
			t.Errorf("%s: cpltPresetFromConfigGet(%q) = %q, want %q", tc.name, tc.out, got, tc.want)
		}
	}
}

func TestCpltRecommendStrict(t *testing.T) {
	tests := []struct {
		preset string
		want   bool
	}{
		{"strict", false},
		{"standard", true},
		{"permissive", true},
		{"full-trust", true},
		{"", false}, // unknown: skip the recommendation rather than guess
	}
	for _, tc := range tests {
		if got := cpltRecommendStrict(tc.preset); got != tc.want {
			t.Errorf("cpltRecommendStrict(%q) = %v, want %v", tc.preset, got, tc.want)
		}
	}
}
