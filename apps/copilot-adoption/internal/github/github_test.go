package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-adoption/internal/adoption"
	"github.com/navikt/copilot/apps/copilot-adoption/internal/scanner"
)

func TestBuildGraphQLQuery(t *testing.T) {
	repos := []adoption.RepoInfo{
		{Name: "repo-a", DefaultBranch: "main"},
		{Name: "repo-b", DefaultBranch: "master"},
	}
	criteria := []adoption.SearchCriteria{
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: adoption.CheckFile},
		{Category: "agents", TreePath: ".github/agents", CheckType: adoption.CheckDirectory, FilePattern: "*.agent.md"},
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
	repos := []adoption.RepoInfo{
		{Name: "test-repo", DefaultBranch: "main"},
	}
	criteria := []adoption.SearchCriteria{
		{Category: "agents", TreePath: ".github/agents", CheckType: adoption.CheckDirectory, FilePattern: "*.agent.md"},
		{Category: "instructions", TreePath: ".github/instructions", CheckType: adoption.CheckDirectory, FilePattern: "*.instructions.md"},
		{Category: "prompts", TreePath: ".github/prompts", CheckType: adoption.CheckDirectory, FilePattern: "*.prompt.md"},
		{Category: "skills", TreePath: ".github/skills", CheckType: adoption.CheckDirectory, FilePattern: "*"},
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
	repos := []adoption.RepoInfo{
		{Name: "repo-no-branch", DefaultBranch: ""},
	}
	criteria := []adoption.SearchCriteria{
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: adoption.CheckFile},
	}

	query := buildGraphQLQuery("navikt", repos, criteria)

	if !contains(query, `expression: "HEAD:.github/copilot-instructions.md"`) {
		t.Errorf("expected HEAD fallback for empty branch, query:\n%s", query)
	}
}

func TestBuildGraphQLQueryFileCheckFormat(t *testing.T) {
	// File checks use inline __typename, directory checks use multi-line format
	repos := []adoption.RepoInfo{
		{Name: "test-repo", DefaultBranch: "main"},
	}
	fileOnlyCriteria := []adoption.SearchCriteria{
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: adoption.CheckFile},
		{Category: "agents_md", TreePath: "AGENTS.md", CheckType: adoption.CheckFile},
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
	criteria := []adoption.SearchCriteria{
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: adoption.CheckFile},
		{Category: "agents", TreePath: ".github/agents", CheckType: adoption.CheckDirectory, FilePattern: "*.agent.md"},
		{Category: "mcp_config", TreePath: ".vscode/mcp.json", CheckType: adoption.CheckFile},
	}
	repos := []adoption.RepoInfo{
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
	if len(ci.Files) != 1 || ci.Files[0] != "copilot-instructions.md" {
		t.Errorf("expected copilot_instructions files [copilot-instructions.md], got %v", ci.Files)
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
	criteria := []adoption.SearchCriteria{
		{Category: "agents", TreePath: ".github/agents", CheckType: adoption.CheckDirectory, FilePattern: "*.agent.md"},
	}
	repos := []adoption.RepoInfo{{Name: "test-repo"}}

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
	criteria := []adoption.SearchCriteria{
		{Category: "agents", TreePath: ".github/agents", CheckType: adoption.CheckDirectory, FilePattern: "*.agent.md"},
	}
	repos := []adoption.RepoInfo{{Name: "test-repo"}}

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
	criteria := []adoption.SearchCriteria{
		{Category: "skills", TreePath: ".github/skills", CheckType: adoption.CheckDirectory, FilePattern: "*"},
	}
	repos := []adoption.RepoInfo{{Name: "test-repo"}}

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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestBuildTeamMapFailsWhenTeamLookupFails(t *testing.T) {
	respond := func(body string) *http.Response {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
	}
	c := &GitHubClient{httpClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/teams") {
			return respond(`[{"slug":"a","name":"A"}]`), nil
		}
		return respond(`not json`), nil
	})}}
	if _, err := c.BuildTeamMap(context.Background(), "org"); err == nil {
		t.Fatal("expected error when a team's repos cannot be listed")
	}
}

func TestScanBatchFailsOnGraphQLErrors(t *testing.T) {
	repos := []adoption.RepoInfo{{Name: "repo-a", DefaultBranch: "main"}}
	criteria := []adoption.SearchCriteria{
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: adoption.CheckFile},
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{
			name: "partial error with data",
			body: `{"data":{"repo0":null},"errors":[{"message":"Resource not accessible by integration"}]}`,
		},
		{
			name: "empty data without errors",
			body: `{"data":{}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &GitHubClient{httpClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(tc.body)),
					Header:     http.Header{},
				}, nil
			})}}
			if _, _, err := c.scanBatch(context.Background(), "navikt", repos, criteria); err == nil {
				t.Fatal("expected scanBatch to return an error")
			}
		})
	}
}

func TestParseGraphQLResponseWithActivity(t *testing.T) {
	criteria := []adoption.SearchCriteria{
		{Category: "copilot_hooks", TreePath: ".github/hooks/copilot-hooks.json", CheckType: adoption.CheckFile},
	}
	repos := []adoption.RepoInfo{{Name: "test-repo", OpenIssuesCount: 14}}

	data := map[string]json.RawMessage{
		"repo0": json.RawMessage(`{
			"defaultBranchRef": {
				"target": {
					"committedDate": "2026-10-14T10:30:00Z",
					"commits7d": {"totalCount": 8},
					"commits30d": {"totalCount": 31}
				}
			},
			"copilot_hooks": {"__typename": "Blob", "oid": "abc123"}
		}`),
	}

	results, lastCommits, commits7d, commits30d := parseGraphQLResponseWithActivity(data, repos, criteria)
	if !results["test-repo"]["copilot_hooks"].Exists {
		t.Fatal("expected copilot_hooks to exist")
	}
	if commits7d["test-repo"] != 8 || commits30d["test-repo"] != 31 {
		t.Fatalf("expected commits 7d=8, 30d=31, got 7d=%d, 30d=%d", commits7d["test-repo"], commits30d["test-repo"])
	}
	if lastCommits["test-repo"] == nil {
		t.Fatal("expected lastCommit to be parsed")
	}

	res := scanner.AssembleResult("navikt", repos[0], nil, results["test-repo"], lastCommits["test-repo"])
	res.DefaultBranchCommits7d = commits7d["test-repo"]
	res.DefaultBranchCommits30d = commits30d["test-repo"]
	if res.DefaultBranchCommits7d != 8 || res.DefaultBranchCommits30d != 31 || res.OpenIssuesCount != 14 {
		t.Fatalf("unexpected assembled activity fields: %+v", res)
	}
}

func parseGraphQLResponse(data map[string]json.RawMessage, repos []adoption.RepoInfo, criteria []adoption.SearchCriteria) (map[string]map[string]adoption.SearchResult, map[string]*time.Time) {
	results, lastCommits, _, _ := parseGraphQLResponseWithActivity(data, repos, criteria)
	return results, lastCommits
}
