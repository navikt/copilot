package main

import "path/filepath"

// CheckType determines how a search criterion is evaluated.
type CheckType string

const (
	// CheckFile checks for the existence of a single file.
	CheckFile CheckType = "file"
	// CheckDirectory lists entries in a directory and filters by pattern.
	CheckDirectory CheckType = "directory"
)

// SearchCriteria defines what to look for in a repository.
// Add new entries to DefaultCriteria() to track additional AI/Copilot features.
type SearchCriteria struct {
	Category    string    // Unique key, e.g. "agents", "copilot_instructions"
	Description string    // Human-readable description
	TreePath    string    // Git tree path, e.g. ".github/agents"
	CheckType   CheckType // "file" or "directory"
	FilePattern string    // Glob for filtering directory entries, e.g. "*.agent.md"
}

// GraphQLAlias returns a safe alias for use in GraphQL queries.
// GraphQL aliases must start with a letter and contain only [a-zA-Z0-9_].
func (c SearchCriteria) GraphQLAlias() string {
	return sanitizeAlias(c.Category)
}

func sanitizeAlias(s string) string {
	out := make([]byte, 0, len(s))
	for i, b := range []byte(s) {
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' && i > 0 {
			out = append(out, b)
		} else if b == '_' || b == '-' || b == '.' {
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "alias"
	}
	return string(out)
}

// MatchFiles filters filenames against the criterion's FilePattern.
// Returns all names if FilePattern is empty or "*".
func (c SearchCriteria) MatchFiles(names []string) []string {
	if c.FilePattern == "" || c.FilePattern == "*" {
		return names
	}
	var matched []string
	for _, name := range names {
		ok, _ := filepath.Match(c.FilePattern, name)
		if ok {
			matched = append(matched, name)
		}
	}
	return matched
}

// DefaultCriteria returns the standard set of Copilot customization search criteria.
// To track new AI features, append entries here — no other code changes needed.
//
// Documented paths for GitHub Copilot (per https://github.com/navikt/copilot/tree/main/docs):
//   - .github/copilot-instructions.md      Repository-wide instructions (file)
//   - AGENTS.md                            Navigation index for agent workflows (file)
//   - .github/agents/*.agent.md            Custom agent definitions (directory)
//   - .github/instructions/*.instructions.md   File-scoped instructions (directory)
//   - .github/prompts/*.prompt.md          Reusable prompt templates (directory)
//   - .github/skills/*/SKILL.md            Agent skill folders (directory)
//   - .vscode/mcp.json                     MCP server configuration (file)
//   - .github/copilot/*                    Newer Copilot config directory (directory)
//
// Other AI tools (for comparison metrics):
//   - Cursor: .cursorrules, .cursor/rules/*.mdc, .cursorignore
//   - Claude Code: CLAUDE.md, .claude/settings.json
//   - Windsurf: .windsurfrules
func DefaultCriteria() []SearchCriteria {
	return []SearchCriteria{
		{
			Category:    "copilot_instructions",
			Description: "Repository-wide Copilot instructions",
			TreePath:    ".github/copilot-instructions.md",
			CheckType:   CheckFile,
		},
		{
			Category:    "agents_md",
			Description: "AGENTS.md navigation index",
			TreePath:    "AGENTS.md",
			CheckType:   CheckFile,
		},
		{
			Category:    "agents",
			Description: "Custom Copilot agent definitions",
			TreePath:    ".github/agents",
			CheckType:   CheckDirectory,
			FilePattern: "*.agent.md",
		},
		{
			Category:    "instructions",
			Description: "File-scoped Copilot instructions",
			TreePath:    ".github/instructions",
			CheckType:   CheckDirectory,
			FilePattern: "*.instructions.md",
		},
		{
			Category:    "prompts",
			Description: "Reusable prompt templates",
			TreePath:    ".github/prompts",
			CheckType:   CheckDirectory,
			FilePattern: "*.prompt.md",
		},
		{
			Category:    "skills",
			Description: "Agent skill definitions with bundled assets",
			TreePath:    ".github/skills",
			CheckType:   CheckDirectory,
			FilePattern: "*",
		},
		{
			Category:    "mcp_config",
			Description: "MCP server configuration for VS Code",
			TreePath:    ".vscode/mcp.json",
			CheckType:   CheckFile,
		},

		// --- Cursor ---
		{
			Category:    "cursorrules",
			Description: "Cursor AI rules file (legacy root-level)",
			TreePath:    ".cursorrules",
			CheckType:   CheckFile,
		},
		{
			Category:    "cursor_rules_dir",
			Description: "Cursor AI rules directory",
			TreePath:    ".cursor/rules",
			CheckType:   CheckDirectory,
			FilePattern: "*.mdc",
		},
		{
			Category:    "cursorignore",
			Description: "Cursor AI ignore file",
			TreePath:    ".cursorignore",
			CheckType:   CheckFile,
		},

		// --- Claude Code ---
		{
			Category:    "claude_md",
			Description: "Claude Code project instructions",
			TreePath:    "CLAUDE.md",
			CheckType:   CheckFile,
		},
		{
			Category:    "claude_settings",
			Description: "Claude Code project settings",
			TreePath:    ".claude/settings.json",
			CheckType:   CheckFile,
		},

		// --- Windsurf (Codeium) ---
		{
			Category:    "windsurfrules",
			Description: "Windsurf AI rules file",
			TreePath:    ".windsurfrules",
			CheckType:   CheckFile,
		},

		// --- GitHub Copilot (newer directory structure) ---
		{
			Category:    "copilot_dir",
			Description: "GitHub Copilot directory (newer config format)",
			TreePath:    ".github/copilot",
			CheckType:   CheckDirectory,
			FilePattern: "*",
		},
	}
}
