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

	slog.Info("Starting MCP Registry server",
		"port", config.Port,
		"domain_internal", config.DomainInternal,
		"domain_external", config.DomainExternal,
		"log_level", config.LogLevel.String(),
		"logged_endpoints", getEndpointsList(config.LoggedEndpoints),
	)

	// Initialize OTel tracing — must happen before any instrumented handlers are
	// set up. Reads OTEL_EXPORTER_OTLP_ENDPOINT injected by Nais (runtime: sdk).
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

	if err := validateAllowListFile(); err != nil {
		slog.Error("Server startup failed - invalid allowlist.json", "error", err)
		os.Exit(1)
	}

	// otelHandler wraps a content handler with OTel HTTP tracing and a
	// per-route span name (method + path). Health/ready/metrics and static
	// endpoints are left unwrapped to avoid tracing probe/scrape noise.
	otelHandler := func(name string, h http.HandlerFunc) http.Handler {
		return otelhttp.NewHandler(h, name, otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", loggingMiddleware(config, healthHandler))
	mux.HandleFunc("/ready", loggingMiddleware(config, readyHandler))
	mux.Handle("/metrics", metricsHandler())
	mux.Handle("/v0.1/servers", otelHandler("servers-list", loggingMiddleware(config, makeServersListHandler(config))))
	mux.Handle("/v0.1/servers/", otelHandler("server-version", loggingMiddleware(config, makeServerVersionHandler(config))))
	mux.HandleFunc("/robots.txt", robotsTxtHandler)
	mux.HandleFunc("/favicon.ico", faviconHandler)
	mux.HandleFunc("/.well-known/security.txt", securityTxtHandler)
	mux.Handle("/", otelHandler("root", loggingMiddleware(config, rootHandler)))

	slog.Info("Allowlist validation passed - registry contains valid server configurations")
	slog.Info("Server listening", "port", config.Port)

	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown so the deferred tracer flush runs on SIGTERM.
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
