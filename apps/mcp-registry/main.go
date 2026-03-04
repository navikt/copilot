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

	http.HandleFunc("/health", loggingMiddleware(config, healthHandler))
	http.HandleFunc("/ready", loggingMiddleware(config, readyHandler))
	http.Handle("/metrics", metricsHandler())
	http.HandleFunc("/v0.1/servers", loggingMiddleware(config, makeServersListHandler(config)))
	http.HandleFunc("/v0.1/servers/", loggingMiddleware(config, makeServerVersionHandler(config)))
	http.HandleFunc("/", loggingMiddleware(config, rootHandler))

	slog.Info("Allowlist validation passed - registry contains valid server configurations")
	slog.Info("Server listening", "port", config.Port)

	server := &http.Server{
		Addr:         ":" + config.Port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
