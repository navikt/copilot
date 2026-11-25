// Package api is the HTTP API. Survey definitions and aggregation are in
// internal/survey, storage in internal/store behind Store.
package api

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/navikt/copilot/apps/copilot-survey/internal/config"
	"github.com/navikt/copilot/apps/copilot-survey/internal/store"
	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
	"github.com/navikt/copilot/apps/copilot-survey/internal/texas"
)

// Serve runs the HTTP server until SIGINT or SIGTERM.
func Serve(config *config.Config) {
	defs, err := survey.LoadSurveyDir(survey.SurveyFiles)
	if err != nil {
		slog.Error("Invalid survey definitions", "error", err)
		os.Exit(1)
	}
	defs = survey.ForCluster(defs, os.Getenv("NAIS_CLUSTER_NAME"))
	survey.ActivateInDev(defs, os.Getenv("NAIS_CLUSTER_NAME"), os.Getenv("SURVEY_ACTIVE_IDS"))
	auth := newAuthenticator(config.NaisTokenIntrospectionEndpoint, config.AzurePreAuthorizedApps)
	lookup := newNameIDClient(config.CopilotAPIURL, texas.New(config.NaisTokenEndpoint, config.CopilotAPIAudience))
	codebooks, err := survey.LoadCodebooks(survey.SurveyFiles)
	if err != nil {
		slog.Error("Invalid codebooks", "error", err)
		os.Exit(1)
	}
	surveys := &surveyAPI{surveys: defs, keys: survey.Keys(defs, time.Now()), emailFor: lookup.emailFor, now: time.Now, textKey: make([]byte, 32), codebooks: codebooks}
	_, _ = rand.Read(surveys.textKey) // crypto/rand: never fails

	ctx := context.Background()
	if config.DatabaseURL != "" {
		st, err := store.Open(ctx, config.DatabaseURL)
		if err != nil {
			slog.Error("Survey storage unavailable", "error", err)
			os.Exit(1)
		}
		st.Defs, st.Codebooks = defs, codebooks
		surveys.store = st
		go st.PurgeExpired(ctx, time.Now)
	}
	slog.Info("Starting copilot-survey", "port", config.Port, "storage", surveys.store != nil,
		"copilot_cli_trusted", auth.cliID != "", "my_copilot_trusted", auth.webID != "",
		"surveys", len(defs), "surveys_with_key", len(surveys.keys))

	server := &http.Server{
		Addr:              ":" + config.Port,
		Handler:           makeRouter(auth, surveys),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}
}
