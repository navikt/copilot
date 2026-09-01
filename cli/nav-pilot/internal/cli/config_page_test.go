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
		{"model", "GPT-5.5 (gpt-5.5)", "file"},
		{"mode", "default", "default"},
		{"reasoning_effort", "", "unset"},
		{"context_tier", "", "unset"},
		{"ask_user", "false", "file"},
		{"allow_all_tools", "false", "default"},
		{"log_level", "", "unset"},
		{"otel_log_level", "none", "default"},
		{"local_enabled", "false", "default"},
		{"local_autostart", "false", "default"},
		{"local_loop_guard", findKeyDef("local_loop_guard").defaultVal, "default"},
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

func TestModelValueLabel(t *testing.T) {
	writeTestConfig(t, "version = 1\n")

	tests := []struct {
		model, want string
	}{
		{"gpt-5.5", "GPT-5.5 (gpt-5.5)"},
		{"some-unknown-model", "some-unknown-model"},
		{"", ""},
	}
	for _, tc := range tests {
		r := resolve(nil, CLIOverrides{})
		r.Client = "copilot"
		r.Model = tc.model
		if got := modelValueLabel(r); got != tc.want {
			t.Errorf("modelValueLabel(%q) = %q, want %q", tc.model, got, tc.want)
		}
	}
}

func TestWordWrap(t *testing.T) {
	got := wordWrap("one two three four five", 11)
	if got != "one two\nthree four\nfive" {
		t.Errorf("wordWrap = %q", got)
	}
	// Long words overflow rather than being split.
	if got := wordWrap("supercalifragilistic", 5); got != "supercalifragilistic" {
		t.Errorf("wordWrap long word = %q", got)
	}
}

func TestModelPickerOptions(t *testing.T) {
	p, err := providerFor("copilot")
	if err != nil {
		t.Fatalf("providerFor: %v", err)
	}
	opts := modelPickerOptions(p)

	if len(opts) != len(p.KnownModels())+2 {
		t.Fatalf("got %d options, want %d known models + unset + custom", len(opts), len(p.KnownModels()))
	}
	if opts[0].Value != "" {
		t.Errorf("first option = %q, want unset (empty value)", opts[0].Value)
	}
	if last := opts[len(opts)-1]; last.Value != customModelSentinel {
		t.Errorf("last option = %q, want the custom sentinel", last.Value)
	}
	for i, m := range p.KnownModels() {
		if opts[i+1].Value != m.ID {
			t.Errorf("option %d = %q, want %q", i+1, opts[i+1].Value, m.ID)
		}
		if !strings.Contains(opts[i+1].Key, m.Label) {
			t.Errorf("option %d label %q missing %q", i+1, opts[i+1].Key, m.Label)
		}
	}
}

// TestConfigPageCoversAllKeys is the drift guard: a user-facing key added to
// configKeyDefs must show up on the settings page with a description, and an
// internal key must never appear. Deriving the page from configKeyDefs makes
// this hold by construction; the test fails the build if that ever changes.
func TestConfigPageCoversAllKeys(t *testing.T) {
	page := make(map[string]bool, len(configPageKeys))
	for _, k := range configPageKeys {
		page[k] = true
	}
	for _, kd := range configKeyDefs {
		if kd.internal {
			if page[kd.name] {
				t.Errorf("internal key %q must not be listed on the settings page", kd.name)
			}
			continue
		}
		if !page[kd.name] {
			t.Errorf("user-facing key %q missing from the settings page", kd.name)
		}
		if strings.TrimSpace(kd.description) == "" {
			t.Errorf("user-facing key %q has no description for the settings page", kd.name)
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

func TestCpltPostureValue(t *testing.T) {
	tests := []struct {
		preset, want string
	}{
		{"strict", "strict"},
		{"standard", "standard (recommended: strict)"},
		{"", "(unknown — could not read it from cplt)"},
	}
	for _, tc := range tests {
		if got := cpltPostureValue(tc.preset); got != tc.want {
			t.Errorf("cpltPostureValue(%q) = %q, want %q", tc.preset, got, tc.want)
		}
	}
}
