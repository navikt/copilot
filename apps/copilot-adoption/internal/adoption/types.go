package adoption

import (
	"context"
	"time"
)

type RepoInfo struct {
	Name            string
	DefaultBranch   string
	PrimaryLanguage string
	IsArchived      bool
	IsFork          bool
	Visibility      string // "public", "private", "internal"
	CreatedAt       time.Time
	PushedAt        time.Time
	Topics          []string
	OpenIssuesCount int
}

type TeamAccess struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Permission string `json:"permission"` // "pull", "push", "admin", "maintain", "triage"
}

type SearchResult struct {
	Exists bool     `json:"exists"`
	Files  []string `json:"files,omitempty"`
	Oids   []string `json:"oids,omitempty"`    // Git blob OIDs (SHA-1), parallel to Files for directory checks
	InSync []bool   `json:"in_sync,omitempty"` // Per-file sync status vs source repo, parallel to Files/Oids
}

type RepoScanResult struct {
	Org                     string
	Repo                    string
	DefaultBranch           string
	PrimaryLanguage         string
	IsArchived              bool
	IsFork                  bool
	Visibility              string
	CreatedAt               time.Time
	PushedAt                time.Time
	DefaultBranchLastCommit *time.Time // nil when not scanned (archived repos, failed batches)
	DefaultBranchCommits7d  int
	DefaultBranchCommits30d int
	OpenIssuesCount         int
	Topics                  []string
	Teams                   []TeamAccess
	Customizations          map[string]SearchResult // keyed by SearchCriteria.Category
	HasAny                  bool
	CustomizationCount      int
}

type RepoLister interface {
	ListRepos(ctx context.Context, org string) ([]RepoInfo, error)
}

type TeamMapper interface {
	BuildTeamMap(ctx context.Context, org string) (map[string][]TeamAccess, error)
}

type ScanOutput struct {
	Customizations map[string]map[string]SearchResult // repo name → category → result
	LastCommits    map[string]*time.Time              // repo name → last commit to default branch (nil = unknown)
	Commits7d      map[string]int                     // repo name → commits on default branch in last 7d
	Commits30d     map[string]int                     // repo name → commits on default branch in last 30d
}

type CustomizationScanner interface {
	ScanRepos(ctx context.Context, org string, repos []RepoInfo, criteria []SearchCriteria) (*ScanOutput, error)
}

type AdoptionStore interface {
	EnsureTableExists(ctx context.Context) error
	EnsureViewsExist(ctx context.Context) error
	InsertScanResults(ctx context.Context, scanDate time.Time, results []RepoScanResult) error
	Close() error
}

// SourceOIDs maps category → (filename → blob OID) for the canonical source repo.
type SourceOIDs map[string]map[string]string

type SourceOIDResolver interface {
	ResolveSourceOIDs(ctx context.Context, criteria []SearchCriteria) (SourceOIDs, error)
}

func EmptyResults(criteria []SearchCriteria) map[string]SearchResult {
	m := make(map[string]SearchResult, len(criteria))
	for _, c := range criteria {
		m[c.Category] = SearchResult{Exists: false}
	}
	return m
}
