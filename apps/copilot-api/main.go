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
	if config.GitHubAppID != "" && config.GitHubAppPrivateKey != "" && config.GitHubInstallationID != "" {
		var err error
		githubClient, err = newGitHubClient(config)
		if err != nil {
			slog.Warn("GitHub client initialization failed - metrics will be unavailable", "error", err)
		} else {
			slog.Info("GitHub client initialized successfully")
		}
	} else {
		slog.Warn("GitHub App credentials not configured - metrics will show zeros")
	}

	// Start background metrics collector
	startMetricsCollector(config, githubClient)

	// Middleware
	authMiddleware := makeAuthMiddleware(config)

	// Public endpoints
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/ready", readyHandler)
	http.Handle("/metrics", metricsHandler())

	// Protected API endpoints
	http.Handle("/api/v1/", loggingMiddleware(config, authMiddleware(makeAPIRouter(config))))

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
