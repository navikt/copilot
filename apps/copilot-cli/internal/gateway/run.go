// Package gateway implements copilot-cli, a NAIS-hosted gateway for nav-pilot.
// It signs callers in with a GitHub token (navikt org members only) and
// forwards usage requests to copilot-api and survey requests to
// copilot-survey.
package gateway

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

	"github.com/navikt/copilot/apps/copilot-cli/internal/config"
)

// Run starts the server and blocks until SIGINT or SIGTERM.
func Run() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting copilot-cli server",
		"port", cfg.Port,
		"environment", cfg.Environment,
		"github_org", cfg.GitHubOrg,
		"copilot_api_url", cfg.CopilotAPIURL,
	)

	if cfg.NaisTokenEndpoint == "" {
		slog.Warn("NAIS_TOKEN_ENDPOINT not configured — M2M calls to copilot-api and copilot-survey will fail (expected in local dev)")
	}

	api := newUpstream("copilot-api", cfg.CopilotAPIURL, newTexasClient(cfg.NaisTokenEndpoint, cfg.CopilotAPIAudience))
	auth := &authenticator{
		github:   newGitHubClient(cfg.GitHubClientID, cfg.GitHubClientSecret),
		org:      cfg.GitHubOrg,
		isMember: api.isOrgMember,
		cache:    newTokenCache(cfg.OrgMembershipCacheTTL),
		limit:    rate.NewLimiter(1, 10),
	}
	slog.Info("GitHub sign-in", "configured", auth.github.configured())

	surveys := newUpstream("copilot-survey", cfg.CopilotSurveyURL, newTexasClient(cfg.NaisTokenEndpoint, cfg.CopilotSurveyAudience))

	server := &http.Server{
		Addr:              ":" + cfg.Port,
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
