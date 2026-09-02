package cli

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// ─── Key listing (shared with config show) ───────────────────────────────────

// configPageKeys lists the settings-page keys in display order: every key
// definition not marked internal. Adding a key to configKeyDefs is enough —
// a user-facing key cannot silently miss the page.
var configPageKeys = userFacingPageKeys()

func userFacingPageKeys() []string {
	var keys []string
	for _, kd := range configKeyDefs {
		if !kd.internal {
			keys = append(keys, kd.name)
		}
	}
	return keys
}

// configKeyValue returns the effective value of a key as displayed, spelling
// out the built-in default repo when no source is persisted.
func configKeyValue(r ResolvedConfig, key string) string {
	if key == "source" {
		return effectiveSourceLabel(r)
	}
	if key == "model" {
		return modelValueLabel(r)
	}
	return resolvedFieldStr(r, key)
}

// modelValueLabel renders the model as its curated label plus id when the
// configured client knows the id, so the page shows "Claude Sonnet 5
// (claude-sonnet-5)" instead of a bare id the user had to memorize.
func modelValueLabel(r ResolvedConfig) string {
	id := resolvedFieldStr(r, "model")
	if id == "" {
		return ""
	}
	if p, err := providerFor(r.Client); err == nil {
		for _, m := range p.KnownModels() {
			if m.ID == id {
				return m.Label + " (" + id + ")"
			}
		}
	}
	return id
}

// configKeySource labels where a key's effective value comes from: "file" when
// the config file sets it, otherwise "default" or "unset" depending on whether
// the key has a built-in default.
func configKeySource(cfg *Config, key string) string {
	if cfg != nil && configKeyInFile(cfg, key) {
		return "file"
	}
	if kd := findKeyDef(key); kd != nil && kd.defaultVal != "" {
		return "default"
	}
	return "unset"
}

// configKeyInFile reports whether the config file sets the key. It matches the
// Config struct's toml tags, so a new key needs no case here and cannot drift
// out of sync with the page. An empty source counts as unset — it is the
// documented way to fall back to the default.
func configKeyInFile(cfg *Config, key string) bool {
	if cfg == nil {
		return false
	}
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		tag, _, _ := strings.Cut(t.Field(i).Tag.Get("toml"), ",")
		if tag != key {
			continue
		}
		f := v.Field(i)
		if f.Kind() == reflect.Pointer {
			if f.IsNil() {
				return false
			}
			if key == "source" {
				if s, ok := f.Interface().(*string); ok {
					return strings.TrimSpace(*s) != ""
				}
			}
			return true
		}
		return !f.IsZero()
	}
	return false
}

// configPageEntry is one row of the settings page.
type configPageEntry struct {
	Key         string
	Value       string // "" = unset
	Source      string // file / default / unset
	Description string
}

// buildConfigPageEntries lists the user-facing keys with their current value,
// source and description.
func buildConfigPageEntries(cfg *Config, r ResolvedConfig) []configPageEntry {
	entries := make([]configPageEntry, 0, len(configPageKeys))
	for _, key := range configPageKeys {
		kd := findKeyDef(key)
		if kd == nil {
			continue
		}
		entries = append(entries, configPageEntry{
			Key:         key,
			Value:       configKeyValue(r, key),
			Source:      configKeySource(cfg, key),
			Description: kd.description,
		})
	}
	return entries
}

// ─── The page ────────────────────────────────────────────────────────────────

// Sentinel option values; no config key can collide with them.
const (
	configPageSandbox = "\x00sandbox"
	configPagePosture = "\x00posture"
	configPageDone    = "\x00done"
)

// errConfigPageUnavailable reports that the settings page never started, e.g.
// because there is no usable terminal. Callers fall back to their non-
// interactive behaviour; every other error is a real failure worth surfacing.
var errConfigPageUnavailable = errors.New("settings page unavailable")

// cmdConfigPage runs the interactive settings page: navigate the list, edit a
// key, repeat. It is what `nav-pilot config` with no subcommand does on a
// terminal. The list itself is a Bubble Tea page (config_tui.go); editing a
// key drops back to the huh prompts in editConfigKey, then the page reopens.
func cmdConfigPage() error {
	if !isInteractive() {
		return fmt.Errorf("%w: no usable terminal", errConfigPageUnavailable)
	}

	// Reading the preset costs a cplt spawn, so it is read once per page and
	// refreshed only when the user actually changes it — not on every redraw.
	preset := cpltSandboxPreset()

	fmt.Printf("%s %s\n\n", dim("Config file:"), configPath())

	for {
		cfg, err := readConfig()
		if err != nil {
			return err
		}
		resolved := resolve(cfg, CLIOverrides{})
		entries := buildConfigPageEntries(cfg, resolved)

		choice, err := runConfigPageTUI(entries, preset)
		if err != nil {
			return fmt.Errorf("%w: %v", errConfigPageUnavailable, err)
		}

		switch choice {
		case "", configPageDone:
			return nil
		case configPageSandbox:
			// "cplt not on PATH" is a notice, not a reason to leave the page.
			if err := cmdConfigSandbox(); err != nil {
				fmt.Printf("%s %v\n", yellow("⚠"), err)
			}
		case configPagePosture:
			if err := cmdConfigStrictPreset(); err != nil {
				fmt.Printf("%s %v\n", yellow("⚠"), err)
			} else {
				preset = cpltSandboxPreset()
			}
		default:
			if err := editConfigKey(choice, resolved); err != nil {
				fmt.Printf("%s %v\n", red("✗"), err)
			}
		}
		fmt.Println()
	}
}

// editConfigKey prompts for a new value for one key and persists it.
func editConfigKey(key string, r ResolvedConfig) error {
	kd := findKeyDef(key)
	if kd == nil {
		return fmt.Errorf("unknown key: %q", key)
	}
	current := configKeyValue(r, key)

	if key == "model" {
		return editModelKey(r, resolvedFieldStr(r, "model"))
	}

	if key == "local_model" {
		return editLocalModelKey(r.LocalModel)
	}

	value := current

	var opts []huh.Option[string]
	switch {
	case kd.kind == keyKindBool:
		opts = []huh.Option[string]{huh.NewOption("true", "true"), huh.NewOption("false", "false")}
	case key == "client":
		// Display names, not bare ids, so the picker reads like the setup wizard.
		for _, p := range allProviders() {
			opts = append(opts, huh.NewOption(p.DisplayName(), p.ID()))
		}
	case len(kd.allowed) > 0:
		for _, a := range kd.allowed {
			opts = append(opts, huh.NewOption(a, a))
		}
		// Keys without a built-in default can be cleared again.
		if kd.defaultVal == "" {
			opts = append(opts, huh.NewOption("(unset)", ""))
		}
	}

	var field huh.Field
	if opts == nil {
		field = huh.NewInput().Title(key).Description(kd.description).Value(&value)
	} else {
		field = huh.NewSelect[string]().Title(key).Description(kd.description).Options(opts...).Value(&value)
	}

	if err := field.WithTheme(navTheme()).Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}

	return persistConfigValue(key, strings.TrimSpace(value))
}

// editModelKey prompts for the model using the configured client's curated
// model list: the user picks a label instead of memorizing a model id.
func editModelKey(r ResolvedConfig, current string) error {
	kd := findKeyDef("model")
	p, _ := providerFor(r.Client)
	value, err := promptModel(p, "model", kd.description, current)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}
	return persistConfigValue("model", value)
}

// editLocalModelKey prompts for the served local model from the manifest's own
// entries rather than free text: the ids are long enough that typing one from
// memory is how you end up on the default without noticing. "(default)" clears
// the key.
func editLocalModelKey(current string) error {
	kd := findKeyDef("local_model")
	opts := []huh.Option[string]{huh.NewOption("(manifest default)", "")}
	// Marked, not hidden. A model this machine cannot hold is still worth
	// seeing: it is what the developer with 64 GB two desks over is running,
	// and hiding it makes the list look like the model does not exist. A
	// machine that will not tell us its memory marks nothing.
	ramGB, ramErr := local.MachineRAMGB()
	for _, m := range local.Active().Models {
		label := m.Model
		if m.Name != "" {
			label = m.Name + " (" + m.Model + ")"
		}
		if ramErr == nil && m.MinRAMGB > 0 && m.MinRAMGB > ramGB {
			label += fmt.Sprintf("  — needs %d GB, this machine has %d", m.MinRAMGB, ramGB)
		}
		opts = append(opts, huh.NewOption(label, m.Model))
	}
	value := current
	field := huh.NewSelect[string]().Title("local_model").Description(kd.description).Options(opts...).Value(&value)
	if err := field.WithTheme(navTheme()).Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}
	return persistConfigValue("local_model", strings.TrimSpace(value))
}

// persistConfigValue writes value for key, clearing the key when value is
// blank so it falls back to its built-in default.
func persistConfigValue(key, value string) error {
	if value == "" {
		if err := clearConfigKey(key); err != nil {
			return err
		}
		fmt.Printf("%s %s cleared\n", green("✓"), key)
		return nil
	}
	tomlVal, err := writeConfigKey(key, value)
	if err != nil {
		return err
	}
	fmt.Printf("%s %s = %s\n", green("✓"), key, tomlVal)
	return nil
}

// wordWrap wraps s at width on word boundaries.
func wordWrap(s string, width int) string {
	var lines []string
	for _, paragraph := range strings.Split(s, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			switch {
			case line == "":
				line = word
			case len(line)+1+len(word) <= width:
				line += " " + word
			default:
				lines = append(lines, line)
				line = word
			}
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// clearConfigKey drops a key from the config file so it falls back to its
// built-in default. writeConfigKey cannot express this: an allowlisted key
// rejects the empty string.
func clearConfigKey(key string) error {
	if findKeyDef(key) == nil {
		return fmt.Errorf("unknown key: %q", key)
	}
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("reading config: %w", err)
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if isConfigKeyLine(line, key) && !strings.HasPrefix(trimmed, "#") {
			continue
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return os.Chmod(path, 0o600)
}
