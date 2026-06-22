package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
)

// ErrStreamingBuffer is returned when BigQuery rejects a DELETE because rows are
// still in the streaming buffer (typically within 30–90 min of being inserted).
// Callers should skip the day and retry after the buffer has flushed.
var ErrStreamingBuffer = errors.New("rows still in streaming buffer")

type BigQueryClient struct {
	client           *bigquery.Client
	projectID        string
	dataset          string
	table            string
	userTeamsTable   string
	userMetricsTable string
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
		client:           client,
		projectID:        cfg.BigQueryProjectID,
		dataset:          cfg.BigQueryDataset,
		table:            cfg.BigQueryTable,
		userTeamsTable:   cfg.BigQueryUserTeamsTable,
		userMetricsTable: cfg.BigQueryUserMetricsTable,
	}, nil
}

func (c *BigQueryClient) Close() error {
	return c.client.Close()
}

func (c *BigQueryClient) EnsureTableExists(ctx context.Context) error {
	return c.ensureMetricsTable(ctx, c.table, "GitHub Copilot usage metrics from the Usage Metrics API")
}

func (c *BigQueryClient) EnsureUserTeamsTableExists(ctx context.Context) error {
	return c.ensureMetricsTable(ctx, c.userTeamsTable, "GitHub Copilot user-to-team mappings from the Usage Metrics API")
}

func (c *BigQueryClient) EnsureUserMetricsTableExists(ctx context.Context) error {
	return c.ensureMetricsTable(ctx, c.userMetricsTable, "GitHub Copilot per-user usage metrics from the Usage Metrics API")
}

// ensureMetricsTable creates a table with the standard metrics schema if it doesn't exist.
func (c *BigQueryClient) ensureMetricsTable(ctx context.Context, tableName, description string) error {
	dataset := c.client.Dataset(c.dataset)
	table := dataset.Table(tableName)

	_, err := table.Metadata(ctx)
	if err == nil {
		slog.Debug("Table already exists", "dataset", c.dataset, "table", tableName)
		return nil
	}

	slog.Info("Creating table", "dataset", c.dataset, "table", tableName)

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
		Description: description,
	}

	if err := table.Create(ctx, metadata); err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	slog.Info("Table created successfully", "table", tableName)
	return nil
}

func (c *BigQueryClient) InsertMetrics(ctx context.Context, day time.Time, scope, scopeID string, records []json.RawMessage) error {
	return c.insertRecords(ctx, c.table, day, scope, scopeID, records)
}

func (c *BigQueryClient) InsertUserTeams(ctx context.Context, day time.Time, scope, scopeID string, records []json.RawMessage) error {
	return c.insertRecords(ctx, c.userTeamsTable, day, scope, scopeID, records)
}

func (c *BigQueryClient) InsertUserMetrics(ctx context.Context, day time.Time, scope, scopeID string, records []json.RawMessage) error {
	return c.insertRecords(ctx, c.userMetricsTable, day, scope, scopeID, records)
}

func (c *BigQueryClient) insertRecords(ctx context.Context, tableName string, day time.Time, scope, scopeID string, records []json.RawMessage) error {
	if len(records) == 0 {
		slog.Warn("No records to insert", "table", tableName, "day", day.Format("2006-01-02"))
		return nil
	}

	table := c.client.Dataset(c.dataset).Table(tableName)
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
		return fmt.Errorf("failed to insert rows into %s: %w", tableName, err)
	}

	slog.Info("Inserted records", "table", tableName, "day", dayStr, "records", len(rows))
	return nil
}

func (c *BigQueryClient) DayExists(ctx context.Context, day time.Time, scopeID string) (bool, error) {
	return c.dayExistsInTable(ctx, c.table, day, scopeID)
}

func (c *BigQueryClient) UserTeamsDayExists(ctx context.Context, day time.Time, scopeID string) (bool, error) {
	return c.dayExistsInTable(ctx, c.userTeamsTable, day, scopeID)
}

func (c *BigQueryClient) UserMetricsDayExists(ctx context.Context, day time.Time, scopeID string) (bool, error) {
	return c.dayExistsInTable(ctx, c.userMetricsTable, day, scopeID)
}

func (c *BigQueryClient) dayExistsInTable(ctx context.Context, tableName string, day time.Time, scopeID string) (bool, error) {
	dayStr := day.Format("2006-01-02")

	query := c.client.Query(`
		SELECT COUNT(*) as cnt
		FROM ` + "`" + c.projectID + "." + c.dataset + "." + tableName + "`" + `
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
	return c.deleteDayFromTable(ctx, c.table, day, scopeID)
}

func (c *BigQueryClient) DeleteUserTeamsDay(ctx context.Context, day time.Time, scopeID string) error {
	return c.deleteDayFromTable(ctx, c.userTeamsTable, day, scopeID)
}

func (c *BigQueryClient) DeleteUserMetricsDay(ctx context.Context, day time.Time, scopeID string) error {
	return c.deleteDayFromTable(ctx, c.userMetricsTable, day, scopeID)
}

func (c *BigQueryClient) deleteDayFromTable(ctx context.Context, tableName string, day time.Time, scopeID string) error {
	dayStr := day.Format("2006-01-02")

	query := c.client.Query(`
		DELETE FROM ` + "`" + c.projectID + "." + c.dataset + "." + tableName + "`" + `
		WHERE day = @day AND scope_id = @scopeID
	`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "day", Value: dayStr},
		{Name: "scopeID", Value: scopeID},
	}

	return c.runDeleteQuery(ctx, query, tableName)
}

// runDeleteQuery runs a DELETE query and maps streaming buffer errors to ErrStreamingBuffer.
// All delete functions should use this instead of calling query.Run + job.Wait directly.
func (c *BigQueryClient) runDeleteQuery(ctx context.Context, query *bigquery.Query, table string) error {
	job, err := query.Run(ctx)
	if err != nil {
		return fmt.Errorf("failed to run delete query: %w", err)
	}

	status, err := job.Wait(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "streaming buffer") {
			return fmt.Errorf("%w: table=%s", ErrStreamingBuffer, table)
		}
		return fmt.Errorf("delete job failed: %w", err)
	}
	if status.Err() != nil {
		if strings.Contains(status.Err().Error(), "streaming buffer") {
			return fmt.Errorf("%w: table=%s", ErrStreamingBuffer, table)
		}
		return fmt.Errorf("delete query failed: %w", status.Err())
	}
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
