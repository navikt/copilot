package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const (
	// rateLimitDelay is the delay between API calls to avoid hitting GitHub rate limits.
	// GitHub API limit is 5000 requests/hour = ~1.4/second. We use 1 second to be safe.
	rateLimitDelay = 1 * time.Second

	// maxErrorRate is the maximum percentage of failed days before we abort.
	// If more than 50% of days fail, something is fundamentally wrong.
	maxErrorRate = 50
)

func runBackfill(ctx context.Context, gh MetricsFetcher, bq MetricsStore, cfg *Config, startDate time.Time) error {
	endDate := time.Now().UTC().AddDate(0, 0, -1)

	slog.Info("Starting historical backfill",
		"start", startDate.Format("2006-01-02"),
		"end", endDate.Format("2006-01-02"),
	)

	latestDay, err := bq.GetLatestDay(ctx, cfg.EnterpriseSlug)
	if err != nil {
		slog.Warn("Could not get latest day from BigQuery", "error", err)
	} else if !latestDay.IsZero() {
		slog.Info("Found existing data", "latest_day", latestDay.Format("2006-01-02"))
		startDate = latestDay.AddDate(0, 0, 1)
		slog.Info("Adjusted start date to continue from latest", "new_start", startDate.Format("2006-01-02"))
	}

	if startDate.After(endDate) {
		slog.Info("No days to backfill - already up to date")
		return nil
	}

	totalDays := int(endDate.Sub(startDate).Hours()/24) + 1
	slog.Info("Backfill plan", "total_days", totalDays)

	successCount := 0
	errorCount := 0

	for day := startDate; !day.After(endDate); day = day.AddDate(0, 0, 1) {
		select {
		case <-ctx.Done():
			slog.Warn("Backfill interrupted", "completed", successCount, "errors", errorCount)
			return ctx.Err()
		default:
		}

		if err := ingestDay(ctx, gh, bq, cfg, day); err != nil {
			slog.Error("Failed to ingest day",
				"day", day.Format("2006-01-02"),
				"error", err,
			)
			errorCount++

			// Check if error rate exceeds threshold
			totalAttempted := successCount + errorCount
			if totalAttempted >= 10 && (errorCount*100)/totalAttempted > maxErrorRate {
				return fmt.Errorf("aborting backfill: error rate %d%% exceeds threshold %d%% (success: %d, errors: %d)",
					(errorCount*100)/totalAttempted, maxErrorRate, successCount, errorCount)
			}
			continue
		}
		successCount++

		if successCount%30 == 0 {
			slog.Info("Backfill progress",
				"completed", successCount,
				"total", totalDays,
				"errors", errorCount,
				"percent", (successCount*100)/totalDays,
			)
		}

		// Rate limiting - GitHub API allows ~1.4 req/sec
		time.Sleep(rateLimitDelay)
	}

	slog.Info("Backfill completed",
		"success", successCount,
		"errors", errorCount,
		"total_days", totalDays,
	)

	if errorCount > 0 {
		slog.Warn("Some days failed to backfill - they may not have data available")
	}

	return nil
}
