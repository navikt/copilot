package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/fetchers"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

// ingestTodaySeatCounts stores today's seat totals. Errors are returned so the
// job fails and the Nais retry runs: the API has no history, so a missed day
// is lost for good. A permission 403 on both endpoints is logged and
// skipped: a retry cannot fix it, and failing would mark the whole job failed.
func ingestTodaySeatCounts(
	ctx context.Context,
	fetch func(context.Context) (*types.SeatCounts, error),
	upsert func(context.Context, time.Time, *types.SeatCounts) error,
	now time.Time,
) error {
	today := now.UTC().Truncate(24 * time.Hour)
	counts, err := fetch(ctx)
	if errors.Is(err, fetchers.ErrSeatsNoPermission) {
		slog.Error("Skipping seat count ingestion: check the token's Copilot billing permission", "error", err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("fetch seat counts: %w", err)
	}
	if err := upsert(ctx, today, counts); err != nil {
		return fmt.Errorf("store seat counts: %w", err)
	}
	slog.Info("Seat counts ingested", "date", today.Format("2006-01-02"), "scope_id", counts.ScopeID, "total", counts.Total)
	return nil
}
