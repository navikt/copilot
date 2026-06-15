package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── configPath ───────────────────────────────────────────────────────────────

func TestConfigPath_Default(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", "")
	got := configPath()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".nav-pilot", "config.toml")
	if got != want {
		t.Errorf("configPath() = %q, want %q", got, want)
	}
}

func TestConfigPath_EnvOverride(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", "/custom/path/config.toml")
	got := configPath()
	if got != "/custom/path/config.toml" {
		t.Errorf("configPath() = %q, want /custom/path/config.toml", got)
	}
}

// ─── readConfig ───────────────────────────────────────────────────────────────

func TestReadConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	cfg, err := readConfig()
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config for missing file (fail-soft)")
	}
}

func TestReadConfig_InvalidTOML(t *testing.T) {
	path := writeTempConfig(t, `{not valid toml`)

	t.Setenv("NAV_PILOT_CONFIG", path)
	_, err := readConfig()
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

func TestReadConfig_ValidMinimal(t *testing.T) {
	path := writeTempConfig(t, `version = 1`)
	t.Setenv("NAV_PILOT_CONFIG", path)

	cfg, err := readConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Version != 1 {
		t.Errorf("version = %d, want 1", cfg.Version)
	}
}

func TestReadConfig_AllFields(t *testing.T) {
	path := writeTempConfig(t, `
version = 1
agent = "opencode"
model = "gpt-4"
mode = "plan"
reasoning_effort = "high"
context_tier = "long_context"
allow_all_tools = true
ask_user = false
log_level = "debug"
`)
	t.Setenv("NAV_PILOT_CONFIG", path)

	cfg, err := readConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	assertStrPtr(t, "agent", cfg.Agent, "opencode")
	assertStrPtr(t, "model", cfg.Model, "gpt-4")
	assertStrPtr(t, "mode", cfg.Mode, "plan")
	assertStrPtr(t, "reasoning_effort", cfg.ReasoningEffort, "high")
	assertStrPtr(t, "context_tier", cfg.ContextTier, "long_context")
	assertBoolPtr(t, "allow_all_tools", cfg.AllowAllTools, true)
	assertBoolPtr(t, "ask_user", cfg.AskUser, false)
	assertStrPtr(t, "log_level", cfg.LogLevel, "debug")
}

// ─── validateConfig ───────────────────────────────────────────────────────────

func TestValidateConfig_NilConfig(t *testing.T) {
	if err := validateConfig(nil); err != nil {
		t.Errorf("expected nil error for nil config, got: %v", err)
	}
}

func TestValidateConfig_VersionZero(t *testing.T) {
	cfg := &Config{Version: 0}
	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for version=0")
	}
	if !strings.Contains(err.Error(), "version must be 1") {
		t.Errorf("error should mention version, got: %v", err)
	}
}

func TestValidateConfig_VersionTwo(t *testing.T) {
	cfg := &Config{Version: 2}
	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error for version=2")
	}
}

func TestValidateConfig_ValidVersion(t *testing.T) {
	cfg := &Config{Version: 1}
	if err := validateConfig(cfg); err != nil {
		t.Errorf("expected nil error for version=1, got: %v", err)
	}
}

func TestValidateConfig_UnknownAgent(t *testing.T) {
	tests := []struct {
		agent string
		valid bool
	}{
		{"copilot", true},
		{"opencode", true},
		{"pi", true},
		{"cursor", false},
		{"gpt", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			s := tt.agent
			cfg := &Config{Version: 1, Agent: &s}
			err := validateConfig(cfg)
			if tt.valid && err != nil {
				t.Errorf("expected valid agent %q, got error: %v", tt.agent, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected error for agent %q, got nil", tt.agent)
			}
		})
	}
}

func TestValidateConfig_UnknownMode(t *testing.T) {
	tests := []struct {
		mode  string
		valid bool
	}{
		{"default", true},
		{"plan", true},
		{"autopilot", true},
		{"auto", false},
		{"interactive", false},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			s := tt.mode
			cfg := &Config{Version: 1, Mode: &s}
			err := validateConfig(cfg)
			if tt.valid && err != nil {
				t.Errorf("expected valid mode %q, got error: %v", tt.mode, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected error for mode %q, got nil", tt.mode)
			}
		})
	}
}

func TestValidateConfig_UnknownReasoningEffort(t *testing.T) {
	tests := []struct {
		effort string
		valid  bool
	}{
		{"none", true},
		{"low", true},
		{"medium", true},
		{"high", true},
		{"xhigh", true},
		{"max", true},
		{"ultra", false},
		{"extreme", false},
	}
	for _, tt := range tests {
		t.Run(tt.effort, func(t *testing.T) {
			s := tt.effort
			cfg := &Config{Version: 1, ReasoningEffort: &s}
			err := validateConfig(cfg)
			if tt.valid && err != nil {
				t.Errorf("expected valid effort %q, got error: %v", tt.effort, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected error for effort %q, got nil", tt.effort)
			}
		})
	}
}

func TestValidateConfig_UnknownContextTier(t *testing.T) {
	tests := []struct {
		tier  string
		valid bool
	}{
		{"default", true},
		{"long_context", true},
		{"extended", false},
		{"full", false},
	}
	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			s := tt.tier
			cfg := &Config{Version: 1, ContextTier: &s}
			err := validateConfig(cfg)
			if tt.valid && err != nil {
				t.Errorf("expected valid tier %q, got error: %v", tt.tier, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected error for tier %q, got nil", tt.tier)
			}
		})
	}
}

func TestValidateConfig_UnknownLogLevel(t *testing.T) {
	tests := []struct {
		level string
		valid bool
	}{
		{"none", true},
		{"error", true},
		{"warning", true},
		{"info", true},
		{"debug", true},
		{"all", true},
		{"default", true},
		{"verbose", false},
		{"trace", false},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			s := tt.level
			cfg := &Config{Version: 1, LogLevel: &s}
			err := validateConfig(cfg)
			if tt.valid && err != nil {
				t.Errorf("expected valid log_level %q, got error: %v", tt.level, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected error for log_level %q, got nil", tt.level)
			}
		})
	}
}

func TestValidateConfig_ModelNoAllowlist(t *testing.T) {
	// model accepts any non-empty string; validateConfig does not reject it.
	s := "some-custom-model-name"
	cfg := &Config{Version: 1, Model: &s}
	if err := validateConfig(cfg); err != nil {
		t.Errorf("expected nil error for any model name, got: %v", err)
	}
}

func TestValidateConfig_MultipleErrors(t *testing.T) {
	badAgent := "cursor"
	badMode := "interactive"
	cfg := &Config{Version: 2, Agent: &badAgent, Mode: &badMode}
	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "version") {
		t.Errorf("error should mention version, got: %s", msg)
	}
	if !strings.Contains(msg, "agent") {
		t.Errorf("error should mention agent, got: %s", msg)
	}
	if !strings.Contains(msg, "mode") {
		t.Errorf("error should mention mode, got: %s", msg)
	}
}

// ─── validateConfigProblems (no doubled-dash regression) ─────────────────────

func TestValidateConfigProblems_NilConfig(t *testing.T) {
	if p := validateConfigProblems(nil); len(p) != 0 {
		t.Errorf("expected empty problems for nil config, got: %v", p)
	}
}

func TestValidateConfigProblems_MultipleProblems(t *testing.T) {
	badAgent := "cursor"
	badMode := "interactive"
	cfg := &Config{Version: 2, Agent: &badAgent, Mode: &badMode}
	problems := validateConfigProblems(cfg)
	if len(problems) != 3 {
		t.Fatalf("expected 3 problems (version, agent, mode), got %d: %v", len(problems), problems)
	}
	// Each problem must be a plain string, not starting with "- ".
	for _, p := range problems {
		if strings.HasPrefix(p, "- ") {
			t.Errorf("problem has leading '- ' (doubled-dash bug): %q", p)
		}
	}
}

func TestCmdConfigValidate_NoBulletDoubling(t *testing.T) {
	// Regression: validate must print "  - version must be 1 (got 0)", not "  - - version…"
	path := writeTempConfig(t, "version = 0\n")
	t.Setenv("NAV_PILOT_CONFIG", path)

	// cmdConfigValidate prints to stdout and returns an error for invalid configs.
	err := cmdConfigValidate()
	if err == nil {
		t.Fatal("expected error for version=0")
	}
	// The error itself is just "config validation failed", not the bullet-prefixed message.
	if strings.Contains(err.Error(), "- -") {
		t.Errorf("doubled-dash bug in error: %q", err.Error())
	}
}

// ─── config init produces a valid config ─────────────────────────────────────

func TestCmdConfigInit_ValidatesOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)

	if err := cmdConfigInit(); err != nil {
		t.Fatalf("cmdConfigInit() error: %v", err)
	}
	// The generated file must pass validation without any user edits.
	if err := cmdConfigValidate(); err != nil {
		t.Errorf("config validate after init should succeed, got: %v", err)
	}
}

func TestCmdConfigInit_HasActiveVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)

	if err := cmdConfigInit(); err != nil {
		t.Fatalf("cmdConfigInit() error: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	// Active version line must be present (not commented out).
	if !strings.Contains(content, "\nversion = 1\n") {
		t.Errorf("init template must contain active 'version = 1' line, got:\n%s", content)
	}
	// The commented-out form must NOT be the only version line.
	if strings.Contains(content, "\n# version = 1\n") && !strings.Contains(content, "\nversion = 1\n") {
		t.Error("version must be active, not just commented out")
	}
}

func TestReadConfigWithMeta_UnknownKeys(t *testing.T) {
	path := writeTempConfig(t, `
version = 1
agent = "copilot"
unknown_key = "oops"
another_bad = 42
`)
	t.Setenv("NAV_PILOT_CONFIG", path)

	_, meta, err := readConfigWithMeta()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	undecoded := meta.Undecoded()
	if len(undecoded) != 2 {
		t.Errorf("expected 2 undecoded keys, got %d: %v", len(undecoded), undecoded)
	}
}

func TestReadConfigWithMeta_NoUnknownKeys(t *testing.T) {
	path := writeTempConfig(t, `
version = 1
agent = "copilot"
mode = "default"
`)
	t.Setenv("NAV_PILOT_CONFIG", path)

	_, meta, err := readConfigWithMeta()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	undecoded := meta.Undecoded()
	if len(undecoded) != 0 {
		t.Errorf("expected 0 undecoded keys, got %d: %v", len(undecoded), undecoded)
	}
}

// ─── resolve (precedence matrix) ─────────────────────────────────────────────

func TestResolve_Defaults(t *testing.T) {
	r := resolve(nil, CLIOverrides{})
	if r.Agent != "copilot" {
		t.Errorf("Agent = %q, want copilot", r.Agent)
	}
	if r.Mode != "default" {
		t.Errorf("Mode = %q, want default", r.Mode)
	}
	if r.AskUser != true {
		t.Error("AskUser = false, want true")
	}
	if r.AllowAllTools != false {
		t.Error("AllowAllTools = true, want false")
	}
	if r.Model != "" {
		t.Errorf("Model = %q, want empty", r.Model)
	}
	if r.ReasoningEffort != "" {
		t.Errorf("ReasoningEffort = %q, want empty", r.ReasoningEffort)
	}
	if r.ContextTier != "" {
		t.Errorf("ContextTier = %q, want empty", r.ContextTier)
	}
	if r.LogLevel != "" {
		t.Errorf("LogLevel = %q, want empty", r.LogLevel)
	}
}

func TestResolve_FileOverridesDefaults(t *testing.T) {
	agent := "opencode"
	mode := "plan"
	model := "gpt-4"
	effort := "high"
	tier := "long_context"
	allowAll := true
	askUser := false
	logLevel := "debug"

	cfg := &Config{
		Version:         1,
		Agent:           &agent,
		Mode:            &mode,
		Model:           &model,
		ReasoningEffort: &effort,
		ContextTier:     &tier,
		AllowAllTools:   &allowAll,
		AskUser:         &askUser,
		LogLevel:        &logLevel,
	}

	r := resolve(cfg, CLIOverrides{})
	if r.Agent != "opencode" {
		t.Errorf("Agent = %q, want opencode", r.Agent)
	}
	if r.Mode != "plan" {
		t.Errorf("Mode = %q, want plan", r.Mode)
	}
	if r.Model != "gpt-4" {
		t.Errorf("Model = %q, want gpt-4", r.Model)
	}
	if r.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want high", r.ReasoningEffort)
	}
	if r.ContextTier != "long_context" {
		t.Errorf("ContextTier = %q, want long_context", r.ContextTier)
	}
	if !r.AllowAllTools {
		t.Error("AllowAllTools = false, want true")
	}
	if r.AskUser {
		t.Error("AskUser = true, want false")
	}
	if r.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", r.LogLevel)
	}
}

func TestResolve_CLIOverridesFile(t *testing.T) {
	fileAgent := "opencode"
	fileMode := "plan"
	cfg := &Config{
		Version: 1,
		Agent:   &fileAgent,
		Mode:    &fileMode,
	}

	trueVal := true
	r := resolve(cfg, CLIOverrides{
		Agent:         "pi",
		Mode:          "autopilot",
		AllowAllTools: &trueVal,
	})

	if r.Agent != "pi" {
		t.Errorf("Agent = %q, want pi (CLI overrides file)", r.Agent)
	}
	if r.Mode != "autopilot" {
		t.Errorf("Mode = %q, want autopilot (CLI overrides file)", r.Mode)
	}
	if !r.AllowAllTools {
		t.Error("AllowAllTools = false, want true (CLI overrides default)")
	}
}

func TestResolve_CLIOverridesDefaults(t *testing.T) {
	falseVal := false
	r := resolve(nil, CLIOverrides{
		Agent:   "opencode",
		AskUser: &falseVal,
	})
	if r.Agent != "opencode" {
		t.Errorf("Agent = %q, want opencode", r.Agent)
	}
	if r.AskUser {
		t.Error("AskUser = true, want false (CLI override)")
	}
	// Unset CLI fields still fall back to built-in defaults.
	if r.Mode != "default" {
		t.Errorf("Mode = %q, want default", r.Mode)
	}
}

func TestResolve_FileNilFieldsKeepDefaults(t *testing.T) {
	// File sets agent but leaves mode/ask_user unset.
	agent := "pi"
	cfg := &Config{Version: 1, Agent: &agent}
	r := resolve(cfg, CLIOverrides{})

	if r.Agent != "pi" {
		t.Errorf("Agent = %q, want pi", r.Agent)
	}
	if r.Mode != "default" {
		t.Errorf("Mode = %q, want default (unset in file → default)", r.Mode)
	}
	if !r.AskUser {
		t.Error("AskUser = false, want true (unset in file → default)")
	}
}

// ─── cmdConfigPath ────────────────────────────────────────────────────────────

func TestCmdConfigPath(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "myconfig.toml")
	t.Setenv("NAV_PILOT_CONFIG", want)

	// Redirect stdout capture via the fact that cmdConfigPath just prints.
	// We test indirectly: configPath() returns the correct value.
	if got := configPath(); got != want {
		t.Errorf("configPath() = %q, want %q", got, want)
	}
}

// ─── cmdConfigInit ────────────────────────────────────────────────────────────

func TestCmdConfigInit_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)

	if err := cmdConfigInit(); err != nil {
		t.Fatalf("cmdConfigInit() returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	content := string(data)
	// Should contain all key names as comments.
	for _, kd := range configKeyDefs {
		if !strings.Contains(content, kd.name) {
			t.Errorf("init template missing key: %s", kd.name)
		}
	}
}

func TestCmdConfigInit_DoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)

	// Create file with distinct content.
	_ = os.WriteFile(path, []byte("version = 1\n"), 0o644)

	err := cmdConfigInit()
	if err == nil {
		t.Fatal("expected error when file already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}

	// Original content must be unchanged.
	data, _ := os.ReadFile(path)
	if string(data) != "version = 1\n" {
		t.Error("cmdConfigInit must not overwrite existing file")
	}
}

// ─── cmdConfigSet / cmdConfigGet ─────────────────────────────────────────────

func TestCmdConfigSet_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	if err := cmdConfigSet("agent", "opencode"); err != nil {
		t.Fatalf("cmdConfigSet() error: %v", err)
	}

	// File should exist and contain the key.
	data, err := os.ReadFile(configPath())
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if !strings.Contains(string(data), `agent = "opencode"`) {
		t.Errorf("config file missing expected line, got:\n%s", string(data))
	}
}

func TestCmdConfigSet_UpdatesExistingKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)

	_ = os.WriteFile(path, []byte("version = 1\nagent = \"copilot\"\n"), 0o644)

	if err := cmdConfigSet("agent", "opencode"); err != nil {
		t.Fatalf("cmdConfigSet() error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `agent = "opencode"`) {
		t.Errorf("agent not updated, got:\n%s", string(data))
	}
	if strings.Contains(string(data), `agent = "copilot"`) {
		t.Errorf("old agent value should be gone, got:\n%s", string(data))
	}
}

func TestCmdConfigSet_UncommentsCommentedKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)

	_ = os.WriteFile(path, []byte("version = 1\n# agent = \"copilot\"\n"), 0o644)

	if err := cmdConfigSet("agent", "pi"); err != nil {
		t.Fatalf("cmdConfigSet() error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `agent = "pi"`) {
		t.Errorf("expected uncommented agent = pi, got:\n%s", string(data))
	}
}

func TestCmdConfigSet_InvalidKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	err := cmdConfigSet("not_a_key", "value")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestCmdConfigSet_InvalidValue(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	err := cmdConfigSet("agent", "cursor") // not in allowed list
	if err == nil {
		t.Fatal("expected error for invalid value")
	}
}

func TestCmdConfigSet_BoolField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)

	if err := cmdConfigSet("allow_all_tools", "true"); err != nil {
		t.Fatalf("cmdConfigSet() error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "allow_all_tools = true") {
		t.Errorf("expected allow_all_tools = true, got:\n%s", string(data))
	}
}

func TestCmdConfigSet_IntField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)

	if err := cmdConfigSet("version", "1"); err != nil {
		t.Fatalf("cmdConfigSet() error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "version = 1") {
		t.Errorf("expected version = 1, got:\n%s", string(data))
	}
}

func TestCmdConfigGet_KnownKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)
	_ = os.WriteFile(path, []byte("version = 1\nagent = \"pi\"\n"), 0o644)

	// Verify the key def lookup works for all known keys.
	for _, kd := range configKeyDefs {
		if err := cmdConfigGet(kd.name); err != nil {
			t.Errorf("cmdConfigGet(%q) returned unexpected error: %v", kd.name, err)
		}
	}
}

func TestCmdConfigGet_UnknownKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	err := cmdConfigGet("does_not_exist")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

// ─── cmdConfigValidate ────────────────────────────────────────────────────────

func TestCmdConfigValidate_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	// Missing file should not return an error (just print a warning).
	if err := cmdConfigValidate(); err != nil {
		t.Errorf("expected nil error for missing file, got: %v", err)
	}
}

func TestCmdConfigValidate_InvalidTOML(t *testing.T) {
	path := writeTempConfig(t, `{invalid toml`)
	t.Setenv("NAV_PILOT_CONFIG", path)

	err := cmdConfigValidate()
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestCmdConfigValidate_UnknownKey(t *testing.T) {
	path := writeTempConfig(t, "version = 1\nbad_key = \"oops\"\n")
	t.Setenv("NAV_PILOT_CONFIG", path)

	err := cmdConfigValidate()
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestCmdConfigValidate_InvalidVersion(t *testing.T) {
	path := writeTempConfig(t, "version = 99\n")
	t.Setenv("NAV_PILOT_CONFIG", path)

	err := cmdConfigValidate()
	if err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestCmdConfigValidate_Valid(t *testing.T) {
	path := writeTempConfig(t, `
version = 1
agent = "copilot"
mode = "default"
allow_all_tools = false
ask_user = true
`)
	t.Setenv("NAV_PILOT_CONFIG", path)

	if err := cmdConfigValidate(); err != nil {
		t.Errorf("expected nil error for valid config, got: %v", err)
	}
}

// ─── isConfigKeyLine ─────────────────────────────────────────────────────────

func TestIsConfigKeyLine(t *testing.T) {
	tests := []struct {
		line string
		key  string
		want bool
	}{
		{`agent = "copilot"`, "agent", true},
		{`# agent = "copilot"`, "agent", true},
		{`  # agent = "copilot"`, "agent", true},
		{`## agent = "copilot"`, "agent", true},
		{`agent="copilot"`, "agent", true},
		{`# This is a general comment`, "agent", false},
		{`# model = "gpt-4"`, "agent", false},
		{`allow_all_tools = false`, "allow_all_tools", true},
		{`# allow_all_tools = false`, "allow_all_tools", true},
		// Key must not match a longer key name.
		{`allow_all_tools_extra = "x"`, "allow_all_tools", false},
		{`reasoning_effort = "high"`, "reasoning_effort", true},
		{`reason = "other"`, "reasoning_effort", false},
	}
	for _, tt := range tests {
		t.Run(tt.line+"_"+tt.key, func(t *testing.T) {
			got := isConfigKeyLine(tt.line, tt.key)
			if got != tt.want {
				t.Errorf("isConfigKeyLine(%q, %q) = %v, want %v", tt.line, tt.key, got, tt.want)
			}
		})
	}
}

// ─── validateKeyValue ────────────────────────────────────────────────────────

func TestValidateKeyValue(t *testing.T) {
	tests := []struct {
		key     string
		value   string
		wantErr bool
	}{
		{"agent", "copilot", false},
		{"agent", "opencode", false},
		{"agent", "pi", false},
		{"agent", "cursor", true},
		{"mode", "default", false},
		{"mode", "autopilot", false},
		{"mode", "bad", true},
		{"reasoning_effort", "max", false},
		{"reasoning_effort", "ultra", true},
		{"context_tier", "long_context", false},
		{"context_tier", "extended", true},
		{"log_level", "debug", false},
		{"log_level", "verbose", true},
		{"allow_all_tools", "true", false},
		{"allow_all_tools", "false", false},
		{"allow_all_tools", "yes", false},
		{"allow_all_tools", "maybe", true},
		{"ask_user", "true", false},
		{"ask_user", "false", false},
		{"version", "1", false},
		{"version", "2", true},
		{"version", "abc", true},
		{"model", "any-model-name", false},      // no allowlist
		{"model", "gpt-4-turbo-preview", false}, // no allowlist
	}
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			kd := findKeyDef(tt.key)
			if kd == nil {
				t.Fatalf("key %q not found in configKeyDefs", tt.key)
			}
			err := validateKeyValue(kd, tt.value)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %s=%s, got nil", tt.key, tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %s=%s: %v", tt.key, tt.value, err)
			}
		})
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeTempConfig: %v", err)
	}
	return path
}

func assertStrPtr(t *testing.T, field string, got *string, want string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = nil, want %q", field, want)
		return
	}
	if *got != want {
		t.Errorf("%s = %q, want %q", field, *got, want)
	}
}

func assertBoolPtr(t *testing.T, field string, got *bool, want bool) {
	t.Helper()
	if got == nil {
		t.Errorf("%s = nil, want %v", field, want)
		return
	}
	if *got != want {
		t.Errorf("%s = %v, want %v", field, *got, want)
	}
}
