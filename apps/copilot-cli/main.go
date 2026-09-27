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

	"golang.org/x/time/rate"
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

	auth := &authenticator{
		github: newGitHubClient(config.GitHubClientID, config.GitHubClientSecret),
		org:    config.GitHubOrg,
		cache:  newTokenCache(config.OrgMembershipCacheTTL),
		limit:  rate.NewLimiter(1, 10),
	}
	slog.Info("GitHub sign-in", "configured", auth.github.configured())

	api := newUpstream("copilot-api", config.CopilotAPIURL, newTexasClient(config.NaisTokenEndpoint, config.CopilotAPIAudience))
	surveys := newUpstream("copilot-survey", config.CopilotSurveyURL, newTexasClient(config.NaisTokenEndpoint, config.CopilotSurveyAudience))

	server := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           makeRouter(auth, api, surveys),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		MaxHeaderBytes:    32 << 10,
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
