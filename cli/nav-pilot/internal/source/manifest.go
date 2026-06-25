package source

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Manifest represents a collection manifest.json.
type Manifest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Agents       []string `json:"agents"`
	Skills       []string `json:"skills"`
	Instructions []string `json:"instructions"`
	Prompts      []string `json:"prompts"`
}

// CollectionAll is the collection name used in state files for "install everything".
const CollectionAll = "(all)"

// ValidateManifest checks that a loaded manifest has valid content.
func ValidateManifest(m *Manifest) error {
	if m.Name == "" {
		return fmt.Errorf("manifest has empty name")
	}
	seen := make(map[string]bool)
	for _, list := range []struct {
		kind  string
		names []string
	}{
		{"agent", m.Agents},
		{"skill", m.Skills},
		{"instruction", m.Instructions},
		{"prompt", m.Prompts},
	} {
		for _, name := range list.names {
			if err := ValidateName(name); err != nil {
				return fmt.Errorf("invalid %s in manifest: %w", list.kind, err)
			}
			key := list.kind + ":" + name
			if seen[key] {
				return fmt.Errorf("duplicate %s in manifest: %q", list.kind, name)
			}
			seen[key] = true
		}
	}
	return nil
}

// LoadManifest loads and validates a collection manifest from the source directory.
func LoadManifest(sourceDir, collection string) (*Manifest, error) {
	path := filepath.Join(sourceDir, "collections", collection, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("collection %q not found: %w", collection, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest for %q: %w", collection, err)
	}
	if err := ValidateManifest(&m); err != nil {
		return nil, fmt.Errorf("collection %q: %w", collection, err)
	}
	return &m, nil
}

// ListCollectionDirs returns the names of all collections in the source directory.
func ListCollectionDirs(sourceDir string) ([]string, error) {
	collectionsDir := filepath.Join(sourceDir, "collections")
	entries, err := os.ReadDir(collectionsDir)
	if err != nil {
		return nil, fmt.Errorf("reading collections dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			manifest := filepath.Join(collectionsDir, e.Name(), "manifest.json")
			if _, err := os.Stat(manifest); err == nil {
				names = append(names, e.Name())
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

// CollectAllItems scans the source directory for all agents, skills, and instructions,
// returning a synthetic manifest. Used for user-scope "install everything".
func CollectAllItems(sourceDir string) (*Manifest, error) {
	resolver := NewSourceResolver(sourceDir)
	m := &Manifest{
		Name:        "(all)",
		Description: "All agents, skills, and instructions",
	}
	for _, a := range resolver.List(KindAgent) {
		m.Agents = append(m.Agents, a.Name)
	}
	for _, s := range resolver.List(KindSkill) {
		m.Skills = append(m.Skills, s.Name)
	}
	for _, i := range resolver.List(KindInstruction) {
		m.Instructions = append(m.Instructions, i.Name)
	}
	return m, nil
}

// ValidateName checks that a manifest entry name is safe for use in file paths.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("empty name")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("name %q contains '..'", name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("name %q contains path separator", name)
	}
	if name != filepath.Clean(name) {
		return fmt.Errorf("name %q is not clean", name)
	}
	return nil
}
