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
	path := filepath.Join(targetDir, stateFilePath)
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
	path := filepath.Join(targetDir, stateFilePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
