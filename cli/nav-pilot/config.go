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
	Client          *string `toml:"client"`
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
	Client          string
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
	Client          string
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
	validClients         = []string{"copilot", "opencode", "pi"}
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

// openCodeDefaultModel is the Nav-curated default opencode model applied when the
// user launches opencode without an explicit model set. Using a concrete default
// ensures Nav's Anthropic API credentials are always exercised rather than letting
// opencode fall back to whatever its own built-in default happens to be.
// Source: https://opencode.ai/docs/models — current Claude Sonnet 4.5 via the
// Anthropic provider (anthropic/claude-sonnet-4-5).
const openCodeDefaultModel = "anthropic/claude-sonnet-4-5"

// knownOpenCodeModels lists Nav-blessed opencode provider/model ids for the
// first-run wizard picker. Like knownCopilotModels this is a convenience list —
// valid but unlisted ids still work via the "Custom" free-text option.
// Sources: https://opencode.ai/docs/models, https://deepwiki.com/sst/opencode/4.4-supported-providers
var knownOpenCodeModels = []modelChoice{
	{openCodeDefaultModel, "Claude Sonnet 4.5 (Nav default)"},
	{"anthropic/claude-opus-4-5", "Claude Opus 4.5"},
	{"anthropic/claude-haiku-4-5", "Claude Haiku 4.5"},
	{"openai/gpt-4o", "GPT-4o"},
	{"google/gemini-2-0-flash", "Gemini 2.0 Flash"},
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

// validateModelForClient validates a model identifier for a specific client.
// For opencode the model must be in provider/model format (exactly one '/'),
// e.g. "anthropic/claude-sonnet-4-5". For other clients (copilot, pi) only
// the generic shape check from validateModelValue is applied.
func validateModelForClient(model, client string) error {
	if err := validateModelValue(model); err != nil {
		return err
	}
	if client == "opencode" && (strings.Count(model, "/") != 1 || strings.HasSuffix(model, "/")) {
		return fmt.Errorf("model %q must be in provider/model format for opencode (e.g. %q)", model, openCodeDefaultModel)
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
// MetaData.Undecoded() is used to detect unknown keys (hard error on launch).
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
	if cfg.Client != nil && !containsStr(validClients, *cfg.Client) {
		problems = append(problems, fmt.Sprintf("client %q is not valid (allowed: %s)",
			*cfg.Client, strings.Join(validClients, ", ")))
	}
	if cfg.Model != nil {
		client := ""
		if cfg.Client != nil {
			client = *cfg.Client
		}
		if err := validateModelForClient(*cfg.Model, client); err != nil {
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

// isKnownOpenCodeModel reports whether id matches one of the curated opencode
// provider/model ids (case-insensitive).
func isKnownOpenCodeModel(id string) bool {
	for _, m := range knownOpenCodeModels {
		if strings.EqualFold(m.ID, id) {
			return true
		}
	}
	return false
}

// knownOpenCodeModelIDs returns the curated opencode model ids as a
// comma-separated string for use in warning/help messages.
func knownOpenCodeModelIDs() string {
	ids := make([]string, len(knownOpenCodeModels))
	for i, m := range knownOpenCodeModels {
		ids[i] = m.ID
	}
	return strings.Join(ids, ", ")
}

// validateOptionalOpenCodeModel is a huh form validator for an opencode model
// text input. Accepts blank (Nav default will be used) or a provider/model id.
func validateOptionalOpenCodeModel(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return validateModelForClient(s, "opencode")
}

// configAdvisories returns non-fatal warnings for a parsed config.
// Flags Copilot model ids that are well-formed but not in the curated catalog,
// and opencode model ids that are valid provider/model shape but not in the
// Nav-curated list (might still work — opencode supports many providers).
// Unknown TOML keys are handled as hard errors in loadConfigForLaunch, not here.
func configAdvisories(cfg *Config, meta toml.MetaData) []string {
	if cfg == nil {
		return nil
	}
	var warnings []string
	if cfg.Model != nil {
		client := "copilot"
		if cfg.Client != nil {
			client = *cfg.Client
		}
		if client == "copilot" && validateModelValue(*cfg.Model) == nil && !isKnownCopilotModel(*cfg.Model) {
			warnings = append(warnings, fmt.Sprintf(
				"model %q is not a recognized Copilot model id; it will be sent as-is and may be rejected by the server (known ids: %s)",
				*cfg.Model, knownCopilotModelIDs()))
		}
		if client == "opencode" && validateModelForClient(*cfg.Model, "opencode") == nil && !isKnownOpenCodeModel(*cfg.Model) {
			warnings = append(warnings, fmt.Sprintf(
				"model %q is not a Nav-curated opencode model id; it will be passed as-is (Nav default: %s, known ids: %s)",
				*cfg.Model, openCodeDefaultModel, knownOpenCodeModelIDs()))
		}
	}
	return warnings
}

// loadConfigForLaunch reads, validates, and resolves the user config ahead of a
// launch. Hard validation errors (unknown keys, invalid enum values, wrong version,
// malformed model) cause it to refuse with an error so nav-pilot does not start
// with a broken config. Non-fatal advisories (unrecognized model ids) are printed
// to stderr but do not block the launch.
func loadConfigForLaunch(cli CLIOverrides) (ResolvedConfig, error) {
	file, meta, err := readConfigWithMeta()
	if err != nil {
		return ResolvedConfig{}, err
	}
	if err := validateConfig(file); err != nil {
		return ResolvedConfig{}, fmt.Errorf("%w\n\nFix %s or run `nav-pilot config setup`", err, configPath())
	}
	// Unknown keys are a hard error: a stray key (e.g. `agent = "..."`) would
	// otherwise be silently ignored, masking intent.
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		var keys []string
		for _, k := range undecoded {
			keys = append(keys, strings.Join(k, "."))
		}
		return ResolvedConfig{}, fmt.Errorf("config has unknown key(s): %s\n\nFix %s or run `nav-pilot config setup`",
			strings.Join(keys, ", "), configPath())
	}
	for _, w := range configAdvisories(file, meta) {
		fmt.Fprintf(os.Stderr, "%s %s\n", yellow("⚠"), w)
	}
	resolved := resolve(file, cli)
	telemetry.RecordConfig(
		resolved.Client,
		resolved.Mode,
		resolved.Model,
		resolved.ReasoningEffort,
		resolved.ContextTier,
		resolved.OtelLogLevel,
		resolved.AllowAllTools,
		resolved.AskUser,
	)
	return resolved, nil
}

// resolve builds a ResolvedConfig from file config and CLI overrides.
// Precedence: CLI flag > file value > built-in default.
func resolve(file *Config, cli CLIOverrides) ResolvedConfig {
	r := ResolvedConfig{
		Client:       "copilot",
		Mode:         "default",
		AskUser:      true,
		OtelLogLevel: "none",
	}

	// Apply file values.
	if file != nil {
		if file.Client != nil {
			r.Client = *file.Client
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
	if cli.Client != "" {
		r.Client = cli.Client
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
