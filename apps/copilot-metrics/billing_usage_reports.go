package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

var billingUsageReportRateLimitDelay = 500 * time.Millisecond

// UsageReportFetcher fetches organization billing usage report data for a day.
type UsageReportFetcher interface {
	FetchOrganizationUsage(ctx context.Context, org string, day time.Time) (*OrganizationBillingUsageResponse, error)
}

// UsageReportStore stores organization billing usage report data.
type UsageReportStore interface {
	DeleteBillingUsageReportDay(ctx context.Context, day time.Time, org string) error
	InsertBillingUsageReportDay(ctx context.Context, day time.Time, org string, items []OrganizationBillingUsageItem) error
	GetLatestBillingUsageReportDay(ctx context.Context, org string) (time.Time, error)
}

func ingestBillingUsageReportDay(
	ctx context.Context,
	fetcher UsageReportFetcher,
	store UsageReportStore,
	cfg *Config,
	day time.Time,
	force bool,
) error {
	day = day.UTC().Truncate(24 * time.Hour)
	dayStr := day.Format("2006-01-02")
	org := cfg.OrganizationSlug

	resp, err := fetcher.FetchOrganizationUsage(ctx, org, day)
	if err != nil {
		return fmt.Errorf("fetch organization billing usage report: %w", err)
	}

	if len(resp.UsageItems) == 0 {
		slog.Info("No billing usage report rows returned", "day", dayStr, "org", org)
		return nil
	}

	// Idempotent re-run behavior.
	if err := store.DeleteBillingUsageReportDay(ctx, day, org); err != nil {
		slog.Warn("Failed to delete existing billing usage report rows (continuing)", "day", dayStr, "org", org, "error", err)
	}

	if err := store.InsertBillingUsageReportDay(ctx, day, org, resp.UsageItems); err != nil {
		return fmt.Errorf("insert billing usage report rows: %w", err)
	}

	return nil
}

func ingestMissingBillingUsageReports(ctx context.Context, fetcher UsageReportFetcher, store UsageReportStore, cfg *Config) error {
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)

	latestDay, err := store.GetLatestBillingUsageReportDay(ctx, cfg.OrganizationSlug)
	if err != nil {
		slog.Warn("Could not get latest billing usage report day, ingesting yesterday only", "error", err)
		return ingestBillingUsageReportDay(ctx, fetcher, store, cfg, yesterday, false)
	}

	startDate := yesterday
	if !latestDay.IsZero() {
		startDate = latestDay.AddDate(0, 0, 1)
	}

	if startDate.After(yesterday) {
		slog.Info("Billing usage reports already up to date", "latest_day", latestDay.Format("2006-01-02"))
		return nil
	}

	for day := startDate; !day.After(yesterday); day = day.AddDate(0, 0, 1) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := ingestBillingUsageReportDay(ctx, fetcher, store, cfg, day, false); err != nil {
			return fmt.Errorf("ingest billing usage report day %s: %w", day.Format("2006-01-02"), err)
		}

		// Keep requests below API thresholds.
		time.Sleep(billingUsageReportRateLimitDelay)
	}

	return nil
}

func runBillingUsageReportBackfill(
	ctx context.Context,
	fetcher UsageReportFetcher,
	store UsageReportStore,
	cfg *Config,
	startDate time.Time,
	force bool,
) error {
	endDate := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	startDate = startDate.UTC().Truncate(24 * time.Hour)

	if !force {
		latestDay, err := store.GetLatestBillingUsageReportDay(ctx, cfg.OrganizationSlug)
		if err != nil {
			slog.Warn("Could not get latest billing usage report day from BigQuery", "error", err)
		} else if !latestDay.IsZero() {
			next := latestDay.AddDate(0, 0, 1)
			if next.After(startDate) {
				startDate = next
			}
		}
	}

	if startDate.After(endDate) {
		slog.Info("No billing usage report days to backfill")
		return nil
	}

	slog.Info("Starting billing usage report backfill",
		"from", startDate.Format("2006-01-02"),
		"to", endDate.Format("2006-01-02"),
		"force", force,
	)

	successCount := 0
	errorCount := 0
	for day := startDate; !day.After(endDate); day = day.AddDate(0, 0, 1) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := ingestBillingUsageReportDay(ctx, fetcher, store, cfg, day, force); err != nil {
			errorCount++
			slog.Warn("Billing usage report day ingestion failed",
				"day", day.Format("2006-01-02"),
				"error", err,
			)
		} else {
			successCount++
		}

		time.Sleep(billingUsageReportRateLimitDelay)
	}

	slog.Info("Billing usage report backfill completed",
		"success", successCount,
		"errors", errorCount,
	)

	if errorCount > 0 && successCount == 0 {
		return fmt.Errorf("billing usage report backfill failed for all days")
	}
	return nil
}
