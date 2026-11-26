// Command fleet-control is the single source of truth for the MCP registry
// and fleet-gateway routes (#2334). It serves a read-only internal API and a write API for teams.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/navikt/copilot/fleet-control/internal/api"
	"github.com/navikt/copilot/fleet-control/internal/auth"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("fleet-control stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	authz, err := auth.NewAuthzFromEnv()
	if err != nil {
		return err
	}
	cluster := os.Getenv("NAIS_CLUSTER_NAME")
	teams, err := auth.TeamsFromEnv(cluster)
	if err != nil {
		return err
	}
	myCopilotID, err := auth.ClientIDFromEnv("MY_COPILOT_CLIENT_ID", "my-copilot")
	if err != nil {
		return err
	}
	a, err := api.New(api.Config{
		Authz:         authz,
		Teams:         teams,
		AdminClientID: myCopilotID,
		Cluster:       cluster,
		// FLEET_CONTROL_OPERATOR_TEAM is the platform team: it approves MCP servers,
		// may revoke them anywhere and sets the global job kill switch. Unset: nobody can.
		OperatorTeam:    strings.TrimSpace(os.Getenv("FLEET_CONTROL_OPERATOR_TEAM")),
		SlackWebhookURL: os.Getenv("FLEET_SLACK_WEBHOOK_URL"),
	})
	if err != nil {
		return err
	}
	httpSrv := api.NewHTTPServer(port, a.Handler())

	go a.Connect(ctx)

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()
	slog.Info("fleet-control listening", "port", port)
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
