package main

import (
	"context"
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

	if err := bqClient.EnsureViewsExist(ctx); err != nil {
		slog.Error("Failed to ensure views exist", "error", err)
		os.Exit(1)
	}

	if *backfill {
		startDate, err := time.Parse("2006-01-02", *backfillFrom)
		if err != nil {
			slog.Error("Invalid backfill-from date", "error", err)
			os.Exit(1)
		}
		if err := runBackfill(ctx, ghClient, bqClient, config, startDate); err != nil {
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

func ingestDay(ctx context.Context, gh MetricsFetcher, bq MetricsStore, _ *Config, day time.Time) error {
	dayStr := day.Format("2006-01-02")
	slog.Info("Ingesting metrics", "day", dayStr)

	// Fetch first to determine which scope (enterprise vs org) has data
	result, err := gh.FetchDailyMetrics(ctx, day)
	if err != nil {
		if errors.Is(err, ErrReportNotAvailable) {
			slog.Warn("Report not available yet — will be picked up by next run or backfill",
				"day", dayStr,
			)
			return nil
		}
		return fmt.Errorf("failed to fetch metrics: %w", err)
	}

	if len(result.Records) == 0 {
		slog.Warn("No records returned for day", "day", dayStr)
		return nil
	}

	slog.Debug("Fetched metrics", "day", dayStr, "scope", result.Scope, "scope_id", result.ScopeID, "records", len(result.Records))

	// Check if we already have data for this day/scope - delete for idempotent re-ingestion
	exists, err := bq.DayExists(ctx, day, result.ScopeID)
	if err != nil {
		return fmt.Errorf("failed to check if day exists: %w", err)
	}
	if exists {
		slog.Info("Day already exists, deleting for re-ingestion", "day", dayStr, "scope_id", result.ScopeID)
		if err := bq.DeleteDay(ctx, day, result.ScopeID); err != nil {
			return fmt.Errorf("failed to delete existing data: %w", err)
		}
	}

	if err := bq.InsertMetrics(ctx, day, result.Scope, result.ScopeID, result.Records); err != nil {
		return fmt.Errorf("failed to insert metrics: %w", err)
	}

	slog.Info("Successfully ingested metrics", "day", dayStr, "scope", result.Scope, "records", len(result.Records))
	return nil
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
