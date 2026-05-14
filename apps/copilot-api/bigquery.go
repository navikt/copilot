package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery"
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
	ScanDate                      string `bigquery:"scan_date" json:"scan_date"`
	TotalRepos                    int64  `bigquery:"total_repos" json:"total_repos"`
	ReposWithCustomizations       int64  `bigquery:"repos_with_customizations" json:"repos_with_customizations"`
	ReposRecentlyActive           int64  `bigquery:"repos_recently_active" json:"repos_recently_active"`
	ActiveReposWithCustomizations int64  `bigquery:"active_repos_with_customizations" json:"active_repos_with_customizations"`
	TotalAgents                   int64  `bigquery:"total_agents" json:"total_agents"`
	TotalSkills                   int64  `bigquery:"total_skills" json:"total_skills"`
	TotalInstructions             int64  `bigquery:"total_instructions" json:"total_instructions"`
	TotalPrompts                  int64  `bigquery:"total_prompts" json:"total_prompts"`
}

// TeamAdoption represents adoption metrics for a single team
type TeamAdoption struct {
	ScanDate                      string `bigquery:"scan_date" json:"scan_date"`
	Team                          string `bigquery:"team" json:"team"`
	TotalRepos                    int64  `bigquery:"total_repos" json:"total_repos"`
	ReposWithCustomizations       int64  `bigquery:"repos_with_customizations" json:"repos_with_customizations"`
	ReposRecentlyActive           int64  `bigquery:"repos_recently_active" json:"repos_recently_active"`
	ActiveReposWithCustomizations int64  `bigquery:"active_repos_with_customizations" json:"active_repos_with_customizations"`
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
	ScanDate                      string `bigquery:"scan_date" json:"scan_date"`
	Language                      string `bigquery:"language" json:"language"`
	TotalRepos                    int64  `bigquery:"total_repos" json:"total_repos"`
	ReposWithCustomizations       int64  `bigquery:"repos_with_customizations" json:"repos_with_customizations"`
	ReposRecentlyActive           int64  `bigquery:"repos_recently_active" json:"repos_recently_active"`
	ActiveReposWithCustomizations int64  `bigquery:"active_repos_with_customizations" json:"active_repos_with_customizations"`
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

	whereClause := ""
	if days != nil && *days > 0 {
		whereClause = fmt.Sprintf("WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL %d DAY)", *days)
	}

	queryStr := fmt.Sprintf("SELECT raw_record FROM %s %s ORDER BY day ASC", tableRef, whereClause)

	query := bq.client.Query(queryStr)
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	var results []EnterpriseMetrics
	for {
		var row struct {
			RawRecord string `bigquery:"raw_record"`
		}
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate results: %w", err)
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

	var summary AdoptionSummary
	err = it.Next(&summary)
	if err == iterator.Done {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read result: %w", err)
	}

	return &summary, nil
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

	var results []TeamAdoption
	for {
		var team TeamAdoption
		err := it.Next(&team)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate results: %w", err)
		}
		results = append(results, team)
	}

	return results, nil
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

	var results []CustomizationDetail
	for {
		var detail CustomizationDetail
		err := it.Next(&detail)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate results: %w", err)
		}
		results = append(results, detail)
	}

	return results, nil
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

	var results []CustomizationUsage
	for {
		var usage CustomizationUsage
		err := it.Next(&usage)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate results: %w", err)
		}
		results = append(results, usage)
	}

	return results, nil
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

	var results []LanguageAdoption
	for {
		var lang LanguageAdoption
		err := it.Next(&lang)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("iterate results: %w", err)
		}
		results = append(results, lang)
	}

	return results, nil
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

func (c *CachedBigQueryClient) GetDailyMetrics(ctx context.Context, days *int) ([]EnterpriseMetrics, error) {
	cacheKey := fmt.Sprintf("daily_metrics_%v", days)

	if cached, ok := c.cache.Get(cacheKey); ok {
		if metrics, ok := cached.([]EnterpriseMetrics); ok {
			slog.Debug("Cache hit", "key", cacheKey)
			return metrics, nil
		}
	}

	metrics, err := c.client.GetDailyMetrics(ctx, days)
	if err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, metrics)
	return metrics, nil
}

func (c *CachedBigQueryClient) GetAdoptionSummary(ctx context.Context) (*AdoptionSummary, error) {
	cacheKey := "adoption_summary"

	if cached, ok := c.cache.Get(cacheKey); ok {
		if summary, ok := cached.(*AdoptionSummary); ok {
			slog.Debug("Cache hit", "key", cacheKey)
			return summary, nil
		}
	}

	summary, err := c.client.GetAdoptionSummary(ctx)
	if err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, summary)
	return summary, nil
}

func (c *CachedBigQueryClient) GetTeamAdoption(ctx context.Context) ([]TeamAdoption, error) {
	cacheKey := "team_adoption"

	if cached, ok := c.cache.Get(cacheKey); ok {
		if teams, ok := cached.([]TeamAdoption); ok {
			slog.Debug("Cache hit", "key", cacheKey)
			return teams, nil
		}
	}

	teams, err := c.client.GetTeamAdoption(ctx)
	if err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, teams)
	return teams, nil
}

func (c *CachedBigQueryClient) GetCustomizationDetails(ctx context.Context) ([]CustomizationDetail, error) {
	cacheKey := "customization_details"

	if cached, ok := c.cache.Get(cacheKey); ok {
		if details, ok := cached.([]CustomizationDetail); ok {
			slog.Debug("Cache hit", "key", cacheKey)
			return details, nil
		}
	}

	details, err := c.client.GetCustomizationDetails(ctx)
	if err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, details)
	return details, nil
}

func (c *CachedBigQueryClient) GetCustomizationUsage(ctx context.Context) ([]CustomizationUsage, error) {
	cacheKey := "customization_usage"

	if cached, ok := c.cache.Get(cacheKey); ok {
		if usage, ok := cached.([]CustomizationUsage); ok {
			slog.Debug("Cache hit", "key", cacheKey)
			return usage, nil
		}
	}

	usage, err := c.client.GetCustomizationUsage(ctx)
	if err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, usage)
	return usage, nil
}

func (c *CachedBigQueryClient) GetLanguageAdoption(ctx context.Context) ([]LanguageAdoption, error) {
	cacheKey := "language_adoption"

	if cached, ok := c.cache.Get(cacheKey); ok {
		if langs, ok := cached.([]LanguageAdoption); ok {
			slog.Debug("Cache hit", "key", cacheKey)
			return langs, nil
		}
	}

	langs, err := c.client.GetLanguageAdoption(ctx)
	if err != nil {
		return nil, err
	}

	c.cache.Set(cacheKey, langs)
	return langs, nil
}
