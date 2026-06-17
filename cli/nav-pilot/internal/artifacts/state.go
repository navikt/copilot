package artifacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/navikt/copilot/cli/nav-pilot/internal/domain"
	"github.com/navikt/copilot/cli/nav-pilot/internal/source"
)

const StateFilePath = ".github/.nav-pilot-state.json"

// ReadState reads state for the given repo directory (legacy convenience wrapper).
func ReadState(targetDir string) (*domain.StateFile, error) {
	return ReadScopedState(domain.ScopeRepo(targetDir))
}

// ReadScopedState reads state from the scope's state file location.
// Validates that the persisted scope matches the expected scope and that
// all file paths are safe. This is the single entry point for state validation.
func ReadScopedState(scope *domain.InstallScope) (*domain.StateFile, error) {
	s, err := ReadStateRaw(scope.StatePath())
	if err != nil || s == nil {
		return s, err
	}

	fileScope := s.Scope
	if fileScope == "" {
		fileScope = "repo"
	}
	if fileScope != scope.Name {
		return nil, fmt.Errorf("state file scope mismatch: expected %q, got %q", scope.Name, fileScope)
	}

	for _, f := range s.Files {
		if err := scope.ValidateStatePath(f.Path); err != nil {
			return nil, fmt.Errorf("unsafe state file: %w", err)
		}
	}
	return s, nil
}

// ReadStateRaw parses a state file without validation. Used internally.
func ReadStateRaw(path string) (*domain.StateFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s domain.StateFile
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}
	return &s, nil
}

func WriteState(targetDir string, state *domain.StateFile) error {
	return WriteStateAt(filepath.Join(targetDir, StateFilePath), targetDir, state)
}

// WriteScopedState writes state to the scope's state file location.
func WriteScopedState(scope *domain.InstallScope, state *domain.StateFile) error {
	return WriteStateAt(scope.StatePath(), scope.RootDir, state)
}

func WriteStateAt(path, boundary string, state *domain.StateFile) error {
	if err := source.CheckSymlink(path, boundary); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
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
