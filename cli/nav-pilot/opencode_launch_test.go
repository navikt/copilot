package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCodeArgs(t *testing.T) {
	tests := []struct {
		name     string
		resolved ResolvedConfig
		want     []string
	}{
		{
			name:     "empty resolved",
			resolved: ResolvedConfig{Mode: "default", AskUser: true},
			want:     []string{},
		},
		{
			name:     "model only",
			resolved: ResolvedConfig{Model: "anthropic/claude-3-5-sonnet", Mode: "default", AskUser: true},
			want:     []string{"--model", "anthropic/claude-3-5-sonnet"},
		},
		{
			name:     "plan mode maps to --agent plan",
			resolved: ResolvedConfig{Mode: "plan", AskUser: true},
			want:     []string{"--agent", "plan"},
		},
		{
			name:     "default mode not emitted",
			resolved: ResolvedConfig{Mode: "default", AskUser: true},
			want:     []string{},
		},
		{
			name:     "reasoning effort maps to --variant",
			resolved: ResolvedConfig{Mode: "default", ReasoningEffort: "high", AskUser: true},
			want:     []string{"--variant", "high"},
		},
		{
			name:     "allow_all_tools maps to --dangerously-skip-permissions",
			resolved: ResolvedConfig{Mode: "default", AllowAllTools: true, AskUser: true},
			want:     []string{"--dangerously-skip-permissions"},
		},
		{
			name:     "log level",
			resolved: ResolvedConfig{Mode: "default", LogLevel: "debug", AskUser: true},
			want:     []string{"--log-level", "DEBUG"},
		},
		{
			name: "all fields",
			resolved: ResolvedConfig{
				Model:           "openai/gpt-4o",
				Mode:            "plan",
				ReasoningEffort: "max",
				AllowAllTools:   true,
				LogLevel:        "info",
			},
			want: []string{"--model", "openai/gpt-4o", "--agent", "plan",
				"--variant", "max", "--dangerously-skip-permissions", "--log-level", "INFO"},
		},
		{
			name:     "ask_user false not emitted (opencode has no ask-user flag)",
			resolved: ResolvedConfig{Mode: "default", AskUser: false},
			want:     []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := openCodeArgs(tt.resolved)
			if len(got) != len(tt.want) {
				t.Fatalf("openCodeArgs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("openCodeArgs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestOpenCodeLogLevel(t *testing.T) {
	cases := map[string]string{
		"debug":   "DEBUG",
		"all":     "DEBUG",
		"info":    "INFO",
		"warning": "WARN",
		"error":   "ERROR",
		"none":    "",
		"default": "",
		"":        "",
	}
	for in, want := range cases {
		if got := openCodeLogLevel(in); got != want {
			t.Errorf("openCodeLogLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnsureOpenCodeOTelConfig(t *testing.T) {
	t.Run("creates file with defaults when absent", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "opencode.json")
		openCodeConfigPathOverride = configFile
		defer func() { openCodeConfigPathOverride = "" }()

		if err := ensureOpenCodeOTelConfig(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(configFile)
		if err != nil {
			t.Fatalf("file not created: %v", err)
		}
		var cfg map[string]any
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		exp, _ := cfg["experimental"].(map[string]any)
		if exp == nil || exp["openTelemetry"] != true {
			t.Errorf("experimental.openTelemetry not set: %v", cfg)
		}
		if cfg["autoupdate"] != "notify" {
			t.Errorf("expected autoupdate=notify, got %v", cfg["autoupdate"])
		}
	})

	t.Run("merges into existing file preserving other keys", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "opencode.json")
		openCodeConfigPathOverride = configFile
		defer func() { openCodeConfigPathOverride = "" }()

		existing := map[string]any{
			"theme":      "dark",
			"autoupdate": "always",
			"experimental": map[string]any{
				"someOtherFlag": true,
			},
		}
		data, _ := json.MarshalIndent(existing, "", "  ")
		_ = os.WriteFile(configFile, data, 0o600)

		if err := ensureOpenCodeOTelConfig(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ = os.ReadFile(configFile)
		var cfg map[string]any
		_ = json.Unmarshal(data, &cfg)

		if cfg["theme"] != "dark" {
			t.Errorf("theme changed: %v", cfg["theme"])
		}
		if cfg["autoupdate"] != "always" {
			t.Errorf("autoupdate changed: %v", cfg["autoupdate"])
		}
		exp, _ := cfg["experimental"].(map[string]any)
		if exp == nil {
			t.Fatal("experimental missing")
		}
		if exp["someOtherFlag"] != true {
			t.Errorf("someOtherFlag lost: %v", exp)
		}
		if exp["openTelemetry"] != true {
			t.Errorf("openTelemetry not set: %v", exp)
		}
	})

	t.Run("idempotent — second call produces identical output", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "opencode.json")
		openCodeConfigPathOverride = configFile
		defer func() { openCodeConfigPathOverride = "" }()

		if err := ensureOpenCodeOTelConfig(); err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		first, _ := os.ReadFile(configFile)

		if err := ensureOpenCodeOTelConfig(); err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		second, _ := os.ReadFile(configFile)

		if string(first) != string(second) {
			t.Errorf("not idempotent:\nfirst:  %s\nsecond: %s", first, second)
		}
	})

	t.Run("fails on invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		configFile := filepath.Join(dir, "opencode.json")
		openCodeConfigPathOverride = configFile
		defer func() { openCodeConfigPathOverride = "" }()

		_ = os.WriteFile(configFile, []byte("{not valid json"), 0o600)

		if err := ensureOpenCodeOTelConfig(); err == nil {
			t.Error("expected error on invalid JSON, got nil")
		}
	})
}

func TestApplyOpenCodeOTelEnvRespectsTelemetryOptOut(t *testing.T) {
	forceNonInteractive = true
	defer func() { forceNonInteractive = false }()

	env := []string{"OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318"}

	result, changed := applyOpenCodeOTelEnv(env)
	if !changed {
		t.Error("expected changes when OTel endpoint is set")
	}

	found := false
	for _, e := range result {
		if e == "OPENCODE_CLIENT=nav-pilot" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("OPENCODE_CLIENT=nav-pilot not found in env: %v", result)
	}
}

func TestApplyOpenCodeOTelEnvDoesNotOverwriteHeaders(t *testing.T) {
	forceNonInteractive = true
	defer func() { forceNonInteractive = false }()

	env := []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318",
		"OPENCODE_CLIENT=my-custom-client",
	}
	result, _ := applyOpenCodeOTelEnv(env)

	for _, e := range result {
		if e == "OPENCODE_CLIENT=nav-pilot" {
			t.Error("OPENCODE_CLIENT was overwritten by nav-pilot")
		}
	}
}
