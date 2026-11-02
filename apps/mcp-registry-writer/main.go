package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	config := loadConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.LogLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting MCP Registry Writer server",
		"port", config.Port,
		"log_level", config.LogLevel.String(),
		"logged_endpoints", getEndpointsList(config.LoggedEndpoints),
	)

	ctx := context.Background()
	shutdownTracer, err := initTracer(ctx, "mcp-registry-writer")
	if err != nil {
		slog.Warn("OTel tracer initialization failed — tracing disabled", "error", err)
	} else {
		defer func() {
			if shutErr := shutdownTracer(ctx); shutErr != nil {
				slog.Warn("OTel tracer shutdown error", "error", shutErr)
			}
		}()
	}

	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if config.DatabaseURL == "" {
		return errors.New("database is not configured: DB_URL or DB_HOST is required")
	}

	store, connErr := NewStore(sigCtx, config.DatabaseURL)
	if connErr != nil {
		return fmt.Errorf("connecting to database: %w", connErr)
	}
	defer store.Close()

	if migErr := store.Migrate(sigCtx); migErr != nil {
		return fmt.Errorf("applying database migrations: %w", migErr)
	}
	slog.Info("Database migrations applied successfully")

	introspect := newTexasIntrospector(config.TexasEndpoint)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", loggingMiddleware(config, healthHandler))
	mux.HandleFunc("/ready", loggingMiddleware(config, readyHandler(store)))
	mux.Handle("/metrics", metricsHandler())
	mux.HandleFunc("POST /api/v1/entries", loggingMiddleware(config, makeSubmitHandler(config, store, introspect)))
	mux.HandleFunc("POST /api/v1/entries/{name...}", loggingMiddleware(config, makeEntryActionHandler(config, store, introspect)))

	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      traceHTTP(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Server listening", "port", config.Port)
		if listenErr := server.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			slog.Error("Server failed", "error", listenErr)
		}
	}()

	<-sigCtx.Done()
	slog.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if shutErr := server.Shutdown(shutdownCtx); shutErr != nil {
		return fmt.Errorf("server shutdown: %w", shutErr)
	}

	return nil
}
