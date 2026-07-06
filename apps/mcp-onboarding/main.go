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

	"github.com/navikt/copilot/mcp-onboarding/internal/discovery"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Config struct {
	Port                string
	BaseURL             string
	GitHubClientID      string
	GitHubClientSecret  string
	AllowedOrganization string
	LogLevel            string
}

func LoadConfig() *Config {
	return &Config{
		Port:                getEnv("PORT", "8080"),
		BaseURL:             getEnv("BASE_URL", "http://localhost:8080"),
		GitHubClientID:      getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:  getEnv("GITHUB_CLIENT_SECRET", ""),
		AllowedOrganization: getEnv("ALLOWED_ORGANIZATION", "navikt"),
		LogLevel:            getEnv("LOG_LEVEL", "INFO"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (c *Config) Validate() error {
	if c.GitHubClientID == "" {
		slog.Warn("GITHUB_CLIENT_ID not set - OAuth will not work")
	}
	if c.GitHubClientSecret == "" {
		slog.Warn("GITHUB_CLIENT_SECRET not set - OAuth will not work")
	}
	return nil
}

func main() {
	cfg := LoadConfig()

	var logLevel slog.Level
	switch cfg.LogLevel {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	if err := cfg.Validate(); err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

// run owns all deferred cleanup (tracer flush, signal handling). Keeping
// os.Exit confined to main ensures these defers actually run on failure.
func run(cfg *Config) error {
	// Initialize OTel tracing — must happen before instrumented handlers are set
	// up. Reads OTEL_EXPORTER_OTLP_ENDPOINT injected by Nais (runtime: sdk).
	ctx := context.Background()
	shutdownTracer, err := initTracer(ctx, "mcp-onboarding")
	if err != nil {
		slog.Warn("OTel tracer initialization failed — tracing disabled", "error", err)
	} else {
		defer func() {
			if err := shutdownTracer(ctx); err != nil {
				slog.Warn("OTel tracer shutdown error", "error", err)
			}
		}()
	}

	store := NewTokenStore()
	githubClient := NewGitHubClient(cfg.GitHubClientID, cfg.GitHubClientSecret)
	oauthServer := NewOAuthServer(cfg.BaseURL, githubClient, store, cfg.AllowedOrganization)

	// Initialize discovery service with embedded manifest
	discoveryService := discovery.NewService("navikt", "copilot", "main", cfg.BaseURL)
	if err := discoveryService.LoadManifest(); err != nil {
		return fmt.Errorf("failed to load embedded manifest: %w", err)
	}
	manifest := discoveryService.GetManifest()
	slog.Info("loaded customizations manifest",
		"agents", len(manifest.Agents),
		"instructions", len(manifest.Instructions),
		"prompts", len(manifest.Prompts),
		"skills", len(manifest.Skills),
	)
	mcpHandler := NewMCPHandler(githubClient, discoveryService)
	authMiddleware := NewAuthMiddleware(store)

	mux := http.NewServeMux()

	oauthServer.RegisterRoutes(mux)

	mux.Handle("GET /mcp", authMiddleware.Authenticate(mcpHandler))
	mux.Handle("POST /mcp", authMiddleware.Authenticate(mcpHandler))

	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /ready", handleReady)
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		updateTokenStoreGauges(store)
		promhttp.Handler().ServeHTTP(w, r)
	})

	mux.HandleFunc("/", handleRoot)

	// Wrap the handler chain with OTel HTTP tracing (per-route span names).
	// loggingMiddleware runs inside the span; health/ready/metrics are filtered
	// out to avoid tracing probe/scrape noise.
	handler := otelhttp.NewHandler(
		loggingMiddleware(mux),
		"mcp-onboarding",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
		otelhttp.WithFilter(func(r *http.Request) bool {
			switch r.URL.Path {
			case "/health", "/ready", "/metrics":
				return false
			default:
				return true
			}
		}),
	)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	slog.Info("starting mcp-onboarding server",
		"port", cfg.Port,
		"base_url", cfg.BaseURL,
		"allowed_org", cfg.AllowedOrganization,
	)

	// Graceful shutdown so the deferred tracer flush runs on SIGTERM.
	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-sigCtx.Done():
	}
	slog.Info("shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	return nil
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy"}`))
}

func handleReady(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Nav MCP Hello World + Discovery</title></head>
<body>
<h1>Nav MCP Hello World + Discovery Server</h1>
<p>This is a reference MCP (Model Context Protocol) server with GitHub OAuth authentication and NAV Copilot customization discovery.</p>
<h2>Endpoints</h2>
<ul>
<li><a href="/.well-known/oauth-authorization-server">OAuth Authorization Server Metadata</a></li>
<li><a href="/.well-known/oauth-protected-resource">OAuth Protected Resource Metadata</a></li>
<li><code>POST /mcp</code> - MCP JSON-RPC endpoint (requires authentication)</li>
</ul>
<h2>Available Tools</h2>
<ul>
<li><code>hello_world</code> - Returns a friendly greeting with your GitHub username</li>
<li><code>greet</code> - Returns a personalized greeting</li>
<li><code>whoami</code> - Returns information about the authenticated user</li>
<li><code>echo</code> - Echoes back a message</li>
<li><code>get_time</code> - Returns the current server time</li>
<li><code>search_customizations</code> - Search NAV Copilot customizations</li>
<li><code>list_agents</code> - List all NAV Copilot agents</li>
<li><code>list_instructions</code> - List all NAV Copilot instructions</li>
<li><code>list_prompts</code> - List all NAV Copilot prompts</li>
<li><code>list_skills</code> - List all NAV Copilot skills</li>
<li><code>get_installation_guide</code> - Get installation guide for a customization</li>
</ul>
</body>
</html>`))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		if r.URL.Path == "/health" || r.URL.Path == "/ready" || r.URL.Path == "/metrics" {
			return
		}

		duration := time.Since(start)
		recordHTTPMetrics(r.Method, r.URL.Path, wrapped.statusCode, duration)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
