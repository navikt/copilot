package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeArgs(t *testing.T) {
	def := openCodeDefaultModel
	tests := []struct {
		name     string
		resolved ResolvedConfig
		want     []string
	}{
		{
			name:     "empty resolved applies Nav default model",
			resolved: ResolvedConfig{Mode: "default", AskUser: true},
			want:     []string{"--model", def},
		},
		{
			name:     "explicit model overrides default",
			resolved: ResolvedConfig{Model: "anthropic/claude-3-5-sonnet", Mode: "default", AskUser: true},
			want:     []string{"--model", "anthropic/claude-3-5-sonnet"},
		},
		{
			name:     "plan mode maps to --agent plan (default model still emitted)",
			resolved: ResolvedConfig{Mode: "plan", AskUser: true},
			want:     []string{"--model", def, "--agent", "plan"},
		},
		{
			name:     "default mode not emitted (only default model)",
			resolved: ResolvedConfig{Mode: "default", AskUser: true},
			want:     []string{"--model", def},
		},
		{
			name:     "reasoning effort maps to --variant",
			resolved: ResolvedConfig{Mode: "default", ReasoningEffort: "high", AskUser: true},
			want:     []string{"--model", def, "--variant", "high"},
		},
		{
			name:     "allow_all_tools maps to --dangerously-skip-permissions",
			resolved: ResolvedConfig{Mode: "default", AllowAllTools: true, AskUser: true},
			want:     []string{"--model", def, "--dangerously-skip-permissions"},
		},
		{
			name:     "log level",
			resolved: ResolvedConfig{Mode: "default", LogLevel: "debug", AskUser: true},
			want:     []string{"--model", def, "--log-level", "DEBUG"},
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
			want:     []string{"--model", def},
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

func TestOpenCodeUnsupportedConfigWarnings(t *testing.T) {
	tests := []struct {
		name     string
		resolved ResolvedConfig
		wantMsgs []string // substrings that must appear in warnings
		wantNone bool     // true if no warnings expected
	}{
		{
			name:     "default config — no warnings",
			resolved: ResolvedConfig{Mode: "default", AskUser: true},
			wantNone: true,
		},
		{
			name:     "autopilot mode warns",
			resolved: ResolvedConfig{Mode: "autopilot", AskUser: true},
			wantMsgs: []string{"autopilot", "no opencode equivalent"},
		},
		{
			name:     "context_tier set warns",
			resolved: ResolvedConfig{Mode: "default", ContextTier: "long_context", AskUser: true},
			wantMsgs: []string{"context_tier", "no opencode equivalent"},
		},
		{
			name:     "ask_user false warns",
			resolved: ResolvedConfig{Mode: "default", AskUser: false},
			wantMsgs: []string{"ask_user", "no opencode equivalent"},
		},
		{
			name: "all three unmapped fields warn",
			resolved: ResolvedConfig{
				Mode:        "autopilot",
				ContextTier: "long_context",
				AskUser:     false,
			},
			wantMsgs: []string{"autopilot", "context_tier", "ask_user"},
		},
		{
			name:     "plan mode — no warning (has opencode equivalent)",
			resolved: ResolvedConfig{Mode: "plan", AskUser: true},
			wantNone: true,
		},
		{
			name:     "allow_all_tools — no warning (has opencode equivalent)",
			resolved: ResolvedConfig{Mode: "default", AllowAllTools: true, AskUser: true},
			wantNone: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := openCodeUnsupportedConfigWarnings(tt.resolved)
			if tt.wantNone {
				if len(got) != 0 {
					t.Errorf("expected no warnings, got: %v", got)
				}
				return
			}
			joined := strings.Join(got, " ")
			for _, sub := range tt.wantMsgs {
				if !strings.Contains(joined, sub) {
					t.Errorf("warnings %v missing substring %q", got, sub)
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

func TestApplyOpenCodeOTelEnvInjectsClientWhenEndpointSet(t *testing.T) {
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

func TestApplyOpenCodeOTelEnvDoesNotOverwriteExistingClient(t *testing.T) {
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

func TestOpenCodeNavContextDir_Override(t *testing.T) {
	old := openCodeNavContextDirOverride
	dir := t.TempDir()
	openCodeNavContextDirOverride = dir
	defer func() { openCodeNavContextDirOverride = old }()

	got := openCodeNavContextDir()
	if got != dir {
		t.Errorf("openCodeNavContextDir() = %q, want %q", got, dir)
	}
}

func TestOpenCodeNavContextDir_DefaultSuffix(t *testing.T) {
	old := openCodeNavContextDirOverride
	openCodeNavContextDirOverride = ""
	defer func() { openCodeNavContextDirOverride = old }()

	got := openCodeNavContextDir()
	want := filepath.Join(".config", "opencode")
	if !strings.HasSuffix(got, want) {
		t.Errorf("openCodeNavContextDir() = %q, want suffix %q", got, want)
	}
}

func TestEnsureOpenCodeNavContext(t *testing.T) {
	// Override the output dir so we don't touch ~/.config/opencode
	outputDir := t.TempDir()
	old := openCodeNavContextDirOverride
	openCodeNavContextDirOverride = outputDir
	defer func() { openCodeNavContextDirOverride = old }()

	// Override cloneRemoteFn to return a local fixture source
	origClone := cloneRemoteFn
	defer func() { cloneRemoteFn = origClone }()
	sourceDir := setupTestSource(t)
	cloneRemoteFn = func(ref, sourceRepo string) (*Source, error) {
		return &Source{Dir: sourceDir, SHA: "test"}, nil
	}

	summary, err := ensureOpenCodeNavContext()
	if err != nil {
		t.Fatalf("ensureOpenCodeNavContext() error: %v", err)
	}
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if !strings.Contains(summary, "skill") && !strings.Contains(summary, "AGENTS") {
		t.Errorf("summary should mention artifacts, got: %q", summary)
	}

	// Verify AGENTS.md was written
	if _, err := os.Stat(filepath.Join(outputDir, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md not found after ensureOpenCodeNavContext: %v", err)
	}
}

func TestEnsureOpenCodeNavContextIdempotent(t *testing.T) {
	outputDir := t.TempDir()
	old := openCodeNavContextDirOverride
	openCodeNavContextDirOverride = outputDir
	defer func() { openCodeNavContextDirOverride = old }()

	origClone := cloneRemoteFn
	defer func() { cloneRemoteFn = origClone }()
	sourceDir := setupTestSource(t)
	cloneRemoteFn = func(ref, sourceRepo string) (*Source, error) {
		return &Source{Dir: sourceDir, SHA: "test"}, nil
	}

	// First call
	s1, err := ensureOpenCodeNavContext()
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	first, _ := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))

	// Second call — must succeed and produce identical AGENTS.md
	s2, err := ensureOpenCodeNavContext()
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	second, _ := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))

	if s1 != s2 {
		t.Errorf("summary changed between calls: %q vs %q", s1, s2)
	}
	if string(first) != string(second) {
		t.Errorf("AGENTS.md not idempotent")
	}
}
