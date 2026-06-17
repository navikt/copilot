package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
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
client = "opencode"
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

	assertStrPtr(t, "client", cfg.Client, "opencode")
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

func TestValidateConfig_UnknownClient(t *testing.T) {
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
			cfg := &Config{Version: 1, Client: &s}
			err := validateConfig(cfg)
			if tt.valid && err != nil {
				t.Errorf("expected valid client %q, got error: %v", tt.agent, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected error for client %q, got nil", tt.agent)
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

func TestValidateConfig_ModelFormatValidation(t *testing.T) {
	good := []string{"auto", "gpt-5.5", "claude-opus-4.8", "anthropic/claude-3-5-sonnet", "some_model"}
	for _, m := range good {
		m := m
		cfg := &Config{Version: 1, Model: &m}
		if err := validateConfig(cfg); err != nil {
			t.Errorf("expected nil error for model %q, got: %v", m, err)
		}
	}
	bad := []string{"", " gpt-4o", "gpt-4o ", "gpt 4o", "gpt@4o", "-bad"}
	for _, m := range bad {
		m := m
		cfg := &Config{Version: 1, Model: &m}
		if err := validateConfig(cfg); err == nil {
			t.Errorf("expected error for invalid model %q, got nil", m)
		}
	}
}

func TestValidateConfig_MultipleErrors(t *testing.T) {
	badClient := "cursor"
	badMode := "interactive"
	cfg := &Config{Version: 2, Client: &badClient, Mode: &badMode}
	err := validateConfig(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "version") {
		t.Errorf("error should mention version, got: %s", msg)
	}
	if !strings.Contains(msg, "client") {
		t.Errorf("error should mention client, got: %s", msg)
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
	badClient := "cursor"
	badMode := "interactive"
	cfg := &Config{Version: 2, Client: &badClient, Mode: &badMode}
	problems := validateConfigProblems(cfg)
	if len(problems) != 3 {
		t.Fatalf("expected 3 problems (version, client, mode), got %d: %v", len(problems), problems)
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
client = "copilot"
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
client = "copilot"
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
	if r.Client != "copilot" {
		t.Errorf("Client = %q, want copilot", r.Client)
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
	if r.OtelLogLevel != "none" {
		t.Errorf("OtelLogLevel = %q, want none", r.OtelLogLevel)
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
		Client:          &agent,
		Mode:            &mode,
		Model:           &model,
		ReasoningEffort: &effort,
		ContextTier:     &tier,
		AllowAllTools:   &allowAll,
		AskUser:         &askUser,
		LogLevel:        &logLevel,
	}

	r := resolve(cfg, CLIOverrides{})
	if r.Client != "opencode" {
		t.Errorf("Client = %q, want opencode", r.Client)
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
		Client:  &fileAgent,
		Mode:    &fileMode,
	}

	trueVal := true
	r := resolve(cfg, CLIOverrides{
		Client:        "pi",
		Mode:          "autopilot",
		AllowAllTools: &trueVal,
	})

	if r.Client != "pi" {
		t.Errorf("Client = %q, want pi (CLI overrides file)", r.Client)
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
		Client:  "opencode",
		AskUser: &falseVal,
	})
	if r.Client != "opencode" {
		t.Errorf("Client = %q, want opencode", r.Client)
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
	// File sets client but leaves mode/ask_user unset.
	agent := "pi"
	cfg := &Config{Version: 1, Client: &agent}
	r := resolve(cfg, CLIOverrides{})

	if r.Client != "pi" {
		t.Errorf("Client = %q, want pi", r.Client)
	}
	if r.Mode != "default" {
		t.Errorf("Mode = %q, want default (unset in file → default)", r.Mode)
	}
	if !r.AskUser {
		t.Error("AskUser = false, want true (unset in file → default)")
	}
}

// ─── unknown key rejection ────────────────────────────────────────────────────

func TestLoadConfigForLaunch_UnknownKeyRejected(t *testing.T) {
	// A config file with the old `agent = "..."` key must now be REJECTED as an
	// unknown key (no backward compat, no silent ignore).
	path := writeTempConfig(t, "version = 1\nagent = \"opencode\"\n")
	t.Setenv("NAV_PILOT_CONFIG", path)

	_, err := loadConfigForLaunch(CLIOverrides{})
	if err == nil {
		t.Fatal("expected error for config with old 'agent' key")
	}
	if !strings.Contains(err.Error(), "agent") {
		t.Errorf("expected 'agent' in error message, got: %v", err)
	}
}

func TestLoadConfigForLaunch_AnyBogusKeyRejected(t *testing.T) {
	path := writeTempConfig(t, "version = 1\nclient = \"copilot\"\nnosuchkey = \"x\"\n")
	t.Setenv("NAV_PILOT_CONFIG", path)

	_, err := loadConfigForLaunch(CLIOverrides{})
	if err == nil {
		t.Fatal("expected error for config with unknown key")
	}
	if !strings.Contains(err.Error(), "nosuchkey") {
		t.Errorf("expected 'nosuchkey' in error message, got: %v", err)
	}
}

func TestCLIClientOverride(t *testing.T) {
	path := writeTempConfig(t, "version = 1\nclient = \"copilot\"\n")
	t.Setenv("NAV_PILOT_CONFIG", path)
	resolved, err := loadConfigForLaunch(CLIOverrides{Client: "opencode"})
	if err != nil {
		t.Fatalf("loadConfigForLaunch() error: %v", err)
	}
	if resolved.Client != "opencode" {
		t.Errorf("Client = %q, want opencode (--client override)", resolved.Client)
	}
}

func TestCLIClientOverride_InvalidRejected(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	err := run([]string{"--client", "bogus"})
	if err == nil {
		t.Fatal("expected error for --client bogus")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("expected 'bogus' in error message, got: %v", err)
	}
}

func TestCLIAgentDeprecatedFlag_ReturnsError(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	err := run([]string{"--agent", "opencode"})
	if err == nil {
		t.Fatal("expected error for deprecated --agent flag")
	}
	if !strings.Contains(err.Error(), "--client") {
		t.Errorf("expected error to mention --client, got: %v", err)
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

	if err := cmdConfigSet("client", "opencode"); err != nil {
		t.Fatalf("cmdConfigSet() error: %v", err)
	}

	// File should exist and contain the key.
	data, err := os.ReadFile(configPath())
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if !strings.Contains(string(data), `client = "opencode"`) {
		t.Errorf("config file missing expected line, got:\n%s", string(data))
	}
}

// TestCmdConfigSet_NewFileSeedsVersion is a regression test: a config created by
// `config set` (without a prior `config init`) must include version = 1 so the
// on-launch validation (validateConfig / loadConfigForLaunch) does not refuse to
// start with "version must be 1 (got 0)".
func TestCmdConfigSet_NewFileSeedsVersion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	if err := cmdConfigSet("mode", "plan"); err != nil {
		t.Fatalf("cmdConfigSet() error: %v", err)
	}

	data, err := os.ReadFile(configPath())
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if !strings.Contains(string(data), "version = 1") {
		t.Errorf("new config missing seeded version, got:\n%s", string(data))
	}

	// The freshly written config must pass launch validation.
	if _, err := loadConfigForLaunch(CLIOverrides{}); err != nil {
		t.Errorf("loadConfigForLaunch() rejected config-set output: %v", err)
	}
}

func TestCmdConfigSet_UpdatesExistingKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)

	_ = os.WriteFile(path, []byte("version = 1\nclient = \"copilot\"\n"), 0o644)

	if err := cmdConfigSet("client", "opencode"); err != nil {
		t.Fatalf("cmdConfigSet() error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `client = "opencode"`) {
		t.Errorf("client not updated, got:\n%s", string(data))
	}
	if strings.Contains(string(data), `client = "copilot"`) {
		t.Errorf("old client value should be gone, got:\n%s", string(data))
	}
}

func TestCmdConfigSet_UncommentsCommentedKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)

	_ = os.WriteFile(path, []byte("version = 1\n# client = \"copilot\"\n"), 0o644)

	if err := cmdConfigSet("client", "pi"); err != nil {
		t.Fatalf("cmdConfigSet() error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `client = "pi"`) {
		t.Errorf("expected uncommented client = pi, got:\n%s", string(data))
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

	err := cmdConfigSet("client", "cursor") // not in allowed list
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
client = "copilot"
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
		{`client = "copilot"`, "client", true},
		{`# client = "copilot"`, "client", true},
		{`  # client = "copilot"`, "client", true},
		{`## client = "copilot"`, "client", true},
		{`client="copilot"`, "client", true},
		{`# This is a general comment`, "client", false},
		{`# model = "gpt-4"`, "client", false},
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
		{"client", "copilot", false},
		{"client", "opencode", false},
		{"client", "pi", false},
		{"client", "cursor", true},
		{"mode", "default", false},
		{"mode", "autopilot", false},
		{"mode", "bad", true},
		{"reasoning_effort", "max", false},
		{"reasoning_effort", "ultra", true},
		{"context_tier", "long_context", false},
		{"context_tier", "extended", true},
		{"log_level", "debug", false},
		{"log_level", "verbose", true},
		{"otel_log_level", "none", false},
		{"otel_log_level", "verbose", false},
		{"otel_log_level", "warn", false},
		{"otel_log_level", "loud", true},
		{"allow_all_tools", "true", false},
		{"allow_all_tools", "false", false},
		{"allow_all_tools", "yes", false},
		{"allow_all_tools", "maybe", true},
		{"ask_user", "true", false},
		{"ask_user", "false", false},
		{"version", "1", false},
		{"version", "2", true},
		{"version", "abc", true},
		{"model", "any-model-name", false},              // well-formed
		{"model", "gpt-4-turbo-preview", false},         // well-formed
		{"model", "claude-opus-4.8", false},             // dots allowed
		{"model", "anthropic/claude-3-5-sonnet", false}, // provider/model
		{"model", "", true},                             // empty rejected
		{"model", "gpt 4o", true},                       // space rejected
		{"model", " gpt-4o", true},                      // leading space rejected
		{"model", "gpt-4o ", true},                      // trailing space rejected
		{"model", "gpt@4o", true},                       // illegal char rejected
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

// ─── isKnownCopilotModel / knownCopilotModelIDs ──────────────────────────────

func TestIsKnownCopilotModel(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"claude-sonnet-4.6", true},
		{"Claude-Sonnet-4.6", true}, // case-insensitive
		{"auto", true},
		{"gpt-5.5", true},
		{"sonnet", false}, // alias, not a real id
		{"opus", false},
		{"", false},
		{"anthropic/claude-3-5-sonnet", false},
	}
	for _, c := range cases {
		if got := isKnownCopilotModel(c.id); got != c.want {
			t.Errorf("isKnownCopilotModel(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}

func TestKnownCopilotModelIDs(t *testing.T) {
	got := knownCopilotModelIDs()
	for _, want := range []string{"auto", "claude-sonnet-4.6", "gpt-5.5", "gemini-3.5-flash"} {
		if !strings.Contains(got, want) {
			t.Errorf("knownCopilotModelIDs() = %q, missing %q", got, want)
		}
	}
}

// ─── configAdvisories ────────────────────────────────────────────────────────

func TestConfigAdvisories_Nil(t *testing.T) {
	if w := configAdvisories(nil, tomlMetaForTest(t, "")); len(w) != 0 {
		t.Errorf("configAdvisories(nil) = %v, want empty", w)
	}
}

func TestConfigAdvisories_UnknownKey_NotAWarning(t *testing.T) {
	// Unknown keys are now hard errors (in loadConfigForLaunch), not advisories.
	// configAdvisories itself must not emit any warning for them.
	cfg, meta := decodeConfigForTest(t, "version = 1\nmode = \"plan\"\nmdoel = \"x\"\n")
	w := configAdvisories(cfg, meta)
	for _, warning := range w {
		if strings.Contains(warning, "mdoel") {
			t.Errorf("configAdvisories() emitted unknown-key warning (should be hard error): %s", warning)
		}
	}
}

func TestConfigAdvisories_UnrecognizedCopilotModel(t *testing.T) {
	cfg, meta := decodeConfigForTest(t, "version = 1\nclient = \"copilot\"\nmodel = \"sonnet\"\n")
	w := configAdvisories(cfg, meta)
	if len(w) != 1 || !strings.Contains(w[0], "sonnet") || !strings.Contains(w[0], "not a recognized") {
		t.Errorf("configAdvisories() = %v, want one warning about unrecognized model", w)
	}
}

func TestConfigAdvisories_KnownCopilotModel_NoWarning(t *testing.T) {
	cfg, meta := decodeConfigForTest(t, "version = 1\nclient = \"copilot\"\nmodel = \"claude-opus-4.8\"\n")
	if w := configAdvisories(cfg, meta); len(w) != 0 {
		t.Errorf("configAdvisories() = %v, want no warnings for known model", w)
	}
}

func TestConfigAdvisories_NonCopilotModel_NoWarning(t *testing.T) {
	// Known Nav-curated opencode model must not generate any warning.
	cfg, meta := decodeConfigForTest(t, "version = 1\nclient = \"opencode\"\nmodel = \"anthropic/claude-sonnet-4-5\"\n")
	if w := configAdvisories(cfg, meta); len(w) != 0 {
		t.Errorf("configAdvisories() = %v, want no warnings for known opencode model", w)
	}
}

func TestConfigAdvisories_UnrecognizedOpenCodeModel(t *testing.T) {
	// Valid shape but not in the Nav-curated list → soft advisory.
	cfg, meta := decodeConfigForTest(t, "version = 1\nclient = \"opencode\"\nmodel = \"anthropic/claude-3-5-sonnet\"\n")
	w := configAdvisories(cfg, meta)
	if len(w) == 0 || !strings.Contains(w[0], "anthropic/claude-3-5-sonnet") {
		t.Errorf("configAdvisories() = %v, want advisory for unrecognized opencode model", w)
	}
}

// ─── validateModelForClient ──────────────────────────────────────────────────

func TestValidateModelForClient(t *testing.T) {
	tests := []struct {
		model   string
		client  string
		wantErr bool
		errSnip string // substring expected in error
	}{
		// opencode: valid provider/model format
		{"anthropic/claude-sonnet-4-5", "opencode", false, ""},
		{"openai/gpt-4o", "opencode", false, ""},
		{"google/gemini-2-0-flash", "opencode", false, ""},
		// opencode: bare id (no slash) — must error
		{"claude-opus-4.8", "opencode", true, "provider/model"},
		{"gpt-5.5", "opencode", true, "provider/model"},
		// opencode: double-slash — must error
		{"a/b/c", "opencode", true, "provider/model"},
		// opencode: trailing slash (empty model part) — must error
		{"anthropic/", "opencode", true, "provider/model"},
		// copilot: bare id is fine
		{"claude-opus-4.8", "copilot", false, ""},
		{"gpt-5.5", "copilot", false, ""},
		// copilot: provider/model is also fine (allowed by validateModelValue)
		{"anthropic/claude-sonnet-4-5", "copilot", false, ""},
		// empty client: acts like base validation only
		{"claude-opus-4.8", "", false, ""},
		// always-invalid (base validateModelValue failures)
		{"", "copilot", true, ""},
		{" bad", "opencode", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.model+"@"+tt.client, func(t *testing.T) {
			err := validateModelForClient(tt.model, tt.client)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateModelForClient(%q, %q) = nil, want error", tt.model, tt.client)
				}
				if tt.errSnip != "" && !strings.Contains(err.Error(), tt.errSnip) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.errSnip)
				}
			} else if err != nil {
				t.Fatalf("validateModelForClient(%q, %q) = %v, want nil", tt.model, tt.client, err)
			}
		})
	}
}

// ─── isKnownOpenCodeModel / knownOpenCodeModelIDs ────────────────────────────

func TestIsKnownOpenCodeModel(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{openCodeDefaultModel, true},
		{"anthropic/claude-opus-4-5", true},
		{strings.ToUpper(openCodeDefaultModel), true}, // case-insensitive
		{"anthropic/claude-3-5-sonnet", false},        // older, not in list
		{"claude-sonnet-4.6", false},                  // copilot id, not opencode
		{"", false},
	}
	for _, c := range cases {
		if got := isKnownOpenCodeModel(c.id); got != c.want {
			t.Errorf("isKnownOpenCodeModel(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}

func TestKnownOpenCodeModelIDs(t *testing.T) {
	got := knownOpenCodeModelIDs()
	for _, want := range []string{openCodeDefaultModel, "anthropic/claude-opus-4-5"} {
		if !strings.Contains(got, want) {
			t.Errorf("knownOpenCodeModelIDs() = %q, missing %q", got, want)
		}
	}
}

// TestValidateConfigProblems_OpenCodeBareModel ensures that an opencode config
// with a bare (non-provider/model) id is a hard validation error.
func TestValidateConfigProblems_OpenCodeBareModel(t *testing.T) {
	client := "opencode"
	model := "claude-opus-4.8"
	cfg := &Config{Version: 1, Client: &client, Model: &model}
	problems := validateConfigProblems(cfg)
	if len(problems) == 0 {
		t.Fatal("expected validation error for opencode bare model id, got none")
	}
	if !strings.Contains(problems[0], "provider/model") {
		t.Errorf("problem = %q, want mention of provider/model format", problems[0])
	}
}

// TestValidateConfigProblems_OpenCodeValidModel ensures valid opencode models pass.
func TestValidateConfigProblems_OpenCodeValidModel(t *testing.T) {
	client := "opencode"
	model := "anthropic/claude-sonnet-4-5"
	cfg := &Config{Version: 1, Client: &client, Model: &model}
	if p := validateConfigProblems(cfg); len(p) != 0 {
		t.Errorf("expected no problems for valid opencode model, got: %v", p)
	}
}

// ─── loadConfigForLaunch ─────────────────────────────────────────────────────

func TestLoadConfigForLaunch_NoFile(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))
	resolved, err := loadConfigForLaunch(CLIOverrides{})
	if err != nil {
		t.Fatalf("loadConfigForLaunch() error = %v, want nil", err)
	}
	if resolved.Client != "copilot" || resolved.Mode != "default" {
		t.Errorf("loadConfigForLaunch() defaults = %+v", resolved)
	}
}

func TestLoadConfigForLaunch_ValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFileForTest(t, path, "version = 1\nclient = \"copilot\"\nmode = \"autopilot\"\nmodel = \"claude-opus-4.8\"\n")
	t.Setenv("NAV_PILOT_CONFIG", path)
	resolved, err := loadConfigForLaunch(CLIOverrides{})
	if err != nil {
		t.Fatalf("loadConfigForLaunch() error = %v, want nil", err)
	}
	if resolved.Mode != "autopilot" || resolved.Model != "claude-opus-4.8" {
		t.Errorf("loadConfigForLaunch() = %+v", resolved)
	}
}

func TestLoadConfigForLaunch_RefusesInvalidMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFileForTest(t, path, "version = 1\nmode = \"auto\"\n")
	t.Setenv("NAV_PILOT_CONFIG", path)
	_, err := loadConfigForLaunch(CLIOverrides{})
	if err == nil {
		t.Fatal("loadConfigForLaunch() error = nil, want refusal for invalid mode")
	}
	if !strings.Contains(err.Error(), "mode") {
		t.Errorf("loadConfigForLaunch() error = %v, want mention of mode", err)
	}
}

func TestLoadConfigForLaunch_RefusesInvalidVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFileForTest(t, path, "version = 2\n")
	t.Setenv("NAV_PILOT_CONFIG", path)
	if _, err := loadConfigForLaunch(CLIOverrides{}); err == nil {
		t.Fatal("loadConfigForLaunch() error = nil, want refusal for version 2")
	}
}

func TestLoadConfigForLaunch_WarnsButLaunchesUnrecognizedModel(t *testing.T) {
	// "sonnet" is format-valid but not a known id: warn, do not refuse.
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFileForTest(t, path, "version = 1\nclient = \"copilot\"\nmodel = \"sonnet\"\n")
	t.Setenv("NAV_PILOT_CONFIG", path)
	resolved, err := loadConfigForLaunch(CLIOverrides{})
	if err != nil {
		t.Fatalf("loadConfigForLaunch() error = %v, want nil (warn-not-refuse)", err)
	}
	if resolved.Model != "sonnet" {
		t.Errorf("loadConfigForLaunch() Model = %q, want sonnet", resolved.Model)
	}
}

// ─── test helpers ────────────────────────────────────────────────────────────

func decodeConfigForTest(t *testing.T, body string) (*Config, toml.MetaData) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFileForTest(t, path, body)
	t.Setenv("NAV_PILOT_CONFIG", path)
	cfg, meta, err := readConfigWithMeta()
	if err != nil {
		t.Fatalf("readConfigWithMeta() error = %v", err)
	}
	return cfg, meta
}

func tomlMetaForTest(t *testing.T, body string) toml.MetaData {
	t.Helper()
	_, meta := decodeConfigForTest(t, body)
	return meta
}

func writeFileForTest(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ─── otel_log_level ───────────────────────────────────────────────────────────

func TestResolve_OtelLogLevel_DefaultIsNone(t *testing.T) {
	r := resolve(nil, CLIOverrides{})
	if r.OtelLogLevel != "none" {
		t.Errorf("OtelLogLevel = %q, want none (default)", r.OtelLogLevel)
	}
}

func TestResolve_OtelLogLevel_FileOverridesDefault(t *testing.T) {
	level := "debug"
	cfg := &Config{Version: 1, OtelLogLevel: &level}
	r := resolve(cfg, CLIOverrides{})
	if r.OtelLogLevel != "debug" {
		t.Errorf("OtelLogLevel = %q, want debug (file override)", r.OtelLogLevel)
	}
}

func TestResolve_OtelLogLevel_CLIOverridesFile(t *testing.T) {
	level := "info"
	cfg := &Config{Version: 1, OtelLogLevel: &level}
	r := resolve(cfg, CLIOverrides{OtelLogLevel: "verbose"})
	if r.OtelLogLevel != "verbose" {
		t.Errorf("OtelLogLevel = %q, want verbose (CLI beats file)", r.OtelLogLevel)
	}
}

func TestResolve_OtelLogLevel_UnsetFileKeepsDefault(t *testing.T) {
	// File exists but does not set otel_log_level — default "none" should hold.
	agent := "copilot"
	cfg := &Config{Version: 1, Client: &agent}
	r := resolve(cfg, CLIOverrides{})
	if r.OtelLogLevel != "none" {
		t.Errorf("OtelLogLevel = %q, want none (unset in file → default)", r.OtelLogLevel)
	}
}

func TestValidateConfig_OtelLogLevel(t *testing.T) {
	tests := []struct {
		level   string
		wantErr bool
	}{
		{"none", false},
		{"error", false},
		{"warning", false},
		{"warn", false},
		{"info", false},
		{"debug", false},
		{"verbose", false},
		{"all", false},
		{"loud", true},
		{"trace", true},
		{"default", true},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			s := tt.level
			cfg := &Config{Version: 1, OtelLogLevel: &s}
			err := validateConfig(cfg)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for otel_log_level %q, got nil", tt.level)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected valid otel_log_level %q, got error: %v", tt.level, err)
			}
		})
	}
}
