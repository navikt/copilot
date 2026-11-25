package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/civil"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/fetchers"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

// ingestTodayCopilotSeats stores today's seat list. Like the seat counts it
// fails the job on error (a missed day is lost) but skips a permission 403.
func ingestTodayCopilotSeats(
	ctx context.Context,
	fetch func(context.Context) ([]*types.SeatRow, error),
	store func(context.Context, time.Time, []*types.SeatRow) error,
	now time.Time,
) error {
	rows, err := fetch(ctx)
	if errors.Is(err, fetchers.ErrSeatsNoPermission) {
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
