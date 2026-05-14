package main

import (
	"log/slog"
	"net/http"
	"os"
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
	if config.GCPProjectID != "" {
		bqClient, err := newBigQueryClient(config)
		if err != nil {
			slog.Warn("BigQuery client initialization failed - data endpoints will be unavailable", "error", err)
		} else {
			cacheTTL := time.Duration(config.CacheTTLHours) * time.Hour
			cachedBQClient := newCachedBigQueryClient(bqClient, cacheTTL)
			bqHandlers = newBigQueryHandlers(cachedBQClient)
			slog.Info("BigQuery client initialized successfully", "cache_ttl", cacheTTL)
		}
	} else {
		slog.Warn("GCP_TEAM_PROJECT_ID not configured - BigQuery endpoints will be unavailable")
	}

	// Middleware
	authMiddleware := makeAuthMiddleware(config)

	// Public endpoints
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/ready", readyHandler)
	http.Handle("/metrics", metricsHandler())

	// Protected API endpoints
	http.Handle("/api/v1/", loggingMiddleware(config, authMiddleware(makeAPIRouter(config, bqHandlers, ghHandlers))))

	slog.Info("Server listening", "port", config.Port)

	server := &http.Server{
		Addr:         ":" + config.Port,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
