package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	// repoMetricsGADate is the first day GitHub publishes the repos-1-day
	// report. Earlier days always return ErrReportNotAvailable, so there is
	// no point starting a backfill before this.
	repoMetricsGADate = "2026-07-17"
)

// repoMetricsRateLimitDelay matches the entity backfill delay — the
// repos-1-day report goes through the same GitHub report API.
// Declared as a var so tests can disable the delay.
var repoMetricsRateLimitDelay = 1 * time.Second

type RepoMetricsFetcher interface {
	FetchDailyRepoMetrics(ctx context.Context, day time.Time) (*FetchResult, error)
}

type RepoMetricsStore interface {
	RepoMetricsDayExists(ctx context.Context, day time.Time, scopeID string) (bool, error)
	DeleteRepoMetricsDay(ctx context.Context, day time.Time, scopeID string) error
	InsertRepoMetrics(ctx context.Context, day time.Time, scope, scopeID string, records []json.RawMessage) error
}

// runRepoMetricsBackfill fills gaps in the repository_metrics table without
// touching usage_metrics, user_metrics or user_teams.
//
// The nightly job only gap-fills the last 7 days (ingestMissingSupplementary),
// so days between the report's GA date and the deploy of repository ingestion
// fall outside that window permanently. This backfill closes those gaps.
//
// Days are skipped individually when data already exists, so the command is
// cheap to re-run and handles interior gaps — unlike the entity backfill,
// which resumes from a single high-water mark.
func runRepoMetricsBackfill(ctx context.Context, gh RepoMetricsFetcher, bq RepoMetricsStore, cfg *Config, startDay time.Time, force bool) error {
	endDay := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -1)

	slog.Info("Starting repository metrics backfill",
		"start", startDay.Format("2006-01-02"),
		"end", endDay.Format("2006-01-02"),
		"force", force,
	)

	if endDay.Before(startDay) {
		slog.Info("No days to backfill - start day is after yesterday")
		return nil
	}

	var ingested, skipped, unavailable int
	var failedDays []string

	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		select {
		case <-ctx.Done():
			slog.Warn("Repository metrics backfill interrupted",
				"ingested", ingested, "errors", len(failedDays))
			return ctx.Err()
		default:
		}

		dayStr := day.Format("2006-01-02")

		// Cheap skip using the enterprise slug. If the report was stored under
		// an org scope_id instead, this reports "not exists" and we fall
		// through to upsertReport, which re-checks with the real scope.
		if !force {
			exists, err := bq.RepoMetricsDayExists(ctx, day, cfg.EnterpriseSlug)
			if err != nil {
				slog.Warn("Failed to check repo-metrics existence", "day", dayStr, "error", err)
			} else if exists {
				skipped++
				continue
			}
		}

		result, err := gh.FetchDailyRepoMetrics(ctx, day)
		if err != nil {
			if errors.Is(err, ErrReportNotAvailable) {
				slog.Info("Repository report not available", "day", dayStr)
				unavailable++
			} else {
				slog.Error("Failed to fetch repository metrics", "day", dayStr, "error", err)
				failedDays = append(failedDays, dayStr)
			}
			time.Sleep(repoMetricsRateLimitDelay)
			continue
		}

		// Never delete existing rows for an empty response — an upstream
		// hiccup should not wipe a day that already has data.
		if len(result.Records) == 0 {
			slog.Warn("No repository records returned for day", "day", dayStr)
			time.Sleep(repoMetricsRateLimitDelay)
			continue
		}

		if err := upsertReport(ctx, bq.RepoMetricsDayExists, bq.DeleteRepoMetricsDay, bq.InsertRepoMetrics,
			day, result); err != nil {
			if errors.Is(err, ErrStreamingBuffer) {
				slog.Info("Skipping repo-metrics re-import (streaming buffer not yet flushed, re-run in ~90 min)", "day", dayStr)
				skipped++
			} else {
				// The insert is a streaming Put and is not atomic: a row-level
				// failure can leave the day partially written. A later run
				// would see rows present and skip it, so the day is named
				// explicitly and needs --force to be repaired.
				slog.Error("Failed to store repository metrics (day may be partially written, re-run with --force)", "day", dayStr, "error", err)
				failedDays = append(failedDays, dayStr)
			}
			time.Sleep(repoMetricsRateLimitDelay)
			continue
		}

		slog.Info("Ingested repository metrics",
			"day", dayStr,
			"scope", result.Scope,
			"records", len(result.Records),
		)
		ingested++
		time.Sleep(repoMetricsRateLimitDelay)
	}

	slog.Info("Repository metrics backfill completed",
		"ingested", ingested,
		"skipped", skipped,
		"unavailable", unavailable,
		"errors", len(failedDays),
		"failed_days", failedDays,
	)

	if len(failedDays) > 0 {
		return fmt.Errorf("repository metrics backfill finished with %d failed day(s): %s — re-run with --force, a failed day may have been partially written and would otherwise be skipped",
			len(failedDays), strings.Join(failedDays, ", "))
	}
	return nil
}
