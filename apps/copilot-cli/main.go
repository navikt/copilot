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

	slog.Info("Starting copilot-cli server",
		"port", config.Port,
		"environment", config.Environment,
		"github_org", config.GitHubOrg,
		"copilot_api_url", config.CopilotAPIURL,
	)

	if config.NaisTokenEndpoint == "" {
		slog.Warn("NAIS_TOKEN_ENDPOINT not configured — M2M proxy calls to copilot-api will fail (expected in local dev)")
	}

	gh := newGitHubClient()
	cache := newOrgMembershipCache(config.OrgMembershipCacheTTL)
	texas := newTexasClient(config.NaisTokenEndpoint, config.CopilotAPIAudience)
	proxy := newCopilotAPIProxy(config.CopilotAPIURL, texas)

	handler := makeRouter(config, gh, cache, proxy)

	server := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("Server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}
	slog.Info("Server stopped")
}
