package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
		SupportedTypes: []string{"agent", "skill"},
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
func (s *InstallScope) DstPath(parts ...string) string {
	if s.PathPrefix != "" {
		return filepath.Join(append([]string{s.RootDir, s.PathPrefix}, parts...)...)
	}
	return filepath.Join(append([]string{s.RootDir}, parts...)...)
}

// RelPath returns the relative path for state tracking.
// For repo: .github/agents/name.agent.md
// For user: agents/name.agent.md
func (s *InstallScope) RelPath(parts ...string) string {
	if s.PathPrefix != "" {
		return filepath.Join(append([]string{s.PathPrefix}, parts...)...)
	}
	return filepath.Join(parts...)
}

// SourcePath returns the path to read from in the source repo.
// Source is always .github/ regardless of scope.
func (s *InstallScope) SourcePath(sourceDir string, parts ...string) string {
	return filepath.Join(append([]string{sourceDir, ".github"}, parts...)...)
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

	// User scope: only agents/ and skills/ allowed
	if !strings.HasPrefix(p, "agents/") && !strings.HasPrefix(p, "skills/") {
		return fmt.Errorf("path outside agents/ or skills/ not allowed in user scope: %s", p)
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

// ShouldInstallMetadata returns whether agent metadata files should be installed.
// User scope skips metadata since ~/.copilot doesn't support .metadata.json.
func (s *InstallScope) ShouldInstallMetadata() bool {
	return s.Name != "user"
}
