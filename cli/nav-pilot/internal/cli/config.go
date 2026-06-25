package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// validateModelForClient validates a model identifier by delegating to the
// Provider implementation for the given client id. Kept as a free function for
// use in validateConfigProblems and tests.
func validateModelForClient(model, client string) error {
	p, err := providerFor(client)
	if err != nil {
		// Unknown provider: fall back to base shape validation.
		return validateModelValue(model)
	}
	return p.ValidateModel(model)
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
	if cfg.Client != nil && !containsStr(validProviderIDs, *cfg.Client) {
		problems = append(problems, fmt.Sprintf("client %q is not valid (allowed: %s)",
			*cfg.Client, strings.Join(validProviderIDs, ", ")))
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

// configAdvisories returns non-fatal warnings for a parsed config.
// Delegates to each client's ModelAdvisory for client-specific advisory logic.
// Unknown TOML keys are handled as hard errors in loadConfigForLaunch, not here.
func configAdvisories(cfg *Config, meta toml.MetaData) []string {
	if cfg == nil {
		return nil
	}
	if cfg.Model == nil || validateModelValue(*cfg.Model) != nil {
		return nil
	}
	clientID := "copilot"
	if cfg.Client != nil {
		clientID = *cfg.Client
	}
	p, err := providerFor(clientID)
	if err != nil {
		return nil
	}
	if msg := p.ModelAdvisory(*cfg.Model); msg != "" {
		return []string{msg}
	}
	return nil
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
		configModelLabel(resolved.Model),
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
		if file.AutoLaunch != nil {
			r.AutoLaunch = *file.AutoLaunch
		}
		if file.AutoUpdate != nil {
			r.AutoUpdate = *file.AutoUpdate
		}
		if file.LogLevel != nil {
			r.LogLevel = *file.LogLevel
		}
		if file.OtelLogLevel != nil {
			r.OtelLogLevel = *file.OtelLogLevel
		}
		if file.RtkPromptedClient != nil {
			r.RtkPromptedClient = *file.RtkPromptedClient
		}
		if file.RtkPromptedAt != nil {
			r.RtkPromptedAt = *file.RtkPromptedAt
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
	if cli.AutoLaunch != nil {
		r.AutoLaunch = *cli.AutoLaunch
	}
	if cli.LogLevel != "" {
		r.LogLevel = cli.LogLevel
	}
	if cli.OtelLogLevel != "" {
		r.OtelLogLevel = cli.OtelLogLevel
	}
	return r
}
