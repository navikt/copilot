package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

// copilotSeatsTable holds one row per day and Copilot seat. copilot-api reads
// it to estimate licence cost per team: created_at and
// pending_cancellation_date say which months a seat covered. No backfill: the
// seats API only reports current seats.
const copilotSeatsTable = "copilot_seats"

// SeatRow is one seat on one snapshot day.
type SeatRow struct {
	SnapshotDate            civil.Date             `bigquery:"snapshot_date"`
	ScopeID                 string                 `bigquery:"scope_id"`
	UserID                  int64                  `bigquery:"user_id"`
	Login                   string                 `bigquery:"login"`
	PlanType                string                 `bigquery:"plan_type"`
	CreatedAt               bigquery.NullTimestamp `bigquery:"created_at"`
	PendingCancellationDate bigquery.NullDate      `bigquery:"pending_cancellation_date"`
	LoadedAt                time.Time              `bigquery:"loaded_at"`
}

// EnsureCopilotSeatsTableExists creates the copilot_seats table if it doesn't exist.
func (c *BigQueryClient) EnsureCopilotSeatsTableExists(ctx context.Context) error {
	table := c.client.Dataset(c.dataset).Table(copilotSeatsTable)
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
func (c *BigQueryClient) ReplaceCopilotSeats(ctx context.Context, day time.Time, rows []*SeatRow) error {
	table := c.client.Dataset(c.dataset).Table(copilotSeatsTable + "$" + day.Format("20060102"))
	return loadRowsWith(ctx, table, rows, bigquery.WriteTruncate)
}

// FetchCopilotSeats returns today's org seats as rows. Like
// FetchAllCopilotLogins it uses the org installation token: the enterprise
// seats endpoint does not accept GitHub App installation tokens.
func (c *GitHubClient) FetchCopilotSeats(ctx context.Context) ([]*SeatRow, error) {
	if c.orgHttpClient == nil {
		return nil, errors.New("org GitHub client not configured: org installation is required to list Copilot seats")
	}
	seats, _, err := c.fetchSeats(ctx, "https://api.github.com/orgs/"+c.org, c.orgHttpClient)
	if seatsPermissionDenied(err) {
		return nil, fmt.Errorf("%w: %w", errSeatsNoPermission, err)
	}
	if err != nil {
		return nil, err
	}
	return seatRows(c.org, seats, time.Now()), nil
}

func seatRows(scopeID string, seats []copilotSeat, now time.Time) []*SeatRow {
	day, loaded := civil.DateOf(now.UTC()), now.UTC()
	rows := make([]*SeatRow, 0, len(seats))
	seen := make(map[int64]bool, len(seats))
	for _, s := range seats {
		if s.Assignee.ID == 0 || seen[s.Assignee.ID] {
			continue // seat without a user, or repeated across pages
		}
		seen[s.Assignee.ID] = true
		r := &SeatRow{SnapshotDate: day, ScopeID: scopeID, UserID: s.Assignee.ID, Login: s.Assignee.Login, PlanType: s.PlanType, LoadedAt: loaded}
		if !s.CreatedAt.IsZero() {
			r.CreatedAt = bigquery.NullTimestamp{Timestamp: s.CreatedAt, Valid: true}
		}
		if d, err := civil.ParseDate(s.PendingCancellationDate); err == nil {
			r.PendingCancellationDate = bigquery.NullDate{Date: d, Valid: true}
		}
		rows = append(rows, r)
	}
	return rows
}

// ingestTodayCopilotSeats stores today's seat list. Like the seat counts it
// fails the job on error (a missed day is lost) but skips a permission 403.
func ingestTodayCopilotSeats(
	ctx context.Context,
	fetch func(context.Context) ([]*SeatRow, error),
	store func(context.Context, time.Time, []*SeatRow) error,
	now time.Time,
) error {
	rows, err := fetch(ctx)
	if errors.Is(err, errSeatsNoPermission) {
		slog.Error("Skipping seat snapshot: check the token's Copilot billing permission", "error", err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("fetch seats: %w", err)
	}
	if len(rows) == 0 {
		slog.Warn("No Copilot seats returned, nothing written")
		return nil
	}
	// The load job truncates the partition for now's day, and every row must
	// fall in it. Stamp the rows here so a fetch just before midnight cannot
	// disagree with the store after it.
	for _, r := range rows {
		r.SnapshotDate = civil.DateOf(now.UTC())
	}
	if err := store(ctx, now.UTC(), rows); err != nil {
		return fmt.Errorf("store seats: %w", err)
	}
	slog.Info("Copilot seats ingested", "date", now.UTC().Format("2006-01-02"), "rows", len(rows))
	return nil
}
