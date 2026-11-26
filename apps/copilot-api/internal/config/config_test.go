package config

import "testing"

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string // We'll compare string representations
	}{
		{"DEBUG", "DEBUG"},
		{"debug", "DEBUG"},
		{"INFO", "INFO"},
		{"info", "INFO"},
		{"WARN", "WARN"},
		{"WARNING", "WARN"},
		{"ERROR", "ERROR"},
		{"invalid", "INFO"}, // Default
		{"", "INFO"},        // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLogLevel(tt.input)
			if got.String() != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]bool
	}{
		{
			name:  "single endpoint",
			input: "/api/v1/",
			want:  map[string]bool{"/api/v1/": true},
		},
		{
			name:  "multiple endpoints",
			input: "/api/v1/,/health,/ready",
			want:  map[string]bool{"/api/v1/": true, "/health": true, "/ready": true},
		},
		{
			name:  "with spaces",
			input: "/api/v1/ , /health , /ready",
			want:  map[string]bool{"/api/v1/": true, "/health": true, "/ready": true},
		},
		{
			name:  "empty string",
			input: "",
			want:  map[string]bool{},
		},
		{
			name:  "empty items",
			input: "/api/v1/,,/health",
			want:  map[string]bool{"/api/v1/": true, "/health": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEndpoints(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseEndpoints() length = %v, want %v", len(got), len(tt.want))
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("parseEndpoints() missing key %q", k)
				}
			}
		})
	}
}
