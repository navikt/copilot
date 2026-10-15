package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// No OpenTelemetry here, on purpose: a trace of a submission would time-stamp
// who answered (see store.go and the privacy notes in README.md).
func main() {
	if len(os.Args) > 1 && os.Args[1] == "export" {
		if err := runExport(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "export:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "import" {
		if err := runImport(os.Args[2:], os.Stdin, os.Stdout, os.Getenv("DB_URL"), time.Now()); err != nil {
			fmt.Fprintln(os.Stderr, "import:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "aggregate" {
		if err := runAggregate(os.Args[2:], os.Stdout, os.Getenv("DB_URL"), time.Now()); err != nil {
			fmt.Fprintln(os.Stderr, "aggregate:", err)
			os.Exit(1)
		}
		return
	}
	config := loadConfig()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel})))

	defs, err := loadSurveyDir(surveyFiles)
	if err != nil {
		slog.Error("Invalid survey definitions", "error", err)
		os.Exit(1)
	}
	defs = forCluster(defs, os.Getenv("NAIS_CLUSTER_NAME"))
	activateInDev(defs, os.Getenv("NAIS_CLUSTER_NAME"), os.Getenv("SURVEY_ACTIVE_IDS"))
	auth := newAuthenticator(config.NaisTokenIntrospectionEndpoint, config.AzurePreAuthorizedApps)
	lookup := newNameIDClient(config.CopilotAPIURL, newTexasClient(config.NaisTokenEndpoint, config.CopilotAPIAudience))
	surveys := &surveyAPI{surveys: defs, keys: surveyKeys(defs, time.Now()), emailFor: lookup.emailFor, now: time.Now, textKey: make([]byte, 32)}
	_, _ = rand.Read(surveys.textKey) // crypto/rand: never fails

	ctx := context.Background()
	var store *surveyStore
	if config.DatabaseURL != "" {
		if store, err = openSurveyStore(ctx, config.DatabaseURL); err != nil {
			slog.Error("Survey storage unavailable", "error", err)
			os.Exit(1)
		}
		surveys.store = store.submit
		surveys.answers = store.answers
		surveys.answered = store.answered
		surveys.final = store.final
		store.defs = defs
		surveys.hidden = store.hidden
		surveys.hide = store.hide
		go store.purgeExpired(ctx, time.Now)
	}
	slog.Info("Starting copilot-survey", "port", config.Port, "storage", store != nil,
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
