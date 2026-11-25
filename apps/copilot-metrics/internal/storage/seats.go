package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

// copilotSeatsTable holds one row per day and Copilot seat. copilot-api reads
// it to estimate licence cost per team: created_at and
// pending_cancellation_date say which months a seat covered. No backfill: the
// seats API only reports current seats.
const copilotSeatsTable = "copilot_seats"

// EnsureCopilotSeatsTableExists creates the copilot_seats table if it doesn't exist.
func (c *BigQueryClient) EnsureCopilotSeatsTableExists(ctx context.Context) error {
	table := c.Client.Dataset(c.dataset).Table(copilotSeatsTable)
	if _, err := table.Metadata(ctx); err == nil {
		return nil
	}
	slog.Info("Creating copilot_seats table", "dataset", c.dataset)
	metadata := &bigquery.TableMetadata{
		Schema: bigquery.Schema{
			{Name: "snapshot_date", Type: bigquery.DateFieldType, Required: true, Description: "Date the seat list was read"},
			{Name: "scope_id", Type: bigquery.StringFieldType, Required: true, Description: "Enterprise slug or org name"},
			{Name: "user_id", Type: bigquery.IntegerFieldType, Required: true, Description: "GitHub user id of the assignee"},
			{Name: "login", Type: bigquery.StringFieldType, Required: true, Description: "GitHub login of the assignee"},
			{Name: "plan_type", Type: bigquery.StringFieldType, Description: "business or enterprise"},
			{Name: "created_at", Type: bigquery.TimestampFieldType, Description: "When the current seat was assigned"},
			{Name: "pending_cancellation_date", Type: bigquery.DateFieldType, Description: "First day of the next billing cycle, if cancelled; the seat's last day is the day before"},
			{Name: "loaded_at", Type: bigquery.TimestampFieldType, Required: true, Description: "When the row was written"},
		},
		TimePartitioning: &bigquery.TimePartitioning{Type: bigquery.DayPartitioningType, Field: "snapshot_date", Expiration: 400 * 24 * time.Hour},
		Clustering:       &bigquery.Clustering{Fields: []string{"user_id"}},
		Description:      "Daily snapshot of Copilot seats. History starts at first deploy; no backfill. Partitions expire after 400 days.",
	}
	if err := table.Create(ctx, metadata); err != nil {
		return fmt.Errorf("failed to create copilot_seats table: %w", err)
	}
	return nil
}

// ReplaceCopilotSeats loads the day's seats into the day's partition,
// replacing it, so a re-run the same day leaves one copy.
func (c *BigQueryClient) ReplaceCopilotSeats(ctx context.Context, day time.Time, rows []*types.SeatRow) error {
	table := c.Client.Dataset(c.dataset).Table(copilotSeatsTable + "$" + day.Format("20060102"))
	return loadRowsWith(ctx, table, rows, bigquery.WriteTruncate)
}
