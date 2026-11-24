// Command fleet-control is the single source of truth for the MCP registry
// and fleet-gateway routes (#2334). It serves a read-only internal API and a write API for teams.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/navikt/copilot/fleet-control/internal/model"
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
	auth, err := newAuthzFromEnv()
	if err != nil {
		return err
	}
	cluster := os.Getenv("NAIS_CLUSTER_NAME")
	teams, err := teamsFromEnv(cluster)
	if err != nil {
		return err
	}
	srv := newServer()
	myCopilotID, err := clientIDFromEnv("MY_COPILOT_CLIENT_ID", "my-copilot")
	if err != nil {
		return err
	}
	adminClientID := adminClientFor(cluster, myCopilotID)
	srv.cluster = cluster
	srv.notifier = newSlackNotifier(os.Getenv("FLEET_SLACK_WEBHOOK_URL"))
	if _, ok := teams.(unboundTeams); ok {
		srv.teamsUnbound = true
		slog.Warn("no Console service account bound: team lookups answer 503")
	}
	// FLEET_CONTROL_OPERATOR_TEAM may set the global job kill switch; unset, nobody can.
	srv.operatorTeam = strings.TrimSpace(os.Getenv("FLEET_CONTROL_OPERATOR_TEAM"))
	ra, err := runtimeAuthFromEnv(auth)
	if err != nil {
		return err
	}
	sweepEvery, err := sweepIntervalFromEnv()
	if err != nil {
		return err
	}
	httpSrv := newHTTPServer(port, srv.routes(auth, adminClientID, teams, ra))

	go func() {
		db, err := connect(ctx)
		if err != nil {
			slog.Error("database setup failed; snapshots stay unavailable", "error", err)
			return
		}
		srv.store.Store(&storeBox{db})
		if ra != nil {
			go sweepLoop(ctx, db, sweepEvery)
		}
	}()

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

// newHTTPServer bounds header and body reads, so a caller cannot hold a
// connection open by stalling an upload. ReadTimeout covers reading the
// request only; there is no WriteTimeout, which would cut the claim
// long-poll (claimWait).
func newHTTPServer(port string, h http.Handler) *http.Server {
	return &http.Server{Addr: ":" + port, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second}
}

// connect migrates and seeds as the owner role, then opens the runtime pool
// (Nais READWRITE_ role when present). It retries until ctx ends.
func connect(ctx context.Context) (*DB, error) {
	for delay := time.Second; ; delay = min(delay*2, 30*time.Second) {
		db, err := setupDB(ctx)
		if err == nil {
			return db, nil
		}
		slog.Error("database unavailable; retrying", "retry_in", delay.String(), "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
}

func setupDB(ctx context.Context) (*DB, error) {
	owner, err := OpenDB(ctx, os.Getenv("DB_URL"))
	if err != nil {
		return nil, err
	}
	defer owner.Close()
	if err := owner.Migrate(ctx); err != nil {
		return nil, err
	}
	if err := seedFromEnv(ctx, owner); err != nil {
		return nil, err
	}
	// Every start: fills spec and spec_hash on published versions written
	// before 005 or by an older pod.
	if n, err := owner.BackfillSpecs(ctx); err != nil {
		return nil, fmt.Errorf("backfill specs: %w", err)
	} else if n > 0 {
		slog.Info("backfilled v1alpha1 specs", "versions", n)
	}
	entraIssuer = strings.TrimSpace(os.Getenv("AZURE_OPENID_CONFIG_ISSUER"))
	if entraIssuer == "" {
		slog.Warn("AZURE_OPENID_CONFIG_ISSUER is not set; agent profiles and delegated jobs are refused")
	}
	if n, err := owner.BackfillIdentity(ctx, entraIssuer); err != nil {
		return nil, fmt.Errorf("backfill identity: %w", err)
	} else if n > 0 {
		slog.Info("backfilled issuer and subject", "rows", n)
	}
	if err := seedAgentsFromEnv(ctx, owner); err != nil {
		return nil, err
	}
	// Names published before #2853 stay; the platform team sees them here.
	if names, err := owner.ForeignRegistryNames(ctx, strings.TrimSpace(os.Getenv("FLEET_CONTROL_OPERATOR_TEAM"))); err != nil {
		slog.Warn("list registry names outside their team's prefix", "error", err)
	} else if len(names) > 0 {
		slog.Warn("published registry names outside their team's prefix", "servers", names)
	}
	// Off by default until the export target in #2762 is decided. Runs on
	// each start, with the owner role.
	if os.Getenv("FLEET_AUDIT_RETENTION_DELETE") == "true" {
		if n, err := owner.PruneAudit(ctx); err != nil {
			slog.Warn("prune audit_events", "error", err)
		} else {
			slog.Info("pruned audit_events older than 90 days", "rows", n)
		}
	}
	// A failure here is logged, not fatal: CheckAuditLocked still refuses
	// startup if the runtime role kept its change rights.
	if err := owner.RevokeAuditChange(ctx); err != nil {
		slog.Warn("revoke audit_events rights", "error", err)
	}
	if os.Getenv("READWRITE_PGHOST") == "" {
		if err := ownerFallbackAllowed(os.Getenv("NAIS_CLUSTER_NAME")); err != nil {
			return nil, err
		}
		return OpenDB(ctx, os.Getenv("DB_URL"))
	}
	db, openErr := OpenDB(ctx, pgEnvDSN("READWRITE_"))
	if openErr != nil {
		return nil, openErr
	}
	if lockErr := db.CheckAuditLocked(ctx); lockErr != nil {
		db.Close()
		return nil, lockErr
	}
	return db, nil
}

// ownerFallbackAllowed refuses the owner role as runtime pool in Nais: there
// it would skip CheckAuditLocked and run with change rights on audit_events (#2854).
func ownerFallbackAllowed(cluster string) error {
	if cluster != "" {
		return errors.New("READWRITE_PGHOST is not set; in Nais fleet-control refuses to run as the owner role")
	}
	return nil
}

// seedFromEnv imports SEED_ROUTES_FILE and SEED_ALLOWLIST_FILE when both are set.
func seedFromEnv(ctx context.Context, db *DB) error {
	routesPath, allowlistPath := os.Getenv("SEED_ROUTES_FILE"), os.Getenv("SEED_ALLOWLIST_FILE")
	if routesPath == "" || allowlistPath == "" {
		return nil
	}
	// A running instance never needs the files again.
	if seeded, err := db.HasServers(ctx); err != nil || seeded {
		return err
	}
	routes, err := os.ReadFile(routesPath) //nolint:gosec // operator-set path baked into the image
	if err != nil {
		return err
	}
	allowlist, err := os.ReadFile(allowlistPath) //nolint:gosec // operator-set path baked into the image
	if err != nil {
		return err
	}
	servers, err := seedServers(routes, allowlist)
	if err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	seeded, err := db.Seed(ctx, servers, routesPath+" + "+allowlistPath)
	if err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	slog.Info("seed", "imported", seeded, "servers", len(servers))
	return nil
}

// storeBox lets atomic.Pointer hold an interface.
type storeBox struct{ snapshotStore }

type snapshotStore interface {
	Ping(ctx context.Context) error
	Servers(ctx context.Context) ([]model.Server, error)
	LatestSnapshot(ctx context.Context, kind string) (*model.Snapshot, error)
	// Publish reads servers and the latest snapshot of kind, builds the next
	// one and stores it, all in one transaction serialised per kind.
	Publish(ctx context.Context, kind string, build func(string, []model.Server, *model.Snapshot) (*model.Snapshot, error)) (*model.Snapshot, error)
}

type server struct {
	// teamsUnbound: no Console service account yet; /ready says so but stays 200.
	teamsUnbound bool
	// cluster is NAIS_CLUSTER_NAME; route targets must be in it, since the
	// gateway resolves service DNS in its own cluster.
	cluster string
	// operatorTeam may set the global job kill switch.
	operatorTeam string
	store        atomic.Pointer[storeBox]
	// lastGood holds the last served snapshot per kind, so a database outage
	// keeps serving it; after a restart there is none and the answer is 503.
	lastGood map[string]*atomic.Pointer[model.Snapshot]
	// verifier binds an agent's client id to its app; nil leaves profiles unverified.
	verifier identityVerifier
	// notifier posts MCPServer changes to Slack; nil sends nothing.
	notifier *slackNotifier
}

func newServer() *server {
	return &server{lastGood: map[string]*atomic.Pointer[model.Snapshot]{"routes": {}, "registry": {}}}
}
