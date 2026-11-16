package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
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

	// The DB is optional: without DATABASE_URL, or while it cannot be reached,
	// the reader serves static entries and the write endpoints answer 503.
	// Reads must stay up when the DB is down, so a failing DB never blocks startup.
	var db atomic.Pointer[DB]
	var loader EntryLoader
	if config.DatabaseURL != "" {
		loader = func(ctx context.Context) ([]DBEntry, error) {
			d := db.Load()
			if d == nil {
				return nil, errors.New("database not connected")
			}
			return d.GetApprovedEntries(ctx)
		}
	}
	store := NewStore(config, "allowlist.json", loader)
	if loader != nil {
		go func() {
			opened, err := connectWithRetry(sigCtx, func(ctx context.Context) (*DB, error) {
				return openAndMigrate(ctx, config.DatabaseURL)
			}, 10*time.Second, time.Second, time.Minute)
			if err != nil {
				return // ctx cancelled
			}
			db.Store(opened)
			slog.Info("Database connected and migrated, writes enabled")
			if err := store.Reload(sigCtx); err != nil {
				slog.Warn("Initial database reload failed, serving static entries only", "error", err)
			}
			store.StartRefresher(sigCtx, 60*time.Second)
			<-sigCtx.Done()
			opened.Close()
		}()
	}
	config.Store = store

	http.HandleFunc("/health", loggingMiddleware(config, healthHandler))
	http.HandleFunc("/ready", loggingMiddleware(config, readyHandler))
	http.Handle("/metrics", metricsHandler())
	http.HandleFunc("/v0.1/servers", loggingMiddleware(config, makeServersListHandler(config)))
	http.HandleFunc("/v0.1/servers/", loggingMiddleware(config, makeServerVersionHandler(config)))
	introspect := newTexasIntrospector(config.TexasEndpoint)
	http.HandleFunc("POST /api/v1/entries", loggingMiddleware(config, makeSubmitHandler(config, &db, introspect)))
	http.HandleFunc("POST /api/v1/entries/{name...}", loggingMiddleware(config, makeEntryActionHandler(config, &db, introspect)))
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

// openAndMigrate opens the DB and applies migrations, closing it on failure.
func openAndMigrate(ctx context.Context, dbURL string) (*DB, error) {
	db, err := OpenDB(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// connectWithRetry calls connect until it succeeds or ctx is cancelled. Each
// attempt is bounded by timeout; the delay doubles from minDelay up to maxDelay.
func connectWithRetry(ctx context.Context, connect func(context.Context) (*DB, error), timeout, minDelay, maxDelay time.Duration) (*DB, error) {
	delay := minDelay
	for attempt := 1; ; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		db, err := connect(attemptCtx)
		cancel()
		if err == nil {
			return db, nil
		}
		slog.Error("Database unavailable, serving static entries only; retrying", "attempt", attempt, "retry_in", delay.String(), "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
		delay = min(delay*2, maxDelay)
	}
}
