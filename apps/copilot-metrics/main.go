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
	billingMonthlyBackfill := flag.Bool("billing-monthly-backfill", false, "Backfill monthly billing data from billing-monthly-from month to current month")
	billingMonthlyFrom := flag.String("billing-monthly-from", "2025-01", "Start month for monthly billing backfill (YYYY-MM)")
	billingDailyReportBackfill := flag.Bool("billing-daily-report-backfill", false, "Backfill daily organization billing usage report data")
	billingDailyReportFrom := flag.String("billing-daily-report-from", "2025-10-10", "Start day for daily billing usage report backfill (YYYY-MM-DD)")
	billingModelDailyBackfill := flag.Bool("billing-model-daily-backfill", false, "Backfill daily model billing data")
	billingModelDailyFrom := flag.String("billing-model-daily-from", "2025-10-10", "Start day for daily model billing backfill (YYYY-MM-DD)")
	legacyBillingBackfill := flag.Bool("billing-backfill", false, "Deprecated: use --billing-monthly-backfill")
	legacyBillingFrom := flag.String("billing-from", "", "Deprecated: use --billing-monthly-from")
	legacyBillingUsageBackfill := flag.Bool("billing-usage-backfill", false, "Deprecated: use --billing-daily-report-backfill")
	legacyBillingUsageFrom := flag.String("billing-usage-from", "", "Deprecated: use --billing-daily-report-from")
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

	// Set up billing + budget clients (optional — require classic PAT with admin:enterprise scope)
	billingClient := NewBillingClient(config.GitHubBillingToken, config.EnterpriseSlug)
	budgetClient := NewBudgetClient(config.GitHubBillingToken, config.EnterpriseSlug)
	if billingClient != nil {
		slog.Info("Billing client configured — premium request usage and budget snapshot ingestion enabled")
		if err := bqClient.EnsureBillingTableExists(ctx); err != nil {
			slog.Error("Failed to ensure billing table exists", "error", err)
			os.Exit(1)
		}
		if err := bqClient.EnsureBudgetSnapshotsTableExists(ctx); err != nil {
			slog.Error("Failed to ensure budget_snapshots table exists", "error", err)
			os.Exit(1)
		}
		if err := bqClient.EnsureUserBudgetSnapshotsTableExists(ctx); err != nil {
			slog.Error("Failed to ensure user_budget_snapshots table exists", "error", err)
			os.Exit(1)
		}
		if err := bqClient.EnsureBillingUsageReportsTableExists(ctx); err != nil {
			slog.Error("Failed to ensure billing_usage_reports table exists", "error", err)
			os.Exit(1)
		}
		if err := bqClient.EnsureBillingUsageDailyModelTableExists(ctx); err != nil {
			slog.Error("Failed to ensure billing_usage_daily_model table exists", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Warn("No GITHUB_BILLING_TOKEN configured — billing ingestion and budget snapshot ingestion disabled")
	}

	runBillingMonthlyBackfill := *billingMonthlyBackfill || *legacyBillingBackfill
	runBillingDailyReportBackfill := *billingDailyReportBackfill || *legacyBillingUsageBackfill

	effectiveBillingMonthlyFrom := *billingMonthlyFrom
	if *legacyBillingFrom != "" {
		slog.Warn("Flag --billing-from is deprecated, use --billing-monthly-from")
		effectiveBillingMonthlyFrom = *legacyBillingFrom
	}
	if *legacyBillingBackfill {
		slog.Warn("Flag --billing-backfill is deprecated, use --billing-monthly-backfill")
	}

	effectiveBillingDailyReportFrom := *billingDailyReportFrom
	if *legacyBillingUsageFrom != "" {
		slog.Warn("Flag --billing-usage-from is deprecated, use --billing-daily-report-from")
		effectiveBillingDailyReportFrom = *legacyBillingUsageFrom
	}
	if *legacyBillingUsageBackfill {
		slog.Warn("Flag --billing-usage-backfill is deprecated, use --billing-daily-report-backfill")
	}

	if runBillingDailyReportBackfill {
		if billingClient == nil {
			slog.Error("Billing daily report backfill requires GITHUB_BILLING_TOKEN")
			os.Exit(1)
		}
		startDay, err := time.Parse("2006-01-02", effectiveBillingDailyReportFrom)
		if err != nil {
			slog.Error("Invalid billing-daily-report-from day", "error", err)
			os.Exit(1)
		}
		if err := runBillingUsageReportBackfill(ctx, billingClient, bqClient, config, startDay, *backfillForce); err != nil {
			slog.Error("Billing daily report backfill failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if *billingModelDailyBackfill {
		if billingClient == nil {
			slog.Error("Billing model daily backfill requires GITHUB_BILLING_TOKEN")
			os.Exit(1)
		}
		startDay, err := time.Parse("2006-01-02", *billingModelDailyFrom)
		if err != nil {
			slog.Error("Invalid billing-model-daily-from day", "error", err)
			os.Exit(1)
		}
		if err := runBillingDailyModelBackfill(ctx, billingClient, bqClient, config, startDay, *backfillForce); err != nil {
			slog.Error("Billing model daily backfill failed", "error", err)
			os.Exit(1)
		}
		return
	}

	if runBillingMonthlyBackfill {
		if billingClient == nil {
			slog.Error("Billing monthly backfill requires GITHUB_BILLING_TOKEN")
			os.Exit(1)
		}
		startMonth, err := time.Parse("2006-01", effectiveBillingMonthlyFrom)
		if err != nil {
			slog.Error("Invalid billing-monthly-from month", "error", err)
			os.Exit(1)
		}
		if err := runBillingBackfill(ctx, billingClient, bqClient, config, startMonth, *backfillForce); err != nil {
			slog.Error("Billing monthly backfill failed", "error", err)
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
			if err := ingestMissingBillingUsageReports(ctx, billingClient, bqClient, config); err != nil {
				slog.Warn("Billing usage report ingestion failed", "error", err)
			}
			if err := ingestRecentBillingModelDaily(ctx, billingClient, bqClient, config); err != nil {
				if slack != nil {
					slack.NotifyError(ctx, fmt.Sprintf("Billing daily model ingestion failed: %v", err))
				}
				slog.Error("Billing daily model ingestion failed", "error", err)
				os.Exit(1)
			}
		}
		// Ingest today's budget snapshot (always re-ingests since consumption is live)
		if budgetClient != nil {
			ingestTodayBudgetSnapshot(ctx, budgetClient, bqClient, config)
			ingestTodayUserBudgetSnapshot(ctx, ghClient, budgetClient, bqClient, config)
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
				if errors.Is(delErr, ErrStreamingBuffer) {
					slog.Info("Skipping entity re-import (streaming buffer not yet flushed, re-run in ~90 min)", "day", dayStr)
				} else {
					slog.Warn("Entity data already exists and cannot be replaced (skipping entity, continuing with supplementary)", "day", dayStr, "error", delErr)
				}
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
			if errors.Is(err, ErrStreamingBuffer) {
				slog.Info("Skipping user-teams re-import (streaming buffer not yet flushed, re-run in ~90 min)", "day", dayStr)
			} else {
				slog.Warn("Failed to store user-teams report", "day", dayStr, "error", err)
			}
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
			if errors.Is(err, ErrStreamingBuffer) {
				slog.Info("Skipping user-metrics re-import (streaming buffer not yet flushed, re-run in ~90 min)", "day", dayStr)
			} else {
				slog.Warn("Failed to store per-user metrics report", "day", dayStr, "error", err)
			}
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

// ingestCurrentMonthBilling fetches and stores billing data for the current and previous month.
// Always re-ingests to handle cumulative updates and post-month adjustments.
func ingestCurrentMonthBilling(ctx context.Context, billing *BillingClient, bq *BigQueryClient, cfg *Config) {
	now := time.Now().UTC()
	year, month := now.Year(), int(now.Month())

	if err := ingestBillingMonth(ctx, billing, bq, cfg, year, month, true); err != nil {
		slog.Warn("Failed to ingest current month billing", "year", year, "month", month, "error", err)
	}

	prevMonth := now.AddDate(0, -1, 0)
	if err := ingestBillingMonth(ctx, billing, bq, cfg, prevMonth.Year(), int(prevMonth.Month()), true); err != nil {
		slog.Warn("Failed to ingest previous month billing", "year", prevMonth.Year(), "month", int(prevMonth.Month()), "error", err)
	}
}

// ingestBillingMonth fetches and stores billing data for a specific month.
func ingestBillingMonth(ctx context.Context, billing *BillingClient, bq *BigQueryClient, cfg *Config, year, month int, force bool) error {
	slog.Info("Ingesting billing data", "year", year, "month", month, "force", force)

	if !force {
		exists, err := bq.BillingMonthExists(ctx, year, month, cfg.EnterpriseSlug)
		if err != nil {
			slog.Warn("Failed to check billing month existence", "year", year, "month", month, "error", err)
		} else if exists {
			slog.Info("Billing data already exists, skipping", "year", year, "month", month)
			return nil
		}
	}

	resp, err := billing.FetchMonthlyUsage(ctx, year, month)
	if err != nil {
		return fmt.Errorf("fetch billing data: %w", err)
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
		return nil
	}

	// Delete existing data before re-inserting (idempotent)
	if err := bq.DeleteBillingMonth(ctx, year, month, cfg.EnterpriseSlug); err != nil {
		return fmt.Errorf("delete existing billing data: %w", err)
	}

	if err := bq.InsertBillingUsage(ctx, year, month, cfg.EnterpriseSlug, items); err != nil {
		return fmt.Errorf("insert billing data: %w", err)
	}

	slog.Info("Billing data ingested successfully", "year", year, "month", month, "items", len(items))
	return nil
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
		if err := ingestBillingMonth(ctx, billing, bq, cfg, year, month, force); err != nil {
			slog.Warn("Failed to ingest billing month", "year", year, "month", month, "error", err)
			errorCount++
		} else {
			successCount++
		}

		current = current.AddDate(0, 1, 0)

		// Small delay to avoid rate limiting
		time.Sleep(500 * time.Millisecond)
	}

	slog.Info("Billing backfill completed", "months_processed", successCount, "errors", errorCount)
	return nil
}

// ingestTodayBudgetSnapshot fetches all budget entries from GitHub and stores a daily snapshot.
// Always overwrites today's snapshot since consumed_amount is live and cumulative within the month.
// Errors are logged as warnings — budget snapshots are supplementary data.
func ingestTodayBudgetSnapshot(ctx context.Context, budget *BudgetClient, bq *BigQueryClient, cfg *Config) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	dateStr := today.Format("2006-01-02")

	slog.Info("Ingesting budget snapshot", "date", dateStr)

	entries, err := budget.FetchAllBudgets(ctx)
	if err != nil {
		slog.Warn("Failed to fetch budget entries from GitHub API", "date", dateStr, "error", err)
		return
	}

	if len(entries) == 0 {
		slog.Info("No budget entries returned", "date", dateStr)
		return
	}

	// Delete existing snapshot for today before re-inserting (idempotent).
	// If the check or delete fails we still attempt insert — BigQuery data may not exist yet.
	exists, checkErr := bq.BudgetSnapshotExists(ctx, today, cfg.EnterpriseSlug)
	if checkErr != nil {
		slog.Warn("Could not check if budget snapshot exists (proceeding with insert)", "date", dateStr, "error", checkErr)
	} else if exists {
		if delErr := bq.DeleteBudgetSnapshot(ctx, today, cfg.EnterpriseSlug); delErr != nil {
			slog.Warn("Failed to delete existing budget snapshot (proceeding with insert)", "date", dateStr, "error", delErr)
		}
	}

	if err := bq.InsertBudgetSnapshots(ctx, today, cfg.EnterpriseSlug, entries); err != nil {
		slog.Warn("Failed to insert budget snapshot", "date", dateStr, "error", err)
		return
	}

	slog.Info("Budget snapshot ingested successfully", "date", dateStr, "entries", len(entries))
}

// ingestTodayUserBudgetSnapshot fetches the current budget consumption for ALL Copilot
// seat holders and stores a daily snapshot. This gives a complete picture of AI credit
// spend across all users (not just the 27 with override budgets).
// Errors are logged as warnings — this is supplementary data.
func ingestTodayUserBudgetSnapshot(ctx context.Context, gh *GitHubClient, budget *BudgetClient, bq *BigQueryClient, cfg *Config) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	dateStr := today.Format("2006-01-02")

	slog.Info("Ingesting user budget snapshot", "date", dateStr)

	logins, err := gh.FetchAllCopilotLogins(ctx)
	if err != nil {
		slog.Warn("Failed to fetch Copilot seat holders — skipping user budget snapshot", "date", dateStr, "error", err)
		return
	}
	if len(logins) == 0 {
		slog.Info("No active Copilot seat holders found", "date", dateStr)
		return
	}

	entries, err := budget.FetchAllUserBudgets(ctx, logins)
	if err != nil {
		slog.Warn("Failed to fetch user budgets", "date", dateStr, "error", err)
		return
	}
	if len(entries) == 0 {
		slog.Info("No user budget data returned", "date", dateStr)
		return
	}

	// Delete existing snapshot for today before re-inserting (idempotent).
	exists, checkErr := bq.UserBudgetSnapshotExists(ctx, today, cfg.EnterpriseSlug)
	if checkErr != nil {
		slog.Warn("Could not check if user budget snapshot exists (proceeding with insert)", "date", dateStr, "error", checkErr)
	} else if exists {
		if delErr := bq.DeleteUserBudgetSnapshot(ctx, today, cfg.EnterpriseSlug); delErr != nil {
			slog.Warn("Failed to delete existing user budget snapshot (proceeding with insert)", "date", dateStr, "error", delErr)
		}
	}

	if err := bq.InsertUserBudgetSnapshots(ctx, today, cfg.EnterpriseSlug, entries); err != nil {
		slog.Warn("Failed to insert user budget snapshot", "date", dateStr, "error", err)
		return
	}

	slog.Info("User budget snapshot ingested successfully", "date", dateStr, "users", len(entries))
}
