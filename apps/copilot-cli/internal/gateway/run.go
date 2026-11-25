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

	"github.com/navikt/copilot/apps/copilot-cli/internal/auth"
	"github.com/navikt/copilot/apps/copilot-cli/internal/config"
	"github.com/navikt/copilot/apps/copilot-cli/internal/texas"
	"github.com/navikt/copilot/apps/copilot-cli/internal/upstream"
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

	api := upstream.New("copilot-api", cfg.CopilotAPIURL, texas.New(cfg.NaisTokenEndpoint, cfg.CopilotAPIAudience))
	github := auth.NewGitHubClient(auth.GitHubAPIBaseURL, cfg.GitHubClientID, cfg.GitHubClientSecret)
	authenticator := auth.New(github, cfg.GitHubOrg, api.IsOrgMember, cfg.OrgMembershipCacheTTL, rate.NewLimiter(1, 10))
	slog.Info("GitHub sign-in", "configured", github.Configured())

	surveys := upstream.New("copilot-survey", cfg.CopilotSurveyURL, texas.New(cfg.NaisTokenEndpoint, cfg.CopilotSurveyAudience))

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           makeRouter(authenticator, api, surveys),
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
