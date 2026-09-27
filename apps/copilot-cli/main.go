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

	ctx := context.Background()

	auth := &authenticator{
		github: newGitHubClient(config.GitHubClientID, config.GitHubClientSecret),
		entra:  newEntraClient(config.NaisTokenIntrospectionEndpoint, config.AzurePreAuthorizedApps),
		org:    config.GitHubOrg,
		cache:  newTokenCache(config.OrgMembershipCacheTTL),
		limit:  rate.NewLimiter(1, 10),
	}
	slog.Info("Sign-in paths", "github", auth.github.configured(), "entra", auth.entra.configured())

	texas := newTexasClient(config.NaisTokenEndpoint, config.CopilotAPIAudience)
	proxy := newCopilotAPIProxy(config.CopilotAPIURL, texas)

	defs, err := loadSurveyDir(surveyFiles)
	if err != nil {
		slog.Error("Invalid survey definitions", "error", err)
		os.Exit(1)
	}
	lookup, err := newSAMLLookup(config.GitHubOrg, config.GitHubAppID, config.GitHubAppInstallationID, config.GitHubAppPrivateKey)
	if err != nil {
		slog.Error("Invalid GitHub App credentials", "error", err)
		os.Exit(1)
	}
	surveys := &surveyAPI{surveys: defs, keys: surveyKeys(defs), emailFor: emailFor(lookup), now: time.Now}
	var store *surveyStore
	if config.DatabaseURL != "" {
		if store, err = openSurveyStore(ctx, config.DatabaseURL); err != nil {
			slog.Error("Survey storage unavailable", "error", err)
			os.Exit(1)
		}
		surveys.participate, surveys.record = store.participate, store.record
		go store.purgeExpired(ctx)
	}
	slog.Info("Survey submissions", "storage", store != nil, "github_email_lookup", lookup != nil,
		"surveys", len(defs), "surveys_with_key", len(surveys.keys))

	server := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           makeRouter(auth, proxy, surveys),
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
	if store != nil {
		store.flush(ctx) // write answers still queued for a batch
	}
	slog.Info("Server stopped")
}
