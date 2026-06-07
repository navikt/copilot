package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"google.golang.org/api/iterator"
)

// BigQueryClient wraps BigQuery operations for Copilot data
type BigQueryClient struct {
	client          *bigquery.Client
	projectID       string
	metricsDataset  string
	metricsTable    string
	adoptionDataset string
}

func newBigQueryClient(config *Config) (*BigQueryClient, error) {
	if config.GCPProjectID == "" {
		return nil, fmt.Errorf("GCP_TEAM_PROJECT_ID not configured")
	}

	ctx := context.Background()
	client, err := bigquery.NewClient(ctx, config.GCPProjectID)
	if err != nil {
		return nil, fmt.Errorf("create BigQuery client: %w", err)
	}

	return &BigQueryClient{
		client:          client,
		projectID:       config.GCPProjectID,
		metricsDataset:  config.CopilotMetricsDataset,
		metricsTable:    config.CopilotMetricsTable,
		adoptionDataset: config.CopilotAdoptionDataset,
	}, nil
}

// EnterpriseMetrics represents a single day of Copilot usage metrics
type EnterpriseMetrics map[string]interface{}

// AdoptionSummary represents the latest adoption scan summary
type AdoptionSummary struct {
	ScanDate                     civil.Date `bigquery:"scan_date" json:"scan_date"`
	TotalRepos                   int64      `bigquery:"total_repos" json:"total_repos"`
	ActiveRepos                  int64      `bigquery:"active_repos" json:"active_repos"`
	ArchivedRepos                int64      `bigquery:"archived_repos" json:"archived_repos"`
	ActiveReposWithRecentCommits int64      `bigquery:"active_repos_with_recent_commits" json:"active_repos_with_recent_commits"`
	DormantRepos                 int64      `bigquery:"dormant_repos" json:"dormant_repos"`
	UnknownLastCommitRepos       int64      `bigquery:"unknown_last_commit_repos" json:"unknown_last_commit_repos"`
	ReposWithAnyCustomization    int64      `bigquery:"repos_with_any_customization" json:"repos_with_any_customization"`
	ReposWithoutCustomization    int64      `bigquery:"repos_without_customization" json:"repos_without_customization"`
	AdoptionRate                 float64    `bigquery:"adoption_rate" json:"adoption_rate"`
	AdoptionRateActiveOnly       float64    `bigquery:"adoption_rate_active_only" json:"adoption_rate_active_only"`
	ReposWithCopilotInstructions int64      `bigquery:"repos_with_copilot_instructions" json:"repos_with_copilot_instructions"`
	ReposWithAgentsMD            int64      `bigquery:"repos_with_agents_md" json:"repos_with_agents_md"`
	ReposWithAgents              int64      `bigquery:"repos_with_agents" json:"repos_with_agents"`
	ReposWithInstructions        int64      `bigquery:"repos_with_instructions" json:"repos_with_instructions"`
	ReposWithPrompts             int64      `bigquery:"repos_with_prompts" json:"repos_with_prompts"`
	ReposWithSkills              int64      `bigquery:"repos_with_skills" json:"repos_with_skills"`
	ReposWithMCPConfig           int64      `bigquery:"repos_with_mcp_config" json:"repos_with_mcp_config"`
	ReposWithCopilotDir          int64      `bigquery:"repos_with_copilot_dir" json:"repos_with_copilot_dir"`
	ReposWithCopilotReviewInst   int64      `bigquery:"repos_with_copilot_review_instructions" json:"repos_with_copilot_review_instructions"`
	ReposWithCursorRules         int64      `bigquery:"repos_with_cursorrules" json:"repos_with_cursorrules"`
	ReposWithCursorRulesDir      int64      `bigquery:"repos_with_cursor_rules_dir" json:"repos_with_cursor_rules_dir"`
	ReposWithClaudeMD            int64      `bigquery:"repos_with_claude_md" json:"repos_with_claude_md"`
	ReposWithWindsurfRules       int64      `bigquery:"repos_with_windsurfrules" json:"repos_with_windsurfrules"`
	ReposWithCursorIgnore        int64      `bigquery:"repos_with_cursorignore" json:"repos_with_cursorignore"`
	ReposWithClaudeSettings      int64      `bigquery:"repos_with_claude_settings" json:"repos_with_claude_settings"`
	ReposWithCopilotSetupSteps   int64      `bigquery:"repos_with_copilot_setup_steps" json:"repos_with_copilot_setup_steps"`
	ReposWithAgenticWorkflows    int64      `bigquery:"repos_with_agentic_workflows" json:"repos_with_agentic_workflows"`
	ReposWithAgentsSkills        int64      `bigquery:"repos_with_agents_skills" json:"repos_with_agents_skills"`
	ReposWithNavPilotState       int64      `bigquery:"repos_with_nav_pilot_state" json:"repos_with_nav_pilot_state"`
	ReposWithCPLTToml            int64      `bigquery:"repos_with_cplt_toml" json:"repos_with_cplt_toml"`
	ReposWithAnyNonCopilotAI     int64      `bigquery:"repos_with_any_non_copilot_ai" json:"repos_with_any_non_copilot_ai"`
	AvgCustomizationCount        float64    `bigquery:"avg_customization_count" json:"avg_customization_count"`
	MaxCustomizationCount        int64      `bigquery:"max_customization_count" json:"max_customization_count"`
}

// TeamAdoption represents adoption metrics for a single team
type TeamAdoption struct {
	ScanDate                civil.Date `bigquery:"scan_date" json:"scan_date"`
	TeamSlug                string     `bigquery:"team_slug" json:"team_slug"`
	TeamName                string     `bigquery:"team_name" json:"team_name"`
	TeamRepos               int64      `bigquery:"team_repos" json:"team_repos"`
	ActiveRepos             int64      `bigquery:"active_repos" json:"active_repos"`
	RecentlyActiveRepos     int64      `bigquery:"recently_active_repos" json:"recently_active_repos"`
	ReposWithCustomizations int64      `bigquery:"repos_with_customizations" json:"repos_with_customizations"`
	AdoptionRate            float64    `bigquery:"adoption_rate" json:"adoption_rate"`
	AdoptionRateActiveOnly  float64    `bigquery:"adoption_rate_active_only" json:"adoption_rate_active_only"`
	WithCopilotInstructions int64      `bigquery:"with_copilot_instructions" json:"with_copilot_instructions"`
	WithAgentsMD            int64      `bigquery:"with_agents_md" json:"with_agents_md"`
	WithAgents              int64      `bigquery:"with_agents" json:"with_agents"`
	WithInstructions        int64      `bigquery:"with_instructions" json:"with_instructions"`
	WithPrompts             int64      `bigquery:"with_prompts" json:"with_prompts"`
	WithSkills              int64      `bigquery:"with_skills" json:"with_skills"`
	WithMCPConfig           int64      `bigquery:"with_mcp_config" json:"with_mcp_config"`
	WithCopilotSetupSteps   int64      `bigquery:"with_copilot_setup_steps" json:"with_copilot_setup_steps"`
	WithAgenticWorkflows    int64      `bigquery:"with_agentic_workflows" json:"with_agentic_workflows"`
	WithAgentsSkills        int64      `bigquery:"with_agents_skills" json:"with_agents_skills"`
	WithNavPilotState       int64      `bigquery:"with_nav_pilot_state" json:"with_nav_pilot_state"`
	WithCPLTToml            int64      `bigquery:"with_cplt_toml" json:"with_cplt_toml"`
}

// CustomizationDetail represents usage of a specific customization file
type CustomizationDetail struct {
	Category        string `bigquery:"category" json:"category"`
	FileName        string `bigquery:"file_name" json:"file_name"`
	RepoCount       int64  `bigquery:"repo_count" json:"repo_count"`
	ActiveRepoCount int64  `bigquery:"active_repo_count" json:"active_repo_count"`
}

// CustomizationUsage includes sample repos for catalog enrichment
type CustomizationUsage struct {
	Category    string   `bigquery:"category" json:"category"`
	FileName    string   `bigquery:"file_name" json:"file_name"`
	RepoCount   int64    `bigquery:"repo_count" json:"repo_count"`
	SampleRepos []string `bigquery:"sample_repos" json:"sample_repos"`
}

// LanguageAdoption represents adoption metrics for a programming language
type LanguageAdoption struct {
	ScanDate                civil.Date `bigquery:"scan_date" json:"scan_date"`
	Language                string     `bigquery:"language" json:"language"`
	TotalRepos              int64      `bigquery:"total_repos" json:"total_repos"`
	RecentlyActiveRepos     int64      `bigquery:"recently_active_repos" json:"recently_active_repos"`
	ReposWithCustomizations int64      `bigquery:"repos_with_customizations" json:"repos_with_customizations"`
	AdoptionRate            float64    `bigquery:"adoption_rate" json:"adoption_rate"`
	AdoptionRateActiveOnly  float64    `bigquery:"adoption_rate_active_only" json:"adoption_rate_active_only"`
	WithCopilotInstructions int64      `bigquery:"with_copilot_instructions" json:"with_copilot_instructions"`
	WithAgents              int64      `bigquery:"with_agents" json:"with_agents"`
	WithInstructions        int64      `bigquery:"with_instructions" json:"with_instructions"`
	WithMCPConfig           int64      `bigquery:"with_mcp_config" json:"with_mcp_config"`
}

// StalenessFile represents sync status for a single customization file across repos
type StalenessFile struct {
	Category            string  `bigquery:"category" json:"category"`
	FileName            string  `bigquery:"file_name" json:"file_name"`
	TotalRepos          int64   `bigquery:"total_repos" json:"total_repos"`
	InSyncRepos         int64   `bigquery:"in_sync_repos" json:"in_sync_repos"`
	OutOfSyncRepos      int64   `bigquery:"out_of_sync_repos" json:"out_of_sync_repos"`
	SyncRate            float64 `bigquery:"sync_rate" json:"sync_rate"`
	RecentlyActiveRepos int64   `bigquery:"recently_active_repos" json:"recently_active_repos"`
}

// StalenessSummary aggregates staleness across all tracked files
type StalenessSummary struct {
	TotalFiles         int64           `json:"total_files"`
	TotalFileInstances int64           `json:"total_file_instances"`
	InSyncCount        int64           `json:"in_sync_count"`
	OutOfSyncCount     int64           `json:"out_of_sync_count"`
	SyncRate           float64         `json:"sync_rate"`
	Files              []StalenessFile `json:"files"`
}

func (bq *BigQueryClient) tableRef(dataset, table string) string {
	return fmt.Sprintf("`%s.%s.%s`", bq.projectID, dataset, table)
}

func (bq *BigQueryClient) viewRef(dataset, view string) string {
	return fmt.Sprintf("`%s.%s.%s`", bq.projectID, dataset, view)
}

// GetDailyMetrics fetches daily usage metrics from BigQuery
func (bq *BigQueryClient) GetDailyMetrics(ctx context.Context, days *int) ([]EnterpriseMetrics, error) {
	tableRef := bq.tableRef(bq.metricsDataset, bq.metricsTable)
	effectiveDays := 365
	if days != nil && *days > 0 {
		effectiveDays = *days
	}

	queryStr := fmt.Sprintf(`
		SELECT raw_record
		FROM %s
		WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL @days DAY)
			AND scope = 'enterprise'
		ORDER BY day ASC
	`, tableRef)

	query := bq.client.Query(queryStr)
	query.Parameters = append(query.Parameters, bigquery.QueryParameter{
		Name:  "days",
		Value: effectiveDays,
	})
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	var results []EnterpriseMetrics
	for {
		var values []bigquery.Value
		err := it.Next(&values)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate results: %w", err)
		}

		var row struct {
			RawRecord string `bigquery:"raw_record"`
		}
		if err := decodeBQRow(it.Schema, values, &row); err != nil {
			slog.Warn("Failed to decode row", "error", err)
			continue
		}
		if row.RawRecord == "" {
			continue
		}

		var metrics EnterpriseMetrics
		if err := json.Unmarshal([]byte(row.RawRecord), &metrics); err != nil {
			slog.Warn("Failed to parse raw_record", "error", err)
			continue
		}
		results = append(results, metrics)
	}

	slog.Debug("Fetched daily metrics", "count", len(results), "days", days)
	return results, nil
}

// GetAdoptionSummary fetches the latest adoption scan summary
func (bq *BigQueryClient) GetAdoptionSummary(ctx context.Context) (*AdoptionSummary, error) {
	viewRef := bq.viewRef(bq.adoptionDataset, "v_adoption_summary")
	queryStr := fmt.Sprintf(`
		SELECT * FROM %s
		ORDER BY scan_date DESC
		LIMIT 1
	`, viewRef)

	query := bq.client.Query(queryStr)
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	return readSingleRow[AdoptionSummary](it)
}

// GetTeamAdoption fetches team adoption metrics for the latest scan
func (bq *BigQueryClient) GetTeamAdoption(ctx context.Context) ([]TeamAdoption, error) {
	viewRef := bq.viewRef(bq.adoptionDataset, "v_team_adoption")
	queryStr := fmt.Sprintf(`
		SELECT * FROM %s
		WHERE scan_date = (SELECT MAX(scan_date) FROM %s)
		ORDER BY repos_with_customizations DESC
	`, viewRef, viewRef)

	query := bq.client.Query(queryStr)
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	return readAllRows[TeamAdoption](it)
}

// GetCustomizationDetails fetches aggregated customization file usage
func (bq *BigQueryClient) GetCustomizationDetails(ctx context.Context) ([]CustomizationDetail, error) {
	viewRef := bq.viewRef(bq.adoptionDataset, "v_customization_details")
	queryStr := fmt.Sprintf(`
		SELECT category, file_name,
			COUNT(DISTINCT repo) AS repo_count,
			COUNTIF(is_recently_active) AS active_repo_count
		FROM %s
		WHERE scan_date = (SELECT MAX(scan_date) FROM %s)
		GROUP BY category, file_name
		ORDER BY repo_count DESC
	`, viewRef, viewRef)

	query := bq.client.Query(queryStr)
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	return readAllRows[CustomizationDetail](it)
}

// GetCustomizationUsage fetches customization usage with sample repositories
func (bq *BigQueryClient) GetCustomizationUsage(ctx context.Context) ([]CustomizationUsage, error) {
	viewRef := bq.viewRef(bq.adoptionDataset, "v_customization_details")
	queryStr := fmt.Sprintf(`
		SELECT
			category,
			file_name,
			COUNT(DISTINCT repo) AS repo_count,
			ARRAY_AGG(DISTINCT repo ORDER BY repo LIMIT 5) AS sample_repos
		FROM %s
		WHERE scan_date = (SELECT MAX(scan_date) FROM %s)
		GROUP BY category, file_name
		ORDER BY repo_count DESC
	`, viewRef, viewRef)

	query := bq.client.Query(queryStr)
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	return readAllRows[CustomizationUsage](it)
}

// GetLanguageAdoption fetches language adoption metrics for the latest scan
func (bq *BigQueryClient) GetLanguageAdoption(ctx context.Context) ([]LanguageAdoption, error) {
	viewRef := bq.viewRef(bq.adoptionDataset, "v_language_adoption")
	queryStr := fmt.Sprintf(`
		SELECT * FROM %s
		WHERE scan_date = (SELECT MAX(scan_date) FROM %s)
		ORDER BY total_repos DESC
	`, viewRef, viewRef)

	query := bq.client.Query(queryStr)
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	return readAllRows[LanguageAdoption](it)
}

// GetStalenessData fetches file-level sync status from v_staleness_summary view
func (bq *BigQueryClient) GetStalenessData(ctx context.Context) ([]StalenessFile, error) {
	viewRef := bq.viewRef(bq.adoptionDataset, "v_staleness_summary")
	queryStr := fmt.Sprintf(`
		SELECT
			category,
			file_name,
			COUNT(*) AS total_repos,
			COUNTIF(in_sync) AS in_sync_repos,
			COUNTIF(NOT in_sync) AS out_of_sync_repos,
			SAFE_DIVIDE(COUNTIF(in_sync), COUNT(*)) AS sync_rate,
			COUNTIF(is_recently_active) AS recently_active_repos
		FROM %s
		WHERE scan_date = (SELECT MAX(scan_date) FROM %s)
			AND in_sync IS NOT NULL
		GROUP BY category, file_name
		ORDER BY total_repos DESC
	`, viewRef, viewRef)

	query := bq.client.Query(queryStr)
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	return readAllRows[StalenessFile](it)
}

// BigQueryQuerier abstracts BigQuery operations for testability
type BigQueryQuerier interface {
	GetDailyMetrics(ctx context.Context, days *int) ([]EnterpriseMetrics, error)
	GetAdoptionSummary(ctx context.Context) (*AdoptionSummary, error)
	GetTeamAdoption(ctx context.Context) ([]TeamAdoption, error)
	GetCustomizationDetails(ctx context.Context) ([]CustomizationDetail, error)
	GetCustomizationUsage(ctx context.Context) ([]CustomizationUsage, error)
	GetLanguageAdoption(ctx context.Context) ([]LanguageAdoption, error)
	GetStalenessData(ctx context.Context) ([]StalenessFile, error)
	GetTeamUsageSummary(ctx context.Context, days int) ([]TeamUsageSummary, error)
	GetUserMetrics(ctx context.Context, userLogin string, days int) (*UserMetricsSummary, error)
	GetMonthlyTrends(ctx context.Context, months int) ([]MonthlyTrend, error)
	GetMonthlyModelUsage(ctx context.Context, months int) ([]MonthlyModelUsage, error)
	GetMonthlyBillingUsage(ctx context.Context, months int) ([]MonthlyBillingUsage, error)
	GetBillingModelDailyCosts(ctx context.Context, month string) ([]BillingModelDailyCost, error)
	GetBillingModelForecast(ctx context.Context, month string) (*BillingModelForecast, error)
	GetUserWeeklyTrends(ctx context.Context, userLogin string, weeks int) ([]WeeklyTrend, error)
	GetAdoptionCohorts(ctx context.Context, days int) ([]AdoptionCohortDay, error)
}

// Cache wrapper for BigQuery operations
type CachedBigQueryClient struct {
	client *BigQueryClient
	cache  *Cache
}

func newCachedBigQueryClient(client *BigQueryClient, ttl time.Duration) *CachedBigQueryClient {
	return &CachedBigQueryClient{
		client: client,
		cache:  NewCache(ttl),
	}
}

func getCachedValue[T any](cache *Cache, cacheKey string, loader func() (T, error)) (T, error) {
	var zero T
	if cached, ok := cache.Get(cacheKey); ok {
		if value, ok := cached.(T); ok {
			slog.Debug("Cache hit", "key", cacheKey)
			return value, nil
		}
	}

	value, err := loader()
	if err != nil {
		return zero, err
	}

	cache.Set(cacheKey, value)
	return value, nil
}

func (c *CachedBigQueryClient) GetDailyMetrics(ctx context.Context, days *int) ([]EnterpriseMetrics, error) {
	cacheKey := fmt.Sprintf("daily_metrics_%v", days)
	return getCachedValue(c.cache, cacheKey, func() ([]EnterpriseMetrics, error) {
		return c.client.GetDailyMetrics(ctx, days)
	})
}

func (c *CachedBigQueryClient) GetAdoptionSummary(ctx context.Context) (*AdoptionSummary, error) {
	cacheKey := "adoption_summary"
	return getCachedValue(c.cache, cacheKey, func() (*AdoptionSummary, error) {
		return c.client.GetAdoptionSummary(ctx)
	})
}

func (c *CachedBigQueryClient) GetTeamAdoption(ctx context.Context) ([]TeamAdoption, error) {
	cacheKey := "team_adoption"
	return getCachedValue(c.cache, cacheKey, func() ([]TeamAdoption, error) {
		return c.client.GetTeamAdoption(ctx)
	})
}

func (c *CachedBigQueryClient) GetCustomizationDetails(ctx context.Context) ([]CustomizationDetail, error) {
	cacheKey := "customization_details"
	return getCachedValue(c.cache, cacheKey, func() ([]CustomizationDetail, error) {
		return c.client.GetCustomizationDetails(ctx)
	})
}

func (c *CachedBigQueryClient) GetCustomizationUsage(ctx context.Context) ([]CustomizationUsage, error) {
	cacheKey := "customization_usage"
	return getCachedValue(c.cache, cacheKey, func() ([]CustomizationUsage, error) {
		return c.client.GetCustomizationUsage(ctx)
	})
}

func (c *CachedBigQueryClient) GetLanguageAdoption(ctx context.Context) ([]LanguageAdoption, error) {
	cacheKey := "language_adoption"
	return getCachedValue(c.cache, cacheKey, func() ([]LanguageAdoption, error) {
		return c.client.GetLanguageAdoption(ctx)
	})
}

func (c *CachedBigQueryClient) GetStalenessData(ctx context.Context) ([]StalenessFile, error) {
	cacheKey := "staleness_data"
	return getCachedValue(c.cache, cacheKey, func() ([]StalenessFile, error) {
		return c.client.GetStalenessData(ctx)
	})
}

func (c *CachedBigQueryClient) GetTeamUsageSummary(ctx context.Context, days int) ([]TeamUsageSummary, error) {
	cacheKey := fmt.Sprintf("team_usage_summary_%d", days)
	return getCachedValue(c.cache, cacheKey, func() ([]TeamUsageSummary, error) {
		return c.client.GetTeamUsageSummary(ctx, days)
	})
}

func (c *CachedBigQueryClient) GetUserMetrics(ctx context.Context, userLogin string, days int) (*UserMetricsSummary, error) {
	cacheKey := fmt.Sprintf("user_metrics_%s_%d", userLogin, days)
	return getCachedValue(c.cache, cacheKey, func() (*UserMetricsSummary, error) {
		return c.client.GetUserMetrics(ctx, userLogin, days)
	})
}

func (c *CachedBigQueryClient) GetMonthlyTrends(ctx context.Context, months int) ([]MonthlyTrend, error) {
	cacheKey := fmt.Sprintf("monthly_trends_%d", months)
	return getCachedValue(c.cache, cacheKey, func() ([]MonthlyTrend, error) {
		return c.client.GetMonthlyTrends(ctx, months)
	})
}

func (c *CachedBigQueryClient) GetMonthlyModelUsage(ctx context.Context, months int) ([]MonthlyModelUsage, error) {
	cacheKey := fmt.Sprintf("monthly_model_usage_%d", months)
	return getCachedValue(c.cache, cacheKey, func() ([]MonthlyModelUsage, error) {
		return c.client.GetMonthlyModelUsage(ctx, months)
	})
}

func (c *CachedBigQueryClient) GetMonthlyBillingUsage(ctx context.Context, months int) ([]MonthlyBillingUsage, error) {
	cacheKey := fmt.Sprintf("monthly_billing_usage_%d", months)
	return getCachedValue(c.cache, cacheKey, func() ([]MonthlyBillingUsage, error) {
		return c.client.GetMonthlyBillingUsage(ctx, months)
	})
}

func (c *CachedBigQueryClient) GetBillingModelDailyCosts(ctx context.Context, month string) ([]BillingModelDailyCost, error) {
	cacheKey := fmt.Sprintf("billing_model_daily_costs_%s", month)
	return getCachedValue(c.cache, cacheKey, func() ([]BillingModelDailyCost, error) {
		return c.client.GetBillingModelDailyCosts(ctx, month)
	})
}

func (c *CachedBigQueryClient) GetBillingModelForecast(ctx context.Context, month string) (*BillingModelForecast, error) {
	cacheKey := fmt.Sprintf("billing_model_forecast_%s", month)
	return getCachedValue(c.cache, cacheKey, func() (*BillingModelForecast, error) {
		return c.client.GetBillingModelForecast(ctx, month)
	})
}

func (c *CachedBigQueryClient) GetUserWeeklyTrends(ctx context.Context, userLogin string, weeks int) ([]WeeklyTrend, error) {
	cacheKey := fmt.Sprintf("user_weekly_trends_%s_%d", userLogin, weeks)
	return getCachedValue(c.cache, cacheKey, func() ([]WeeklyTrend, error) {
		return c.client.GetUserWeeklyTrends(ctx, userLogin, weeks)
	})
}

func (c *CachedBigQueryClient) GetAdoptionCohorts(ctx context.Context, days int) ([]AdoptionCohortDay, error) {
	cacheKey := fmt.Sprintf("adoption_cohorts_%d", days)
	return getCachedValue(c.cache, cacheKey, func() ([]AdoptionCohortDay, error) {
		return c.client.GetAdoptionCohorts(ctx, days)
	})
}

func (c *CachedBigQueryClient) Close() {
	c.cache.Stop()
}
