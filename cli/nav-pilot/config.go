package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds user-specific nav-pilot configuration read from ~/.nav-pilot/config.toml.
// Pointer types are used for optional fields to distinguish "unset" from zero-value,
// enabling correct per-field precedence in resolve().
type Config struct {
	Version         int     `toml:"version"`
	Agent           *string `toml:"agent"`
	Model           *string `toml:"model"`
	Mode            *string `toml:"mode"`
	ReasoningEffort *string `toml:"reasoning_effort"`
	ContextTier     *string `toml:"context_tier"`
	AllowAllTools   *bool   `toml:"allow_all_tools"`
	AskUser         *bool   `toml:"ask_user"`
	LogLevel        *string `toml:"log_level"`
	OtelLogLevel    *string `toml:"otel_log_level"`
}

// ResolvedConfig holds the final configuration after applying precedence:
// CLI flag > file value > built-in default.
type ResolvedConfig struct {
	Agent           string
	Model           string // empty = use agent default
	Mode            string
	ReasoningEffort string // empty = unset
	ContextTier     string // empty = unset
	AllowAllTools   bool
	AskUser         bool
	LogLevel        string // empty = unset
	OtelLogLevel    string // always set; defaults to "none"
}

// CLIOverrides holds optional CLI flag values. Empty string means "not provided via CLI".
type CLIOverrides struct {
	Agent           string
	Model           string
	Mode            string
	ReasoningEffort string
	ContextTier     string
	AllowAllTools   *bool
	AskUser         *bool
	LogLevel        string
	OtelLogLevel    string
}

var (
	validAgents          = []string{"copilot", "opencode", "pi"}
	validModes           = []string{"default", "plan", "autopilot"}
	validReasoningEffort = []string{"none", "low", "medium", "high", "xhigh", "max"}
	validContextTiers    = []string{"default", "long_context"}
	validLogLevels       = []string{"none", "error", "warning", "info", "debug", "all", "default"}
	validOtelLogLevels   = []string{"none", "error", "warning", "warn", "info", "debug", "verbose", "all"}
)

// modelChoice pairs a model id (the --model value) with a human-readable label.
type modelChoice struct {
	ID    string
	Label string
}

// knownCopilotModels lists the models the Copilot CLI commonly offers. It is
// used to populate the first-run wizard picker. The Copilot CLI validates
// --model server-side against the live catalog, so this is a convenience list,
// NOT an exhaustive allowlist — unrecognized-but-well-formed ids remain accepted
// (see validateModelValue) so newly released models keep working.
var knownCopilotModels = []modelChoice{
	{"auto", "Auto (let Copilot pick)"},
	{"claude-sonnet-4.6", "Claude Sonnet 4.6 (default)"},
	{"claude-haiku-4.5", "Claude Haiku 4.5"},
	{"claude-opus-4.8", "Claude Opus 4.8"},
	{"claude-opus-4.6", "Claude Opus 4.6"},
	{"gpt-5.5", "GPT-5.5"},
	{"gpt-5.4", "GPT-5.4"},
	{"gpt-5.3-codex", "GPT-5.3-Codex"},
	{"gpt-5.4-mini", "GPT-5.4 mini"},
	{"gpt-5-mini", "GPT-5 mini"},
	{"gemini-3.1-pro-preview", "Gemini 3.1 Pro (Preview)"},
	{"gemini-3.5-flash", "Gemini 3.5 Flash"},
}

// modelValuePattern restricts model identifiers to a sane character set that
// covers Copilot ids (e.g. "claude-opus-4.8", "gpt-5.5") and opencode
// provider/model ids (e.g. "anthropic/claude-3-5-sonnet").
var modelValuePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

// validateModelValue applies strong format validation to a model identifier.
// The model catalog is dynamic (Copilot validates server-side), so this checks
// shape rather than membership: non-empty, no surrounding/inner whitespace, and
// a restricted character set. This rejects typos and garbage while remaining
// correct as the model catalog evolves.
func validateModelValue(model string) error {
	if strings.TrimSpace(model) != model {
		return fmt.Errorf("model %q must not have leading or trailing whitespace", model)
	}
	if model == "" {
		return errors.New("model must not be empty (omit the key to use the agent default)")
	}
	if !modelValuePattern.MatchString(model) {
		return fmt.Errorf("model %q is not a valid identifier (allowed characters: letters, digits, '.', '_', '-', '/')", model)
	}
	return nil
}

// configPath returns the path to the user config file.
// Honors NAV_PILOT_CONFIG env var if set.
func configPath() string {
	if p := os.Getenv("NAV_PILOT_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".nav-pilot", "config.toml")
}

// readConfig reads and parses the config file at configPath().
// Returns (nil, nil) if the file does not exist (fail-soft).
// Returns an error if the file exists but cannot be parsed.
func readConfig() (*Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &cfg, nil
}

// readConfigWithMeta reads and parses the config file, returning TOML metadata.
// MetaData.Undecoded() is used by cmdConfigValidate to detect unknown keys.
// Returns (nil, zero-meta, nil) if the file does not exist.
func readConfigWithMeta() (*Config, toml.MetaData, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, toml.MetaData{}, nil
		}
		return nil, toml.MetaData{}, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	meta, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return nil, toml.MetaData{}, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &cfg, meta, nil
}

// validateConfigProblems checks semantic correctness of a parsed config and
// returns a list of human-readable problem strings (empty = valid).
// It does NOT check for unknown TOML keys — use MetaData.Undecoded() for that.
func validateConfigProblems(cfg *Config) []string {
	if cfg == nil {
		return nil
	}
	var problems []string

	if cfg.Version != 1 {
		problems = append(problems, fmt.Sprintf("version must be 1 (got %d)", cfg.Version))
	}
	if cfg.Agent != nil && !containsStr(validAgents, *cfg.Agent) {
		problems = append(problems, fmt.Sprintf("agent %q is not valid (allowed: %s)",
			*cfg.Agent, strings.Join(validAgents, ", ")))
	}
	if cfg.Model != nil {
		if err := validateModelValue(*cfg.Model); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if cfg.Mode != nil && !containsStr(validModes, *cfg.Mode) {
		problems = append(problems, fmt.Sprintf("mode %q is not valid (allowed: %s)",
			*cfg.Mode, strings.Join(validModes, ", ")))
	}
	if cfg.ReasoningEffort != nil && !containsStr(validReasoningEffort, *cfg.ReasoningEffort) {
		problems = append(problems, fmt.Sprintf("reasoning_effort %q is not valid (allowed: %s)",
			*cfg.ReasoningEffort, strings.Join(validReasoningEffort, ", ")))
	}
	if cfg.ContextTier != nil && !containsStr(validContextTiers, *cfg.ContextTier) {
		problems = append(problems, fmt.Sprintf("context_tier %q is not valid (allowed: %s)",
			*cfg.ContextTier, strings.Join(validContextTiers, ", ")))
	}
	if cfg.LogLevel != nil && !containsStr(validLogLevels, *cfg.LogLevel) {
		problems = append(problems, fmt.Sprintf("log_level %q is not valid (allowed: %s)",
			*cfg.LogLevel, strings.Join(validLogLevels, ", ")))
	}
	if cfg.OtelLogLevel != nil && !containsStr(validOtelLogLevels, *cfg.OtelLogLevel) {
		problems = append(problems, fmt.Sprintf("otel_log_level %q is not valid (allowed: %s)",
			*cfg.OtelLogLevel, strings.Join(validOtelLogLevels, ", ")))
	}
	return problems
}

// validateConfig checks semantic correctness of a parsed config.
// Returns an error listing all problems found, or nil if valid.
// It does NOT check for unknown TOML keys — use MetaData.Undecoded() for that.
func validateConfig(cfg *Config) error {
	problems := validateConfigProblems(cfg)
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("config validation failed:\n  - %s", strings.Join(problems, "\n  - "))
}

// isKnownCopilotModel reports whether id matches one of the curated Copilot
// model ids (case-insensitive). Used to surface a soft warning for likely typos
// such as "sonnet" (the real id is "claude-sonnet-4.6").
func isKnownCopilotModel(id string) bool {
	for _, m := range knownCopilotModels {
		if strings.EqualFold(m.ID, id) {
			return true
		}
	}
	return false
}

// knownCopilotModelIDs returns the curated Copilot model ids as a
// comma-separated string for use in warning/help messages.
func knownCopilotModelIDs() string {
	ids := make([]string, len(knownCopilotModels))
	for i, m := range knownCopilotModels {
		ids[i] = m.ID
	}
	return strings.Join(ids, ", ")
}

// configAdvisories returns non-fatal warnings for a parsed config: unknown TOML
// keys (likely typos that are silently ignored) and Copilot model ids that are
// well-formed but not in the curated catalog (likely typos like "sonnet").
// These do not block launch — they are printed so the user can fix them.
func configAdvisories(cfg *Config, meta toml.MetaData) []string {
	if cfg == nil {
		return nil
	}
	var warnings []string
	for _, key := range meta.Undecoded() {
		warnings = append(warnings, fmt.Sprintf("unknown config key %q (ignored)", strings.Join(key, ".")))
	}
	if cfg.Model != nil {
		agent := "copilot"
		if cfg.Agent != nil {
			agent = *cfg.Agent
		}
		if agent == "copilot" && validateModelValue(*cfg.Model) == nil && !isKnownCopilotModel(*cfg.Model) {
			warnings = append(warnings, fmt.Sprintf(
				"model %q is not a recognized Copilot model id; it will be sent as-is and may be rejected by the server (known ids: %s)",
				*cfg.Model, knownCopilotModelIDs()))
		}
	}
	return warnings
}

// loadConfigForLaunch reads, validates, and resolves the user config ahead of a
// launch. Hard validation errors (invalid enum values, wrong version, malformed
// model) cause it to refuse with an error so nav-pilot does not start with a
// broken config. Non-fatal advisories (unknown keys, unrecognized model ids)
// are printed to stderr but do not block the launch.
func loadConfigForLaunch(cli CLIOverrides) (ResolvedConfig, error) {
	file, meta, err := readConfigWithMeta()
	if err != nil {
		return ResolvedConfig{}, err
	}
	if err := validateConfig(file); err != nil {
		return ResolvedConfig{}, fmt.Errorf("%w\n\nFix %s or run `nav-pilot config setup`", err, configPath())
	}
	for _, w := range configAdvisories(file, meta) {
		fmt.Fprintf(os.Stderr, "%s %s\n", yellow("⚠"), w)
	}
	return resolve(file, cli), nil
}

// resolve builds a ResolvedConfig from file config and CLI overrides.
// Precedence: CLI flag > file value > built-in default.
func resolve(file *Config, cli CLIOverrides) ResolvedConfig {
	r := ResolvedConfig{
		Agent:        "copilot",
		Mode:         "default",
		AskUser:      true,
		OtelLogLevel: "none",
	}

	// Apply file values.
	if file != nil {
		if file.Agent != nil {
			r.Agent = *file.Agent
		}
		if file.Model != nil {
			r.Model = *file.Model
		}
		if file.Mode != nil {
			r.Mode = *file.Mode
		}
		if file.ReasoningEffort != nil {
			r.ReasoningEffort = *file.ReasoningEffort
		}
		if file.ContextTier != nil {
			r.ContextTier = *file.ContextTier
		}
		if file.AllowAllTools != nil {
			r.AllowAllTools = *file.AllowAllTools
		}
		if file.AskUser != nil {
			r.AskUser = *file.AskUser
		}
		if file.LogLevel != nil {
			r.LogLevel = *file.LogLevel
		}
		if file.OtelLogLevel != nil {
			r.OtelLogLevel = *file.OtelLogLevel
		}
	}

	// Apply CLI overrides (higher precedence than file).
	if cli.Agent != "" {
		r.Agent = cli.Agent
	}
	if cli.Model != "" {
		r.Model = cli.Model
	}
	if cli.Mode != "" {
		r.Mode = cli.Mode
	}
	if cli.ReasoningEffort != "" {
		r.ReasoningEffort = cli.ReasoningEffort
	}
	if cli.ContextTier != "" {
		r.ContextTier = cli.ContextTier
	}
	if cli.AllowAllTools != nil {
		r.AllowAllTools = *cli.AllowAllTools
	}
	if cli.AskUser != nil {
		r.AskUser = *cli.AskUser
	}
	if cli.LogLevel != "" {
		r.LogLevel = cli.LogLevel
	}
	if cli.OtelLogLevel != "" {
		r.OtelLogLevel = cli.OtelLogLevel
	}
	return r
}

// validateOptionalModel is a huh form validator: it accepts a blank value
// (meaning "unset / agent default") and otherwise applies validateModelValue.
func validateOptionalModel(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return validateModelValue(s)
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
