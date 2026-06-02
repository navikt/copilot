package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	config := loadConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.LogLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting Copilot API server",
		"port", config.Port,
		"environment", config.Environment,
		"log_level", config.LogLevel.String(),
	)

	// Initialize GitHub client (optional - metrics will show zeros if not configured)
	var githubClient *GitHubClient
	var ghHandlers *GitHubHandlers
	if config.GitHubAppID != "" && config.GitHubAppPrivateKey != "" && config.GitHubInstallationID != "" {
		var err error
		githubClient, err = newGitHubClient(config)
		if err != nil {
			slog.Warn("GitHub client initialization failed - metrics will be unavailable", "error", err)
		} else {
			ghHandlers = newGitHubHandlers(githubClient)
			slog.Info("GitHub client initialized successfully")
		}
	} else {
		slog.Warn("GitHub App credentials not configured - metrics will show zeros")
	}

	// Start background metrics collector
	startMetricsCollector(config, githubClient)

	// Initialize BigQuery client (optional - endpoints will error if not configured)
	var bqHandlers *BigQueryHandlers
	var cachedBQClient *CachedBigQueryClient
	if config.GCPProjectID != "" {
		bqClient, err := newBigQueryClient(config)
		if err != nil {
			slog.Warn("BigQuery client initialization failed - data endpoints will be unavailable", "error", err)
		} else {
			cacheTTL := time.Duration(config.CacheTTLHours) * time.Hour
			cachedBQClient = newCachedBigQueryClient(bqClient, cacheTTL)
			bqHandlers = newBigQueryHandlers(cachedBQClient)
			slog.Info("BigQuery client initialized successfully", "cache_ttl", cacheTTL)
		}
	} else {
		slog.Warn("GCP_TEAM_PROJECT_ID not configured - BigQuery endpoints will be unavailable")
	}
	if cachedBQClient != nil {
		defer cachedBQClient.Close()
	}

	// Initialize budget client (optional - requires GITHUB_BILLING_TOKEN classic PAT)
	var budgetHandlers *BudgetHandlers
	if config.GitHubBillingToken != "" && githubClient != nil {
		budgetClient := newBudgetClient(config.GitHubBillingToken, config.GitHubEnterprise)
		budgetHandlers = newBudgetHandlers(budgetClient, githubClient)
		slog.Info("Budget client initialized successfully")
	} else {
		slog.Warn("GITHUB_BILLING_TOKEN not configured - budget endpoint will be unavailable")
	}

	// Middleware
	authMiddleware := makeAuthMiddleware(config)

	// Set up routing on a local mux (not global DefaultServeMux)
	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)
	mux.Handle("/metrics", metricsHandler())

	// Protected API endpoints
	mux.Handle("/api/v1/", loggingMiddleware(config, authMiddleware(makeAPIRouter(config, bqHandlers, ghHandlers, budgetHandlers))))

	slog.Info("Server listening", "port", config.Port)

	server := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           mux,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped")
}
