package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// SourcePath returns the path to read from in the source repo.
//
// Deprecated: use resolveArtifactFile or resolveSkillDir instead.
// Those handle root-level vs .github/ resolution for all artifact types.
func (s *InstallScope) SourcePath(sourceDir string, parts ...string) string {
	return filepath.Join(append([]string{sourceDir, ".github"}, parts...)...)
}

// ─── Artifact resolution (root-level vs .github/) ──────────────────────────
//
// Distribution artifacts may live at root level (matching github/awesome-copilot
// convention) or under .github/ (legacy). Root wins when present.
//
// Skills are special: a directory must contain SKILL.md to be valid.
// Other types check file/directory existence directly.

// resolveArtifactFile finds a file at root/<typeDir>/ or .github/<typeDir>/.
// Root location wins. Returns the absolute path and true if found.
// Used for agents (*.agent.md, *.metadata.json) and instructions (*.instructions.md).
func resolveArtifactFile(sourceDir, typeDir, fileName string) (string, bool) {
	root := filepath.Join(sourceDir, typeDir, fileName)
	if _, err := os.Stat(root); err == nil {
		return root, true
	}
	legacy := filepath.Join(sourceDir, ".github", typeDir, fileName)
	if _, err := os.Stat(legacy); err == nil {
		return legacy, true
	}
	return "", false
}

// resolveArtifactRel returns the source-relative path for a file.
// Checks root/<typeDir>/<fileName> first, falls back to .github/<typeDir>/<fileName>.
func resolveArtifactRel(sourceDir, typeDir, fileName string) (string, bool) {
	if _, err := os.Stat(filepath.Join(sourceDir, typeDir, fileName)); err == nil {
		return filepath.Join(typeDir, fileName), true
	}
	if _, err := os.Stat(filepath.Join(sourceDir, ".github", typeDir, fileName)); err == nil {
		return filepath.Join(".github", typeDir, fileName), true
	}
	return "", false
}

// scanArtifactFiles discovers files matching a suffix across root and .github locations.
// Root wins on name collision. Results sorted by name.
func scanArtifactFiles(sourceDir, typeDir, suffix string) []artifactEntry {
	seen := make(map[string]bool)
	var entries []artifactEntry

	for _, base := range []string{
		filepath.Join(sourceDir, typeDir),
		filepath.Join(sourceDir, ".github", typeDir),
	} {
		files, err := filepath.Glob(filepath.Join(base, "*"+suffix))
		if err != nil {
			continue
		}
		for _, f := range files {
			name := strings.TrimSuffix(filepath.Base(f), suffix)
			if seen[name] {
				continue
			}
			if validateName(name) == nil {
				entries = append(entries, artifactEntry{Name: name, Path: f})
				seen[name] = true
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

// resolvePrompt finds a prompt at root or .github/ location.
// Precedence: root dir > root file > legacy dir > legacy file.
// Returns absolute path, whether it's a directory, and whether it was found.
func resolvePrompt(sourceDir, name string) (string, bool, bool) {
	for _, base := range []string{sourceDir, filepath.Join(sourceDir, ".github")} {
		dir := filepath.Join(base, "prompts", name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, true, true
		}
		file := filepath.Join(base, "prompts", name+".prompt.md")
		if _, err := os.Stat(file); err == nil {
			return file, false, true
		}
	}
	return "", false, false
}

// scanPromptEntries discovers all prompts across root and .github locations.
// Root wins on name collision. Directories take precedence over files.
func scanPromptEntries(sourceDir string) []artifactEntry {
	seen := make(map[string]bool)
	var entries []artifactEntry

	for _, base := range []string{
		filepath.Join(sourceDir, "prompts"),
		filepath.Join(sourceDir, ".github", "prompts"),
	} {
		items, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, item := range items {
			var name string
			isDir := item.IsDir()
			if isDir {
				name = item.Name()
			} else if strings.HasSuffix(item.Name(), ".prompt.md") {
				name = strings.TrimSuffix(item.Name(), ".prompt.md")
			} else {
				continue
			}
			if seen[name] {
				continue
			}
			entries = append(entries, artifactEntry{Name: name, Path: filepath.Join(base, item.Name()), IsDir: isDir})
			seen[name] = true
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

type artifactEntry struct {
	Name  string
	Path  string
	IsDir bool
}

// resolveSkillDir returns the absolute path to a skill directory in the source repo.
// Skills may live at root level (skills/<name>/) for gh skill auto-discovery,
// or under .github/skills/<name>/ (legacy). Root wins only if it contains SKILL.md.
// Returns ("", false) if the skill is not found in either location.
func resolveSkillDir(sourceDir, name string) (string, bool) {
	rootDir := filepath.Join(sourceDir, "skills", name)
	if _, err := os.Stat(filepath.Join(rootDir, "SKILL.md")); err == nil {
		return rootDir, true
	}
	legacyDir := filepath.Join(sourceDir, ".github", "skills", name)
	if _, err := os.Stat(filepath.Join(legacyDir, "SKILL.md")); err == nil {
		return legacyDir, true
	}
	return "", false
}

// resolveSkillRel returns the source-relative path prefix for a skill.
// Returns "skills/<name>" if root-level exists with SKILL.md, else ".github/skills/<name>".
// Returns ("", false) if neither location has a valid skill.
func resolveSkillRel(sourceDir, name string) (string, bool) {
	if _, err := os.Stat(filepath.Join(sourceDir, "skills", name, "SKILL.md")); err == nil {
		return filepath.Join("skills", name), true
	}
	if _, err := os.Stat(filepath.Join(sourceDir, ".github", "skills", name, "SKILL.md")); err == nil {
		return filepath.Join(".github", "skills", name), true
	}
	return "", false
}

// scanSkillDirs discovers all valid skills across both root and legacy locations.
// Root wins when a skill name exists in both. Each returned entry has its absolute
// directory path and name. Results are sorted by name.
func scanSkillDirs(sourceDir string) []skillEntry {
	seen := make(map[string]bool)
	var skills []skillEntry

	for _, base := range []string{
		filepath.Join(sourceDir, "skills"),
		filepath.Join(sourceDir, ".github", "skills"),
	} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || seen[e.Name()] {
				continue
			}
			if _, err := os.Stat(filepath.Join(base, e.Name(), "SKILL.md")); err == nil {
				if validateName(e.Name()) == nil {
					skills = append(skills, skillEntry{Name: e.Name(), Dir: filepath.Join(base, e.Name())})
					seen[e.Name()] = true
				}
			}
		}
	}

	// Sort for deterministic output
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills
}

type skillEntry struct {
	Name string // skill name (directory name)
	Dir  string // absolute path to the skill directory
}

// resolveSourcePath maps a local/state path to the corresponding source repo path.
// Artifacts may live at root level (new convention matching github/awesome-copilot)
// or under .github/ (legacy). This function probes the filesystem per-path.
//
// For skills, validates that the resolved location contains SKILL.md.
// For other types, checks file/directory existence directly.
//
// Mapping rules:
//
//	Repo scope: ".github/agents/x" → "agents/x" if root-level exists, else unchanged
//	Repo scope: ".github/skills/x/" → "skills/x/" if root-level has SKILL.md, else unchanged
//	User scope: "skills/x/" → "skills/x/" if root-level has SKILL.md, else ".github/skills/x/"
//	User scope: "agents/x" → "agents/x" if root-level exists, else ".github/agents/x"
//	User scope: ".github/instructions/x" → "instructions/x" if root exists, else unchanged
func resolveSourcePath(sourceDir, localPath string, isUserScope bool) string {
	sp := localPath

	if isUserScope && !strings.HasPrefix(sp, ".github/") {
		// User scope: local path has no .github/ prefix.
		// For skills, resolve via resolveSkillRel for SKILL.md validation.
		if strings.HasPrefix(sp, "skills/") {
			trimmed := strings.TrimSuffix(strings.TrimPrefix(sp, "skills/"), "/")
			if rel, ok := resolveSkillRel(sourceDir, trimmed); ok {
				if strings.HasSuffix(sp, "/") {
					return rel + "/"
				}
				return rel
			}
		}
		// For agents: check root-level first
		if strings.HasPrefix(sp, "agents/") {
			fileName := strings.TrimPrefix(sp, "agents/")
			if rel, ok := resolveArtifactRel(sourceDir, "agents", fileName); ok {
				return rel
			}
		}
		// Fall back to .github/ prefix, preserving trailing slash
		hasSuffix := strings.HasSuffix(sp, "/")
		result := filepath.Join(".github", sp)
		if hasSuffix && !strings.HasSuffix(result, "/") {
			result += "/"
		}
		return result
	}

	// Paths with .github/ prefix: check if root-level exists (both scopes)
	if strings.HasPrefix(sp, ".github/") {
		rest := strings.TrimPrefix(sp, ".github/")
		hasSuffix := strings.HasSuffix(sp, "/")

		// Skills: validate SKILL.md exists
		if strings.HasPrefix(rest, "skills/") {
			trimmed := strings.TrimSuffix(strings.TrimPrefix(rest, "skills/"), "/")
			if rel, ok := resolveSkillRel(sourceDir, trimmed); ok {
				if hasSuffix {
					return rel + "/"
				}
				return rel
			}
		}

		// Agents, instructions, prompts: check root-level existence
		for _, typeDir := range []string{"agents", "instructions", "prompts"} {
			if strings.HasPrefix(rest, typeDir+"/") {
				fileName := strings.TrimPrefix(rest, typeDir+"/")
				if rel, ok := resolveArtifactRel(sourceDir, typeDir, fileName); ok {
					if hasSuffix && !strings.HasSuffix(rel, "/") {
						return rel + "/"
					}
					return rel
				}
				break
			}
		}
	}

	return sp
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

// ShouldInstallMetadata returns whether agent metadata files should be installed.
// User scope skips metadata since ~/.copilot doesn't support .metadata.json.
func (s *InstallScope) ShouldInstallMetadata() bool {
	return s.Name != "user"
}
