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

type UsageMetricsRow struct {
	Day       string    `bigquery:"day"`
	Scope     string    `bigquery:"scope"`
	ScopeID   string    `bigquery:"scope_id"`
	RawRecord string    `bigquery:"raw_record"`
	LoadedAt  time.Time `bigquery:"loaded_at"`
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
		{Name: "day", Type: bigquery.DateFieldType, Required: true, Description: "Calendar day of the metrics"},
		{Name: "scope", Type: bigquery.StringFieldType, Required: true, Description: "enterprise or organization"},
		{Name: "scope_id", Type: bigquery.StringFieldType, Required: true, Description: "Enterprise/org identifier"},
		{Name: "raw_record", Type: bigquery.JSONFieldType, Required: true, Description: "Full NDJSON record as-is"},
		{Name: "loaded_at", Type: bigquery.TimestampFieldType, Required: true, Description: "When the row was inserted"},
	}

	metadata := &bigquery.TableMetadata{
		Schema: schema,
		TimePartitioning: &bigquery.TimePartitioning{
			Type:  bigquery.DayPartitioningType,
			Field: "day",
		},
		Clustering: &bigquery.Clustering{
			Fields: []string{"scope", "scope_id"},
		},
		Description: "GitHub Copilot usage metrics from the Usage Metrics API",
	}

	if err := table.Create(ctx, metadata); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	slog.Info("Table created successfully")
	return nil
}

func (c *BigQueryClient) InsertMetrics(ctx context.Context, day time.Time, scope, scopeID string, records []json.RawMessage) error {
	if len(records) == 0 {
		slog.Warn("No records to insert", "day", day.Format("2006-01-02"))
		return nil
	}

	table := c.client.Dataset(c.dataset).Table(c.table)
	inserter := table.Inserter()

	dayStr := day.Format("2006-01-02")
	loadedAt := time.Now().UTC()

	var rows []*UsageMetricsRow
	for _, record := range records {
		rows = append(rows, &UsageMetricsRow{
			Day:       dayStr,
			Scope:     scope,
			ScopeID:   scopeID,
			RawRecord: string(record),
			LoadedAt:  loadedAt,
		})
	}

	if err := inserter.Put(ctx, rows); err != nil {
		return fmt.Errorf("failed to insert rows: %w", err)
	}

	slog.Info("Inserted metrics", "day", dayStr, "records", len(rows))
	return nil
}

func (c *BigQueryClient) DayExists(ctx context.Context, day time.Time, scopeID string) (bool, error) {
	dayStr := day.Format("2006-01-02")

	query := c.client.Query(`
		SELECT COUNT(*) as cnt
		FROM ` + "`" + c.projectID + "." + c.dataset + "." + c.table + "`" + `
		WHERE day = @day AND scope_id = @scopeID
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "day", Value: dayStr},
		{Name: "scopeID", Value: scopeID},
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

func (c *BigQueryClient) DeleteDay(ctx context.Context, day time.Time, scopeID string) error {
	dayStr := day.Format("2006-01-02")

	query := c.client.Query(`
		DELETE FROM ` + "`" + c.projectID + "." + c.dataset + "." + c.table + "`" + `
		WHERE day = @day AND scope_id = @scopeID
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "day", Value: dayStr},
		{Name: "scopeID", Value: scopeID},
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

	slog.Debug("Deleted existing data for day", "day", dayStr, "scope_id", scopeID)
	return nil
}

func (c *BigQueryClient) GetLatestDay(ctx context.Context, scopeID string) (time.Time, error) {
	query := c.client.Query(`
		SELECT MAX(day) as latest_day
		FROM ` + "`" + c.projectID + "." + c.dataset + "." + c.table + "`" + `
		WHERE scope_id = @scopeID
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "scopeID", Value: scopeID},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to execute query: %w", err)
	}

	var row struct {
		LatestDay bigquery.NullDate `bigquery:"latest_day"`
	}
	if err := it.Next(&row); err != nil {
		return time.Time{}, fmt.Errorf("failed to read result: %w", err)
	}

	if !row.LatestDay.Valid {
		return time.Time{}, nil
	}

	return time.Date(
		row.LatestDay.Date.Year,
		row.LatestDay.Date.Month,
		row.LatestDay.Date.Day,
		0, 0, 0, 0, time.UTC,
	), nil
}
