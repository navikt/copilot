package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/fetchers"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/storage"
)

// runMimirDaily copies each day in [from, to] and replaces any earlier copy.
// A day with no rows is an error and leaves BigQuery untouched, so a rerun
// past Mimir's retention never wipes history.
func runMimirDaily(ctx context.Context, m *fetchers.MimirClient, bq *storage.BigQueryClient, from, to time.Time) error {
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		rows, err := fetchers.CollectMimirDay(ctx, m, day)
		if err != nil {
			return fmt.Errorf("%s: %w", day.Format("2006-01-02"), err)
		}
		if len(rows) == 0 {
			return fmt.Errorf("%s: Mimir returned no rows", day.Format("2006-01-02"))
		}
		if err := bq.ReplaceMimirDay(ctx, day, rows); err != nil {
			return fmt.Errorf("%s: %w", day.Format("2006-01-02"), err)
		}
		slog.Info("Copied Mimir aggregates", "day", day.Format("2006-01-02"), "rows", len(rows))
	}
	return nil
}
