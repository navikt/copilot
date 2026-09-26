package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

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
	internal    bool     // bookkeeping key, hidden from the settings page
	group       string   // settings-page section; empty = "General"
}

var configKeyDefs = []configKeyDef{
	{
		name:        "version",
		kind:        keyKindInt,
		description: "Configuration schema version. Must be 1.",
		allowed:     []string{"1"},
		defaultVal:  "",
		flag:        "",
		internal:    true,
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
		flag:        "--allow-all-tools / --no-allow-all-tools",
	},
	{
		name:        "ask_user",
		kind:        keyKindBool,
		description: "Ask the user before taking actions. Set to false to disable.",
		allowed:     nil,
		defaultVal:  "true",
		flag:        "--ask-user / --no-ask-user",
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
		description: "Automatically upgrade nav-pilot when a new version is available, skipping the interactive prompt. A failed upgrade runs the command on the current version and waits 24 hours before trying again.",
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
		group:       "Logging",
	},
	{
		name:        "otel_log_level",
		kind:        keyKindString,
		description: "OpenTelemetry diagnostic log level for the Copilot CLI (OTEL_LOG_LEVEL). Defaults to none to suppress telemetry connection-error spam.",
		allowed:     validOtelLogLevels,
		defaultVal:  "none",
		flag:        "--otel-log-level",
		group:       "Logging",
	},
	{
		name:        "local_enabled",
		kind:        keyKindBool,
		description: "Dispatch to a local model server instead of a hosted one (alpha). Set by 'nav-pilot alpha local init'; local models are hidden and never launched while this is false.",
		allowed:     nil,
		defaultVal:  "false",
		flag:        "",
		group:       "Local models (alpha)",
	},
	{
		name:        "local_autostart",
		kind:        keyKindBool,
		description: "Start the local server automatically when a launch needs it and nothing is running. Off by default: a launch that starts a 21 GB process unasked is a surprise rather than a convenience.",
		allowed:     nil,
		defaultVal:  "false",
		flag:        "",
		group:       "Local models (alpha)",
	},
	{
		name:        "local_loop_guard",
		kind:        keyKindInt,
		description: "Identical consecutive tool calls that end a local turn whatever they return. Half this many (at least 2) end it when the results repeat too. Local models get stuck repeating one call; this is where nav-pilot stops them.",
		allowed:     nil,
		defaultVal:  strconv.Itoa(local.DefaultLoopGuardRepeat),
		flag:        "",
		group:       "Local models (alpha)",
	},
	{
		name:        "local_model",
		kind:        keyKindString,
		description: "Which local model the server loads and serves (alpha). Empty means the manifest default. Separate from model, which is the session model. `nav-pilot alpha local use <key>` sets it by key.",
		allowed:     nil,
		defaultVal:  "",
		flag:        "",
		group:       "Local models (alpha)",
	},
	{
		name:        "hook_loop_guard",
		kind:        keyKindBool,
		description: "Warn the model when it repeats one tool call, in every Copilot CLI session and not only local ones. nav-pilot writes a postToolUse hook to ~/.copilot/hooks/ at launch that applies the local_loop_guard rule; false removes it at the next launch.",
		allowed:     nil,
		defaultVal:  "true",
		flag:        "",
		group:       "Hooks",
	},
	{
		name:        "hook_redact_secrets",
		kind:        keyKindBool,
		description: "Mask secrets (GitHub tokens, AWS key ids, private keys, JWTs, password=/api_key= values) in tool results before the model reads them, in every Copilot CLI session. Written at launch as a postToolUse hook in ~/.copilot/hooks/.",
		allowed:     nil,
		defaultVal:  "true",
		flag:        "",
		group:       "Hooks",
	},
	{
		name:        "hook_redact_fnr",
		kind:        keyKindBool,
		description: "Mask fødselsnummer, D-nummer and H-nummer in tool results before the model reads them. Only eleven digits whose date and both mod-11 control digits check out are masked.",
		allowed:     nil,
		defaultVal:  "true",
		flag:        "",
		group:       "Hooks",
	},
	{
		name:        "hook_injection_note",
		kind:        keyKindBool,
		description: "Put a note in front of a tool result that reads like instructions to the model (\"ignore previous instructions\", role markers), telling it the text is data. Flags; never blocks.",
		allowed:     nil,
		defaultVal:  "true",
		flag:        "",
		group:       "Hooks",
	},
	{
		name:        "rtk_prompted_client",
		kind:        keyKindString,
		description: "Comma-separated list of clients where the RTK setup was prompted.",
		allowed:     nil,
		defaultVal:  "",
		flag:        "",
		internal:    true,
	},
	{
		name:        "rtk_prompted_at",
		kind:        keyKindString,
		description: "Internal flag to track when the user was last prompted to set up rtk (RFC3339 timestamp).",
		allowed:     nil,
		defaultVal:  "",
		flag:        "",
		internal:    true,
	},
	{
		name:        "copilot_auth_mode",
		kind:        keyKindString,
		description: "Which auth source reaches cplt for Copilot. nav-pilot never extracts a token itself. auto constrains nothing; env_only requires GH_TOKEN/GITHUB_TOKEN/COPILOT_GITHUB_TOKEN to hold a value and aborts the launch if none does; gh_only removes those variables so no env token reaches the sandbox.",
		allowed:     validCopilotAuthModes,
		defaultVal:  "auto",
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

// userKeyNames are the keys a person sets: every key but nav-pilot's own
// rtk_* bookkeeping.
func userKeyNames() []string {
	var names []string
	for _, kd := range configKeyDefs {
		if !strings.HasPrefix(kd.name, "rtk_") {
			names = append(names, kd.name)
		}
	}
	return names
}

func knownKeyNames() string {
	return strings.Join(userKeyNames(), ", ")
}

// userKey is the key a person typed, as nav-pilot knows it: a renamed key
// maps to its successor with a note, and an unknown one gets the closest
// real name.
func userKey(key string) (string, error) {
	if findKeyDef(key) != nil {
		return key, nil
	}
	if next, ok := renamedConfigKeys[key]; ok {
		fmt.Fprintf(os.Stderr, "%s %s was renamed to %s\n", yellow("⚠"), key, next)
		return next, nil
	}
	msg := fmt.Sprintf("unknown key: %q", key)
	if hint := suggest(key, userKeyNames()); hint != "" {
		msg += fmt.Sprintf(". Did you mean %s?", hint)
	}
	return "", fmt.Errorf("%s\n\nKnown keys: %s", msg, knownKeyNames())
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

# Model id. Common Copilot models: auto, claude-opus-5.5, claude-sonnet-5,
# claude-haiku-4.5, claude-opus-4.8, claude-opus-4.7,
# gpt-6-sol, gpt-6-luna, gpt-5.6-terra, gpt-5.5, gpt-5.4, gpt-5.3-codex,
# gpt-5.4-mini, gpt-5-mini, gemini-3.6-flash, gemini-3.5-flash,
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
# Corresponds to nav-pilot flags: --ask-user / --no-ask-user
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
# default: a launch that starts a 21 GB process unasked is a surprise rather
# than a convenience.
# Default: false
# local_autostart = false

# Identical consecutive tool calls that end a local turn, whatever they
# return. Half this many (at least 2) end it when each call also got the same
# result back. Local models get stuck repeating one call — we measured runs
# of 203 — and this is where nav-pilot stops them. Minimum 2.
# Default: 8
# local_loop_guard = 8

# Which local model the server loads and serves. Unset means whatever the
# local-model manifest calls its default. This is not the session model: the
# model key picks that, and the two are set independently.
# Default: unset
# local_model = "mlx-community/Qwen3.8-27B-4bit"

# Warn the model when it repeats one tool call, in every Copilot CLI session
# and not only local ones. At launch nav-pilot writes a postToolUse hook to
# ~/.copilot/hooks/nav-pilot-loop-guard.json that applies the local_loop_guard
# rule to each tool result; false removes it at the next launch.
# Default: true
# hook_loop_guard = true

# The redaction hook (~/.copilot/hooks/nav-pilot-redact-tool-output.json)
# looks at every tool result before the model reads it, in every Copilot CLI
# session. Each part can be turned off on its own; with all three off the
# hook is removed at the next launch.
# Mask secrets: GitHub tokens, AWS key ids, private keys, JWTs, and the value
# of password=/api_key=-style assignments.
# Default: true
# hook_redact_secrets = true
# Mask fødselsnummer, D-nummer and H-nummer (date and both mod-11 control
# digits valid).
# Default: true
# hook_redact_fnr = true
# Put a note in front of a result that reads like instructions to the model
# ("ignore previous instructions", role markers). Flags; never blocks.
# Default: true
# hook_injection_note = true

# Internal flag to track which client the user was last prompted to set up rtk for.
# Default: unset
# rtk_prompted_client = ""

# Internal flag to track when the user was last prompted to set up rtk (RFC3339 timestamp).
# Default: unset
# rtk_prompted_at = ""

# ── cplt auth ──────────────────────────────────────────────────────────────────
# nav-pilot never extracts a Copilot token itself. With cplt's gh guard on — the
# default since cplt#335, not only under sandbox.preset = strict — cplt uses an
# inherited
# GH_TOKEN/GITHUB_TOKEN/COPILOT_GITHUB_TOKEN if one is set and otherwise runs
# "gh auth token" outside the sandbox; with it off, Copilot authenticates on its
# own. copilot_auth_mode decides which sources reach cplt.
#
# copilot_auth_mode:
#   auto         : no constraint (default)
#   env_only     : a token must already be in the environment, abort the launch if not
#   gh_only      : strip the token variables so no env token reaches the sandbox
# Allowed: auto, env_only, gh_only — Default: auto
# copilot_auth_mode = "auto"
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
		return fmt.Errorf("config requires a subcommand.\n\n%s", strings.TrimSuffix(fmt.Sprintf(commandHelp["config"], configPath()), "\n"))
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "show", "get", "path", "validate":
	default:
		if jsonOutput {
			return fmt.Errorf("--json is not supported by config %s; it works with config show, get, path and validate", sub)
		}
	}

	switch sub {
	case "init":
		return cmdConfigInit()
	case "setup":
		return cmdConfigSetup(force)
	case "show":
		return cmdConfigShow(jsonOutput)
	case "path":
		return cmdConfigPath(jsonOutput)
	case "get":
		if len(rest) == 0 {
			return fmt.Errorf("config get requires a key.\n\nUsage: nav-pilot config get <key>\n\nKnown keys: %s", knownKeyNames())
		}
		key, err := userKey(rest[0])
		if err != nil {
			return err
		}
		return cmdConfigGet(key, jsonOutput)
	case "set":
		if len(rest) < 2 {
			return fmt.Errorf("config set requires a key and value.\n\nUsage: nav-pilot config set <key> <value>")
		}
		if len(rest) > 2 {
			return fmt.Errorf("config set takes one value, got %d; quote values with spaces\n\nUsage: nav-pilot config set <key> <value>", len(rest)-1)
		}
		key, err := userKey(rest[0])
		if err != nil {
			return err
		}
		return cmdConfigSet(key, rest[1])
	case "unset":
		if len(rest) != 1 {
			return fmt.Errorf("config unset takes one key.\n\nUsage: nav-pilot config unset <key>")
		}
		key, err := userKey(rest[0])
		if err != nil {
			return err
		}
		return cmdConfigUnset(key)
	case "validate":
		return cmdConfigValidate(jsonOutput)
	case "explain":
		key := ""
		if len(rest) > 0 {
			var err error
			if strings.HasPrefix(rest[0], "rtk_") {
				return fmt.Errorf("unknown key: %q\n\nKnown keys: %s", rest[0], knownKeyNames())
			}
			if key, err = userKey(rest[0]); err != nil {
				return err
			}
		}
		return cmdConfigExplain(key)
	case "sandbox":
		return cmdConfigSandbox()
	default:
		return fmt.Errorf("unknown config subcommand: %q\n\nSubcommands: init, setup, show, path, get, set, unset, validate, explain, sandbox", sub)
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

	if err := writeConfigFile(path, []byte(configInitTemplate), nil); err != nil {
		return err
	}

	fmt.Printf("%s Created %s\n", green("✓"), path)
	fmt.Printf("  Edit the file or use %s to set individual options.\n", bold("nav-pilot config set"))
	return nil
}

// ─── config show ─────────────────────────────────────────────────────────────

func cmdConfigShow(jsonOutput bool) error {
	cfg, problems, err := loadConfig()
	if err != nil {
		return err
	}
	warnConfigProblems(problems)
	resolved := resolve(cfg, CLIOverrides{})

	if jsonOutput {
		out := map[string]any{}
		for _, kd := range configKeyDefs {
			out[kd.name] = map[string]any{
				"value":  configJSONValue(resolved, &kd),
				"origin": configKeyOrigin(cfg, kd.name),
			}
		}
		return outputJSON(out)
	}

	path := configPath()
	if cfg == nil {
		fmt.Printf("# Config file: %s %s\n\n", path, dim("(not found, using defaults; NAV_PILOT_CONFIG sets another path)"))
	} else {
		fmt.Printf("# Config file: %s\n\n", path)
	}

	for _, key := range configPageKeys {
		val := configKeyValue(resolved, key)
		if val == "" {
			val = "(unset)"
		}
		fmt.Printf("  %-20s = %-20s (%s)\n", key, val, configKeyOrigin(cfg, key))
	}

	return nil
}

// warnConfigProblems is the one line show and get print about a config with
// problems. They still answer: the values they print are what a launch would
// use once the problems are fixed.
func warnConfigProblems(problems []string) {
	if len(problems) == 0 {
		return
	}
	noun := "problems"
	if len(problems) == 1 {
		noun = "problem"
	}
	fmt.Fprintf(os.Stderr, "%s your config has %d %s: %s\n", yellow("⚠"), len(problems), noun, bold("nav-pilot config validate"))
}

// configJSONValue is a key's effective value with its TOML type: a number or
// a boolean rather than its string form.
func configJSONValue(r ResolvedConfig, kd *configKeyDef) any {
	v := configKeyValue(r, kd.name)
	if kd.name == "model" {
		v = resolvedFieldStr(r, "model") // the id, not the picker label
	}
	switch kd.kind {
	case keyKindInt:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	case keyKindBool:
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return v
}

// ─── config path ─────────────────────────────────────────────────────────────

func cmdConfigPath(jsonOutput bool) error {
	path := configPath()
	if jsonOutput {
		_, err := os.Stat(path)
		return outputJSON(map[string]any{"path": path, "exists": err == nil})
	}
	fmt.Println(path)
	return nil
}

// ─── config get ──────────────────────────────────────────────────────────────

func cmdConfigGet(key string, jsonOutput bool) error {
	kd := findKeyDef(key)
	if kd == nil {
		return fmt.Errorf("unknown key: %q\n\nKnown keys: %s", key, knownKeyNames())
	}

	cfg, problems, err := loadConfig()
	if err != nil {
		return err
	}
	warnConfigProblems(problems)
	resolved := resolve(cfg, CLIOverrides{})

	if jsonOutput {
		return outputJSON(map[string]any{
			"key":    key,
			"value":  configJSONValue(resolved, kd),
			"origin": configKeyOrigin(cfg, key),
		})
	}
	v := resolvedFieldStr(resolved, key)
	if env, ok := configEnvOverrides[key]; ok && os.Getenv(env) != "" {
		v = os.Getenv(env)
	}
	if v != "" {
		fmt.Println(v)
	} else {
		// stdout stays empty for $(nav-pilot config get …); the person at
		// the terminal learns why it is.
		def := kd.defaultVal
		if def == "" {
			def = "none"
			if key == "model" {
				def = "none, so the client picks"
			}
		}
		fmt.Fprintf(os.Stderr, "%s\n", dim(fmt.Sprintf("(not set, default: %s)", def)))
	}
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
	case "local_model":
		return r.LocalModel
	case "hook_loop_guard":
		return strconv.FormatBool(r.HookLoopGuard)
	case "hook_redact_secrets":
		return strconv.FormatBool(r.HookRedactSecrets)
	case "hook_redact_fnr":
		return strconv.FormatBool(r.HookRedactFNR)
	case "hook_injection_note":
		return strconv.FormatBool(r.HookInjectionNote)
	case "rtk_prompted_client":
		return r.RtkPromptedClient
	case "rtk_prompted_at":
		return r.RtkPromptedAt
	case "copilot_auth_mode":
		return r.CopilotAuthMode
	}
	return ""
}

// ─── config set ──────────────────────────────────────────────────────────────

func cmdConfigSet(key, value string) error {
	cfg, _, _ := loadConfig()
	// A model the configured client cannot run is refused here rather than
	// at the next launch.
	if key == "model" && strings.TrimSpace(value) != "" {
		if problem, _ := modelAdvice(value, cfgClient(cfg), cfg != nil && cfg.LocalModel != nil, false); problem != "" {
			return fmt.Errorf("%s, nothing was written", problem)
		}
	}
	before := configChangeLabel(cfg, key)
	old, _ := os.ReadFile(configPath())

	if _, err := writeConfigKey(key, value); err != nil {
		return err
	}
	if key == "source" && strings.TrimSpace(value) == "" {
		fmt.Printf("%s source cleared — nav-pilot installs and syncs from %s again.\n",
			green("✓"), bold(defaultSourceRepo))
		return nil
	}
	after, _, _ := loadConfig()
	printConfigChange(key, before, configChangeLabel(after, key), old)
	if key == "model" || key == "client" {
		warnModelAdvice(after)
	}
	if key == "local_model" {
		warnServerServesSomethingElse(value)
	}
	return nil
}

// cmdConfigUnset removes a key from the file so its built-in default applies:
// the command-line form of ctrl+r on the settings page.
func cmdConfigUnset(key string) error {
	if findKeyDef(key) == nil {
		return fmt.Errorf("unknown key: %q\n\nKnown keys: %s", key, knownKeyNames())
	}
	cfg, _, _ := loadConfig()
	before := configChangeLabel(cfg, key)
	old, _ := os.ReadFile(configPath())
	if err := updateConfigKey(key, ""); err != nil {
		return err
	}
	after, _, _ := loadConfig()
	printConfigChange(key, before, configChangeLabel(after, key), old)
	return nil
}

// configChangeLabel is a key's effective value as `set` and `unset` print it,
// with "(default)" when the file does not set it.
func configChangeLabel(cfg *Config, key string) string {
	v := resolvedFieldStr(resolve(cfg, CLIOverrides{}), key)
	if key == "source" && v == "" {
		v = defaultSourceRepo
	}
	switch origin := configKeyOrigin(cfg, key); {
	case v == "" || origin == "unset":
		return "(unset)"
	case origin != "file":
		return v + " (" + origin + ")"
	}
	return v
}

// printConfigChange prints "✓ client: opencode → pi" and, when the file
// changed, where the previous one was kept.
func printConfigChange(key, before, after string, old []byte) {
	fmt.Printf("%s %s: %s → %s\n", green("✓"), key, before, after)
	if cur, err := os.ReadFile(configPath()); err == nil && old != nil && !bytes.Equal(cur, old) {
		fmt.Printf("  %s\n", dim("Previous file kept as "+configPath()+".bak"))
	}
}

// warnModelAdvice prints what modelAdvice says about the configured model,
// after a change to the model or the client it is for.
func warnModelAdvice(cfg *Config) {
	if cfg == nil || cfg.Model == nil {
		return
	}
	problem, advice := modelAdvice(*cfg.Model, cfgClient(cfg), cfg.LocalModel != nil, false)
	for _, w := range []string{problem, advice} {
		if w != "" {
			fmt.Fprintf(os.Stderr, "%s %s\n", yellow("⚠"), w)
		}
	}
}

// effectiveSourceLabel names the source a resolved config selects, spelling out
// the built-in default when no source is persisted.
func effectiveSourceLabel(r ResolvedConfig) string {
	if r.Source == "" {
		return defaultSourceRepo
	}
	return r.Source
}

// writeConfigKey validates a key/value pair and writes it into the config file
// through updateConfigKey (config_write.go). It returns the TOML literal written. Callers own the user-facing message —
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
	if err := updateConfigKey(key, tomlVal); err != nil {
		return "", err
	}
	return tomlVal, nil
}

// validateKeyValue checks that a value is valid for a given key definition.
func validateKeyValue(kd *configKeyDef, value string) error {
	if strings.TrimSpace(value) == "" && kd.name != "source" {
		return fmt.Errorf("%s cannot be set to an empty value. To clear it, run nav-pilot config unset %s", kd.name, kd.name)
	}
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
	// Shape only, never membership: the manifest Lookup answers from is the
	// embedded single-model copy until local is enabled and installed, so a
	// membership check would reject the very id the docs tell people to set.
	// `alpha local init` and `start` fetch the real manifest and are the gate.
	if kd.name == "model" || kd.name == "local_model" {
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
		return tomlString(value), nil
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

func cmdConfigValidate(jsonOutput bool) error {
	path := configPath()

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if jsonOutput {
			return outputJSON(map[string]any{"path": path, "exists": false, "valid": true, "problems": []string{}, "hints": []string{}})
		}
		fmt.Printf("%s No config file found at %s\n", yellow("⚠"), path)
		fmt.Printf("  Run %s to create one, or set NAV_PILOT_CONFIG to use another file.\n", bold("nav-pilot config init"))
		return nil
	}

	cfg, problems, err := loadConfig()
	if err != nil {
		if jsonOutput {
			_ = outputJSON(map[string]any{"path": path, "exists": true, "valid": false, "problems": []string{err.Error()}, "hints": []string{}})
			return fmt.Errorf("config file has invalid TOML syntax")
		}
		fmt.Printf("%s TOML parse error: %v\n", red("✗"), err)
		return fmt.Errorf("config file has invalid TOML syntax")
	}
	hints := configAdvice(cfg, false)

	if jsonOutput {
		if problems == nil {
			problems = []string{}
		}
		if hints == nil {
			hints = []string{}
		}
		if err := outputJSON(map[string]any{"path": path, "exists": true, "valid": len(problems) == 0, "problems": problems, "hints": hints}); err != nil {
			return err
		}
		if len(problems) > 0 {
			return fmt.Errorf("config validation failed")
		}
		return nil
	}

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
	for i, name := range userKeyNames() {
		if i > 0 {
			fmt.Println()
		}
		printKeyExplain(findKeyDef(name), resolved)
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
	} else if kd.name == "local_model" {
		var ids []string
		for _, m := range local.Active().Models {
			ids = append(ids, m.Model)
		}
		if len(ids) > 0 {
			fmt.Printf("    Manifest: %s\n", strings.Join(ids, ", "))
		}
		fmt.Printf("    Allowed:  any id the local-model manifest names; empty means its default\n")
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

// warnServerServesSomethingElse says so when the server that is up is not
// serving the model just chosen.
//
// A running server keeps the weights it loaded. Without this the config and the
// process disagree in silence: every answer still comes from the old model, and
// the only way to notice is to read the status output closely enough to compare
// two model ids.
func warnServerServesSomethingElse(chosen string) {
	st, running, err := local.LoadState()
	if err != nil || !running {
		return
	}
	// The process read the config before this write, so the selection it holds
	// is the previous one. Ask about the value that was just written instead,
	// which is also what resolves an empty value to the manifest default.
	local.SetSelectedModel(chosen)
	want, ok := local.Chosen(local.Active())
	if !ok || want.Model == st.Model {
		return
	}
	fmt.Printf("\n%s The server that is running still serves %s.\n", yellow("⚠"), bold(st.Model))
	fmt.Printf("  Load the one you just chose:\n\n    %s\n\n", bold("nav-pilot alpha local restart"))
}
