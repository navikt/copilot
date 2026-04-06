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
		`... on Tree { entries { name type object { oid } } }`,
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

	// File checks should use inline format with Blob OID
	if !contains(query, `{ __typename ... on Blob { oid } }`) {
		t.Errorf("file checks should use { __typename ... on Blob { oid } }, query:\n%s", query)
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
			"defaultBranchRef": {"target": {"committedDate": "2026-03-10T12:00:00Z"}},
			"copilot_instructions": {"__typename": "Blob", "oid": "abc123"},
			"agents": {"__typename": "Tree", "entries": [
				{"name": "auth.agent.md", "type": "blob", "object": {"oid": "def456"}},
				{"name": "nais.agent.md", "type": "blob", "object": {"oid": "ghi789"}},
				{"name": "README.md", "type": "blob", "object": {"oid": "jkl012"}}
			]},
			"mcp_config": null
		}`),
		"repo1": json.RawMessage(`{
			"defaultBranchRef": null,
			"copilot_instructions": null,
			"agents": null,
			"mcp_config": null
		}`),
		"repo2": json.RawMessage(`null`),
	}

	results, lastCommits := parseGraphQLResponse(data, repos, criteria)

	// has-stuff: copilot_instructions exists with OID
	ci := results["has-stuff"]["copilot_instructions"]
	if !ci.Exists {
		t.Error("expected copilot_instructions to exist for has-stuff")
	}
	if len(ci.Oids) != 1 || ci.Oids[0] != "abc123" {
		t.Errorf("expected copilot_instructions oid [abc123], got %v", ci.Oids)
	}

	// has-stuff: 2 agent files (README.md filtered out by *.agent.md) with OIDs
	agents := results["has-stuff"]["agents"]
	if !agents.Exists {
		t.Error("expected agents to exist for has-stuff")
	}
	if len(agents.Files) != 2 {
		t.Errorf("expected 2 agent files, got %d: %v", len(agents.Files), agents.Files)
	}
	if len(agents.Oids) != 2 {
		t.Errorf("expected 2 agent oids, got %d: %v", len(agents.Oids), agents.Oids)
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

	// Verify last commit dates
	expectedCommit := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	if lastCommits["has-stuff"] == nil || !lastCommits["has-stuff"].Equal(expectedCommit) {
		t.Errorf("expected last commit %v for has-stuff, got %v", expectedCommit, lastCommits["has-stuff"])
	}
	if lastCommits["empty-repo"] != nil {
		t.Errorf("expected nil for empty-repo, got %v", lastCommits["empty-repo"])
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

	results, _ := parseGraphQLResponse(data, repos, criteria)

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

	results, _ := parseGraphQLResponse(data, repos, criteria)

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

	results, _ := parseGraphQLResponse(data, repos, criteria)

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

	lastCommitTime := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	result := assembleResult("navikt", repo, teams, customizations, &lastCommitTime)

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
	if result.DefaultBranchLastCommit == nil || !result.DefaultBranchLastCommit.Equal(lastCommitTime) {
		t.Errorf("expected last commit %v, got %v", lastCommitTime, result.DefaultBranchLastCommit)
	}
}

func TestAssembleResultNoCustomizations(t *testing.T) {
	repo := RepoInfo{Name: "bare-repo", Visibility: "private"}
	customizations := map[string]SearchResult{
		"copilot_instructions": {Exists: false},
	}

	result := assembleResult("navikt", repo, nil, customizations, nil)

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
	if result.DefaultBranchLastCommit != nil {
		t.Errorf("expected nil last commit, got %v", result.DefaultBranchLastCommit)
	}
}

func TestComputeInSync(t *testing.T) {
	criteria := []SearchCriteria{
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: CheckFile},
		{Category: "agents", TreePath: ".github/agents", CheckType: CheckDirectory, FilePattern: "*.agent.md"},
		{Category: "instructions", TreePath: ".github/instructions", CheckType: CheckDirectory, FilePattern: "*.instructions.md"},
	}

	sourceOIDs := SourceOIDs{
		"copilot_instructions": {"copilot-instructions.md": "source_oid_ci"},
		"agents":               {"auth.agent.md": "source_oid_auth", "nais.agent.md": "source_oid_nais"},
		"instructions":         {"kotlin-ktor.instructions.md": "source_oid_kotlin"},
	}

	results := []RepoScanResult{
		{
			Repo: "in-sync-repo",
			Customizations: map[string]SearchResult{
				"copilot_instructions": {Exists: true, Oids: []string{"source_oid_ci"}},
				"agents":               {Exists: true, Files: []string{"auth.agent.md", "nais.agent.md"}, Oids: []string{"source_oid_auth", "source_oid_nais"}},
				"instructions":         {Exists: true, Files: []string{"kotlin-ktor.instructions.md"}, Oids: []string{"source_oid_kotlin"}},
			},
		},
		{
			Repo: "stale-repo",
			Customizations: map[string]SearchResult{
				"copilot_instructions": {Exists: true, Oids: []string{"different_oid"}},
				"agents":               {Exists: true, Files: []string{"auth.agent.md", "nais.agent.md"}, Oids: []string{"source_oid_auth", "old_nais_oid"}},
				"instructions":         {Exists: false},
			},
		},
		{
			Repo: "empty-repo",
			Customizations: map[string]SearchResult{
				"copilot_instructions": {Exists: false},
				"agents":               {Exists: false},
				"instructions":         {Exists: false},
			},
		},
	}

	ComputeInSync(results, sourceOIDs, criteria)

	// in-sync-repo: all files match source
	ciSync := results[0].Customizations["copilot_instructions"]
	if len(ciSync.InSync) != 1 || !ciSync.InSync[0] {
		t.Errorf("in-sync-repo copilot_instructions: expected [true], got %v", ciSync.InSync)
	}
	agentsSync := results[0].Customizations["agents"]
	if len(agentsSync.InSync) != 2 || !agentsSync.InSync[0] || !agentsSync.InSync[1] {
		t.Errorf("in-sync-repo agents: expected [true, true], got %v", agentsSync.InSync)
	}
	instrSync := results[0].Customizations["instructions"]
	if len(instrSync.InSync) != 1 || !instrSync.InSync[0] {
		t.Errorf("in-sync-repo instructions: expected [true], got %v", instrSync.InSync)
	}

	// stale-repo: copilot_instructions differs, one agent differs
	ciStale := results[1].Customizations["copilot_instructions"]
	if len(ciStale.InSync) != 1 || ciStale.InSync[0] {
		t.Errorf("stale-repo copilot_instructions: expected [false], got %v", ciStale.InSync)
	}
	agentsStale := results[1].Customizations["agents"]
	if len(agentsStale.InSync) != 2 || !agentsStale.InSync[0] || agentsStale.InSync[1] {
		t.Errorf("stale-repo agents: expected [true, false], got %v", agentsStale.InSync)
	}
	// instructions: not present, should have no InSync
	instrStale := results[1].Customizations["instructions"]
	if len(instrStale.InSync) != 0 {
		t.Errorf("stale-repo instructions: expected no InSync, got %v", instrStale.InSync)
	}

	// empty-repo: nothing to compare
	for cat, sr := range results[2].Customizations {
		if len(sr.InSync) != 0 {
			t.Errorf("empty-repo %s: expected no InSync, got %v", cat, sr.InSync)
		}
	}
}

func TestComputeInSyncNoSourceOIDs(t *testing.T) {
	criteria := []SearchCriteria{
		{Category: "agents", TreePath: ".github/agents", CheckType: CheckDirectory, FilePattern: "*.agent.md"},
	}

	results := []RepoScanResult{
		{
			Repo: "custom-agent-repo",
			Customizations: map[string]SearchResult{
				"agents": {Exists: true, Files: []string{"custom.agent.md"}, Oids: []string{"some_oid"}},
			},
		},
	}

	// Source has no agents — repo has a custom file not in source
	ComputeInSync(results, SourceOIDs{}, criteria)

	agents := results[0].Customizations["agents"]
	if len(agents.InSync) != 0 {
		t.Errorf("expected no InSync when source has no OIDs, got %v", agents.InSync)
	}
}

func TestComputeInSyncMissingOIDs(t *testing.T) {
	criteria := []SearchCriteria{
		{Category: "agents", TreePath: ".github/agents", CheckType: CheckDirectory, FilePattern: "*.agent.md"},
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: CheckFile},
	}

	sourceOIDs := SourceOIDs{
		"agents":               {"auth.agent.md": "source_oid_auth"},
		"copilot_instructions": {"copilot-instructions.md": "source_oid_ci"},
	}

	results := []RepoScanResult{
		{
			Repo: "missing-oid-repo",
			Customizations: map[string]SearchResult{
				// Directory entry with empty OID (e.g. unresolved Tree entry)
				"agents": {Exists: true, Files: []string{"auth.agent.md"}, Oids: []string{""}},
				// File entry with empty OID
				"copilot_instructions": {Exists: true, Oids: []string{""}},
			},
		},
	}

	ComputeInSync(results, sourceOIDs, criteria)

	// When OIDs are empty, InSync should remain nil (unknown) rather than [false]
	agents := results[0].Customizations["agents"]
	if agents.InSync != nil {
		t.Errorf("expected nil InSync for missing OIDs, got %v", agents.InSync)
	}
	ci := results[0].Customizations["copilot_instructions"]
	if ci.InSync != nil {
		t.Errorf("expected nil InSync for missing file OID, got %v", ci.InSync)
	}
}
