package domain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Config holds user-specific nav-pilot configuration read from ~/.nav-pilot/config.toml.
// Pointer types are used for optional fields to distinguish "unset" from zero-value,
// enabling correct per-field precedence in resolve().
type Config struct {
	Version           int     `toml:"version"`
	Client            *string `toml:"client"`
	Model             *string `toml:"model"`
	Mode              *string `toml:"mode"`
	ReasoningEffort   *string `toml:"reasoning_effort"`
	ContextTier       *string `toml:"context_tier"`
	AllowAllTools     *bool   `toml:"allow_all_tools"`
	AskUser           *bool   `toml:"ask_user"`
	AutoLaunch        *bool   `toml:"auto_launch"`
	LogLevel          *string `toml:"log_level"`
	OtelLogLevel      *string `toml:"otel_log_level"`
	RtkPromptedClient *string `toml:"rtk_prompted_client"`
	RtkPromptedAt     *string `toml:"rtk_prompted_at"`
	AutoUpdate        *bool   `toml:"auto_update"`
}

// ResolvedConfig holds the final configuration after applying precedence:
// CLI flag > file value > built-in default.
type ResolvedConfig struct {
	Client            string
	Model             string // empty = use agent default
	Mode              string
	ReasoningEffort   string // empty = unset
	ContextTier       string // empty = unset
	AllowAllTools     bool
	AskUser           bool
	AutoLaunch        bool     // skip the interactive "Launch X now?" confirmation
	LogLevel          string   // empty = unset
	OtelLogLevel      string   // always set; defaults to "none"
	RtkPromptedClient string   // comma-separated list of clients where the RTK setup was prompted
	RtkPromptedAt     string   // RFC3339 timestamp of when the user was last prompted
	AutoUpdate        bool     // true to bypass upgrade prompt
	ExtraArgs         []string // pass-through arguments for the client
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
	AutoLaunch      *bool
	LogLevel        string
	OtelLogLevel    string
	ExtraArgs       []string
}

var (
	ValidModes           = []string{"default", "plan", "autopilot"}
	ValidReasoningEffort = []string{"none", "low", "medium", "high", "xhigh", "max"}
	ValidContextTiers    = []string{"default", "long_context"}
	ValidLogLevels       = []string{"none", "error", "warning", "info", "debug", "all", "default"}
	ValidOtelLogLevels   = []string{"none", "error", "warning", "warn", "info", "debug", "verbose", "all"}
)

// ModelChoice pairs a model id (the --model value) with a human-readable label.
// The concrete lists (knownCopilotModels, knownOpenCodeModels) live in provider.go.
type ModelChoice struct {
	ID    string
	Label string
}

// ModelValuePattern restricts model identifiers to a sane character set that
// covers Copilot ids (e.g. "claude-opus-4.8", "gpt-5.5") and opencode
// provider/model ids (e.g. "anthropic/claude-3-5-sonnet").
var ModelValuePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

// ValidateModelValue applies strong format validation to a model identifier.
// The model catalog is dynamic (Copilot validates server-side), so this checks
// shape rather than membership: non-empty, no surrounding/inner whitespace, and
// a restricted character set. This rejects typos and garbage while remaining
// correct as the model catalog evolves.
func ValidateModelValue(model string) error {
	if strings.TrimSpace(model) != model {
		return fmt.Errorf("model %q must not have leading or trailing whitespace", model)
	}
	if model == "" {
		return errors.New("model must not be empty (omit the key to use the agent default)")
	}
	if !ModelValuePattern.MatchString(model) {
		return fmt.Errorf("model %q is not a valid identifier (allowed characters: letters, digits, '.', '_', '-', '/')", model)
	}
	return nil
}

// ValidateOptionalModel is a huh form validator: it accepts a blank value
// (meaning "unset / agent default") and otherwise applies ValidateModelValue.
func ValidateOptionalModel(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return ValidateModelValue(s)
}

func ContainsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// InstallScope encapsulates the differences between repo-level and user-level installs.
type InstallScope struct {
	Name           string   // "repo" or "user"
	RootDir        string   // git root (repo) or ~/.copilot (user)
	StateFile      string   // path relative to RootDir
	PathPrefix     string   // ".github/" (repo) or "" (user)
	SupportedTypes []string // artifact types that can be installed
}

// ScopeRepo creates a scope for repo-level installs (.github/).
func ScopeRepo(targetDir string) *InstallScope {
	return &InstallScope{
		Name:           "repo",
		RootDir:        targetDir,
		StateFile:      ".github/.nav-pilot-state.json",
		PathPrefix:     ".github/",
		SupportedTypes: []string{"agent", "skill", "instruction", "prompt"},
	}
}

// ScopeUser creates a scope for user-level installs (~/.copilot/).
func ScopeUser() (*InstallScope, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	rootDir := filepath.Join(home, ".copilot")
	return &InstallScope{
		Name:           "user",
		RootDir:        rootDir,
		StateFile:      ".nav-pilot-state.json",
		PathPrefix:     "",
		SupportedTypes: []string{"agent", "skill", "instruction"},
	}, nil
}

// SupportsType returns true if this scope supports the given artifact type.
func (s *InstallScope) SupportsType(itemType string) bool {
	for _, t := range s.SupportedTypes {
		if t == itemType {
			return true
		}
	}
	return false
}

// DstPath returns the full destination path for an artifact.
// For repo: <rootDir>/.github/agents/name.agent.md
// For user: <rootDir>/agents/name.agent.md
// For user instructions: <rootDir>/.github/instructions/name.instructions.md
//
//	(cplt requires .github/instructions/ inside COPILOT_CUSTOM_INSTRUCTIONS_DIRS)
func (s *InstallScope) DstPath(parts ...string) string {
	if s.PathPrefix != "" {
		return filepath.Join(append([]string{s.RootDir, s.PathPrefix}, parts...)...)
	}
	if s.needsGitHubPrefix(parts) {
		return filepath.Join(append([]string{s.RootDir, ".github"}, parts...)...)
	}
	return filepath.Join(append([]string{s.RootDir}, parts...)...)
}

// RelPath returns the relative path for state tracking.
// For repo: .github/agents/name.agent.md
// For user: agents/name.agent.md
// For user instructions: .github/instructions/name.instructions.md
func (s *InstallScope) RelPath(parts ...string) string {
	if s.PathPrefix != "" {
		return filepath.Join(append([]string{s.PathPrefix}, parts...)...)
	}
	if s.needsGitHubPrefix(parts) {
		return filepath.Join(append([]string{".github"}, parts...)...)
	}
	return filepath.Join(parts...)
}

// needsGitHubPrefix returns true when user-scope artifacts require a .github/ prefix.
// Instructions need this because COPILOT_CUSTOM_INSTRUCTIONS_DIRS expects
// .github/instructions/**/*.instructions.md inside the directory.
func (s *InstallScope) needsGitHubPrefix(parts []string) bool {
	return s.Name == "user" && len(parts) > 0 && parts[0] == "instructions"
}

// StatePath returns the full path to the state file.
func (s *InstallScope) StatePath() string {
	return filepath.Join(s.RootDir, s.StateFile)
}

// ValidateStatePath checks that a path from the state file is safe for this scope.
func (s *InstallScope) ValidateStatePath(p string) error {
	// Normalize to forward slashes so checks work on all platforms.
	p = filepath.ToSlash(p)

	if filepath.IsAbs(p) {
		return fmt.Errorf("absolute path not allowed: %s", p)
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("path traversal not allowed: %s", p)
	}

	if s.Name == "repo" {
		if !strings.HasPrefix(p, ".github/") {
			return fmt.Errorf("path outside .github/ not allowed in repo scope: %s", p)
		}
		return nil
	}

	// User scope: agents/, skills/, and .github/instructions/ allowed
	if !strings.HasPrefix(p, "agents/") && !strings.HasPrefix(p, "skills/") && !strings.HasPrefix(p, ".github/instructions/") {
		return fmt.Errorf("path outside agents/, skills/, or .github/instructions/ not allowed in user scope: %s", p)
	}
	return nil
}

// CleanupDirs removes empty artifact directories after uninstall.
func (s *InstallScope) CleanupDirs() {
	if s.Name == "repo" {
		for _, sub := range []string{"agents", "skills", "instructions", "prompts"} {
			dir := filepath.Join(s.RootDir, ".github", sub)
			entries, err := os.ReadDir(dir)
			if err == nil && len(entries) == 0 {
				os.Remove(dir)
			}
		}
		return
	}
	// User scope
	for _, sub := range []string{"agents", "skills"} {
		dir := filepath.Join(s.RootDir, sub)
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			os.Remove(dir)
		}
	}
	// Instructions live under .github/instructions/ in user scope
	instrDir := filepath.Join(s.RootDir, ".github", "instructions")
	if entries, err := os.ReadDir(instrDir); err == nil && len(entries) == 0 {
		os.Remove(instrDir)
		// Remove .github/ if now empty too
		if entries, err := os.ReadDir(filepath.Join(s.RootDir, ".github")); err == nil && len(entries) == 0 {
			os.Remove(filepath.Join(s.RootDir, ".github"))
		}
	}
}

// Label returns a display label for UI output.
func (s *InstallScope) Label() string {
	if s.Name == "user" {
		return "~/.copilot (user-wide)"
	}
	return s.RootDir
}

// IsUser returns true for user-scope installs.
func (s *InstallScope) IsUser() bool {
	return s.Name == "user"
}

// StateFile tracks what was installed, for safe updates and uninstall.
type StateFile struct {
	Collection  string          `json:"collection"`
	Version     string          `json:"version"`
	Scope       string          `json:"scope,omitempty"` // "repo" or "user"; empty means "repo" (backwards compat)
	SourceSHA   string          `json:"source_sha"`
	InstalledAt string          `json:"installed_at"`
	Files       []InstalledFile `json:"files"`
}

// InstalledFile records a single installed file with its content hash.
type InstalledFile struct {
	Path   string `json:"path"`
	Hash   string `json:"hash"`
	Status string `json:"status,omitempty"` // "" = active, FileStatusIgnored = intentionally excluded, FileStatusConflict = exists with local modifications
}

// FileStatusIgnored marks a file as intentionally excluded by the user.
// Sync and status skip files with this status.
const FileStatusIgnored = "ignored"

// FileStatusConflict marks a file that existed with local modifications at install time.
// The user declined to overwrite it, so sync should not touch it until resolved.
const FileStatusConflict = "conflict"

var UseColor = true

func init() {
	if os.Getenv("NO_COLOR") != "" {
		UseColor = false
	}
}

func Color(code, msg string) string {
	if !UseColor {
		return msg
	}
	return fmt.Sprintf("\033[%sm%s\033[0m", code, msg)
}

func Red(msg string) string    { return Color("31", msg) }
func Green(msg string) string  { return Color("32", msg) }
func Yellow(msg string) string { return Color("33", msg) }
func Dim(msg string) string    { return Color("2", msg) }
func Bold(msg string) string   { return Color("1", msg) }
