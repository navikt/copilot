package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── bare `config` routing ────────────────────────────────────────────────────

func TestCmdConfig_NoArgs_NonInteractiveKeepsUsageError(t *testing.T) {
	t.Setenv("CI", "1") // makes isInteractive() false

	err := cmdConfig(nil, false, false)
	if err == nil {
		t.Fatal("expected usage error when no subcommand given")
	}
	for _, want := range []string{"config requires a subcommand", "Usage: nav-pilot config <subcommand>", "sandbox"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}
}

// ─── key listing ──────────────────────────────────────────────────────────────

// writeTestConfig points NAV_PILOT_CONFIG at a temp file with the given content.
func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	t.Setenv("NAV_PILOT_CONFIG", path)
	return path
}

func TestBuildConfigPageEntries(t *testing.T) {
	writeTestConfig(t, "version = 1\nmodel = \"gpt-5.5\"\nask_user = false\n")

	cfg, err := readConfig()
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	entries := buildConfigPageEntries(cfg, resolve(cfg, CLIOverrides{}))

	byKey := map[string]configPageEntry{}
	for _, e := range entries {
		byKey[e.Key] = e
	}

	// Internal bookkeeping keys must not be offered on the page.
	for _, hidden := range []string{"version", "rtk_prompted_client", "rtk_prompted_at"} {
		if _, ok := byKey[hidden]; ok {
			t.Errorf("internal key %q must not be listed", hidden)
		}
	}
	if len(entries) != len(configPageKeys) {
		t.Errorf("got %d entries, want %d", len(entries), len(configPageKeys))
	}

	tests := []struct {
		key, wantValue, wantSource string
	}{
		{"client", "copilot", "default"},
		{"source", defaultSourceRepo, "default"},
		{"model", "gpt-5.5", "file"},
		{"mode", "default", "default"},
		{"reasoning_effort", "", "unset"},
		{"context_tier", "", "unset"},
		{"ask_user", "false", "file"},
		{"allow_all_tools", "false", "default"},
		{"log_level", "", "unset"},
		{"otel_log_level", "none", "default"},
	}
	for _, tc := range tests {
		e, ok := byKey[tc.key]
		if !ok {
			t.Errorf("key %q missing from page", tc.key)
			continue
		}
		if e.Value != tc.wantValue || e.Source != tc.wantSource {
			t.Errorf("%s = %q (%s), want %q (%s)", tc.key, e.Value, e.Source, tc.wantValue, tc.wantSource)
		}
		if e.Description == "" {
			t.Errorf("key %q has no description", tc.key)
		}
	}
}

func TestConfigPageEntryLabel(t *testing.T) {
	tests := []struct {
		entry configPageEntry
		want  string
	}{
		{configPageEntry{Key: "model", Value: "gpt-5.5", Source: "file"}, "model = gpt-5.5  (file)"},
		{configPageEntry{Key: "log_level", Value: "", Source: "unset"}, "log_level = (unset)  (unset)"},
	}
	for _, tc := range tests {
		if got := tc.entry.label(); got != tc.want {
			t.Errorf("label() = %q, want %q", got, tc.want)
		}
	}
}

// ─── clearing a key ───────────────────────────────────────────────────────────

func TestClearConfigKey(t *testing.T) {
	path := writeTestConfig(t, "version = 1\nmode = \"plan\"\n# model = \"auto\"\n")

	if err := clearConfigKey("mode"); err != nil {
		t.Fatalf("clearConfigKey: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	if strings.Contains(string(data), "mode = ") {
		t.Errorf("mode still set:\n%s", data)
	}
	// Commented-out lines and other keys are left alone.
	if !strings.Contains(string(data), "# model = \"auto\"") || !strings.Contains(string(data), "version = 1") {
		t.Errorf("unrelated lines were dropped:\n%s", data)
	}

	cfg, err := readConfig()
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	if cfg.Mode != nil {
		t.Errorf("mode still parsed as %q", *cfg.Mode)
	}
}

func TestCpltPostureLabel(t *testing.T) {
	tests := []struct {
		preset, want string
	}{
		{"strict", "cplt security posture   strict"},
		{"standard", "cplt security posture   standard  (recommended: strict)"},
		{"", "cplt security posture   unknown  (could not read it from cplt)"},
	}
	for _, tc := range tests {
		if got := cpltPostureLabel(tc.preset); got != tc.want {
			t.Errorf("cpltPostureLabel(%q) = %q, want %q", tc.preset, got, tc.want)
		}
	}
}
