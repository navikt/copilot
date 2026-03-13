package main

import (
	"testing"
)

func TestDefaultCriteria(t *testing.T) {
	criteria := DefaultCriteria()

	if len(criteria) == 0 {
		t.Fatal("expected at least one criterion")
	}

	// Check for unique categories
	seen := make(map[string]bool)
	for _, c := range criteria {
		if seen[c.Category] {
			t.Errorf("duplicate category: %s", c.Category)
		}
		seen[c.Category] = true

		if c.TreePath == "" {
			t.Errorf("criterion %s has empty TreePath", c.Category)
		}
		if c.CheckType != CheckFile && c.CheckType != CheckDirectory {
			t.Errorf("criterion %s has invalid CheckType: %s", c.Category, c.CheckType)
		}
	}

	// Verify expected categories exist
	expected := []string{
		// GitHub Copilot
		"copilot_instructions", "agents_md", "agents",
		"instructions", "prompts", "skills", "mcp_config", "copilot_dir",
		// Cursor
		"cursorrules", "cursor_rules_dir", "cursorignore",
		// Claude Code
		"claude_md", "claude_settings",
		// Windsurf
		"windsurfrules",
	}
	for _, e := range expected {
		if !seen[e] {
			t.Errorf("expected category %s not found", e)
		}
	}
}

func TestGraphQLAlias(t *testing.T) {
	tests := []struct {
		category string
		want     string
	}{
		{"copilot_instructions", "copilot_instructions"},
		{"agents_md", "agents_md"},
		{"mcp_config", "mcp_config"},
		{"agents", "agents"},
	}

	for _, tt := range tests {
		c := SearchCriteria{Category: tt.category}
		got := c.GraphQLAlias()
		if got != tt.want {
			t.Errorf("GraphQLAlias(%q) = %q, want %q", tt.category, got, tt.want)
		}
	}
}

func TestMatchFiles(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		files   []string
		want    int
	}{
		{
			name:    "match agent files",
			pattern: "*.agent.md",
			files:   []string{"auth.agent.md", "nais-platform.agent.md", "README.md", "test.txt"},
			want:    2,
		},
		{
			name:    "match instruction files",
			pattern: "*.instructions.md",
			files:   []string{"kotlin-ktor.instructions.md", "README.md"},
			want:    1,
		},
		{
			name:    "match all with wildcard",
			pattern: "*",
			files:   []string{"aksel-spacing", "security-review", "flyway-migration"},
			want:    3,
		},
		{
			name:    "match all with empty pattern",
			pattern: "",
			files:   []string{"a", "b", "c"},
			want:    3,
		},
		{
			name:    "no matches",
			pattern: "*.agent.md",
			files:   []string{"README.md", "test.txt"},
			want:    0,
		},
		{
			name:    "empty file list",
			pattern: "*.agent.md",
			files:   []string{},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := SearchCriteria{FilePattern: tt.pattern}
			got := c.MatchFiles(tt.files)
			if len(got) != tt.want {
				t.Errorf("MatchFiles() returned %d files, want %d (got: %v)", len(got), tt.want, got)
			}
		})
	}
}
