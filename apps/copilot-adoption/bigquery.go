package main

import (
	"bytes"
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
	ScanDate                string     `bigquery:"scan_date"`
	Org                     string     `bigquery:"org"`
	Repo                    string     `bigquery:"repo"`
	DefaultBranch           string     `bigquery:"default_branch"`
	PrimaryLanguage         string     `bigquery:"primary_language"`
	IsArchived              bool       `bigquery:"is_archived"`
	IsFork                  bool       `bigquery:"is_fork"`
	Visibility              string     `bigquery:"visibility"`
	CreatedAt               time.Time  `bigquery:"created_at"`
	PushedAt                time.Time  `bigquery:"pushed_at"`
	DefaultBranchLastCommit *time.Time `bigquery:"default_branch_last_commit"`
	Topics                  []string   `bigquery:"topics"`
	Teams                   string     `bigquery:"teams"`
	Customizations          string     `bigquery:"customizations"`
	HasAny                  bool       `bigquery:"has_any_customization"`
	CustomizationCount      int        `bigquery:"customization_count"`
	LoadedAt                time.Time  `bigquery:"loaded_at"`
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

	md, err := table.Metadata(ctx)
	if err == nil {
		slog.Debug("Table already exists", "dataset", c.dataset, "table", c.table)
		return c.ensureColumns(ctx, table, md)
	}

	slog.Info("Creating table", "dataset", c.dataset, "table", c.table)

	metadata := &bigquery.TableMetadata{
		Schema: desiredSchema(),
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

// desiredSchema returns the canonical schema for the repo_scan table.
// Used both for table creation and for detecting missing columns.
func desiredSchema() bigquery.Schema {
	return bigquery.Schema{
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
		{Name: "default_branch_last_commit", Type: bigquery.TimestampFieldType, Description: "Last commit to default branch"},
		{Name: "topics", Type: bigquery.StringFieldType, Repeated: true, Description: "Repository topics"},
		{Name: "teams", Type: bigquery.JSONFieldType, Description: "Teams with access [{slug, name, permission}]"},
		{Name: "customizations", Type: bigquery.JSONFieldType, Required: true, Description: "Search results per category"},
		{Name: "has_any_customization", Type: bigquery.BooleanFieldType, Required: true, Description: "Quick filter: any customization found"},
		{Name: "customization_count", Type: bigquery.IntegerFieldType, Required: true, Description: "Number of distinct categories found"},
		{Name: "loaded_at", Type: bigquery.TimestampFieldType, Required: true, Description: "When the row was inserted"},
	}
}

// ensureColumns adds any columns present in desiredSchema() but missing from the live table.
func (c *BigQueryClient) ensureColumns(ctx context.Context, table *bigquery.Table, md *bigquery.TableMetadata) error {
	existing := make(map[string]bool, len(md.Schema))
	for _, f := range md.Schema {
		existing[f.Name] = true
	}

	var missing bigquery.Schema
	for _, f := range desiredSchema() {
		if !existing[f.Name] {
			missing = append(missing, f)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	newSchema := append(md.Schema, missing...)
	update := bigquery.TableMetadataToUpdate{Schema: newSchema}
	if _, err := table.Update(ctx, update, md.ETag); err != nil {
		return fmt.Errorf("failed to add columns %v: %w", missing, err)
	}

	names := make([]string, len(missing))
	for i, f := range missing {
		names[i] = f.Name
	}
	slog.Info("Added missing columns to table", "columns", names)
	return nil
}

func (c *BigQueryClient) InsertScanResults(ctx context.Context, scanDate time.Time, results []RepoScanResult) error {
	if len(results) == 0 {
		slog.Warn("No results to insert")
		return nil
	}

	dateStr := scanDate.Format("2006-01-02")
	loadedAt := time.Now().UTC()

	// Build rows
	var rows []*RepoScanRow
	for _, r := range results {
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
			ScanDate:                dateStr,
			Org:                     r.Org,
			Repo:                    r.Repo,
			DefaultBranch:           r.DefaultBranch,
			PrimaryLanguage:         r.PrimaryLanguage,
			IsArchived:              r.IsArchived,
			IsFork:                  r.IsFork,
			Visibility:              r.Visibility,
			CreatedAt:               r.CreatedAt,
			PushedAt:                r.PushedAt,
			DefaultBranchLastCommit: r.DefaultBranchLastCommit,
			Topics:                  r.Topics,
			Teams:                   string(teamsJSON),
			Customizations:          string(customizationsJSON),
			HasAny:                  r.HasAny,
			CustomizationCount:      r.CustomizationCount,
			LoadedAt:                loadedAt,
		})
	}

	// Use load job instead of streaming inserts - no streaming buffer, supports WriteTruncate
	// Partition decorator: table$YYYYMMDD - replaces entire partition atomically
	partitionDecorator := scanDate.Format("20060102")
	table := c.client.Dataset(c.dataset).Table(c.table + "$" + partitionDecorator)

	// Create JSONL data in memory
	var buf bytes.Buffer
	for _, row := range rows {
		jsonMap := map[string]any{
			"scan_date":             row.ScanDate,
			"org":                   row.Org,
			"repo":                  row.Repo,
			"default_branch":        row.DefaultBranch,
			"primary_language":      row.PrimaryLanguage,
			"is_archived":           row.IsArchived,
			"is_fork":               row.IsFork,
			"visibility":            row.Visibility,
			"created_at":            row.CreatedAt.Format(time.RFC3339),
			"pushed_at":             row.PushedAt.Format(time.RFC3339),
			"topics":                row.Topics,
			"teams":                 json.RawMessage(row.Teams),
			"customizations":        json.RawMessage(row.Customizations),
			"has_any_customization": row.HasAny,
			"customization_count":   row.CustomizationCount,
			"loaded_at":             row.LoadedAt.Format(time.RFC3339),
		}
		if row.DefaultBranchLastCommit != nil {
			jsonMap["default_branch_last_commit"] = row.DefaultBranchLastCommit.Format(time.RFC3339)
		}
		jsonRow, err := json.Marshal(jsonMap)
		if err != nil {
			return fmt.Errorf("failed to marshal row for %s: %w", row.Repo, err)
		}
		buf.Write(jsonRow)
		buf.WriteByte('\n')
	}

	source := bigquery.NewReaderSource(&buf)
	source.SourceFormat = bigquery.JSON

	loader := table.LoaderFrom(source)
	loader.WriteDisposition = bigquery.WriteTruncate // Replace entire partition

	job, err := loader.Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to start load job: %w", err)
	}

	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("load job failed: %w", err)
	}
	if status.Err() != nil {
		return fmt.Errorf("load job error: %w", status.Err())
	}

	slog.Info("Loaded scan results", "date", dateStr, "repos", len(results), "job_id", job.ID())
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
