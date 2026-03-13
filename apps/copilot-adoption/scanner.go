package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const maxErrorRate = 30 // percent — abort if more than 30% of repos fail

// RunScan executes the full adoption scan pipeline:
// 1. List all org repos
// 2. Build team→repo map
// 3. Filter archived repos (record metadata only, skip file scan)
// 4. Batch GraphQL scans for active repos
// 5. Combine results and insert to BigQuery
func RunScan(ctx context.Context, gh interface {
	RepoLister
	TeamMapper
	CustomizationScanner
}, bq AdoptionStore, cfg *Config, scanDate time.Time) error {
	dateStr := scanDate.Format("2006-01-02")
	slog.Info("Starting adoption scan", "date", dateStr, "org", cfg.OrganizationSlug)

	criteria := DefaultCriteria()
	slog.Info("Search criteria loaded", "count", len(criteria))
	for _, c := range criteria {
		slog.Debug("Criterion", "category", c.Category, "path", c.TreePath, "type", c.CheckType)
	}

	// Step 1: List all repos
	slog.Info("Listing repositories...")
	repos, err := gh.ListRepos(ctx, cfg.OrganizationSlug)
	if err != nil {
		return fmt.Errorf("failed to list repos: %w", err)
	}
	slog.Info("Repositories listed", "total", len(repos))

	// Step 2: Build team map
	slog.Info("Building team map...")
	teamMap, err := gh.BuildTeamMap(ctx, cfg.OrganizationSlug)
	if err != nil {
		slog.Warn("Failed to build team map, continuing without team data", "error", err)
		teamMap = make(map[string][]TeamAccess)
	}

	// Step 3: Split archived vs active
	var activeRepos []RepoInfo
	var archivedRepos []RepoInfo
	for _, r := range repos {
		if r.IsArchived {
			archivedRepos = append(archivedRepos, r)
		} else {
			activeRepos = append(activeRepos, r)
		}
	}
	slog.Info("Repository split", "active", len(activeRepos), "archived", len(archivedRepos))

	// Step 4: Scan active repos for customization files
	slog.Info("Scanning active repositories for customizations...")
	scanResults, err := gh.ScanRepos(ctx, cfg.OrganizationSlug, activeRepos, criteria)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Check error rate — count repos where GraphQL returned no response at all
	emptyCount := 0
	for _, res := range scanResults {
		if len(res) == 0 {
			emptyCount++
		}
	}
	if len(activeRepos) > 10 && emptyCount*100/len(activeRepos) > maxErrorRate {
		return fmt.Errorf("too many scan failures: %d/%d repos returned empty results (>%d%%)",
			emptyCount, len(activeRepos), maxErrorRate)
	}

	// Step 5: Assemble results
	slog.Info("Assembling results...")
	var allResults []RepoScanResult

	// Active repos: full scan results
	for _, repo := range activeRepos {
		customizations := scanResults[repo.Name]
		if customizations == nil {
			customizations = emptyResults(criteria)
		}
		allResults = append(allResults, assembleResult(cfg.OrganizationSlug, repo, teamMap[repo.Name], customizations))
	}

	// Archived repos: metadata only, no customization scan
	for _, repo := range archivedRepos {
		allResults = append(allResults, assembleResult(cfg.OrganizationSlug, repo, teamMap[repo.Name], emptyResults(criteria)))
	}

	// Step 6: Idempotent insert
	exists, err := bq.ScanDateExists(ctx, scanDate)
	if err != nil {
		return fmt.Errorf("failed to check scan date: %w", err)
	}
	if exists {
		slog.Info("Scan date already exists, deleting for re-scan", "date", dateStr)
		if err := bq.DeleteScanDate(ctx, scanDate); err != nil {
			return fmt.Errorf("failed to delete existing scan: %w", err)
		}
	}

	if err := bq.InsertScanResults(ctx, scanDate, allResults); err != nil {
		return fmt.Errorf("failed to insert results: %w", err)
	}

	// Summary
	withAny := 0
	for _, r := range allResults {
		if r.HasAny {
			withAny++
		}
	}
	slog.Info("Scan completed",
		"date", dateStr,
		"total_repos", len(allResults),
		"active_repos", len(activeRepos),
		"archived_repos", len(archivedRepos),
		"repos_with_customizations", withAny,
	)

	return nil
}

// DryRunScan runs the scan pipeline without BigQuery, returning assembled results.
func DryRunScan(ctx context.Context, gh interface {
	RepoLister
	TeamMapper
	CustomizationScanner
}, cfg *Config, scanDate time.Time) ([]RepoScanResult, error) {
	dateStr := scanDate.Format("2006-01-02")
	slog.Info("Starting DRY RUN adoption scan", "date", dateStr, "org", cfg.OrganizationSlug)

	criteria := DefaultCriteria()
	slog.Info("Search criteria loaded", "count", len(criteria))

	// Step 1: List all repos
	slog.Info("Listing repositories...")
	repos, err := gh.ListRepos(ctx, cfg.OrganizationSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to list repos: %w", err)
	}
	slog.Info("Repositories listed", "total", len(repos))

	// Step 2: Build team map
	slog.Info("Building team map...")
	teamMap, err := gh.BuildTeamMap(ctx, cfg.OrganizationSlug)
	if err != nil {
		slog.Warn("Failed to build team map, continuing without team data", "error", err)
		teamMap = make(map[string][]TeamAccess)
	}

	// Step 3: Split archived vs active
	var activeRepos []RepoInfo
	var archivedRepos []RepoInfo
	for _, r := range repos {
		if r.IsArchived {
			archivedRepos = append(archivedRepos, r)
		} else {
			activeRepos = append(activeRepos, r)
		}
	}
	slog.Info("Repository split", "active", len(activeRepos), "archived", len(archivedRepos))

	// Step 4: Scan active repos
	slog.Info("Scanning active repositories for customizations...")
	scanResults, err := gh.ScanRepos(ctx, cfg.OrganizationSlug, activeRepos, criteria)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	// Step 5: Assemble results
	var allResults []RepoScanResult
	for _, repo := range activeRepos {
		customizations := scanResults[repo.Name]
		if customizations == nil {
			customizations = emptyResults(criteria)
		}
		allResults = append(allResults, assembleResult(cfg.OrganizationSlug, repo, teamMap[repo.Name], customizations))
	}
	for _, repo := range archivedRepos {
		allResults = append(allResults, assembleResult(cfg.OrganizationSlug, repo, teamMap[repo.Name], emptyResults(criteria)))
	}

	// Summary
	withAny := 0
	for _, r := range allResults {
		if r.HasAny {
			withAny++
		}
	}
	slog.Info("DRY RUN scan completed",
		"date", dateStr,
		"total_repos", len(allResults),
		"active_repos", len(activeRepos),
		"archived_repos", len(archivedRepos),
		"repos_with_customizations", withAny,
	)

	return allResults, nil
}

func assembleResult(org string, repo RepoInfo, teams []TeamAccess, customizations map[string]SearchResult) RepoScanResult {
	hasAny := false
	count := 0
	for _, sr := range customizations {
		if sr.Exists {
			hasAny = true
			count++
		}
	}

	if teams == nil {
		teams = []TeamAccess{}
	}

	topics := repo.Topics
	if topics == nil {
		topics = []string{}
	}

	return RepoScanResult{
		Org:                org,
		Repo:               repo.Name,
		DefaultBranch:      repo.DefaultBranch,
		PrimaryLanguage:    repo.PrimaryLanguage,
		IsArchived:         repo.IsArchived,
		IsFork:             repo.IsFork,
		Visibility:         repo.Visibility,
		CreatedAt:          repo.CreatedAt,
		PushedAt:           repo.PushedAt,
		Topics:             topics,
		Teams:              teams,
		Customizations:     customizations,
		HasAny:             hasAny,
		CustomizationCount: count,
	}
}
