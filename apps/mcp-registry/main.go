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

	var loader EntryLoader
	if config.DatabaseURL != "" {
		pgxLoader, closePool, err := newPGXEntryLoader(sigCtx, config.DatabaseURL)
		if err != nil {
			slog.Warn("Failed to initialize database pool", "error", err)
		} else {
			defer closePool()
			loader = pgxLoader
		}
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
