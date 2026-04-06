package main

import (
	"context"
	"time"
)

// RepoInfo contains metadata about a GitHub repository.
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
}

// TeamAccess represents a team's access to a repository.
type TeamAccess struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Permission string `json:"permission"` // "pull", "push", "admin", "maintain", "triage"
}

// SearchResult holds the outcome of checking a single search criterion against a repo.
type SearchResult struct {
	Exists bool     `json:"exists"`
	Files  []string `json:"files,omitempty"`
	Oids   []string `json:"oids,omitempty"`    // Git blob OIDs (SHA-1), parallel to Files for directory checks
	InSync []bool   `json:"in_sync,omitempty"` // Per-file sync status vs source repo, parallel to Files/Oids
}

// RepoScanResult is the fully assembled result for one repository.
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
	Topics                  []string
	Teams                   []TeamAccess
	Customizations          map[string]SearchResult // keyed by SearchCriteria.Category
	HasAny                  bool
	CustomizationCount      int
}

// RepoLister lists repositories in a GitHub organization.
type RepoLister interface {
	ListRepos(ctx context.Context, org string) ([]RepoInfo, error)
}

// TeamMapper builds a mapping from repository name to teams with access.
type TeamMapper interface {
	BuildTeamMap(ctx context.Context, org string) (map[string][]TeamAccess, error)
}

// ScanOutput holds the combined results of a repository scan batch.
type ScanOutput struct {
	Customizations map[string]map[string]SearchResult // repo name → category → result
	LastCommits    map[string]*time.Time              // repo name → last commit to default branch (nil = unknown)
}

// CustomizationScanner checks repositories for customization files using GraphQL.
type CustomizationScanner interface {
	ScanRepos(ctx context.Context, org string, repos []RepoInfo, criteria []SearchCriteria) (*ScanOutput, error)
}

// AdoptionStore persists scan results to BigQuery.
type AdoptionStore interface {
	EnsureTableExists(ctx context.Context) error
	EnsureViewsExist(ctx context.Context) error
	InsertScanResults(ctx context.Context, scanDate time.Time, results []RepoScanResult) error
	DeleteScanDate(ctx context.Context, scanDate time.Time) error
	ScanDateExists(ctx context.Context, scanDate time.Time) (bool, error)
	Close() error
}

// SourceOIDs maps category → (filename → blob OID) for the canonical source repo.
type SourceOIDs map[string]map[string]string

// SourceOIDResolver fetches the canonical OIDs for customization files from the source repo.
type SourceOIDResolver interface {
	ResolveSourceOIDs(ctx context.Context, criteria []SearchCriteria) (SourceOIDs, error)
}

// Verify implementations satisfy interfaces at compile time.
var (
	_ RepoLister           = (*GitHubClient)(nil)
	_ TeamMapper           = (*GitHubClient)(nil)
	_ CustomizationScanner = (*GitHubClient)(nil)
	_ SourceOIDResolver    = (*GitHubClient)(nil)
	_ AdoptionStore        = (*BigQueryClient)(nil)
)
