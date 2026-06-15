package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── writeSetupConfig ─────────────────────────────────────────────────────────

func TestWriteSetupConfig_AllFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	answers := setupAnswers{
		Agent:           "opencode",
		Mode:            "plan",
		Model:           "gpt-4o",
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
		`agent = "opencode"`,
		`mode = "plan"`,
		`model = "gpt-4o"`,
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
		Agent: "pi",
		Mode:  "autopilot",
		// Model and ReasoningEffort intentionally empty
	}
	if err := writeSetupConfig(answers); err != nil {
		t.Fatalf("writeSetupConfig() error: %v", err)
	}

	data, _ := os.ReadFile(configPath())
	content := string(data)

	for _, want := range []string{
		"version = 1",
		`agent = "pi"`,
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

	answers := setupAnswers{Agent: "copilot", Mode: "default", Model: ""}
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

	answers := setupAnswers{Agent: "copilot", Mode: "default", ReasoningEffort: ""}
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
	if !strings.Contains(content, `agent = "copilot"`) {
		t.Errorf("expected default agent=copilot, got:\n%s", content)
	}
	if !strings.Contains(content, `mode = "default"`) {
		t.Errorf("expected default mode=default, got:\n%s", content)
	}
}

func TestWriteSetupConfig_ProducesValidConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	for _, answers := range []setupAnswers{
		{Agent: "copilot", Mode: "default"},
		{Agent: "opencode", Mode: "plan", Model: "gpt-4o", ReasoningEffort: "high"},
		{Agent: "pi", Mode: "autopilot", ReasoningEffort: "max"},
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

func TestWriteSetupConfig_AllValidAgents(t *testing.T) {
	for _, agent := range validAgents {
		t.Run(agent, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

			err := writeSetupConfig(setupAnswers{Agent: agent, Mode: "default"})
			if err != nil {
				t.Errorf("writeSetupConfig(agent=%q) error: %v", agent, err)
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
				Agent:           "copilot",
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
