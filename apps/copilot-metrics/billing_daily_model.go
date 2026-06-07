package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const billingDailyModelRateLimitDelay = 500 * time.Millisecond

type BillingDailyModelFetcher interface {
	FetchDailyUsage(ctx context.Context, day time.Time) (*BillingUsageResponse, error)
}

type BillingDailyModelStore interface {
	DeleteBillingUsageDailyModelDay(ctx context.Context, day time.Time, scopeID string) error
	InsertBillingUsageDailyModelDay(ctx context.Context, day time.Time, scopeID string, items []BillingUsageItem) error
	GetLatestBillingUsageDailyModelDay(ctx context.Context, scopeID string) (time.Time, error)
}

func runBillingDailyModelBackfill(ctx context.Context, billingClient BillingDailyModelFetcher, bqClient BillingDailyModelStore, config *Config, startDay time.Time, force bool) error {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	endDay := today.AddDate(0, 0, -1)
	if endDay.Before(startDay) {
		slog.Info("Billing daily model backfill skipped: start day is after yesterday", "start_day", startDay.Format("2006-01-02"))
		return nil
	}

	effectiveStart := startDay
	if !force {
		latest, err := bqClient.GetLatestBillingUsageDailyModelDay(ctx, config.EnterpriseSlug)
		if err != nil {
			return fmt.Errorf("failed to get latest billing daily model day: %w", err)
		}
		if !latest.IsZero() && latest.After(effectiveStart) {
			effectiveStart = latest.AddDate(0, 0, 1)
		}
	}

	if endDay.Before(effectiveStart) {
		slog.Info("Billing daily model backfill skipped: data already current", "latest_day", effectiveStart.AddDate(0, 0, -1).Format("2006-01-02"))
		return nil
	}

	for day := effectiveStart; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		if err := ingestBillingModelDay(ctx, billingClient, bqClient, config, day); err != nil {
			return fmt.Errorf("ingest billing daily model day %s: %w", day.Format("2006-01-02"), err)
		}
		time.Sleep(billingDailyModelRateLimitDelay)
	}

	slog.Info("Billing daily model backfill completed",
		"start_day", effectiveStart.Format("2006-01-02"),
		"end_day", endDay.Format("2006-01-02"),
	)
	return nil
}

func ingestRecentBillingModelDaily(ctx context.Context, billingClient BillingDailyModelFetcher, bqClient BillingDailyModelStore, config *Config) error {
	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)

	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for day := currentMonthStart; !day.After(yesterday); day = day.AddDate(0, 0, 1) {
		if err := ingestBillingModelDay(ctx, billingClient, bqClient, config, day); err != nil {
			return fmt.Errorf("ingest current month billing daily model day %s: %w", day.Format("2006-01-02"), err)
		}
		time.Sleep(billingDailyModelRateLimitDelay)
	}

	prevMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	prevMonthEnd := currentMonthStart.AddDate(0, 0, -1)
	prevLookbackStart := prevMonthEnd.AddDate(0, 0, -4)
	if prevLookbackStart.Before(prevMonthStart) {
		prevLookbackStart = prevMonthStart
	}
	lastPrevDay := prevMonthEnd
	if yesterday.Before(prevMonthEnd) {
		lastPrevDay = yesterday
	}

	for day := prevLookbackStart; !day.After(lastPrevDay); day = day.AddDate(0, 0, 1) {
		if err := ingestBillingModelDay(ctx, billingClient, bqClient, config, day); err != nil {
			return fmt.Errorf("ingest previous month billing daily model day %s: %w", day.Format("2006-01-02"), err)
		}
		time.Sleep(billingDailyModelRateLimitDelay)
	}

	return nil
}

func ingestBillingModelDay(ctx context.Context, billingClient BillingDailyModelFetcher, bqClient BillingDailyModelStore, config *Config, day time.Time) error {
	resp, err := billingClient.FetchDailyUsage(ctx, day)
	if err != nil {
		return fmt.Errorf("fetch daily billing usage: %w", err)
	}

	items := make([]BillingUsageItem, 0, len(resp.UsageItems))
	for _, item := range resp.UsageItems {
		if item.GrossQuantity <= 0 {
			continue
		}
		items = append(items, item)
	}

	if len(items) == 0 {
		slog.Warn("Skipping billing daily model overwrite due to empty response",
			"day", day.Format("2006-01-02"),
			"api_rows", len(resp.UsageItems),
		)
		return nil
	}

	if err := bqClient.DeleteBillingUsageDailyModelDay(ctx, day, config.EnterpriseSlug); err != nil {
		return fmt.Errorf("delete existing daily model rows: %w", err)
	}

	if err := bqClient.InsertBillingUsageDailyModelDay(ctx, day, config.EnterpriseSlug, items); err != nil {
		return fmt.Errorf("insert daily model rows: %w", err)
	}

	slog.Info("Ingested billing daily model usage",
		"day", day.Format("2006-01-02"),
		"rows", len(items),
	)
	return nil
}
