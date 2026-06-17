package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStdout captures everything written to os.Stdout during f().
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}

// ─── validateOptionalModel ────────────────────────────────────────────────────

func TestValidateOptionalModel_Blank(t *testing.T) {
	if err := validateOptionalModel(""); err != nil {
		t.Errorf("expected nil for blank string, got: %v", err)
	}
}

func TestValidateOptionalModel_WhitespaceOnly(t *testing.T) {
	if err := validateOptionalModel("   "); err != nil {
		t.Errorf("expected nil for whitespace-only string, got: %v", err)
	}
}

func TestValidateOptionalModel_ValidIDs(t *testing.T) {
	for _, id := range []string{
		"gpt-5.5",
		"claude-opus-4.8",
		"auto",
		"anthropic/claude-3-5-sonnet",
		"gpt-5.3-codex",
	} {
		if err := validateOptionalModel(id); err != nil {
			t.Errorf("validateOptionalModel(%q) = %v, want nil", id, err)
		}
	}
}

func TestValidateOptionalModel_InvalidIDs(t *testing.T) {
	// Note: validateOptionalModel trims surrounding whitespace before validation,
	// so only ids that are invalid after trimming should be listed here.
	for _, id := range []string{
		"gpt 4o",
		"bad model!",
		"model with spaces inside",
		"has@at",
	} {
		if err := validateOptionalModel(id); err == nil {
			t.Errorf("validateOptionalModel(%q): expected error, got nil", id)
		}
	}
}

// ─── cmdConfig router ─────────────────────────────────────────────────────────

func TestCmdConfig_NoArgs(t *testing.T) {
	if err := cmdConfig(nil, false); err == nil {
		t.Error("expected error when no subcommand given")
	}
}

func TestCmdConfig_EmptyArgs(t *testing.T) {
	if err := cmdConfig([]string{}, false); err == nil {
		t.Error("expected error for empty args slice")
	}
}

func TestCmdConfig_UnknownSubcommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))
	if err := cmdConfig([]string{"bogussubcmd"}, false); err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

func TestCmdConfig_GetNoKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))
	if err := cmdConfig([]string{"get"}, false); err == nil {
		t.Error("expected error for 'get' with no key argument")
	}
}

func TestCmdConfig_SetNoArgs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))
	if err := cmdConfig([]string{"set"}, false); err == nil {
		t.Error("expected error for 'set' with no arguments")
	}
}

func TestCmdConfig_SetOneArg(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))
	if err := cmdConfig([]string{"set", "client"}, false); err == nil {
		t.Error("expected error for 'set' with only one argument (missing value)")
	}
}

// ─── cmdConfigShow ────────────────────────────────────────────────────────────

func TestCmdConfigShow_NoFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	var out string
	var showErr error
	out = captureStdout(func() {
		showErr = cmdConfigShow(false)
	})

	if showErr != nil {
		t.Fatalf("cmdConfigShow(false) returned unexpected error: %v", showErr)
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' in output when config file absent, got: %q", out)
	}
	if !strings.Contains(out, "client") {
		t.Errorf("expected 'client' key listed in output, got: %q", out)
	}
}

func TestCmdConfigShow_WithFile(t *testing.T) {
	path := writeTempConfig(t, "version = 1\nclient = \"opencode\"\n")
	t.Setenv("NAV_PILOT_CONFIG", path)

	var out string
	var showErr error
	out = captureStdout(func() {
		showErr = cmdConfigShow(false)
	})

	if showErr != nil {
		t.Fatalf("cmdConfigShow(false) returned unexpected error: %v", showErr)
	}
	if !strings.Contains(out, "opencode") {
		t.Errorf("expected 'opencode' in output, got: %q", out)
	}
	if !strings.Contains(out, path) {
		t.Errorf("expected config file path %q in output, got: %q", path, out)
	}
}

func TestCmdConfigShow_JSON(t *testing.T) {
	path := writeTempConfig(t, "version = 1\nclient = \"opencode\"\nmode = \"plan\"\n")
	t.Setenv("NAV_PILOT_CONFIG", path)

	var out string
	var showErr error
	out = captureStdout(func() {
		showErr = cmdConfigShow(true)
	})

	if showErr != nil {
		t.Fatalf("cmdConfigShow(true) returned unexpected error: %v", showErr)
	}
	for _, want := range []string{`"client"`, `"model"`, `"mode"`, "opencode", "plan"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in JSON output, got: %q", want, out)
		}
	}
}

func TestCmdConfigShow_JSONDefaultValues(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	var out string
	var showErr error
	out = captureStdout(func() {
		showErr = cmdConfigShow(true)
	})

	if showErr != nil {
		t.Fatalf("cmdConfigShow(true) returned unexpected error: %v", showErr)
	}
	// Default agent should be copilot.
	if !strings.Contains(out, "copilot") {
		t.Errorf("expected default agent 'copilot' in JSON output, got: %q", out)
	}
}

// ─── cmdConfigPath ────────────────────────────────────────────────────────────

func TestCmdConfigPath_PrintsConfigPath(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "my-config.toml")
	t.Setenv("NAV_PILOT_CONFIG", customPath)

	out := captureStdout(func() {
		if err := cmdConfigPath(); err != nil {
			t.Errorf("cmdConfigPath() returned unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, customPath) {
		t.Errorf("expected %q in output, got: %q", customPath, out)
	}
}

// ─── cmdConfigExplain + printKeyExplain ───────────────────────────────────────

func TestCmdConfigExplain_AllKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	var out string
	var explainErr error
	out = captureStdout(func() {
		explainErr = cmdConfigExplain("")
	})

	if explainErr != nil {
		t.Fatalf("cmdConfigExplain(\"\") returned error: %v", explainErr)
	}
	for _, key := range []string{"client", "model", "mode", "reasoning_effort", "context_tier", "allow_all_tools", "ask_user", "log_level", "otel_log_level", "version"} {
		if !strings.Contains(out, key) {
			t.Errorf("expected key %q in all-keys explain output", key)
		}
	}
}

func TestCmdConfigExplain_SingleKeyModel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	var out string
	var explainErr error
	out = captureStdout(func() {
		explainErr = cmdConfigExplain("model")
	})

	if explainErr != nil {
		t.Fatalf("cmdConfigExplain(\"model\") returned error: %v", explainErr)
	}
	for _, want := range []string{"claude-opus-4.8", "gpt-5.5", "any well-formed id"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in model explain output, got: %q", want, out)
		}
	}
}

func TestCmdConfigExplain_SingleKeyClient(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	var out string
	var explainErr error
	out = captureStdout(func() {
		explainErr = cmdConfigExplain("client")
	})

	if explainErr != nil {
		t.Fatalf("cmdConfigExplain(\"client\") returned error: %v", explainErr)
	}
	if !strings.Contains(out, "copilot") {
		t.Errorf("expected 'copilot' in client explain output, got: %q", out)
	}
}

func TestCmdConfigExplain_UnknownKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	if err := cmdConfigExplain("nosuchkey"); err == nil {
		t.Error("expected error for unknown key, got nil")
	}
}

// ─── cmdConfigSetup non-interactive paths ─────────────────────────────────────

func TestCmdConfigSetup_FileAlreadyExists(t *testing.T) {
	path := writeTempConfig(t, "version = 1\n")
	t.Setenv("NAV_PILOT_CONFIG", path)

	err := cmdConfigSetup()
	if err == nil {
		t.Fatal("expected error when config file already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error message, got: %v", err)
	}
}

func TestCmdConfigSetup_NonInteractiveNoFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))
	forceNonInteractive = true
	defer func() { forceNonInteractive = false }()

	err := cmdConfigSetup()
	if err == nil {
		t.Fatal("expected error when non-interactive and no config file")
	}
	if !strings.Contains(err.Error(), "config init") {
		t.Errorf("expected 'config init' in error message, got: %v", err)
	}
}

// ─── otel_log_level in config show / get ─────────────────────────────────────

func TestCmdConfigShow_JSON_OtelLogLevel_DefaultNone(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	out := captureStdout(func() {
		if err := cmdConfigShow(true); err != nil {
			t.Fatalf("cmdConfigShow(true) returned unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"otel_log_level"`) {
		t.Errorf("expected otel_log_level in JSON output, got: %q", out)
	}
	if !strings.Contains(out, `"none"`) {
		t.Errorf("expected otel_log_level default value 'none' in JSON output, got: %q", out)
	}
}

func TestCmdConfigGet_OtelLogLevel_DefaultNone(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NAV_PILOT_CONFIG", filepath.Join(dir, "config.toml"))

	out := captureStdout(func() {
		if err := cmdConfigGet("otel_log_level"); err != nil {
			t.Fatalf("cmdConfigGet(otel_log_level) returned unexpected error: %v", err)
		}
	})
	if strings.TrimSpace(out) != "none" {
		t.Errorf("config get otel_log_level = %q, want none", strings.TrimSpace(out))
	}
}

// ─── config set / setup permissions ──────────────────────────────────────────

func TestCmdConfigSet_PermsTightenedOnPreExistingFile(t *testing.T) {
	path := writeTempConfig(t, "version = 1\nagent = \"copilot\"\n")
	t.Setenv("NAV_PILOT_CONFIG", path)

	// Widen permissions so WriteFile's mode arg alone wouldn't fix it.
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	if err := cmdConfigSet("client", "opencode"); err != nil {
		t.Fatalf("cmdConfigSet: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 0600", info.Mode().Perm())
	}
}
