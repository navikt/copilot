package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
	telemetrypkg "github.com/navikt/copilot/cli/nav-pilot/internal/telemetry"
)

func TestToOpenCodeModel(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", OpenCodeDefaultModel},
		{"auto", OpenCodeDefaultModel},
		{"  ", OpenCodeDefaultModel},
		{"claude-sonnet-4.6", "github-copilot/claude-sonnet-4.6"},
		{"gpt-5.5", "github-copilot/gpt-5.5"},
		{"github-copilot/claude-opus-4.8", "github-copilot/claude-opus-4.8"},
		{"anthropic/claude-3-5-sonnet", "anthropic/claude-3-5-sonnet"},
		{"  claude-haiku-4.5 ", "github-copilot/claude-haiku-4.5"},
	}
	for _, tt := range tests {
		if got := ToOpenCodeModel(tt.in); got != tt.want {
			t.Errorf("ToOpenCodeModel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOpenCodeArgs(t *testing.T) {
	def := OpenCodeDefaultModel
	tests := []struct {
		name     string
		resolved domain.ResolvedConfig
		want     []string
	}{
		{
			name:     "empty resolved applies Nav default model",
			resolved: domain.ResolvedConfig{Mode: "default", AskUser: true},
			want:     []string{"--model", def, "--agent", "nav-pilot"},
		},
		{
			name:     "explicit model overrides default",
			resolved: domain.ResolvedConfig{Model: "anthropic/claude-3-5-sonnet", Mode: "default", AskUser: true},
			want:     []string{"--model", "anthropic/claude-3-5-sonnet", "--agent", "nav-pilot"},
		},
		{
			name:     "plan mode maps to --agent plan (default model still emitted)",
			resolved: domain.ResolvedConfig{Mode: "plan", AskUser: true},
			want:     []string{"--model", def, "--agent", "plan"},
		},
		{
			name:     "default mode not emitted (only default model)",
			resolved: domain.ResolvedConfig{Mode: "default", AskUser: true},
			want:     []string{"--model", def, "--agent", "nav-pilot"},
		},
		{
			name:     "reasoning effort maps to --variant",
			resolved: domain.ResolvedConfig{Mode: "default", ReasoningEffort: "high", AskUser: true},
			want:     []string{"--model", def, "--agent", "nav-pilot", "--variant", "high"},
		},
		{
			name:     "allow_all_tools maps to --dangerously-skip-permissions",
			resolved: domain.ResolvedConfig{Mode: "default", AllowAllTools: true, AskUser: true},
			want:     []string{"--model", def, "--agent", "nav-pilot", "--dangerously-skip-permissions"},
		},
		{
			name:     "log level",
			resolved: domain.ResolvedConfig{Mode: "default", LogLevel: "debug", AskUser: true},
			want:     []string{"--model", def, "--agent", "nav-pilot", "--log-level", "DEBUG"},
		},
		{
			name: "all fields",
			resolved: domain.ResolvedConfig{
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
			resolved: domain.ResolvedConfig{Mode: "default", AskUser: false},
			want:     []string{"--model", def, "--agent", "nav-pilot"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OpenCodeArgs(tt.resolved)
			if len(got) != len(tt.want) {
				t.Fatalf("OpenCodeArgs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("OpenCodeArgs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestOpenCodeUnsupportedConfigWarnings(t *testing.T) {
	tests := []struct {
		name     string
		resolved domain.ResolvedConfig
		wantMsgs []string
		wantNone bool
	}{
		{
			name:     "default config — no warnings",
			resolved: domain.ResolvedConfig{Mode: "default", AskUser: true},
			wantNone: true,
		},
		{
			name:     "autopilot mode warns",
			resolved: domain.ResolvedConfig{Mode: "autopilot", AskUser: true},
			wantMsgs: []string{"autopilot", "no opencode equivalent"},
		},
		{
			name:     "context_tier set warns",
			resolved: domain.ResolvedConfig{Mode: "default", ContextTier: "long_context", AskUser: true},
			wantMsgs: []string{"context_tier", "no opencode equivalent"},
		},
		{
			name:     "ask_user false warns",
			resolved: domain.ResolvedConfig{Mode: "default", AskUser: false},
			wantMsgs: []string{"ask_user", "no opencode equivalent"},
		},
		{
			name: "all three unmapped fields warn",
			resolved: domain.ResolvedConfig{
				Mode:        "autopilot",
				ContextTier: "long_context",
				AskUser:     false,
			},
			wantMsgs: []string{"autopilot", "context_tier", "ask_user"},
		},
		{
			name:     "plan mode — no warning (has opencode equivalent)",
			resolved: domain.ResolvedConfig{Mode: "plan", AskUser: true},
			wantNone: true,
		},
		{
			name:     "allow_all_tools — no warning (has opencode equivalent)",
			resolved: domain.ResolvedConfig{Mode: "default", AllowAllTools: true, AskUser: true},
			wantNone: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OpenCodeUnsupportedConfigWarnings(tt.resolved)
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
		ConfigPathOverride = configFile
		defer func() { ConfigPathOverride = "" }()

		if err := EnsureOpenCodeOTelConfig(); err != nil {
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
		ConfigPathOverride = configFile
		defer func() { ConfigPathOverride = "" }()

		existing := map[string]any{
			"theme":      "dark",
			"autoupdate": "always",
			"experimental": map[string]any{
				"someOtherFlag": true,
			},
		}
		data, _ := json.MarshalIndent(existing, "", "  ")
		_ = os.WriteFile(configFile, data, 0o600)

		if err := EnsureOpenCodeOTelConfig(); err != nil {
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
		ConfigPathOverride = configFile
		defer func() { ConfigPathOverride = "" }()

		if err := EnsureOpenCodeOTelConfig(); err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		first, _ := os.ReadFile(configFile)

		if err := EnsureOpenCodeOTelConfig(); err != nil {
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
		ConfigPathOverride = configFile
		defer func() { ConfigPathOverride = "" }()

		_ = os.WriteFile(configFile, []byte("{not valid json"), 0o600)

		if err := EnsureOpenCodeOTelConfig(); err == nil {
			t.Error("expected error on invalid JSON, got nil")
		}
	})
}

func TestApplyOpenCodeOTelEnvInjectsClientWhenEndpointSet(t *testing.T) {
	env := []string{"OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318"}
	result, changed := telemetrypkg.ApplyOpenCodeOTelEnv(env, cliVersion)
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
	env := []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318",
		"OPENCODE_CLIENT=my-custom-client",
	}
	result, _ := telemetrypkg.ApplyOpenCodeOTelEnv(env, cliVersion)

	for _, e := range result {
		if e == "OPENCODE_CLIENT=nav-pilot" {
			t.Error("OPENCODE_CLIENT was overwritten by nav-pilot")
		}
	}
}

func TestOpenCodeNavContextDir_Override(t *testing.T) {
	old := NavContextDirOverride
	dir := t.TempDir()
	NavContextDirOverride = dir
	defer func() { NavContextDirOverride = old }()

	got := openCodeNavContextDir()
	if got != dir {
		t.Errorf("openCodeNavContextDir() = %q, want %q", got, dir)
	}
}

func TestOpenCodeNavContextDir_DefaultSuffix(t *testing.T) {
	old := NavContextDirOverride
	NavContextDirOverride = ""
	defer func() { NavContextDirOverride = old }()

	got := openCodeNavContextDir()
	want := filepath.Join(".config", "opencode")
	if !strings.HasSuffix(got, want) {
		t.Errorf("openCodeNavContextDir() = %q, want suffix %q", got, want)
	}
}

func TestOpenCodeConfigPath_EmptyHome_ReturnsAbsolute(t *testing.T) {
	old := ConfigPathOverride
	ConfigPathOverride = ""
	defer func() { ConfigPathOverride = old }()

	t.Setenv("HOME", "")

	got := openCodeConfigPath()
	if !filepath.IsAbs(got) {
		t.Errorf("openCodeConfigPath() with empty HOME = %q, want absolute path", got)
	}
}

func TestOpenCodeNavContextDir_EmptyHome_ReturnsAbsolute(t *testing.T) {
	old := NavContextDirOverride
	NavContextDirOverride = ""
	defer func() { NavContextDirOverride = old }()

	t.Setenv("HOME", "")

	got := openCodeNavContextDir()
	if !filepath.IsAbs(got) {
		t.Errorf("openCodeNavContextDir() with empty HOME = %q, want absolute path", got)
	}
}

func TestEnsureOpenCodeNavContext(t *testing.T) {
	outputDir := t.TempDir()
	old := NavContextDirOverride
	NavContextDirOverride = outputDir
	defer func() { NavContextDirOverride = old }()

	origClone := source.CloneRemoteFn
	defer func() { source.CloneRemoteFn = origClone }()
	sourceDir := setupTestSource(t)
	source.CloneRemoteFn = func(ref, sourceRepo string) (*source.Source, error) {
		return &source.Source{Dir: sourceDir, SHA: "test"}, nil
	}

	summary, err := EnsureOpenCodeNavContext()
	if err != nil {
		t.Fatalf("EnsureOpenCodeNavContext() error: %v", err)
	}
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if !strings.Contains(summary, "skill") && !strings.Contains(summary, "AGENTS") {
		t.Errorf("summary should mention artifacts, got: %q", summary)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "AGENTS.md")); err != nil {
		t.Errorf("AGENTS.md not found after EnsureOpenCodeNavContext: %v", err)
	}
}

func TestEnsureOpenCodeNavContextIdempotent(t *testing.T) {
	outputDir := t.TempDir()
	old := NavContextDirOverride
	NavContextDirOverride = outputDir
	defer func() { NavContextDirOverride = old }()

	origClone := source.CloneRemoteFn
	defer func() { source.CloneRemoteFn = origClone }()
	sourceDir := setupTestSource(t)
	source.CloneRemoteFn = func(ref, sourceRepo string) (*source.Source, error) {
		return &source.Source{Dir: sourceDir, SHA: "test"}, nil
	}

	s1, err := EnsureOpenCodeNavContext()
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	first, _ := os.ReadFile(filepath.Join(outputDir, "AGENTS.md"))

	s2, err := EnsureOpenCodeNavContext()
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
