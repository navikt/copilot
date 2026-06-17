package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	providerpkg "github.com/navikt/copilot/cli/nav-pilot/internal/provider"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

// ─── writeSetupConfig ─────────────────────────────────────────────────────────

func TestWriteSetupConfig_AllFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	answers := setupAnswers{
		Client:          "opencode",
		Mode:            "plan",
		Model:           "openai/gpt-4o",
		ReasoningEffort: "high",
	}
	if err := writeSetupConfig(answers); err != nil {
		t.Fatalf("writeSetupConfig() error: %v", err)
	}

	data, err := os.ReadFile(configPath())
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	content := string(data)

	for _, want := range []string{
		"version = 1",
		`client = "opencode"`,
		`mode = "plan"`,
		`model = "openai/gpt-4o"`,
		`reasoning_effort = "high"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in config, got:\n%s", want, content)
		}
	}
}

func TestWriteSetupConfig_MinimalFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	answers := setupAnswers{
		Client: "pi",
		Mode:   "autopilot",
		// Model and ReasoningEffort intentionally empty
	}
	if err := writeSetupConfig(answers); err != nil {
		t.Fatalf("writeSetupConfig() error: %v", err)
	}

	data, _ := os.ReadFile(configPath())
	content := string(data)

	for _, want := range []string{
		"version = 1",
		`client = "pi"`,
		`mode = "autopilot"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in config, got:\n%s", want, content)
		}
	}
}

func TestWriteSetupConfig_EmptyModelSkipped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	answers := setupAnswers{Client: "copilot", Mode: "default", Model: ""}
	if err := writeSetupConfig(answers); err != nil {
		t.Fatalf("writeSetupConfig() error: %v", err)
	}

	data, _ := os.ReadFile(configPath())
	content := string(data)
	if strings.Contains(content, "model") {
		t.Errorf("model key must not appear when empty, got:\n%s", content)
	}
}

func TestWriteSetupConfig_EmptyEffortSkipped(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	answers := setupAnswers{Client: "copilot", Mode: "default", ReasoningEffort: ""}
	if err := writeSetupConfig(answers); err != nil {
		t.Fatalf("writeSetupConfig() error: %v", err)
	}

	data, _ := os.ReadFile(configPath())
	content := string(data)
	if strings.Contains(content, "reasoning_effort") {
		t.Errorf("reasoning_effort must not appear when empty (unset), got:\n%s", content)
	}
}

func TestWriteSetupConfig_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	// Zero-value setupAnswers — should fall back to "copilot" / "default".
	answers := setupAnswers{}
	if err := writeSetupConfig(answers); err != nil {
		t.Fatalf("writeSetupConfig() error: %v", err)
	}

	data, _ := os.ReadFile(configPath())
	content := string(data)
	if !strings.Contains(content, `client = "copilot"`) {
		t.Errorf("expected default client=copilot, got:\n%s", content)
	}
	if !strings.Contains(content, `mode = "default"`) {
		t.Errorf("expected default mode=default, got:\n%s", content)
	}
}

func TestWriteSetupConfig_ProducesValidConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	for _, answers := range []setupAnswers{
		{Client: "copilot", Mode: "default"},
		{Client: "opencode", Mode: "plan", Model: "openai/gpt-4o", ReasoningEffort: "high"},
		{Client: "pi", Mode: "autopilot", ReasoningEffort: "max"},
	} {
		// Remove config from previous iteration.
		os.Remove(configPath())

		if err := writeSetupConfig(answers); err != nil {
			t.Fatalf("writeSetupConfig(%+v) error: %v", answers, err)
		}

		cfg, err := readConfig()
		if err != nil {
			t.Fatalf("readConfig() after setup error: %v", err)
		}
		if cfg == nil {
			t.Fatal("readConfig() returned nil after setup")
		}
		if err := validateConfig(cfg); err != nil {
			t.Errorf("validateConfig() failed after writeSetupConfig(%+v): %v", answers, err)
		}
	}
}

func TestWriteSetupConfig_AllValidClients(t *testing.T) {
	for _, agent := range validProviderIDs {
		t.Run(agent, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

			err := writeSetupConfig(setupAnswers{Client: agent, Mode: "default"})
			if err != nil {
				t.Errorf("writeSetupConfig(client=%q) error: %v", agent, err)
			}
		})
	}
}

func TestWriteSetupConfig_AllValidEfforts(t *testing.T) {
	for _, effort := range validReasoningEffort {
		t.Run(effort, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

			err := writeSetupConfig(setupAnswers{
				Client:          "copilot",
				Mode:            "default",
				ReasoningEffort: effort,
			})
			if err != nil {
				t.Errorf("writeSetupConfig(effort=%q) error: %v", effort, err)
			}
		})
	}
}

// ─── maybeRunFirstRunSetup guard logic ───────────────────────────────────────

func TestMaybeRunFirstRunSetup_NonInteractive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)
	forceNonInteractive = true
	defer func() { forceNonInteractive = false }()

	if err := maybeRunFirstRunSetup(); err != nil {
		t.Fatalf("expected nil error (non-interactive), got: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("config file must not be created in non-interactive mode")
	}
}

func TestMaybeRunFirstRunSetup_FileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("NAV_PILOT_CONFIG", path)
	// Write a config file so the guard fires.
	_ = os.WriteFile(path, []byte("version = 1\n"), 0o644)
	// Even in interactive mode, must not overwrite.
	// (isInteractive() is false in test, but we test the stat guard here.)

	if err := maybeRunFirstRunSetup(); err != nil {
		t.Fatalf("expected nil error when file exists, got: %v", err)
	}

	// File content must be unchanged.
	data, _ := os.ReadFile(path)
	if string(data) != "version = 1\n" {
		t.Errorf("maybeRunFirstRunSetup must not clobber existing config, got: %q", string(data))
	}
}

func TestMaybeRunFirstRunSetup_NonInteractiveNoFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))
	forceNonInteractive = true
	defer func() { forceNonInteractive = false }()

	// Must not attempt the wizard; returns nil immediately.
	if err := maybeRunFirstRunSetup(); err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
	if _, err := os.Stat(configPath()); !os.IsNotExist(err) {
		t.Error("no file should exist")
	}
}

// ─── opencode setup bootstrap ─────────────────────────────────────────────────

// TestWriteSetupConfig_OpenCode_BootstrapsOTelAndContext verifies that when the
// opencode client is chosen in first-run setup, OTel config and Nav context are
// seeded into the overridden output dirs — hermetic, no network required.
func TestWriteSetupConfig_OpenCode_BootstrapsOTelAndContext(t *testing.T) {
	cfgDir := t.TempDir()
	ocConfigDir := t.TempDir()
	ocNavContextDir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(cfgDir, "config.toml"))

	// Redirect opencode config and Nav context dirs.
	oldCfgOverride := providerpkg.ConfigPathOverride
	oldNavOverride := providerpkg.NavContextDirOverride
	providerpkg.ConfigPathOverride = filepath.Join(ocConfigDir, "opencode.json")
	providerpkg.NavContextDirOverride = ocNavContextDir
	defer func() {
		providerpkg.ConfigPathOverride = oldCfgOverride
		providerpkg.NavContextDirOverride = oldNavOverride
	}()

	// Redirect source.CloneRemoteFn so ensureOpenCodeNavContext uses a local fixture.
	origClone := source.CloneRemoteFn
	defer func() { source.CloneRemoteFn = origClone }()
	sourceDir := setupTestSource(t)
	source.CloneRemoteFn = func(ref, sourceRepo string) (*source.Source, error) {
		return &source.Source{Dir: sourceDir, SHA: "test-bootstrap"}, nil
	}

	// Bootstrap is triggered inside runConfigSetup after writeSetupConfig; call
	// the helpers directly to stay hermetic (no huh TUI in tests).
	answers := setupAnswers{Client: "opencode", Mode: "default"}
	if err := writeSetupConfig(answers); err != nil {
		t.Fatalf("writeSetupConfig: %v", err)
	}
	// Simulate the opencode bootstrap block from runConfigSetup.
	if err := providerpkg.EnsureOpenCodeOTelConfig(); err != nil {
		t.Fatalf("ensureOpenCodeOTelConfig: %v", err)
	}
	summary, err := providerpkg.EnsureOpenCodeNavContext()
	if err != nil {
		t.Fatalf("ensureOpenCodeNavContext: %v", err)
	}

	// OTel config must have been created.
	if _, err := os.Stat(providerpkg.ConfigPathOverride); err != nil {
		t.Errorf("opencode OTel config not created: %v", err)
	}

	// AGENTS.md must exist in the Nav context dir.
	agentsPath := filepath.Join(ocNavContextDir, "AGENTS.md")
	if _, err := os.Stat(agentsPath); err != nil {
		t.Errorf("AGENTS.md not created in context dir: %v", err)
	}

	// Summary must mention artifacts.
	if summary == "" {
		t.Error("expected non-empty context summary after bootstrap")
	}

	// Second run must be idempotent — no error, same AGENTS.md content.
	first, _ := os.ReadFile(agentsPath)
	summary2, err2 := providerpkg.EnsureOpenCodeNavContext()
	if err2 != nil {
		t.Fatalf("second ensureOpenCodeNavContext: %v", err2)
	}
	second, _ := os.ReadFile(agentsPath)
	if string(first) != string(second) {
		t.Error("AGENTS.md changed between runs — not idempotent")
	}
	if summary != summary2 {
		t.Errorf("summary changed: %q → %q", summary, summary2)
	}
}
