package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
)

// ─── Key listing (shared with config show) ───────────────────────────────────

// configPageKeys lists the user-facing keys, in display order. Internal
// bookkeeping keys (version, rtk_prompted_*) are deliberately left out.
var configPageKeys = []string{
	"client",
	"source",
	"model",
	"mode",
	"reasoning_effort",
	"context_tier",
	"allow_all_tools",
	"ask_user",
	"auto_launch",
	"auto_update",
	"log_level",
	"otel_log_level",
}

// configKeyValue returns the effective value of a key as displayed, spelling
// out the built-in default repo when no source is persisted.
func configKeyValue(r ResolvedConfig, key string) string {
	if key == "source" {
		return effectiveSourceLabel(r)
	}
	return resolvedFieldStr(r, key)
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

// configKeyInFile reports whether the config file sets the key. An empty source
// counts as unset — it is the documented way to fall back to the default.
func configKeyInFile(cfg *Config, key string) bool {
	switch key {
	case "client":
		return cfg.Client != nil
	case "source":
		return cfg.Source != nil && strings.TrimSpace(*cfg.Source) != ""
	case "model":
		return cfg.Model != nil
	case "mode":
		return cfg.Mode != nil
	case "reasoning_effort":
		return cfg.ReasoningEffort != nil
	case "context_tier":
		return cfg.ContextTier != nil
	case "allow_all_tools":
		return cfg.AllowAllTools != nil
	case "ask_user":
		return cfg.AskUser != nil
	case "auto_launch":
		return cfg.AutoLaunch != nil
	case "auto_update":
		return cfg.AutoUpdate != nil
	case "log_level":
		return cfg.LogLevel != nil
	case "otel_log_level":
		return cfg.OtelLogLevel != nil
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

// label renders the entry as the select option text.
func (e configPageEntry) label() string {
	val := e.Value
	if val == "" {
		val = "(unset)"
	}
	return fmt.Sprintf("%s = %s  (%s)", e.Key, val, e.Source)
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

// cpltPostureLabel renders the security-posture row the way the key rows read:
// the current value first, and what it should be when it is not that already.
func cpltPostureLabel(preset string) string {
	switch {
	case preset == "":
		return "cplt security posture   unknown  (could not read it from cplt)"
	case cpltRecommendStrict(preset):
		return fmt.Sprintf("cplt security posture   %s  (recommended: %s)", preset, cpltRecommendedPreset)
	default:
		return fmt.Sprintf("cplt security posture   %s", preset)
	}
}

// errConfigPageUnavailable reports that the settings page never started, e.g.
// because there is no usable terminal. Callers fall back to their non-
// interactive behaviour; every other error is a real failure worth surfacing.
var errConfigPageUnavailable = errors.New("settings page unavailable")

// cmdConfigPage runs the interactive settings page: pick a key, edit it, repeat.
// It is what `nav-pilot config` with no subcommand does on a terminal.
func cmdConfigPage() error {
	// The header waits for the first successful render: a page that cannot open
	// a TTY must leave no trace before its caller falls back.
	rendered := false

	// Reading the preset costs a cplt spawn, so it is read once per page and
	// refreshed only when the user actually changes it — not on every redraw.
	preset := cpltSandboxPreset()

	for {
		cfg, err := readConfig()
		if err != nil {
			return err
		}
		resolved := resolve(cfg, CLIOverrides{})
		entries := buildConfigPageEntries(cfg, resolved)

		descriptions := map[string]string{
			configPageSandbox: "Runs the cplt sandbox wizard (requires cplt on your PATH).",
			configPagePosture: "Sets cplt sandbox.preset = strict, which turns on gh_guard, git_guard and forced proxy in one key (requires cplt on your PATH).",
			configPageDone:    "Leave the settings page.",
		}
		opts := make([]huh.Option[string], 0, len(entries)+3)
		for _, e := range entries {
			opts = append(opts, huh.NewOption(e.label(), e.Key))
			descriptions[e.Key] = e.Description
		}
		opts = append(opts,
			huh.NewOption("Configure cplt sandbox settings…", configPageSandbox),
			huh.NewOption(cpltPostureLabel(preset), configPagePosture),
			huh.NewOption("Done", configPageDone),
		)

		choice := entries[0].Key
		err = huh.NewSelect[string]().
			Title("nav-pilot settings").
			Options(opts...).
			DescriptionFunc(func() string { return descriptions[choice] }, &choice).
			Value(&choice).
			WithTheme(navTheme()).
			Run()
		if err != nil {
			// Esc / Ctrl-C is a normal way to leave the page.
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			if !rendered {
				return fmt.Errorf("%w: %v", errConfigPageUnavailable, err)
			}
			return err
		}
		if !rendered {
			rendered = true
			fmt.Printf("%s %s\n\n", dim("Config file:"), configPath())
		}

		switch choice {
		case configPageDone:
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

	value := current

	var opts []huh.Option[string]
	switch {
	case kd.kind == keyKindBool:
		opts = []huh.Option[string]{huh.NewOption("true", "true"), huh.NewOption("false", "false")}
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

	value = strings.TrimSpace(value)
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
