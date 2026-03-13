package main

import (
	"context"
	"encoding/json"
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
	runOnce := flag.Bool("run-once", false, "Run single scan for today and exit")
	dryRun := flag.Bool("dry-run", false, "Scan repos and print results without writing to BigQuery")
	flag.Parse()

	config := loadConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.LogLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting copilot-adoption",
		"port", config.Port,
		"org", config.OrganizationSlug,
		"log_level", config.LogLevel.String(),
		"batch_size", config.GraphQLBatchSize,
		"concurrency", config.ScanConcurrency,
		"run_once", *runOnce,
	)

	if *dryRun {
		// Dry-run: only validate GitHub credentials, skip BigQuery
		if config.GitHubAppID == 0 || config.GitHubAppPrivateKey == "" || config.GitHubAppInstallationID == 0 {
			slog.Error("GitHub credentials required for dry-run")
			os.Exit(1)
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		ghClient, err := NewGitHubClient(config)
		if err != nil {
			slog.Error("Failed to create GitHub client", "error", err)
			os.Exit(1)
		}

		today := time.Now().UTC().Truncate(24 * time.Hour)
		results, err := DryRunScan(ctx, ghClient, config, today)
		if err != nil {
			slog.Error("Dry-run scan failed", "error", err)
			os.Exit(1)
		}

		output, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(output))
		return
	}

	if err := config.Validate(); err != nil {
		slog.Error("Configuration error", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
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

	if *runOnce {
		slack := NewSlackNotifier(config.SlackWebhookURL)
		today := time.Now().UTC().Truncate(24 * time.Hour)
		if err := RunScan(ctx, ghClient, bqClient, config, today, slack); err != nil {
			slog.Error("Scan failed", "error", err)
			if slack != nil {
				slack.NotifyError(ctx, fmt.Sprintf("Scan failed: %v", err))
			}
			os.Exit(1)
		}
		slog.Info("Scan completed successfully")
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

	// Channel to receive server errors
	serverErrCh := make(chan error, 1)
	go func() {
		slog.Info("Server listening", "port", config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		slog.Info("Received shutdown signal")
	case err := <-serverErrCh:
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown error", "error", err)
	}
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
	_, _ = fmt.Fprint(w, "# HELP copilot_adoption_up Application is up\n")
	_, _ = fmt.Fprint(w, "# TYPE copilot_adoption_up gauge\n")
	_, _ = fmt.Fprint(w, "copilot_adoption_up 1\n")
}
