package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The broker spec file: where cplt reads it, and its write and remove.

// mcpBrokerDir is $XDG_CONFIG_HOME/cplt/mcp, where cplt reads specs.
func mcpBrokerDir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		return filepath.Join(x, "cplt", "mcp"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "cplt", "mcp"), nil
}

// mcpBrokerSpecPath is the spec file for a server id, as cplt names it.
func mcpBrokerSpecPath(id string) (string, error) {
	dir, err := mcpBrokerDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, strings.ReplaceAll(id, "/", "__")+".json"), nil
}

// mcpBrokerSafePath is the spec path for a write or a remove: refused when
// it, or any directory between it and the config base, is a symlink, so
// neither lands in a directory the link points at.
func mcpBrokerSafePath(id string) (string, error) {
	path, err := mcpBrokerSpecPath(id)
	if err != nil {
		return "", err
	}
	base := filepath.Dir(filepath.Dir(filepath.Dir(path))) // the config base
	if err := checkSymlink(path, base); err != nil {
		return "", fmt.Errorf("%w, so nav-pilot did not touch it. Fix: replace the link under %s with a directory", err, base)
	}
	return path, nil
}

// mcpBrokerEnabled is whether a spec file for id is there.
func mcpBrokerEnabled(id string) bool {
	p, err := mcpBrokerSpecPath(id)
	if err != nil {
		return false
	}
	_, err = os.Lstat(p)
	return err == nil
}

// mcpWriteBrokerSpec writes the spec for id, mode 0600, and returns its path.
func mcpWriteBrokerSpec(id string, data []byte) (string, error) {
	path, err := mcpBrokerSafePath(id)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("could not create %s: %w", filepath.Dir(path), err)
	}
	if path, err = mcpBrokerSafePath(id); err != nil { // MkdirAll follows links
		return "", err
	}
	// Through a temp file and a rename, so cplt never reads half a spec.
	if err := writeFileAtomic(path, append(data, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("could not write %s: %w", path, err)
	}
	return path, nil
}

// mcpBrokerAllowed is how many tools the spec on disk allows, or -1.
func mcpBrokerAllowed(id string) int {
	p, err := mcpBrokerSpecPath(id)
	if err != nil {
		return -1
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return -1
	}
	var spec struct {
		Meta map[string]struct {
			Tools struct {
				Allow []string `json:"allow"`
			} `json:"tools"`
		} `json:"_meta"`
	}
	if json.Unmarshal(data, &spec) != nil {
		return -1
	}
	return len(spec.Meta[mcpBrokerKey].Tools.Allow)
}

// mcpDisableBroker removes id's spec; false when there was none.
func mcpDisableBroker(id string) (bool, error) {
	if !mcpBrokerEnabled(id) {
		return false, nil
	}
	path, err := mcpBrokerSafePath(id)
	if err != nil {
		return false, err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	fmt.Printf("%s Disabled %s: removed %s\n", green("✓"), bold(id), path)
	return true, nil
}
