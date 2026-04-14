package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const stateFilePath = ".github/.nav-pilot-state.json"

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
	Path string `json:"path"`
	Hash string `json:"hash"`
}

func readState(targetDir string) (*StateFile, error) {
	return readStateAt(filepath.Join(targetDir, stateFilePath))
}

// readScopedState reads state from the scope's state file location.
func readScopedState(scope *InstallScope) (*StateFile, error) {
	return readStateAt(scope.StatePath())
}

func readStateAt(path string) (*StateFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s StateFile
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	// Determine scope for validation (default to "repo" for backwards compat)
	scopeName := s.Scope
	if scopeName == "" {
		scopeName = "repo"
	}

	// B1: Validate all file paths to prevent path traversal attacks
	for _, f := range s.Files {
		var validationErr error
		if scopeName == "user" {
			validationErr = validateUserStatePath(f.Path)
		} else {
			validationErr = validateStatePath(f.Path)
		}
		if validationErr != nil {
			return nil, fmt.Errorf("unsafe state file: %w", validationErr)
		}
	}
	return &s, nil
}

// validateStatePath ensures a path from the state file is safe to use.
// Rejects absolute paths, path traversal, and paths outside .github/.
func validateStatePath(p string) error {
	if filepath.IsAbs(p) {
		return fmt.Errorf("absolute path not allowed: %s", p)
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("path traversal not allowed: %s", p)
	}
	if !strings.HasPrefix(p, ".github/") {
		return fmt.Errorf("path outside .github/ not allowed: %s", p)
	}
	return nil
}

// validateUserStatePath ensures a path from a user-scope state file is safe.
// Only allows agents/ and skills/ prefixes.
func validateUserStatePath(p string) error {
	if filepath.IsAbs(p) {
		return fmt.Errorf("absolute path not allowed: %s", p)
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("path traversal not allowed: %s", p)
	}
	if !strings.HasPrefix(p, "agents/") && !strings.HasPrefix(p, "skills/") {
		return fmt.Errorf("path outside agents/ or skills/ not allowed in user scope: %s", p)
	}
	return nil
}

func writeState(targetDir string, state *StateFile) error {
	return writeStateAt(filepath.Join(targetDir, stateFilePath), state)
}

// writeScopedState writes state to the scope's state file location.
func writeScopedState(scope *InstallScope, state *StateFile) error {
	return writeStateAt(scope.StatePath(), state)
}

func writeStateAt(path string, state *StateFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// B2: Refuse to overwrite symlinks
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlink: %s", path)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	// I4: Atomic write via temp file + rename
	tmp, err := os.CreateTemp(filepath.Dir(path), ".nav-pilot-state-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
