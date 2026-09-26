package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// ─── bare `config` routing ────────────────────────────────────────────────────

func TestCmdConfig_NoArgs_NonInteractiveKeepsUsageError(t *testing.T) {
	t.Setenv("CI", "1") // makes isInteractive() false

	err := cmdConfig(nil, false, false)
	if err == nil {
		t.Fatal("expected usage error when no subcommand given")
	}
	for _, want := range []string{"config requires a subcommand", "Usage: nav-pilot config", "sandbox"} {
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
	opts := modelPickerOptions(p, nil)

	if len(opts) != len(p.KnownModels())+2 {
		t.Fatalf("got %d options, want %d known models + unset + custom", len(opts), len(p.KnownModels()))
	}
	if opts[0].Value != "" {
		t.Errorf("first option = %q, want unset (empty value)", opts[0].Value)
	}
	if opts[1].Value != customModelSentinel {
		t.Errorf("second option = %q, want Other…", opts[1].Value)
	}
	for i, m := range p.KnownModels() {
		if opts[i+2].Value != m.ID {
			t.Errorf("option %d = %q, want %q", i+2, opts[i+2].Value, m.ID)
		}
		if !strings.Contains(opts[i+2].Key, m.Label) {
			t.Errorf("option %d label %q missing %q", i+2, opts[i+2].Key, m.Label)
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

func TestEscHelpFieldEscCancelsUnlessFiltering(t *testing.T) {
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	isCtrlC := func(cmd tea.Cmd) bool {
		if cmd == nil {
			return false
		}
		k, ok := cmd().(tea.KeyMsg)
		return ok && k.Type == tea.KeyCtrlC
	}

	_, cmd := escHelpField{huh.NewInput()}.Update(esc)
	if !isCtrlC(cmd) {
		t.Error("Esc in an input does not cancel")
	}
	sel := huh.NewSelect[string]().Options(huh.NewOption("a", "a")).Filtering(true)
	if _, cmd := (escHelpField{sel}).Update(esc); isCtrlC(cmd) {
		t.Error("Esc while filtering cancelled the prompt instead of clearing the filter")
	}
	if binds := (escHelpField{huh.NewInput()}).KeyBinds(); binds[len(binds)-1].Help().Desc != "cancel" {
		t.Error("footer does not name esc")
	}
}

func TestPrintPageSummary(t *testing.T) {
	t.Setenv("NAV_PILOT_CONFIG", "/x/config.toml")
	for _, tt := range []struct {
		before, after map[string]any
		want          string
	}{
		{map[string]any{"mode": "plan"}, map[string]any{"mode": "plan"}, "No changes."},
		{map[string]any{"mode": "plan"}, map[string]any{"mode": "autopilot"}, "Saved 1 change to /x/config.toml"},
		{map[string]any{"mode": "plan"}, map[string]any{"client": "pi"}, "Saved 2 changes to /x/config.toml"},
	} {
		out := captureStdout(func() { printPageSummary(tt.before, tt.after) })
		if !strings.Contains(out, tt.want) {
			t.Errorf("printPageSummary(%v, %v) = %q, want %q", tt.before, tt.after, out, tt.want)
		}
	}
}
