package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

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
	SourceOIDResolver
}, bq AdoptionStore, cfg *Config, scanDate time.Time, _ *SlackNotifier) error {
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
		// Writing a partition without teams would replace yesterday's good data.
		return fmt.Errorf("failed to build team map: %w", err)
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
	scanOutput, err := gh.ScanRepos(ctx, cfg.OrganizationSlug, activeRepos, criteria)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Check error rate — count repos where GraphQL batch failed (nil result)
	// Note: nil indicates batch failure, vs non-nil map with Exists=false for "no customizations"
	failedCount := 0
	for _, res := range scanOutput.Customizations {
		if res == nil {
			failedCount++
		}
	}
	// Any failed batch fails the run: a partial partition would replace
	// yesterday's complete data with missing customizations.
	if failedCount > 0 {
		return fmt.Errorf("scan incomplete: %d/%d repos failed to scan, nothing written",
			failedCount, len(activeRepos))
	}

	// Step 5: Assemble results
	slog.Info("Assembling results...")
	var allResults []RepoScanResult

	// Active repos: full scan results
	for _, repo := range activeRepos {
		customizations := scanOutput.Customizations[repo.Name]
		if customizations == nil {
			customizations = emptyResults(criteria)
		}
		res := assembleResult(cfg.OrganizationSlug, repo, teamMap[repo.Name], customizations, scanOutput.LastCommits[repo.Name])
		res.DefaultBranchCommits7d = scanOutput.Commits7d[repo.Name]
		res.DefaultBranchCommits30d = scanOutput.Commits30d[repo.Name]
		allResults = append(allResults, res)
	}

	// Archived repos: metadata only, no customization scan
	for _, repo := range archivedRepos {
		allResults = append(allResults, assembleResult(cfg.OrganizationSlug, repo, teamMap[repo.Name], emptyResults(criteria), nil))
	}

	// Step 5b: Resolve source OIDs and compute sync status
	slog.Info("Resolving source OIDs for sync comparison...")
	sourceOIDs, err := gh.ResolveSourceOIDs(ctx, criteria)
	if err != nil {
		slog.Warn("Failed to resolve source OIDs, skipping sync comparison", "error", err)
	} else {
		ComputeInSync(allResults, sourceOIDs, criteria)
	}

	// Step 6: Load results into BigQuery (load job with WriteTruncate replaces partition atomically)
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
	SourceOIDResolver
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
	scanOutput, err := gh.ScanRepos(ctx, cfg.OrganizationSlug, activeRepos, criteria)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	// Step 5: Assemble results
	var allResults []RepoScanResult
	for _, repo := range activeRepos {
		customizations := scanOutput.Customizations[repo.Name]
		if customizations == nil {
			customizations = emptyResults(criteria)
		}
		res := assembleResult(cfg.OrganizationSlug, repo, teamMap[repo.Name], customizations, scanOutput.LastCommits[repo.Name])
		res.DefaultBranchCommits7d = scanOutput.Commits7d[repo.Name]
		res.DefaultBranchCommits30d = scanOutput.Commits30d[repo.Name]
		allResults = append(allResults, res)
	}
	for _, repo := range archivedRepos {
		allResults = append(allResults, assembleResult(cfg.OrganizationSlug, repo, teamMap[repo.Name], emptyResults(criteria), nil))
	}

	// Step 5b: Resolve source OIDs and compute sync status
	slog.Info("Resolving source OIDs for sync comparison...")
	sourceOIDs, err := gh.ResolveSourceOIDs(ctx, criteria)
	if err != nil {
		slog.Warn("Failed to resolve source OIDs, skipping sync comparison", "error", err)
	} else {
		ComputeInSync(allResults, sourceOIDs, criteria)
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

// ComputeInSync annotates each SearchResult with per-file sync status by comparing
// blob OIDs against the canonical source repo. Modifies results in place.
func ComputeInSync(results []RepoScanResult, source SourceOIDs, criteria []SearchCriteria) {
	for i := range results {
		for _, c := range criteria {
			sr, ok := results[i].Customizations[c.Category]
			if !ok || !sr.Exists || len(sr.Oids) == 0 {
				continue
			}

			sourceOids := source[c.Category]
			if len(sourceOids) == 0 {
				continue
			}

			// Only set InSync when all OIDs are resolved; skip when any are
			// missing to avoid marking entries as stale when we simply don't
			// have data (e.g. Tree entries that didn't resolve an OID).
			allResolved := true
			inSync := make([]bool, len(sr.Oids))
			switch c.CheckType {
			case CheckFile:
				base := c.TreePath
				if idx := len(c.TreePath) - 1; idx >= 0 {
					for j := idx; j >= 0; j-- {
						if c.TreePath[j] == '/' {
							base = c.TreePath[j+1:]
							break
						}
					}
				}
				if len(sr.Oids) > 0 && sr.Oids[0] != "" {
					inSync[0] = sr.Oids[0] == sourceOids[base]
				} else {
					allResolved = false
				}
			case CheckDirectory:
				for j, name := range sr.Files {
					if j < len(sr.Oids) && sr.Oids[j] != "" {
						inSync[j] = sr.Oids[j] == sourceOids[name]
					} else {
						allResolved = false
					}
				}
			}

			if allResolved {
				sr.InSync = inSync
			}
			// When not all OIDs resolved, leave InSync nil (unknown) rather
			// than defaulting to false (stale).
			results[i].Customizations[c.Category] = sr
		}
	}
}

func assembleResult(org string, repo RepoInfo, teams []TeamAccess, customizations map[string]SearchResult, lastCommit *time.Time) RepoScanResult {
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
		Org:                     org,
		Repo:                    repo.Name,
		DefaultBranch:           repo.DefaultBranch,
		PrimaryLanguage:         repo.PrimaryLanguage,
		IsArchived:              repo.IsArchived,
		IsFork:                  repo.IsFork,
		Visibility:              repo.Visibility,
		CreatedAt:               repo.CreatedAt,
		PushedAt:                repo.PushedAt,
		DefaultBranchLastCommit: lastCommit,
		OpenIssuesCount:         repo.OpenIssuesCount,
		Topics:                  topics,
		Teams:                   teams,
		Customizations:          customizations,
		HasAny:                  hasAny,
		CustomizationCount:      count,
	}
}
