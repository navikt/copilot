package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery"
)

type BigQueryClient struct {
	client    *bigquery.Client
	projectID string
	dataset   string
	table     string
}

type RepoScanRow struct {
	ScanDate           string    `bigquery:"scan_date"`
	Org                string    `bigquery:"org"`
	Repo               string    `bigquery:"repo"`
	DefaultBranch      string    `bigquery:"default_branch"`
	PrimaryLanguage    string    `bigquery:"primary_language"`
	IsArchived         bool      `bigquery:"is_archived"`
	IsFork             bool      `bigquery:"is_fork"`
	Visibility         string    `bigquery:"visibility"`
	CreatedAt          time.Time `bigquery:"created_at"`
	PushedAt           time.Time `bigquery:"pushed_at"`
	Topics             []string  `bigquery:"topics"`
	Teams              string    `bigquery:"teams"`
	Customizations     string    `bigquery:"customizations"`
	HasAny             bool      `bigquery:"has_any_customization"`
	CustomizationCount int       `bigquery:"customization_count"`
	LoadedAt           time.Time `bigquery:"loaded_at"`
}

func NewBigQueryClient(ctx context.Context, cfg *Config) (*BigQueryClient, error) {
	client, err := bigquery.NewClient(ctx, cfg.BigQueryProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create BigQuery client: %w", err)
	}

	return &BigQueryClient{
		client:    client,
		projectID: cfg.BigQueryProjectID,
		dataset:   cfg.BigQueryDataset,
		table:     cfg.BigQueryTable,
	}, nil
}

func (c *BigQueryClient) Close() error {
	return c.client.Close()
}

func (c *BigQueryClient) EnsureTableExists(ctx context.Context) error {
	dataset := c.client.Dataset(c.dataset)
	table := dataset.Table(c.table)

	_, err := table.Metadata(ctx)
	if err == nil {
		slog.Debug("Table already exists", "dataset", c.dataset, "table", c.table)
		return nil
	}

	slog.Info("Creating table", "dataset", c.dataset, "table", c.table)

	schema := bigquery.Schema{
		{Name: "scan_date", Type: bigquery.DateFieldType, Required: true, Description: "Date of the scan"},
		{Name: "org", Type: bigquery.StringFieldType, Required: true, Description: "GitHub organization"},
		{Name: "repo", Type: bigquery.StringFieldType, Required: true, Description: "Repository name"},
		{Name: "default_branch", Type: bigquery.StringFieldType, Description: "Default branch name"},
		{Name: "primary_language", Type: bigquery.StringFieldType, Description: "Primary programming language"},
		{Name: "is_archived", Type: bigquery.BooleanFieldType, Required: true, Description: "Whether the repo is archived"},
		{Name: "is_fork", Type: bigquery.BooleanFieldType, Required: true, Description: "Whether the repo is a fork"},
		{Name: "visibility", Type: bigquery.StringFieldType, Required: true, Description: "public, private, or internal"},
		{Name: "created_at", Type: bigquery.TimestampFieldType, Description: "Repo creation time"},
		{Name: "pushed_at", Type: bigquery.TimestampFieldType, Description: "Last push time"},
		{Name: "topics", Type: bigquery.StringFieldType, Repeated: true, Description: "Repository topics"},
		{Name: "teams", Type: bigquery.JSONFieldType, Description: "Teams with access [{slug, name, permission}]"},
		{Name: "customizations", Type: bigquery.JSONFieldType, Required: true, Description: "Search results per category"},
		{Name: "has_any_customization", Type: bigquery.BooleanFieldType, Required: true, Description: "Quick filter: any customization found"},
		{Name: "customization_count", Type: bigquery.IntegerFieldType, Required: true, Description: "Number of distinct categories found"},
		{Name: "loaded_at", Type: bigquery.TimestampFieldType, Required: true, Description: "When the row was inserted"},
	}

	metadata := &bigquery.TableMetadata{
		Schema: schema,
		TimePartitioning: &bigquery.TimePartitioning{
			Type:  bigquery.DayPartitioningType,
			Field: "scan_date",
		},
		Clustering: &bigquery.Clustering{
			Fields: []string{"org", "has_any_customization", "primary_language"},
		},
		Description: "Copilot customization adoption scan results per repository",
	}

	if err := table.Create(ctx, metadata); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	slog.Info("Table created successfully")
	return nil
}

func (c *BigQueryClient) InsertScanResults(ctx context.Context, scanDate time.Time, results []RepoScanResult) error {
	if len(results) == 0 {
		slog.Warn("No results to insert")
		return nil
	}

	table := c.client.Dataset(c.dataset).Table(c.table)
	inserter := table.Inserter()

	dateStr := scanDate.Format("2006-01-02")
	loadedAt := time.Now().UTC()

	// Insert in batches of 500 to stay within BigQuery streaming insert limits
	const batchSize = 500
	for i := 0; i < len(results); i += batchSize {
		end := i + batchSize
		if end > len(results) {
			end = len(results)
		}

		var rows []*RepoScanRow
		for _, r := range results[i:end] {
			customizationsJSON, err := json.Marshal(r.Customizations)
			if err != nil {
				slog.Warn("Failed to marshal customizations", "repo", r.Repo, "error", err)
				customizationsJSON = []byte("{}")
			}

			teamsJSON, err := json.Marshal(r.Teams)
			if err != nil {
				slog.Warn("Failed to marshal teams", "repo", r.Repo, "error", err)
				teamsJSON = []byte("[]")
			}

			rows = append(rows, &RepoScanRow{
				ScanDate:           dateStr,
				Org:                r.Org,
				Repo:               r.Repo,
				DefaultBranch:      r.DefaultBranch,
				PrimaryLanguage:    r.PrimaryLanguage,
				IsArchived:         r.IsArchived,
				IsFork:             r.IsFork,
				Visibility:         r.Visibility,
				CreatedAt:          r.CreatedAt,
				PushedAt:           r.PushedAt,
				Topics:             r.Topics,
				Teams:              string(teamsJSON),
				Customizations:     string(customizationsJSON),
				HasAny:             r.HasAny,
				CustomizationCount: r.CustomizationCount,
				LoadedAt:           loadedAt,
			})
		}

		if err := inserter.Put(ctx, rows); err != nil {
			return fmt.Errorf("failed to insert batch starting at %d: %w", i, err)
		}
	}

	slog.Info("Inserted scan results", "date", dateStr, "repos", len(results))
	return nil
}

func (c *BigQueryClient) ScanDateExists(ctx context.Context, scanDate time.Time) (bool, error) {
	dateStr := scanDate.Format("2006-01-02")

	query := c.client.Query(`
		SELECT COUNT(*) as cnt
		FROM ` + "`" + c.projectID + "." + c.dataset + "." + c.table + "`" + `
		WHERE scan_date = @scanDate
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "scanDate", Value: dateStr},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to execute query: %w", err)
	}

	var row struct {
		Cnt int64 `bigquery:"cnt"`
	}
	if err := it.Next(&row); err != nil {
		return false, fmt.Errorf("failed to read result: %w", err)
	}

	return row.Cnt > 0, nil
}

func (c *BigQueryClient) DeleteScanDate(ctx context.Context, scanDate time.Time) error {
	dateStr := scanDate.Format("2006-01-02")

	query := c.client.Query(`
		DELETE FROM ` + "`" + c.projectID + "." + c.dataset + "." + c.table + "`" + `
		WHERE scan_date = @scanDate
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "scanDate", Value: dateStr},
	}

	job, err := query.Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to run delete query: %w", err)
	}

	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("delete job failed: %w", err)
	}
	if status.Err() != nil {
		return fmt.Errorf("delete query failed: %w", status.Err())
	}

	slog.Debug("Deleted existing scan data", "date", dateStr)
	return nil
}
