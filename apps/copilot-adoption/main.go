package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
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

	startRun("copilot-adoption")
	defer finishRun(0)

	slog.Info("Starting copilot-adoption",
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
			exit(1)
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		ghClient, err := NewGitHubClient(config)
		if err != nil {
			slog.Error("Failed to create GitHub client", "error", err)
			exit(1)
		}

		today := time.Now().UTC().Truncate(24 * time.Hour)
		results, err := DryRunScan(ctx, ghClient, config, today)
		if err != nil {
			slog.Error("Dry-run scan failed", "error", err)
			exit(1)
		}

		output, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(output))
		return
	}

	if err := config.Validate(); err != nil {
		slog.Error("Configuration error", "error", err)
		exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ghClient, err := NewGitHubClient(config)
	if err != nil {
		slog.Error("Failed to create GitHub client", "error", err)
		exit(1)
	}

	bqClient, err := NewBigQueryClient(ctx, config)
	if err != nil {
		slog.Error("Failed to create BigQuery client", "error", err)
		exit(1)
	}
	defer func() { _ = bqClient.Close() }()

	if err := bqClient.EnsureTableExists(ctx); err != nil {
		slog.Error("Failed to ensure table exists", "error", err)
		exit(1)
	}

	if err := bqClient.EnsureViewsExist(ctx); err != nil {
		slog.Error("Failed to ensure views exist", "error", err)
		exit(1)
	}

	slack := NewSlackNotifier(config.SlackWebhookURL)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if err := RunScan(ctx, ghClient, bqClient, config, today, slack); err != nil {
		slog.Error("Scan failed", "error", err)
		if slack != nil {
			slack.NotifyError(ctx, fmt.Sprintf("Scan failed: %v", err))
		}
		exit(1)
	}
	slog.Info("Scan completed successfully")
}
