package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	backfill := flag.Bool("backfill", false, "Run historical backfill from Oct 10, 2025 to today")
	backfillFrom := flag.String("backfill-from", "2025-10-10", "Start date for backfill (YYYY-MM-DD)")
	backfillForce := flag.Bool("force", false, "Force re-ingestion even if data already exists")
	billingBackfill := flag.Bool("billing-backfill", false, "Backfill billing data from billing-from month to current month")
	billingFrom := flag.String("billing-from", "2025-01", "Start month for billing backfill (YYYY-MM)")
	runOnce := flag.Bool("run-once", false, "Run single ingestion for yesterday and exit")
	flag.Parse()

	config := loadConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.LogLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting copilot-metrics",
		"port", config.Port,
		"enterprise", config.EnterpriseSlug,
		"org", config.OrganizationSlug,
		"log_level", config.LogLevel.String(),
		"backfill", *backfill,
		"run_once", *runOnce,
	)

	if err := config.Validate(); err != nil {
		slog.Error("Configuration error", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ghClient, err := NewGitHubClient(config)
	if err != nil {
		slog.Error("Failed to create GitHub client", "error", err)
		os.Exit(1)
	}

	bqClient, err := NewBigQueryClient(ctx, config)
	if err != nil {
		slog.Error("Failed to create BigQuery client", "error", err)
		os.Exit(1)
	}
	defer func() { _ = bqClient.Close() }()

	if err := bqClient.EnsureTableExists(ctx); err != nil {
		slog.Error("Failed to ensure table exists", "error", err)
		os.Exit(1)
	}

	if err := bqClient.EnsureUserTeamsTableExists(ctx); err != nil {
		slog.Error("Failed to ensure user_teams table exists", "error", err)
		os.Exit(1)
	}

	if err := bqClient.EnsureUserMetricsTableExists(ctx); err != nil {
		slog.Error("Failed to ensure user_metrics table exists", "error", err)
		os.Exit(1)
	}

	if err := bqClient.EnsureViewsExist(ctx); err != nil {
		slog.Warn("Failed to ensure views exist (continuing without views)", "error", err)
	}

	// Set up billing client (optional — requires classic PAT)
	billingClient := NewBillingClient(config.GitHubBillingToken, config.EnterpriseSlug)
	if billingClient != nil {
		slog.Info("Billing client configured — premium request usage ingestion enabled")
		if err := bqClient.EnsureBillingTableExists(ctx); err != nil {
			slog.Error("Failed to ensure billing table exists", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Warn("No GITHUB_BILLING_TOKEN configured — billing usage ingestion disabled")
	}

	if *billingBackfill {
		if billingClient == nil {
			slog.Error("Billing backfill requires GITHUB_BILLING_TOKEN")
			os.Exit(1)
		}
		startMonth, err := time.Parse("2006-01", *billingFrom)
		if err != nil {
			slog.Error("Invalid billing-from month", "error", err)
			os.Exit(1)
		}
		if err := runBillingBackfill(ctx, billingClient, bqClient, config, startMonth, *backfillForce); err != nil {
			slog.Error("Billing backfill failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if *backfill {
		startDate, err := time.Parse("2006-01-02", *backfillFrom)
		if err != nil {
			slog.Error("Invalid backfill-from date", "error", err)
			os.Exit(1)
		}
		if err := runBackfill(ctx, ghClient, bqClient, config, startDate, *backfillForce); err != nil {
			slog.Error("Backfill failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if *runOnce {
		slack := NewSlackNotifier(config.SlackWebhookURL)
		if err := ingestMissing(ctx, ghClient, bqClient, config, slack); err != nil {
			if slack != nil {
				slack.NotifyError(ctx, fmt.Sprintf("Ingestion failed: %v", err))
			}
			slog.Error("Ingestion failed", "error", err)
			os.Exit(1)
		}
		// Ingest current month's billing data (always re-ingests since it's cumulative)
		if billingClient != nil {
			ingestCurrentMonthBilling(ctx, billingClient, bqClient, config)
		}
		slog.Info("Ingestion completed successfully")
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)
	mux.HandleFunc("/metrics", metricsHandler)

	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Server listening", "port", config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	slog.Info("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown error", "error", err)
	}
}

func ingestMissing(ctx context.Context, gh MetricsFetcher, bq MetricsStore, cfg *Config, slack *SlackNotifier) error {
	yesterday := time.Now().UTC().AddDate(0, 0, -1)

	// Check what we already have in BigQuery to fill gaps automatically
	latestDay, err := bq.GetLatestDay(ctx, cfg.EnterpriseSlug)
	if err != nil {
		slog.Warn("Could not get latest day from BigQuery, ingesting yesterday only", "error", err)
		return ingestDay(ctx, gh, bq, cfg, yesterday)
	}

	if latestDay.IsZero() {
		slog.Info("No existing data found, ingesting yesterday only")
		return ingestDay(ctx, gh, bq, cfg, yesterday)
	}

	startDate := latestDay.AddDate(0, 0, 1)
	if startDate.After(yesterday) {
		slog.Info("Already up to date", "latest_day", latestDay.Format("2006-01-02"))
		return nil
	}

	totalDays := int(yesterday.Sub(startDate).Hours()/24) + 1
	slog.Info("Filling missing days",
		"latest_in_bigquery", latestDay.Format("2006-01-02"),
		"from", startDate.Format("2006-01-02"),
		"to", yesterday.Format("2006-01-02"),
		"days", totalDays,
	)

	var successCount, errorCount int
	var failedDays []string
	for day := startDate; !day.After(yesterday); day = day.AddDate(0, 0, 1) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := ingestDay(ctx, gh, bq, cfg, day); err != nil {
			errorCount++
			failedDays = append(failedDays, day.Format("2006-01-02"))
			slog.Error("Failed to ingest day", "day", day.Format("2006-01-02"), "error", err)
			continue
		}
		successCount++
	}

	slog.Info("Ingestion completed", "success", successCount, "errors", errorCount, "total", totalDays)

	if errorCount > 0 {
		slack.NotifyIngestionResult(ctx, successCount, errorCount, failedDays)
	}

	if errorCount > 0 && successCount == 0 {
		return fmt.Errorf("all %d days failed to ingest", errorCount)
	}
	return nil
}

func ingestDay(ctx context.Context, gh MetricsFetcher, bq MetricsStore, cfg *Config, day time.Time) error {
	dayStr := day.Format("2006-01-02")
	slog.Info("Ingesting metrics", "day", dayStr)

	// Fetch entity-level metrics.
	// Track hard failures so we can propagate them after supplementary ingestion.
	var entityErr error
	result, err := gh.FetchDailyMetrics(ctx, day)
	if err != nil {
		if errors.Is(err, ErrReportNotAvailable) {
			slog.Warn("Entity report not available yet",
				"day", dayStr,
			)
		} else {
			slog.Error("Failed to fetch entity metrics", "day", dayStr, "error", err)
			entityErr = fmt.Errorf("failed to fetch metrics: %w", err)
		}
	} else if len(result.Records) == 0 {
		slog.Warn("No entity records returned for day", "day", dayStr)
	} else {
		slog.Debug("Fetched entity metrics", "day", dayStr, "scope", result.Scope, "scope_id", result.ScopeID, "records", len(result.Records))

		exists, checkErr := bq.DayExists(ctx, day, result.ScopeID)
		if checkErr != nil {
			entityErr = fmt.Errorf("failed to check if day exists: %w", checkErr)
		} else if exists {
			slog.Info("Day already exists, deleting for re-ingestion", "day", dayStr, "scope_id", result.ScopeID)
			if delErr := bq.DeleteDay(ctx, day, result.ScopeID); delErr != nil {
				slog.Warn("Entity data already exists and cannot be replaced (skipping entity, continuing with supplementary)", "day", dayStr, "error", delErr)
			} else if insErr := bq.InsertMetrics(ctx, day, result.Scope, result.ScopeID, result.Records); insErr != nil {
				entityErr = fmt.Errorf("failed to insert metrics: %w", insErr)
			} else {
				slog.Info("Successfully ingested entity metrics", "day", dayStr, "scope", result.Scope, "records", len(result.Records))
			}
		} else {
			if insErr := bq.InsertMetrics(ctx, day, result.Scope, result.ScopeID, result.Records); insErr != nil {
				entityErr = fmt.Errorf("failed to insert metrics: %w", insErr)
			} else {
				slog.Info("Successfully ingested entity metrics", "day", dayStr, "scope", result.Scope, "records", len(result.Records))
			}
		}
	}

	// Always attempt supplementary ingestion, independent of entity metrics.
	// Use entity scope if available, otherwise fall back to enterprise slug from config.
	scopeID := cfg.EnterpriseSlug
	if result != nil && result.ScopeID != "" {
		scopeID = result.ScopeID
	}
	ingestSupplementary(ctx, gh, bq, day, scopeID)

	return entityErr
}

// ingestSupplementary fetches and stores user-teams and per-user reports.
// Failures are logged but don't fail the overall ingestion — these reports
// may not be available for all days (only from May 2026 onwards).
func ingestSupplementary(ctx context.Context, gh MetricsFetcher, bq MetricsStore, day time.Time, scopeID string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in supplementary ingestion (recovered)", "day", day.Format("2006-01-02"), "panic", r)
		}
	}()

	// 5-minute timeout so supplementary work can't starve the main job
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	dayStr := day.Format("2006-01-02")

	// User-teams report
	if teamsResult, err := gh.FetchDailyUserTeams(ctx, day); err != nil {
		if errors.Is(err, ErrReportNotAvailable) {
			slog.Info("User-teams report not available yet", "day", dayStr)
		} else {
			slog.Warn("Failed to fetch user-teams report", "day", dayStr, "error", err)
		}
	} else if len(teamsResult.Records) > 0 {
		if err := upsertReport(ctx, bq.UserTeamsDayExists, bq.DeleteUserTeamsDay, bq.InsertUserTeams,
			day, teamsResult); err != nil {
			slog.Warn("Failed to store user-teams report", "day", dayStr, "error", err)
		} else {
			slog.Info("Ingested user-teams report", "day", dayStr, "records", len(teamsResult.Records))
		}
	}

	// Per-user metrics report
	if usersResult, err := gh.FetchDailyUserMetrics(ctx, day); err != nil {
		if errors.Is(err, ErrReportNotAvailable) {
			slog.Info("Per-user metrics report not available yet", "day", dayStr)
		} else {
			slog.Warn("Failed to fetch per-user metrics report", "day", dayStr, "error", err)
		}
	} else if len(usersResult.Records) > 0 {
		if err := upsertReport(ctx, bq.UserMetricsDayExists, bq.DeleteUserMetricsDay, bq.InsertUserMetrics,
			day, usersResult); err != nil {
			slog.Warn("Failed to store per-user metrics report", "day", dayStr, "error", err)
		} else {
			slog.Info("Ingested per-user metrics report", "day", dayStr, "records", len(usersResult.Records))
		}
	}
}

// upsertReport handles idempotent insert for a report: check exists → delete → insert.
func upsertReport(
	ctx context.Context,
	existsFn func(context.Context, time.Time, string) (bool, error),
	deleteFn func(context.Context, time.Time, string) error,
	insertFn func(context.Context, time.Time, string, string, []json.RawMessage) error,
	day time.Time,
	result *FetchResult,
) error {
	exists, err := existsFn(ctx, day, result.ScopeID)
	if err != nil {
		return fmt.Errorf("check exists: %w", err)
	}
	if exists {
		if err := deleteFn(ctx, day, result.ScopeID); err != nil {
			return fmt.Errorf("delete existing: %w", err)
		}
	}
	return insertFn(ctx, day, result.Scope, result.ScopeID, result.Records)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, `{"status":"healthy"}`)
}

func readyHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, `{"status":"ready"}`)
}

func metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "# HELP copilot_metrics_up Application is up\n")
	_, _ = fmt.Fprint(w, "# TYPE copilot_metrics_up gauge\n")
	_, _ = fmt.Fprint(w, "copilot_metrics_up 1\n")
}

// ingestCurrentMonthBilling fetches and stores billing data for the current month.
// Always re-ingests since monthly data is cumulative and updates throughout the month.
func ingestCurrentMonthBilling(ctx context.Context, billing *BillingClient, bq *BigQueryClient, cfg *Config) {
	now := time.Now().UTC()
	year, month := now.Year(), int(now.Month())

	ingestBillingMonth(ctx, billing, bq, cfg, year, month, true)

	// Also re-ingest previous month if we're in the first few days (data may still be finalizing)
	if now.Day() <= 5 {
		prevMonth := now.AddDate(0, -1, 0)
		ingestBillingMonth(ctx, billing, bq, cfg, prevMonth.Year(), int(prevMonth.Month()), true)
	}
}

// ingestBillingMonth fetches and stores billing data for a specific month.
func ingestBillingMonth(ctx context.Context, billing *BillingClient, bq *BigQueryClient, cfg *Config, year, month int, force bool) {
	slog.Info("Ingesting billing data", "year", year, "month", month, "force", force)

	if !force {
		exists, err := bq.BillingMonthExists(ctx, year, month, cfg.EnterpriseSlug)
		if err != nil {
			slog.Warn("Failed to check billing month existence", "year", year, "month", month, "error", err)
		} else if exists {
			slog.Info("Billing data already exists, skipping", "year", year, "month", month)
			return
		}
	}

	resp, err := billing.FetchMonthlyUsage(ctx, year, month)
	if err != nil {
		slog.Warn("Failed to fetch billing data", "year", year, "month", month, "error", err)
		return
	}

	// Filter to items with actual usage
	var items []BillingUsageItem
	for _, item := range resp.UsageItems {
		if item.GrossQuantity > 0 {
			items = append(items, item)
		}
	}

	if len(items) == 0 {
		slog.Info("No billing usage for month", "year", year, "month", month)
		return
	}

	// Delete existing data before re-inserting (idempotent)
	if err := bq.DeleteBillingMonth(ctx, year, month, cfg.EnterpriseSlug); err != nil {
		slog.Warn("Failed to delete existing billing data (continuing)", "error", err)
	}

	if err := bq.InsertBillingUsage(ctx, year, month, cfg.EnterpriseSlug, items); err != nil {
		slog.Error("Failed to insert billing data", "year", year, "month", month, "error", err)
		return
	}

	slog.Info("Billing data ingested successfully", "year", year, "month", month, "items", len(items))
}

// runBillingBackfill ingests billing data for all months from startMonth to current.
func runBillingBackfill(ctx context.Context, billing *BillingClient, bq *BigQueryClient, cfg *Config, startMonth time.Time, force bool) error {
	now := time.Now().UTC()
	current := time.Date(startMonth.Year(), startMonth.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	var successCount, errorCount int
	for !current.After(end) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		year, month := current.Year(), int(current.Month())
		ingestBillingMonth(ctx, billing, bq, cfg, year, month, force)
		successCount++

		current = current.AddDate(0, 1, 0)

		// Small delay to avoid rate limiting
		time.Sleep(500 * time.Millisecond)
	}

	slog.Info("Billing backfill completed", "months_processed", successCount, "errors", errorCount)
	return nil
}
