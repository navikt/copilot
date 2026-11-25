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

	"github.com/navikt/copilot/apps/copilot-adoption/internal/config"
	"github.com/navikt/copilot/apps/copilot-adoption/internal/github"
	"github.com/navikt/copilot/apps/copilot-adoption/internal/scanner"
	"github.com/navikt/copilot/apps/copilot-adoption/internal/slack"
	"github.com/navikt/copilot/apps/copilot-adoption/internal/store"
)

func main() {
	runOnce := flag.Bool("run-once", false, "Run single scan for today and exit")
	dryRun := flag.Bool("dry-run", false, "Scan repos and print results without writing to BigQuery")
	flag.Parse()

	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	startRun("copilot-adoption")
	defer finishRun(0)

	slog.Info("Starting copilot-adoption",
		"org", cfg.OrganizationSlug,
		"log_level", cfg.LogLevel.String(),
		"batch_size", cfg.GraphQLBatchSize,
		"concurrency", cfg.ScanConcurrency,
		"run_once", *runOnce,
	)

	if *dryRun {
		// Dry-run: only validate GitHub credentials, skip BigQuery
		if cfg.GitHubAppID == 0 || cfg.GitHubAppPrivateKey == "" || cfg.GitHubAppInstallationID == 0 {
			slog.Error("GitHub credentials required for dry-run")
			exit(1)
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		ghClient, err := github.NewGitHubClient(cfg)
		if err != nil {
			slog.Error("Failed to create GitHub client", "error", err)
			exit(1)
		}

		today := time.Now().UTC().Truncate(24 * time.Hour)
		results, err := scanner.DryRunScan(ctx, ghClient, cfg, today)
		if err != nil {
			slog.Error("Dry-run scan failed", "error", err)
			exit(1)
		}

		output, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(output))
		return
	}

	if err := cfg.Validate(); err != nil {
		slog.Error("Configuration error", "error", err)
		exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ghClient, err := github.NewGitHubClient(cfg)
	if err != nil {
		slog.Error("Failed to create GitHub client", "error", err)
		exit(1)
	}

	bqClient, err := store.NewBigQueryClient(ctx, cfg)
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

	slack := slack.NewSlackNotifier(cfg.SlackWebhookURL)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if err := scanner.RunScan(ctx, ghClient, bqClient, cfg, today, slack); err != nil {
		slog.Error("Scan failed", "error", err)
		if slack != nil {
			slack.NotifyError(ctx, fmt.Sprintf("Scan failed: %v", err))
		}
		exit(1)
	}
	slog.Info("Scan completed successfully")
}
