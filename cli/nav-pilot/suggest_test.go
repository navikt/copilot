package main

import "testing"

func TestSuggest(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		candidates []string
		want       string
	}{
		{"exact typo in command", "instal", knownCommands, "install"},
		{"transposition", "exoprt", knownCommands, "export"},
		{"one char off", "synx", knownCommands, "sync"},
		{"missing letter", "staus", knownCommands, "status"},
		{"extra letter", "listt", knownCommands, "list"},
		{"too distant", "foobar", knownCommands, ""},
		{"empty input", "", knownCommands, ""},
		{"exact match", "install", knownCommands, "install"},
		{"flag typo", "--dry-rn", knownFlags, "--dry-run"},
		{"flag typo force", "--forc", knownFlags, "--force"},
		{"flag completely wrong", "--verbose", knownFlags, ""},
		{"no candidates", "install", nil, ""},
		{"single char flag typo", "-n", knownFlags, "-n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggest(tt.input, tt.candidates)
			if got != tt.want {
				t.Errorf("suggest(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
		{"install", "instal", 1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := levenshtein(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
