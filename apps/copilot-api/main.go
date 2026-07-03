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

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

	// Initialize OTel tracing — must happen before any instrumented handlers are set up.
	// Reads OTEL_EXPORTER_OTLP_ENDPOINT injected by Nais (runtime: sdk).
	ctx := context.Background()
	shutdownTracer, err := initTracer(ctx, "copilot-api")
	if err != nil {
		slog.Warn("OTel tracer initialization failed — tracing disabled", "error", err)
	} else {
		defer func() {
			if err := shutdownTracer(ctx); err != nil {
				slog.Warn("OTel tracer shutdown error", "error", err)
			}
		}()
	}

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
	var rawBQClient *BigQueryClient
	if config.GCPProjectID != "" {
		bqClient, err := newBigQueryClient(config)
		if err != nil {
			slog.Warn("BigQuery client initialization failed - data endpoints will be unavailable", "error", err)
		} else {
			rawBQClient = bqClient
			cacheTTL := time.Duration(config.CacheTTLHours) * time.Hour
			cachedBQClient = newCachedBigQueryClient(bqClient, cacheTTL)
			bqHandlers = newBigQueryHandlers(cachedBQClient)
			bqHandlers.environment = config.Environment
			slog.Info("BigQuery client initialized successfully", "cache_ttl", cacheTTL)
		}
	} else {
		slog.Warn("GCP_TEAM_PROJECT_ID not configured - BigQuery endpoints will be unavailable")
	}
	if cachedBQClient != nil {
		defer cachedBQClient.Close()
	}

	// Wire GitHub client into BigQuery handlers for per-user ownership checks.
	// This is independent of the budget client — ownership verification only needs SAML.
	if bqHandlers != nil && githubClient != nil {
		bqHandlers.setGitHubClient(githubClient)
	}

	// Initialize budget client (optional - requires GITHUB_BILLING_TOKEN classic PAT)
	var budgetHandlers *BudgetHandlers
	switch {
	case config.GitHubBillingToken == "":
		slog.Warn("GITHUB_BILLING_TOKEN not configured - budget endpoint will be unavailable")
	case githubClient == nil:
		slog.Warn("GitHub client unavailable - budget endpoint will be unavailable (see GitHub client initialization error above)")
	default:
		budgetClient := newBudgetClient(config.GitHubBillingToken, config.GitHubEnterprise)
		budgetHandlers = newBudgetHandlers(budgetClient, githubClient)
		if bqHandlers != nil {
			bqHandlers.setBudgetClient(budgetClient)
		}
		slog.Info("Budget client initialized successfully")
	}

	// Middleware
	authMiddleware := makeAuthMiddleware(config)
	videoHandlers := newVideoHandlers(config)

	// Set up routing on a local mux (not global DefaultServeMux)
	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)
	mux.Handle("/metrics", metricsHandler())

	// Dev-only raw BigQuery query endpoint (no auth, local only)
	if rawBQClient != nil && config.Environment == "local" {
		mux.HandleFunc("/dev/query", rawBQClient.devQueryHandler)
	}
	mux.Handle("/public/v1/", otelhttp.NewHandler(
		loggingMiddleware(config, makePublicRouter(config, videoHandlers)),
		"public-api",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	))

	// Protected API endpoints — wrapped with OTel tracing
	mux.Handle("/api/v1/", otelhttp.NewHandler(
		loggingMiddleware(config, authMiddleware(makeAPIRouter(config, bqHandlers, ghHandlers, budgetHandlers))),
		"api",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	))

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
