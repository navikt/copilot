package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// seatCountsTable holds one row per day and scope with the Copilot seat totals.
// There is no backfill: the seats API only reports the current state, so the
// history starts on the day this table was first deployed.
const seatCountsTable = "seat_counts"

// EnsureSeatCountsTableExists creates the seat_counts table if it doesn't exist.
func (c *BigQueryClient) EnsureSeatCountsTableExists(ctx context.Context) error {
	table := c.client.Dataset(c.dataset).Table(seatCountsTable)
	if _, err := table.Metadata(ctx); err == nil {
		return nil
	}

	slog.Info("Creating seat_counts table", "dataset", c.dataset)
	metadata := &bigquery.TableMetadata{
		Schema: bigquery.Schema{
			{Name: "snapshot_date", Type: bigquery.DateFieldType, Required: true, Description: "Date the seat count was taken"},
			{Name: "scope", Type: bigquery.StringFieldType, Required: true, Description: "enterprise or organization"},
			{Name: "scope_id", Type: bigquery.StringFieldType, Required: true, Description: "Enterprise slug or org name"},
			{Name: "total_seats", Type: bigquery.IntegerFieldType, Required: true, Description: "Total Copilot seats (total_seats from the API)"},
			{Name: "active_seats", Type: bigquery.IntegerFieldType, Required: true, Description: "Seats not pending cancellation"},
			{Name: "pending_cancellation_seats", Type: bigquery.IntegerFieldType, Required: true, Description: "Seats with a pending cancellation date"},
			{Name: "loaded_at", Type: bigquery.TimestampFieldType, Required: true, Description: "When the row was written"},
		},
		TimePartitioning: &bigquery.TimePartitioning{
			Type:  bigquery.MonthPartitioningType,
			Field: "snapshot_date",
		},
		Clustering:  &bigquery.Clustering{Fields: []string{"scope_id"}},
		Description: "Daily Copilot seat totals per scope (no per-person rows). History starts at first deploy; no backfill.",
	}
	if err := table.Create(ctx, metadata); err != nil {
		return fmt.Errorf("failed to create seat_counts table: %w", err)
	}
	return nil
}

// seatCountsMergeSQL upserts one row keyed on (snapshot_date, scope_id), so a
// re-run on the same day replaces the row instead of adding a second one. A
// single DML statement also avoids the streaming-buffer window that blocks
// delete-then-insert.
func seatCountsMergeSQL(tableRef string) string {
	return `MERGE ` + tableRef + ` t
USING (SELECT @date snapshot_date, @scope scope, @scope_id scope_id, @total total_seats,
  @active active_seats, @pending pending_cancellation_seats, @loaded loaded_at) s
ON t.snapshot_date = s.snapshot_date AND t.scope_id = s.scope_id
WHEN MATCHED THEN UPDATE SET scope = s.scope, total_seats = s.total_seats, active_seats = s.active_seats,
  pending_cancellation_seats = s.pending_cancellation_seats, loaded_at = s.loaded_at
WHEN NOT MATCHED THEN INSERT (snapshot_date, scope, scope_id, total_seats, active_seats, pending_cancellation_seats, loaded_at)
  VALUES (s.snapshot_date, s.scope, s.scope_id, s.total_seats, s.active_seats, s.pending_cancellation_seats, s.loaded_at)`
}

// UpsertSeatCounts writes the seat totals for a day, replacing any earlier row
// for the same day and scope_id.
func (c *BigQueryClient) UpsertSeatCounts(ctx context.Context, day time.Time, counts *SeatCounts) error {
	q := c.client.Query(seatCountsMergeSQL("`" + c.projectID + "." + c.dataset + "." + seatCountsTable + "`"))
	q.Parameters = []bigquery.QueryParameter{
		{Name: "date", Value: civil.DateOf(day)},
		{Name: "scope", Value: counts.Scope},
		{Name: "scope_id", Value: counts.ScopeID},
		{Name: "total", Value: counts.Total},
		{Name: "active", Value: counts.Total - counts.PendingCancellation},
		{Name: "pending", Value: counts.PendingCancellation},
		{Name: "loaded", Value: time.Now().UTC()},
	}
	_, err := q.Read(ctx)
	return err
}

// ingestTodaySeatCounts stores today's seat totals. Errors are logged as
// warnings — this is supplementary data.
func ingestTodaySeatCounts(
	ctx context.Context,
	fetch func(context.Context) (*SeatCounts, error),
	upsert func(context.Context, time.Time, *SeatCounts) error,
	now time.Time,
) {
	today := now.UTC().Truncate(24 * time.Hour)
	counts, err := fetch(ctx)
	if err != nil {
		slog.Warn("Failed to fetch Copilot seat counts", "error", err)
		return
	}
	if err := upsert(ctx, today, counts); err != nil {
		slog.Warn("Failed to store Copilot seat counts", "error", err)
		return
	}
	slog.Info("Seat counts ingested", "date", today.Format("2006-01-02"), "scope_id", counts.ScopeID, "total", counts.Total)
}
