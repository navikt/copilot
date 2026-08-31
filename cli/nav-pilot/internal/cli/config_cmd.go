package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/navikt/copilot/cli/nav-pilot/internal/local"
)

// ─── Key definitions ─────────────────────────────────────────────────────────

type keyKind int

const (
	keyKindString keyKind = iota
	keyKindInt
	keyKindBool
)

type configKeyDef struct {
	name        string
	kind        keyKind
	description string
	allowed     []string // nil = any non-empty string
	defaultVal  string   // empty = no default / unset
	flag        string   // corresponding Copilot CLI flag
}

var configKeyDefs = []configKeyDef{
	{
		name:        "version",
		kind:        keyKindInt,
		description: "Configuration schema version. Must be 1.",
		allowed:     []string{"1"},
		defaultVal:  "",
		flag:        "",
	},
	{
		name:        "client",
		kind:        keyKindString,
		description: "Coding-agent CLI to launch (copilot, opencode, pi).",
		allowed:     validProviderIDs,
		defaultVal:  "copilot",
		flag:        "--client",
	},
	{
		name:        "source",
		kind:        keyKindString,
		description: "Agentpakke content source: a GitHub repo (owner/name) or an absolute path to a local checkout. Set it to \"\" to clear it and go back to the default.",
		allowed:     nil,
		defaultVal:  defaultSourceRepo,
		flag:        "--source",
	},
	{
		name:        "model",
		kind:        keyKindString,
		description: "Model id (e.g. auto, claude-opus-4.8, gpt-5.5). Format-validated; the catalog is checked downstream.",
		allowed:     nil,
		defaultVal:  "",
		flag:        "--model",
	},
	{
		name:        "mode",
		kind:        keyKindString,
		description: "Copilot conversation mode.",
		allowed:     validModes,
		defaultVal:  "default",
		flag:        "--mode",
	},
	{
		name:        "reasoning_effort",
		kind:        keyKindString,
		description: "Reasoning effort level.",
		allowed:     validReasoningEffort,
		defaultVal:  "",
		flag:        "--effort",
	},
	{
		name:        "context_tier",
		kind:        keyKindString,
		description: "Context window tier.",
		allowed:     validContextTiers,
		defaultVal:  "",
		flag:        "--context",
	},
	{
		name:        "allow_all_tools",
		kind:        keyKindBool,
		description: "Allow all tools without per-tool confirmation.",
		allowed:     nil,
		defaultVal:  "false",
		flag:        "--allow-all-tools",
	},
	{
		name:        "ask_user",
		kind:        keyKindBool,
		description: "Ask the user before taking actions. Set to false to disable.",
		allowed:     nil,
		defaultVal:  "true",
		flag:        "--no-ask-user (when false)",
	},
	{
		name:        "auto_launch",
		kind:        keyKindBool,
		description: "Launch the coding agent automatically after install/sync. Set to false to never launch it; nav-pilot prints the command instead.",
		allowed:     nil,
		defaultVal:  "true",
		flag:        "--auto-launch / --no-auto-launch",
	},
	{
		name:        "auto_update",
		kind:        keyKindBool,
		description: "Automatically upgrade nav-pilot when a new version is available, skipping the interactive prompt.",
		allowed:     nil,
		defaultVal:  "false",
		flag:        "",
	},
	{
		name:        "log_level",
		kind:        keyKindString,
		description: "Log level for Copilot CLI output.",
		allowed:     validLogLevels,
		defaultVal:  "",
		flag:        "--log-level",
	},
	{
		name:        "otel_log_level",
		kind:        keyKindString,
		description: "OpenTelemetry diagnostic log level for the Copilot CLI (OTEL_LOG_LEVEL). Defaults to none to suppress telemetry connection-error spam.",
		allowed:     validOtelLogLevels,
		defaultVal:  "none",
		flag:        "--otel-log-level",
	},
	{
		name:        "local_enabled",
		kind:        keyKindBool,
		description: "Dispatch to a local model server instead of a hosted one (alpha). Set by 'nav-pilot alpha local init'; local models are hidden and never launched while this is false.",
		allowed:     nil,
		defaultVal:  "false",
		flag:        "",
	},
	{
		name:        "local_autostart",
		kind:        keyKindBool,
		description: "Start the local server automatically when a launch needs it and nothing is running. Off by default: the first start on a cold cache takes minutes.",
		allowed:     nil,
		defaultVal:  "false",
		flag:        "",
	},
	{
		name:        "local_loop_guard",
		kind:        keyKindInt,
		description: "Identical consecutive tool calls that end a local turn. Local models get stuck repeating one call; this is where nav-pilot stops them.",
		allowed:     nil,
		defaultVal:  strconv.Itoa(local.DefaultLoopGuardRepeat),
		flag:        "",
	},
	{
		name:        "rtk_prompted_client",
		kind:        keyKindString,
		description: "Comma-separated list of clients where the RTK setup was prompted.",
		allowed:     nil,
		defaultVal:  "",
		flag:        "",
	},
	{
		name:        "rtk_prompted_at",
		kind:        keyKindString,
		description: "Internal flag to track when the user was last prompted to set up rtk (RFC3339 timestamp).",
		allowed:     nil,
		defaultVal:  "",
		flag:        "",
	},
}

func findKeyDef(name string) *configKeyDef {
	for i := range configKeyDefs {
		if configKeyDefs[i].name == name {
			return &configKeyDefs[i]
		}
	}
	return nil
}

func knownKeyNames() string {
	names := make([]string, len(configKeyDefs))
	for i, kd := range configKeyDefs {
		names[i] = kd.name
	}
	return strings.Join(names, ", ")
}

// ─── Init template ────────────────────────────────────────────────────────────

const configInitTemplate = `# nav-pilot configuration
# Generated by: nav-pilot config init
# Override path: NAV_PILOT_CONFIG=/path/to/config.toml
# Uncomment and edit the options you want to customize.

# Configuration schema version. Must be 1.
version = 1

# Coding-agent CLI nav-pilot launches.
# Allowed: copilot, opencode, pi — Default: copilot
# client = "copilot"

# Agentpakke content source: a GitHub repo (owner/name) or an absolute path to a
# local checkout. Set by "nav-pilot install --source <repo>" after a successful
# install; clear it with: nav-pilot config set source ""
# Default: navikt/copilot
# Corresponds to nav-pilot flag: --source
# source = "navikt/copilot"

# Model id. Common Copilot models: auto, claude-opus-5, claude-sonnet-5,
# claude-sonnet-4.6, claude-haiku-4.5, claude-opus-4.8, gpt-5.6-sol,
# gpt-5.6-terra, gpt-5.6-luna, gpt-5.5, gpt-5.4, gpt-5.3-codex, gpt-5.4-mini,
# gpt-5-mini, gemini-3.6-flash, gemini-3.1-pro-preview, gemini-3.5-flash,
# kimi-k2.7-code, kimi-k3.
# For opencode (launched via cplt → GitHub Copilot provider): a bare Copilot id
# is mapped to github-copilot/<id>, or set a full provider/model id directly.
# Format-validated locally; the model catalog is checked by the downstream CLI.
# Default: agent-specific default
# Corresponds to Copilot CLI flag: --model
# model = "auto"

# Copilot conversation mode.
# Allowed: default, plan, autopilot — Default: default
# Corresponds to Copilot CLI flag: --mode
# mode = "default"

# Reasoning effort level.
# Allowed: none, low, medium, high, xhigh, max — Default: unset (agent default)
# Corresponds to Copilot CLI flag: --effort
# reasoning_effort = "medium"

# Context window tier.
# Allowed: default, long_context — Default: unset (agent default)
# Corresponds to Copilot CLI flag: --context
# context_tier = "default"

# Allow all tools without per-tool confirmation.
# Default: false
# Corresponds to Copilot CLI flag: --allow-all-tools
# allow_all_tools = false

# Ask the user before taking actions. Set to false to disable.
# Default: true
# Corresponds to Copilot CLI flag: --no-ask-user (when false)
# ask_user = true

# Launch the coding agent automatically after sync/install. Set to false to
# never launch it; nav-pilot prints the ready-to-run command instead.
# Default: true
# Corresponds to nav-pilot flag: --auto-launch / --no-auto-launch
# auto_launch = true

# Automatically upgrade nav-pilot when a new version is available, skipping the
# interactive prompt.
# Default: false
# auto_update = false

# Log level for Copilot CLI output.
# Allowed: none, error, warning, info, debug, all, default — Default: unset
# Corresponds to Copilot CLI flag: --log-level
# log_level = "info"

# OpenTelemetry diagnostic log level for the Copilot CLI (sets OTEL_LOG_LEVEL).
# Allowed: none, error, warning, warn, info, debug, verbose, all — Default: none
# Keep this at "none" to suppress Copilot telemetry connection-error spam when
# the OTLP endpoint is unreachable. A pre-existing OTEL_LOG_LEVEL in your shell
# environment takes precedence.
# otel_log_level = "none"

# Dispatch to a local model server on this machine instead of a hosted one
# (alpha). Set by 'nav-pilot alpha local init'; while this is false, local
# models are hidden from the picker and never launched.
# Default: false
# local_enabled = false

# Start the local server when a launch needs it and nothing is running. Off by
# default: the first start on a cold cache takes minutes, and a launch that
# starts a 21 GB process unasked is a surprise rather than a convenience.
# Default: false
# local_autostart = false

# Identical consecutive tool calls that end a local turn. Local models get
# stuck repeating one call — we measured runs of 203 — and this is where
# nav-pilot stops them. Minimum 2.
# Default: 8
# local_loop_guard = 8

# Internal flag to track which client the user was last prompted to set up rtk for.
# Default: unset
# rtk_prompted_client = ""

# Internal flag to track when the user was last prompted to set up rtk (RFC3339 timestamp).
# Default: unset
# rtk_prompted_at = ""
`

// ─── Subcommand dispatch ──────────────────────────────────────────────────────

// cmdConfigPageFn is the settings page, overridable in tests so the fallback
// can be exercised without a terminal.
var cmdConfigPageFn = cmdConfigPage

func cmdConfig(args []string, force bool, jsonOutput bool) error {
	if len(args) == 0 {
		// On a terminal, bare `config` opens the interactive settings page;
		// scripts and --json keep the usage error, and so does a page that
		// could not start (stdin on a char device with no controlling tty).
		if isInteractive() && !jsonOutput {
			err := cmdConfigPageFn()
			if err == nil {
				return nil
			}
			// A page that could not start falls back to the usage error; a
			// broken config file must still say so.
			if !errors.Is(err, errConfigPageUnavailable) {
				return err
			}
		}
		return fmt.Errorf("config requires a subcommand.\n\nUsage: nav-pilot config <subcommand> [options]\n\nSubcommands:\n  init      Create ~/.nav-pilot/config.toml with all options commented out\n  setup     Run the interactive first-run setup wizard\n  show      Print effective configuration (file values merged with defaults)\n  path      Print the config file path\n  get       Print one key value\n  set       Set a key value (creates file if missing)\n  validate  Validate config syntax, unknown keys, and values\n  explain   Describe configuration keys\n  sandbox   Interactively configure cplt sandbox profile")
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "init":
		return cmdConfigInit()
	case "setup":
		return cmdConfigSetup(force)
	case "show":
		return cmdConfigShow(jsonOutput)
	case "path":
		return cmdConfigPath()
	case "get":
		if len(rest) == 0 {
			return fmt.Errorf("config get requires a key.\n\nUsage: nav-pilot config get <key>\n\nKnown keys: %s", knownKeyNames())
		}
		return cmdConfigGet(rest[0])
	case "set":
		if len(rest) < 2 {
			return fmt.Errorf("config set requires a key and value.\n\nUsage: nav-pilot config set <key> <value>")
		}
		return cmdConfigSet(rest[0], rest[1])
	case "validate":
		return cmdConfigValidate()
	case "explain":
		key := ""
		if len(rest) > 0 {
			key = rest[0]
		}
		return cmdConfigExplain(key)
	case "sandbox":
		return cmdConfigSandbox()
	default:
		return fmt.Errorf("unknown config subcommand: %q\n\nSubcommands: init, setup, show, path, get, set, validate, explain, sandbox", sub)
	}
}

// ─── config init ─────────────────────────────────────────────────────────────

func cmdConfigInit() error {
	path := configPath()

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s\n\nUse %s to see current values, or edit the file directly",
			path, bold("nav-pilot config show"))
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checking config path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(configInitTemplate), 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("%s Created %s\n", green("✓"), path)
	fmt.Printf("  Edit the file or use %s to set individual options.\n", bold("nav-pilot config set"))
	return nil
}

// ─── config show ─────────────────────────────────────────────────────────────

func cmdConfigShow(jsonOutput bool) error {
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	resolved := resolve(cfg, CLIOverrides{})

	if jsonOutput {
		return outputJSON(map[string]interface{}{
			"client":           resolved.Client,
			"source":           effectiveSourceLabel(resolved),
			"model":            resolved.Model,
			"mode":             resolved.Mode,
			"reasoning_effort": resolved.ReasoningEffort,
			"context_tier":     resolved.ContextTier,
			"allow_all_tools":  resolved.AllowAllTools,
			"ask_user":         resolved.AskUser,
			"auto_launch":      resolved.AutoLaunch,
			"auto_update":      resolved.AutoUpdate,
			"log_level":        resolved.LogLevel,
			"otel_log_level":   resolved.OtelLogLevel,
		})
	}

	path := configPath()
	if cfg == nil {
		fmt.Printf("# Config file: %s %s\n\n", path, dim("(not found, using defaults)"))
	} else {
		fmt.Printf("# Config file: %s\n\n", path)
	}

	for _, key := range configPageKeys {
		val := configKeyValue(resolved, key)
		if val == "" {
			val = "(unset)"
		}
		fmt.Printf("  %-20s = %-20s (%s)\n", key, val, configKeySource(cfg, key))
	}

	return nil
}

// ─── config path ─────────────────────────────────────────────────────────────

func cmdConfigPath() error {
	fmt.Println(configPath())
	return nil
}

// ─── config get ──────────────────────────────────────────────────────────────

func cmdConfigGet(key string) error {
	kd := findKeyDef(key)
	if kd == nil {
		return fmt.Errorf("unknown key: %q\n\nKnown keys: %s", key, knownKeyNames())
	}

	cfg, err := readConfig()
	if err != nil {
		return err
	}
	resolved := resolve(cfg, CLIOverrides{})

	val := resolvedFieldStr(resolved, key)
	fmt.Println(val)
	return nil
}

// resolvedFieldStr returns the string representation of a resolved field.
func resolvedFieldStr(r ResolvedConfig, key string) string {
	switch key {
	case "version":
		return "1" // version is always 1 when valid
	case "client":
		return r.Client
	case "source":
		return r.Source
	case "model":
		return r.Model
	case "mode":
		return r.Mode
	case "reasoning_effort":
		return r.ReasoningEffort
	case "context_tier":
		return r.ContextTier
	case "allow_all_tools":
		return strconv.FormatBool(r.AllowAllTools)
	case "ask_user":
		return strconv.FormatBool(r.AskUser)
	case "auto_launch":
		return strconv.FormatBool(r.AutoLaunch)
	case "auto_update":
		return strconv.FormatBool(r.AutoUpdate)
	case "log_level":
		return r.LogLevel
	case "otel_log_level":
		return r.OtelLogLevel
	case "local_enabled":
		return strconv.FormatBool(r.LocalEnabled)
	case "local_autostart":
		return strconv.FormatBool(r.LocalAutostart)
	case "local_loop_guard":
		return strconv.Itoa(localLoopGuard(r))
	case "rtk_prompted_client":
		return r.RtkPromptedClient
	case "rtk_prompted_at":
		return r.RtkPromptedAt
	}
	return ""
}

// ─── config set ──────────────────────────────────────────────────────────────

func cmdConfigSet(key, value string) error {
	tomlVal, err := writeConfigKey(key, value)
	if err != nil {
		return err
	}
	if key == "source" && strings.TrimSpace(value) == "" {
		fmt.Printf("%s source cleared — nav-pilot installs and syncs from %s again.\n",
			green("✓"), bold(defaultSourceRepo))
		return nil
	}
	fmt.Printf("%s %s = %s\n", green("✓"), key, tomlVal)
	return nil
}

// effectiveSourceLabel names the source a resolved config selects, spelling out
// the built-in default when no source is persisted.
func effectiveSourceLabel(r ResolvedConfig) string {
	if r.Source == "" {
		return defaultSourceRepo
	}
	return r.Source
}

// writeConfigKey validates a key/value pair and writes it into the config file,
// replacing the first matching line (active or commented out) or appending it.
// It returns the TOML literal written. Callers own the user-facing message —
// `config set` prints one, install's source persistence prints another.
func writeConfigKey(key, value string) (string, error) {
	kd := findKeyDef(key)
	if kd == nil {
		return "", fmt.Errorf("unknown key: %q\n\nKnown keys: %s", key, knownKeyNames())
	}
	if err := validateKeyValue(kd, value); err != nil {
		return "", err
	}

	tomlVal, err := formatTOMLValue(kd, value)
	if err != nil {
		return "", err
	}
	newLine := key + " = " + tomlVal

	path := configPath()

	// Read existing content (if any).
	var lines []string
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("reading config: %w", err)
		}
		// New file: seed the required schema version so the resulting config
		// passes on-launch validation (validateConfig requires version = 1).
		lines = []string{"version = 1"}
	} else {
		lines = strings.Split(string(data), "\n")
		// Remove trailing empty element from trailing newline.
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
	}

	// Replace an active line for the key if there is one anywhere in the file,
	// and only otherwise a commented-out one: replacing the comment while an
	// active line lives further down would define the key twice and leave the
	// config unparseable.
	replace := -1
	for i, line := range lines {
		if !isConfigKeyLine(line, key) {
			continue
		}
		if !strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			replace = i
			break
		}
		if replace == -1 {
			replace = i
		}
	}
	if replace >= 0 {
		lines[replace] = newLine
	} else {
		lines = append(lines, newLine)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("creating config directory: %w", err)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("writing config: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("setting config permissions: %w", err)
	}

	return tomlVal, nil
}

// isConfigKeyLine returns true if line (active or commented-out) represents the given key.
func isConfigKeyLine(line, key string) bool {
	s := strings.TrimLeft(line, " \t")
	// Strip leading comment characters and spaces.
	s = strings.TrimLeft(s, "#")
	s = strings.TrimLeft(s, " \t")
	// Must start exactly with the key followed by optional spaces then '='.
	if !strings.HasPrefix(s, key) {
		return false
	}
	rest := s[len(key):]
	rest = strings.TrimLeft(rest, " \t")
	return strings.HasPrefix(rest, "=")
}

// validateKeyValue checks that a value is valid for a given key definition.
func validateKeyValue(kd *configKeyDef, value string) error {
	switch kd.kind {
	case keyKindInt:
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("key %q requires an integer value, got: %q", kd.name, value)
		}
	case keyKindBool:
		switch strings.ToLower(value) {
		case "true", "false", "1", "0", "yes", "no":
		default:
			return fmt.Errorf("key %q requires a boolean value (true/false), got: %q", kd.name, value)
		}
	}
	if kd.name == "local_loop_guard" {
		if n, err := strconv.Atoi(value); err == nil && n < 2 {
			return fmt.Errorf("key %q must be at least 2 (got %d) — one tool call is not a loop", kd.name, n)
		}
	}
	// Key-specific validation beyond the generic kind/allowlist checks.
	if kd.name == "model" {
		if err := validateModelValue(value); err != nil {
			return err
		}
	}
	// An empty source is the documented way to clear the key and fall back to
	// the default; any other value must name a repo or a local checkout.
	if kd.name == "source" && strings.TrimSpace(value) != "" {
		if err := validateSourceValue(value); err != nil {
			return err
		}
	}
	// Allowlist check for string and int keys that have one.
	if len(kd.allowed) > 0 && kd.kind != keyKindBool {
		if !containsStr(kd.allowed, value) {
			return fmt.Errorf("key %q value %q is not valid\n\nAllowed: %s",
				kd.name, value, strings.Join(kd.allowed, ", "))
		}
	}
	return nil
}

// formatTOMLValue formats a CLI string value as a TOML literal for the given key kind.
func formatTOMLValue(kd *configKeyDef, value string) (string, error) {
	switch kd.kind {
	case keyKindString:
		return fmt.Sprintf("%q", value), nil
	case keyKindInt:
		n, err := strconv.Atoi(value)
		if err != nil {
			return "", fmt.Errorf("key %q requires an integer value", kd.name)
		}
		return strconv.Itoa(n), nil
	case keyKindBool:
		switch strings.ToLower(value) {
		case "true", "1", "yes":
			return "true", nil
		case "false", "0", "no":
			return "false", nil
		default:
			return "", fmt.Errorf("key %q requires a boolean value", kd.name)
		}
	}
	return "", fmt.Errorf("unknown key kind for %q", kd.name)
}

// ─── config validate ─────────────────────────────────────────────────────────

func cmdConfigValidate() error {
	path := configPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("%s No config file found at %s\n", yellow("⚠"), path)
			fmt.Printf("  Run %s to create one.\n", bold("nav-pilot config init"))
			return nil
		}
		return fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	meta, parseErr := toml.Decode(string(data), &cfg)
	if parseErr != nil {
		fmt.Printf("%s TOML parse error: %v\n", red("✗"), parseErr)
		return fmt.Errorf("config file has invalid TOML syntax")
	}

	var problems []string

	// Unknown keys (keys in file not recognized by nav-pilot).
	for _, key := range meta.Undecoded() {
		problems = append(problems, fmt.Sprintf("unknown key: %s", strings.Join(key, ".")))
	}

	// Semantic validation — append the []string slice directly.
	problems = append(problems, validateConfigProblems(&cfg)...)

	// Non-fatal hints about best practices.
	hints := configHints(&cfg)

	if len(problems) == 0 && len(hints) == 0 {
		fmt.Printf("%s Config is valid (%s)\n", green("✓"), path)
		return nil
	}

	if len(problems) > 0 {
		fmt.Printf("%s Config has %d problem(s) (%s):\n", red("✗"), len(problems), path)
		for _, p := range problems {
			fmt.Printf("  - %s\n", p)
		}
	}
	if len(hints) > 0 {
		if len(problems) == 0 {
			fmt.Printf("%s Config is valid (%s)\n", green("✓"), path)
		}
		fmt.Printf("%s Hints:\n", yellow("⚠"))
		for _, h := range hints {
			fmt.Printf("  - %s\n", h)
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("config validation failed")
	}
	return nil
}

// configHints returns non-fatal informational hints for a config.
// Unlike validateConfigProblems, these do not cause validation to fail.
func configHints(cfg *Config) []string {
	if cfg == nil {
		return nil
	}
	var hints []string
	if cfg.Model != nil {
		m := *cfg.Model
		if strings.Contains(m, "/") {
			shortID := strings.SplitN(m, "/", 2)[1]
			if shortID == "" {
				shortID = m // malformed provider/ with no model — keep full string
			}
			hints = append(hints, fmt.Sprintf(
				"model %q uses a provider-qualified format — nav-pilot translates automatically. Use the canonical short-id instead: run %s.",
				m, bold("nav-pilot config set model "+shortID),
			))
		}
	}
	return hints
}

// ─── config explain ──────────────────────────────────────────────────────────

func cmdConfigExplain(key string) error {
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	resolved := resolve(cfg, CLIOverrides{})

	if key != "" {
		kd := findKeyDef(key)
		if kd == nil {
			return fmt.Errorf("unknown key: %q\n\nKnown keys: %s", key, knownKeyNames())
		}
		printKeyExplain(kd, resolved)
		return nil
	}

	// Print all keys.
	for i := range configKeyDefs {
		if i > 0 {
			fmt.Println()
		}
		printKeyExplain(&configKeyDefs[i], resolved)
	}
	return nil
}

func printKeyExplain(kd *configKeyDef, resolved ResolvedConfig) {
	fmt.Printf("  %s\n", bold(kd.name))
	fmt.Printf("    %s\n", kd.description)
	if len(kd.allowed) > 0 {
		fmt.Printf("    Allowed:  %s\n", strings.Join(kd.allowed, ", "))
	} else if kd.kind == keyKindBool {
		fmt.Printf("    Allowed:  true, false\n")
	} else if kd.kind == keyKindInt {
		fmt.Printf("    Allowed:  a whole number\n")
	} else if kd.name == "model" {
		var ids []string
		for _, p := range allProviders() {
			for _, m := range p.KnownModels() {
				ids = append(ids, m.ID)
			}
		}
		fmt.Printf("    Common:   %s\n", strings.Join(ids, ", "))
		fmt.Printf("    Allowed:  any well-formed id ([A-Za-z0-9._/-], e.g. provider/model for opencode)\n")
	} else {
		fmt.Printf("    Allowed:  any non-empty string\n")
	}
	if kd.defaultVal != "" {
		fmt.Printf("    Default:  %s\n", kd.defaultVal)
	} else {
		fmt.Printf("    Default:  (unset)\n")
	}
	if kd.flag != "" {
		fmt.Printf("    CLI flag: %s\n", kd.flag)
	}

	val := resolvedFieldStr(resolved, kd.name)
	if val == "" {
		fmt.Printf("    Current:  (unset)\n")
	} else {
		fmt.Printf("    Current:  %s\n", val)
	}
	fmt.Printf("    To set:   nav-pilot config set %s <value>\n", kd.name)
}
