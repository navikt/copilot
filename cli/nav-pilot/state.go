package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// readState reads state for the given repo directory (legacy convenience wrapper).
func readState(targetDir string) (*StateFile, error) {
	return readScopedState(ScopeRepo(targetDir))
}

// readScopedState reads state from the scope's state file location.
// Validates that the persisted scope matches the expected scope and that
// all file paths are safe. This is the single entry point for state validation.
func readScopedState(scope *InstallScope) (*StateFile, error) {
	s, err := readStateRaw(scope.StatePath())
	if err != nil || s == nil {
		return s, err
	}

	// Reject scope mismatch (empty defaults to "repo" for backwards compat)
	fileScope := s.Scope
	if fileScope == "" {
		fileScope = "repo"
	}
	if fileScope != scope.Name {
		return nil, fmt.Errorf("state file scope mismatch: expected %q, got %q", scope.Name, fileScope)
	}

	// B1: Validate all file paths using the scope's single validation implementation
	for _, f := range s.Files {
		if err := scope.ValidateStatePath(f.Path); err != nil {
			return nil, fmt.Errorf("unsafe state file: %w", err)
		}
	}
	return s, nil
}

// readStateRaw parses a state file without validation. Used internally.
func readStateRaw(path string) (*StateFile, error) {
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
	return &s, nil
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
	// B2: Refuse to write through symlinks (file or parent directory)
	if err := checkSymlink(path); err != nil {
		return err
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
