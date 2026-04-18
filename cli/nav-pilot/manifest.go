package main

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

// validateManifest checks that a loaded manifest has valid content.
func validateManifest(m *Manifest) error {
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
			if err := validateName(name); err != nil {
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

func loadManifest(sourceDir, collection string) (*Manifest, error) {
	path := filepath.Join(sourceDir, ".github", "collections", collection, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("collection %q not found: %w", collection, err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest for %q: %w", collection, err)
	}
	if err := validateManifest(&m); err != nil {
		return nil, fmt.Errorf("collection %q: %w", collection, err)
	}
	return &m, nil
}

func listCollectionDirs(sourceDir string) ([]string, error) {
	collectionsDir := filepath.Join(sourceDir, ".github", "collections")
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

// collectAllItems scans the source directory for all agents, skills, and instructions,
// returning a synthetic manifest. Used for user-scope "install everything".
func collectAllItems(sourceDir string) (*Manifest, error) {
	m := &Manifest{
		Name:        "(all)",
		Description: "All agents, skills, and instructions",
	}

	// Scan agents
	agentFiles, err := filepath.Glob(filepath.Join(sourceDir, ".github", "agents", "*.agent.md"))
	if err != nil {
		return nil, fmt.Errorf("scanning agents: %w", err)
	}
	for _, f := range agentFiles {
		name := strings.TrimSuffix(filepath.Base(f), ".agent.md")
		if validateName(name) == nil {
			m.Agents = append(m.Agents, name)
		}
	}
	sort.Strings(m.Agents)

	// Scan skills — check both root-level (gh skill convention) and .github/skills/ (legacy)
	seen := make(map[string]bool)
	for _, skillsDir := range []string{
		filepath.Join(sourceDir, "skills"),
		filepath.Join(sourceDir, ".github", "skills"),
	} {
		entries, err := os.ReadDir(skillsDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || seen[e.Name()] {
				continue
			}
			skillFile := filepath.Join(skillsDir, e.Name(), "SKILL.md")
			if _, statErr := os.Stat(skillFile); statErr == nil {
				if validateName(e.Name()) == nil {
					m.Skills = append(m.Skills, e.Name())
					seen[e.Name()] = true
				}
			}
		}
	}
	sort.Strings(m.Skills)

	// Scan instructions
	instrFiles, err := filepath.Glob(filepath.Join(sourceDir, ".github", "instructions", "*.instructions.md"))
	if err != nil {
		return nil, fmt.Errorf("scanning instructions: %w", err)
	}
	for _, f := range instrFiles {
		name := strings.TrimSuffix(filepath.Base(f), ".instructions.md")
		if validateName(name) == nil {
			m.Instructions = append(m.Instructions, name)
		}
	}
	sort.Strings(m.Instructions)

	return m, nil
}

// CollectionAll is the collection name used in state files for "install everything".
const CollectionAll = "(all)"

// validateName checks that a manifest entry name is safe for use in file paths.
func validateName(name string) error {
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
