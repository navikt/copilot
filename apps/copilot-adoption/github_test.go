package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildGraphQLQuery(t *testing.T) {
	repos := []RepoInfo{
		{Name: "repo-a", DefaultBranch: "main"},
		{Name: "repo-b", DefaultBranch: "master"},
	}
	criteria := []SearchCriteria{
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: CheckFile},
		{Category: "agents", TreePath: ".github/agents", CheckType: CheckDirectory, FilePattern: "*.agent.md"},
	}

	query := buildGraphQLQuery("navikt", repos, criteria)

	// Verify query contains expected fragments
	expectations := []string{
		`repo0: repository(owner: "navikt", name: "repo-a")`,
		`repo1: repository(owner: "navikt", name: "repo-b")`,
		`copilot_instructions: object(expression: "main:.github/copilot-instructions.md")`,
		`agents: object(expression: "main:.github/agents")`,
		`agents: object(expression: "master:.github/agents")`,
		`... on Tree { entries { name type } }`,
		`__typename`,
	}

	for _, exp := range expectations {
		if !contains(query, exp) {
			t.Errorf("query missing expected fragment: %q\nquery:\n%s", exp, query)
		}
	}
}

func TestBuildGraphQLQueryDirectoryIncludesTypename(t *testing.T) {
	// Regression test: directory checks MUST include __typename to enable proper parsing.
	// Without __typename, parseGraphQLResponse() can't verify the object is a Tree.
	repos := []RepoInfo{
		{Name: "test-repo", DefaultBranch: "main"},
	}
	criteria := []SearchCriteria{
		{Category: "agents", TreePath: ".github/agents", CheckType: CheckDirectory, FilePattern: "*.agent.md"},
		{Category: "instructions", TreePath: ".github/instructions", CheckType: CheckDirectory, FilePattern: "*.instructions.md"},
		{Category: "prompts", TreePath: ".github/prompts", CheckType: CheckDirectory, FilePattern: "*.prompt.md"},
		{Category: "skills", TreePath: ".github/skills", CheckType: CheckDirectory, FilePattern: "*"},
	}

	query := buildGraphQLQuery("navikt", repos, criteria)

	// Each directory check should have __typename before the Tree fragment
	for _, c := range criteria {
		// The query should contain the directory object expression followed by __typename
		objectExpr := c.Category + `: object(expression: "main:` + c.TreePath + `")`
		if !contains(query, objectExpr) {
			t.Errorf("query missing directory object expression for %s", c.Category)
		}
	}

	// Count occurrences of __typename - should be one per directory check
	count := 0
	for i := 0; i <= len(query)-len("__typename"); i++ {
		if query[i:i+len("__typename")] == "__typename" {
			count++
		}
	}
	if count != len(criteria) {
		t.Errorf("expected %d __typename occurrences for directory checks, got %d", len(criteria), count)
	}
}

func TestBuildGraphQLQueryEmptyBranch(t *testing.T) {
	repos := []RepoInfo{
		{Name: "repo-no-branch", DefaultBranch: ""},
	}
	criteria := []SearchCriteria{
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: CheckFile},
	}

	query := buildGraphQLQuery("navikt", repos, criteria)

	if !contains(query, `expression: "HEAD:.github/copilot-instructions.md"`) {
		t.Errorf("expected HEAD fallback for empty branch, query:\n%s", query)
	}
}

func TestBuildGraphQLQueryFileCheckFormat(t *testing.T) {
	// File checks use inline __typename, directory checks use multi-line format
	repos := []RepoInfo{
		{Name: "test-repo", DefaultBranch: "main"},
	}
	fileOnlyCriteria := []SearchCriteria{
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: CheckFile},
		{Category: "agents_md", TreePath: "AGENTS.md", CheckType: CheckFile},
	}

	query := buildGraphQLQuery("navikt", repos, fileOnlyCriteria)

	// File checks should use simple inline format: { __typename }
	if !contains(query, `{ __typename }`) {
		t.Errorf("file checks should use { __typename }, query:\n%s", query)
	}
	// File checks should NOT have Tree entries
	if contains(query, "entries") {
		t.Error("file-only query should not contain entries")
	}
}

func TestParseGraphQLResponse(t *testing.T) {
	criteria := []SearchCriteria{
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: CheckFile},
		{Category: "agents", TreePath: ".github/agents", CheckType: CheckDirectory, FilePattern: "*.agent.md"},
		{Category: "mcp_config", TreePath: ".vscode/mcp.json", CheckType: CheckFile},
	}
	repos := []RepoInfo{
		{Name: "has-stuff"},
		{Name: "empty-repo"},
		{Name: "missing-repo"},
	}

	data := map[string]json.RawMessage{
		"repo0": json.RawMessage(`{
			"copilot_instructions": {"__typename": "Blob"},
			"agents": {"__typename": "Tree", "entries": [
				{"name": "auth.agent.md", "type": "blob"},
				{"name": "nais.agent.md", "type": "blob"},
				{"name": "README.md", "type": "blob"}
			]},
			"mcp_config": null
		}`),
		"repo1": json.RawMessage(`{
			"copilot_instructions": null,
			"agents": null,
			"mcp_config": null
		}`),
		"repo2": json.RawMessage(`null`),
	}

	results := parseGraphQLResponse(data, repos, criteria)

	// has-stuff: copilot_instructions exists
	if !results["has-stuff"]["copilot_instructions"].Exists {
		t.Error("expected copilot_instructions to exist for has-stuff")
	}

	// has-stuff: 2 agent files (README.md filtered out by *.agent.md)
	agents := results["has-stuff"]["agents"]
	if !agents.Exists {
		t.Error("expected agents to exist for has-stuff")
	}
	if len(agents.Files) != 2 {
		t.Errorf("expected 2 agent files, got %d: %v", len(agents.Files), agents.Files)
	}

	// has-stuff: mcp_config does not exist
	if results["has-stuff"]["mcp_config"].Exists {
		t.Error("expected mcp_config to not exist for has-stuff")
	}

	// empty-repo: nothing exists
	for cat, sr := range results["empty-repo"] {
		if sr.Exists {
			t.Errorf("expected %s to not exist for empty-repo", cat)
		}
	}

	// missing-repo (null response): nothing exists
	for cat, sr := range results["missing-repo"] {
		if sr.Exists {
			t.Errorf("expected %s to not exist for missing-repo", cat)
		}
	}
}

func TestParseGraphQLResponseDirectoryWithoutTypename(t *testing.T) {
	// Edge case: if __typename is missing from response, directory should NOT be detected.
	// This tests that our parsing correctly requires __typename == "Tree".
	criteria := []SearchCriteria{
		{Category: "agents", TreePath: ".github/agents", CheckType: CheckDirectory, FilePattern: "*.agent.md"},
	}
	repos := []RepoInfo{{Name: "test-repo"}}

	// Response without __typename (simulates old/broken query result)
	data := map[string]json.RawMessage{
		"repo0": json.RawMessage(`{
			"agents": {"entries": [
				{"name": "auth.agent.md", "type": "blob"}
			]}
		}`),
	}

	results := parseGraphQLResponse(data, repos, criteria)

	// Without __typename: "Tree", the directory should NOT be detected
	if results["test-repo"]["agents"].Exists {
		t.Error("expected agents to NOT exist when __typename is missing")
	}
}

func TestParseGraphQLResponseDirectoryWithEmptyEntries(t *testing.T) {
	// Edge case: directory exists but has no files matching pattern
	criteria := []SearchCriteria{
		{Category: "agents", TreePath: ".github/agents", CheckType: CheckDirectory, FilePattern: "*.agent.md"},
	}
	repos := []RepoInfo{{Name: "test-repo"}}

	data := map[string]json.RawMessage{
		"repo0": json.RawMessage(`{
			"agents": {"__typename": "Tree", "entries": [
				{"name": "README.md", "type": "blob"},
				{"name": ".gitkeep", "type": "blob"}
			]}
		}`),
	}

	results := parseGraphQLResponse(data, repos, criteria)

	// Directory exists but no matching files - should NOT count as "exists"
	if results["test-repo"]["agents"].Exists {
		t.Error("expected agents to NOT exist when no files match pattern")
	}
}

func TestParseGraphQLResponseWildcardPattern(t *testing.T) {
	// Skills use "*" pattern - any file should match
	criteria := []SearchCriteria{
		{Category: "skills", TreePath: ".github/skills", CheckType: CheckDirectory, FilePattern: "*"},
	}
	repos := []RepoInfo{{Name: "test-repo"}}

	data := map[string]json.RawMessage{
		"repo0": json.RawMessage(`{
			"skills": {"__typename": "Tree", "entries": [
				{"name": "auth-skill", "type": "tree"},
				{"name": "SKILL.md", "type": "blob"}
			]}
		}`),
	}

	results := parseGraphQLResponse(data, repos, criteria)

	skills := results["test-repo"]["skills"]
	if !skills.Exists {
		t.Error("expected skills to exist with wildcard pattern")
	}
	// Wildcard should match both entries
	if len(skills.Files) != 2 {
		t.Errorf("expected 2 files with wildcard, got %d: %v", len(skills.Files), skills.Files)
	}
}

func TestHighestPermission(t *testing.T) {
	tests := []struct {
		perms map[string]bool
		want  string
	}{
		{map[string]bool{"admin": true, "push": true, "pull": true}, "admin"},
		{map[string]bool{"push": true, "pull": true}, "push"},
		{map[string]bool{"pull": true}, "pull"},
		{map[string]bool{"triage": true, "pull": true}, "triage"},
		{map[string]bool{}, "none"},
	}

	for _, tt := range tests {
		got := highestPermission(tt.perms)
		if got != tt.want {
			t.Errorf("highestPermission(%v) = %q, want %q", tt.perms, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAssembleResult(t *testing.T) {
	repo := RepoInfo{
		Name:            "test-repo",
		DefaultBranch:   "main",
		PrimaryLanguage: "Go",
		IsArchived:      false,
		IsFork:          false,
		Visibility:      "internal",
		CreatedAt:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		PushedAt:        time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Topics:          []string{"backend", "go"},
	}
	teams := []TeamAccess{
		{Slug: "team-a", Name: "Team A", Permission: "admin"},
	}
	customizations := map[string]SearchResult{
		"copilot_instructions": {Exists: true},
		"agents":               {Exists: true, Files: []string{"auth.agent.md"}},
		"agents_md":            {Exists: false},
		"instructions":         {Exists: false},
		"prompts":              {Exists: false},
		"skills":               {Exists: false},
		"mcp_config":           {Exists: false},
	}

	result := assembleResult("navikt", repo, teams, customizations)

	if result.Org != "navikt" {
		t.Errorf("expected org navikt, got %s", result.Org)
	}
	if result.Repo != "test-repo" {
		t.Errorf("expected repo test-repo, got %s", result.Repo)
	}
	if !result.HasAny {
		t.Error("expected HasAny to be true")
	}
	if result.CustomizationCount != 2 {
		t.Errorf("expected customization count 2, got %d", result.CustomizationCount)
	}
	if len(result.Teams) != 1 {
		t.Errorf("expected 1 team, got %d", len(result.Teams))
	}
}

func TestAssembleResultNoCustomizations(t *testing.T) {
	repo := RepoInfo{Name: "bare-repo", Visibility: "private"}
	customizations := map[string]SearchResult{
		"copilot_instructions": {Exists: false},
	}

	result := assembleResult("navikt", repo, nil, customizations)

	if result.HasAny {
		t.Error("expected HasAny to be false")
	}
	if result.CustomizationCount != 0 {
		t.Errorf("expected customization count 0, got %d", result.CustomizationCount)
	}
	if result.Teams == nil {
		t.Error("expected teams to be empty slice, not nil")
	}
	if result.Topics == nil {
		t.Error("expected topics to be empty slice, not nil")
	}
}
