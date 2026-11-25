package scanner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-adoption/internal/adoption"
	"github.com/navikt/copilot/apps/copilot-adoption/internal/config"
)

type fakeGH struct {
	teamErr error
	failed  map[string]bool // repos whose batch "failed" (nil result)
}

func (f *fakeGH) ListRepos(context.Context, string) ([]adoption.RepoInfo, error) {
	return []adoption.RepoInfo{{Name: "a"}, {Name: "b"}}, nil
}

func (f *fakeGH) BuildTeamMap(context.Context, string) (map[string][]adoption.TeamAccess, error) {
	return map[string][]adoption.TeamAccess{}, f.teamErr
}

func (f *fakeGH) ScanRepos(_ context.Context, _ string, repos []adoption.RepoInfo, criteria []adoption.SearchCriteria) (*adoption.ScanOutput, error) {
	out := &adoption.ScanOutput{Customizations: map[string]map[string]adoption.SearchResult{}, LastCommits: map[string]*time.Time{}}
	for _, r := range repos {
		if f.failed[r.Name] {
			out.Customizations[r.Name] = nil
		} else {
			out.Customizations[r.Name] = adoption.EmptyResults(criteria)
		}
	}
	return out, nil
}

func (f *fakeGH) ResolveSourceOIDs(context.Context, []adoption.SearchCriteria) (adoption.SourceOIDs, error) {
	return adoption.SourceOIDs{}, nil
}

type fakeStore struct{ inserts int }

func (s *fakeStore) EnsureTableExists(context.Context) error { return nil }
func (s *fakeStore) EnsureViewsExist(context.Context) error  { return nil }
func (s *fakeStore) InsertScanResults(context.Context, time.Time, []adoption.RepoScanResult) error {
	s.inserts++
	return nil
}
func (s *fakeStore) Close() error { return nil }

func TestRunScan_FailsWithoutWritingOnTeamMapError(t *testing.T) {
	store := &fakeStore{}
	err := RunScan(context.Background(), &fakeGH{teamErr: errors.New("boom")}, store, &config.Config{OrganizationSlug: "navikt"}, time.Now(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if store.inserts != 0 {
		t.Errorf("wrote %d times, want 0", store.inserts)
	}
}

func TestRunScan_FailsWithoutWritingOnAnyFailedBatch(t *testing.T) {
	store := &fakeStore{}
	err := RunScan(context.Background(), &fakeGH{failed: map[string]bool{"b": true}}, store, &config.Config{OrganizationSlug: "navikt"}, time.Now(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if store.inserts != 0 {
		t.Errorf("wrote %d times, want 0", store.inserts)
	}
}

func TestRunScan_WritesWhenAllSucceed(t *testing.T) {
	store := &fakeStore{}
	if err := RunScan(context.Background(), &fakeGH{}, store, &config.Config{OrganizationSlug: "navikt"}, time.Now(), nil); err != nil {
		t.Fatal(err)
	}
	if store.inserts != 1 {
		t.Errorf("wrote %d times, want 1", store.inserts)
	}
}

func TestAssembleResult(t *testing.T) {
	repo := adoption.RepoInfo{
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
	teams := []adoption.TeamAccess{
		{Slug: "team-a", Name: "Team A", Permission: "admin"},
	}
	customizations := map[string]adoption.SearchResult{
		"copilot_instructions": {Exists: true},
		"agents":               {Exists: true, Files: []string{"auth.agent.md"}},
		"agents_md":            {Exists: false},
		"instructions":         {Exists: false},
		"prompts":              {Exists: false},
		"skills":               {Exists: false},
		"mcp_config":           {Exists: false},
	}

	lastCommitTime := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	result := AssembleResult("navikt", repo, teams, customizations, &lastCommitTime)

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
	repo := adoption.RepoInfo{Name: "bare-repo", Visibility: "private"}
	customizations := map[string]adoption.SearchResult{
		"copilot_instructions": {Exists: false},
	}

	result := AssembleResult("navikt", repo, nil, customizations, nil)

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
	criteria := []adoption.SearchCriteria{
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: adoption.CheckFile},
		{Category: "agents", TreePath: ".github/agents", CheckType: adoption.CheckDirectory, FilePattern: "*.agent.md"},
		{Category: "instructions", TreePath: ".github/instructions", CheckType: adoption.CheckDirectory, FilePattern: "*.instructions.md"},
	}

	sourceOIDs := adoption.SourceOIDs{
		"copilot_instructions": {"copilot-instructions.md": "source_oid_ci"},
		"agents":               {"auth.agent.md": "source_oid_auth", "nais.agent.md": "source_oid_nais"},
		"instructions":         {"kotlin-ktor.instructions.md": "source_oid_kotlin"},
	}

	results := []adoption.RepoScanResult{
		{
			Repo: "in-sync-repo",
			Customizations: map[string]adoption.SearchResult{
				"copilot_instructions": {Exists: true, Oids: []string{"source_oid_ci"}},
				"agents":               {Exists: true, Files: []string{"auth.agent.md", "nais.agent.md"}, Oids: []string{"source_oid_auth", "source_oid_nais"}},
				"instructions":         {Exists: true, Files: []string{"kotlin-ktor.instructions.md"}, Oids: []string{"source_oid_kotlin"}},
			},
		},
		{
			Repo: "stale-repo",
			Customizations: map[string]adoption.SearchResult{
				"copilot_instructions": {Exists: true, Oids: []string{"different_oid"}},
				"agents":               {Exists: true, Files: []string{"auth.agent.md", "nais.agent.md"}, Oids: []string{"source_oid_auth", "old_nais_oid"}},
				"instructions":         {Exists: false},
			},
		},
		{
			Repo: "empty-repo",
			Customizations: map[string]adoption.SearchResult{
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
	criteria := []adoption.SearchCriteria{
		{Category: "agents", TreePath: ".github/agents", CheckType: adoption.CheckDirectory, FilePattern: "*.agent.md"},
	}

	results := []adoption.RepoScanResult{
		{
			Repo: "custom-agent-repo",
			Customizations: map[string]adoption.SearchResult{
				"agents": {Exists: true, Files: []string{"custom.agent.md"}, Oids: []string{"some_oid"}},
			},
		},
	}

	// Source has no agents — repo has a custom file not in source
	ComputeInSync(results, adoption.SourceOIDs{}, criteria)

	agents := results[0].Customizations["agents"]
	if len(agents.InSync) != 0 {
		t.Errorf("expected no InSync when source has no OIDs, got %v", agents.InSync)
	}
}

func TestComputeInSyncMissingOIDs(t *testing.T) {
	criteria := []adoption.SearchCriteria{
		{Category: "agents", TreePath: ".github/agents", CheckType: adoption.CheckDirectory, FilePattern: "*.agent.md"},
		{Category: "copilot_instructions", TreePath: ".github/copilot-instructions.md", CheckType: adoption.CheckFile},
	}

	sourceOIDs := adoption.SourceOIDs{
		"agents":               {"auth.agent.md": "source_oid_auth"},
		"copilot_instructions": {"copilot-instructions.md": "source_oid_ci"},
	}

	results := []adoption.RepoScanResult{
		{
			Repo: "missing-oid-repo",
			Customizations: map[string]adoption.SearchResult{
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
