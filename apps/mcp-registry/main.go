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

	slog.Info("Starting MCP Registry server",
		"port", config.Port,
		"domain_internal", config.DomainInternal,
		"domain_external", config.DomainExternal,
		"log_level", config.LogLevel.String(),
		"logged_endpoints", getEndpointsList(config.LoggedEndpoints),
	)

	if err := validateAllowListFile(); err != nil {
		slog.Error("Server startup failed - invalid allowlist.json", "error", err)
		os.Exit(1)
	}

	// Initialize OTel tracing. Reads OTEL_EXPORTER_OTLP_ENDPOINT injected by
	// Nais (runtime: sdk); a no-op without it.
	ctx := context.Background()
	shutdownTracer, err := initTracer(ctx, "mcp-registry")
	if err != nil {
		slog.Warn("OTel tracer initialization failed — tracing disabled", "error", err)
	} else {
		defer func() {
			if err := shutdownTracer(ctx); err != nil {
				slog.Warn("OTel tracer shutdown error", "error", err)
			}
		}()
	}

	// Graceful shutdown, so the deferred tracer shutdown flushes spans.
	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// The DB is optional: without DATABASE_URL the reader serves static entries
	// only and the write endpoints answer 503. With it configured, a DB that
	// cannot be reached or migrated is fatal, so a broken replica never turns ready.
	var db *DB
	var loader EntryLoader
	if config.DatabaseURL != "" {
		opened, err := openAndMigrate(sigCtx, config.DatabaseURL, 5, 3*time.Second)
		if err != nil {
			stop()
			slog.Error("Database unavailable, exiting", "error", err)
			os.Exit(1) //nolint:gocritic // fail closed; stop() already ran, tracer flush is best effort

		}
		defer opened.Close()
		db = opened
		loader = db.GetApprovedEntries
	}

	store := NewStore(config, "allowlist.json", loader)
	if loader != nil {
		if err := store.Reload(sigCtx); err != nil {
			slog.Warn("Initial database reload failed, serving static entries only", "error", err)
		}
		store.StartRefresher(sigCtx, 60*time.Second)
	}
	config.Store = store

	http.HandleFunc("/health", loggingMiddleware(config, healthHandler))
	http.HandleFunc("/ready", loggingMiddleware(config, readyHandler))
	http.Handle("/metrics", metricsHandler())
	http.HandleFunc("/v0.1/servers", loggingMiddleware(config, makeServersListHandler(config)))
	http.HandleFunc("/v0.1/servers/", loggingMiddleware(config, makeServerVersionHandler(config)))
	introspect := newTexasIntrospector(config.TexasEndpoint)
	http.HandleFunc("POST /api/v1/entries", loggingMiddleware(config, makeSubmitHandler(config, db, introspect)))
	http.HandleFunc("POST /api/v1/entries/{name...}", loggingMiddleware(config, makeEntryActionHandler(config, db, introspect)))
	http.HandleFunc("/robots.txt", robotsTxtHandler)
	http.HandleFunc("/favicon.ico", faviconHandler)
	http.HandleFunc("/.well-known/security.txt", securityTxtHandler)
	http.HandleFunc("/", loggingMiddleware(config, rootHandler))

	slog.Info("Allowlist validation passed - registry contains valid server configurations")
	slog.Info("Server listening", "port", config.Port)

	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      traceHTTP(http.DefaultServeMux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-sigCtx.Done()
	slog.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	}
}

// openAndMigrate retries OpenDB and Migrate a bounded number of times, since
// the Cloud SQL proxy may start after the app.
func openAndMigrate(ctx context.Context, dbURL string, attempts int, delay time.Duration) (*DB, error) {
	var err error
	for i := 1; i <= attempts; i++ {
		var db *DB
		if db, err = OpenDB(ctx, dbURL); err == nil {
			if err = db.Migrate(ctx); err == nil {
				return db, nil
			}
			db.Close()
		}
		slog.Warn("Database not ready", "attempt", i, "error", err)
		if i < attempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}
	return nil, err
}
