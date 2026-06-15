package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
}

var (
	validAgents          = []string{"copilot", "opencode", "pi"}
	validModes           = []string{"default", "plan", "autopilot"}
	validReasoningEffort = []string{"none", "low", "medium", "high", "xhigh", "max"}
	validContextTiers    = []string{"default", "long_context"}
	validLogLevels       = []string{"none", "error", "warning", "info", "debug", "all", "default"}
)

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

// resolve builds a ResolvedConfig from file config and CLI overrides.
// Precedence: CLI flag > file value > built-in default.
func resolve(file *Config, cli CLIOverrides) ResolvedConfig {
	r := ResolvedConfig{
		Agent:   "copilot",
		Mode:    "default",
		AskUser: true,
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
	return r
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
