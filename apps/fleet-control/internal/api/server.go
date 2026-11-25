// Package api is fleet-control's HTTP layer: the /apis resources, the /admin
// write API, the /internal snapshot API and the database behind them (#2894).
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/navikt/copilot/fleet-control/internal/auth"
	"github.com/navikt/copilot/fleet-control/internal/model"
)

// Config is what main reads from the environment and hands to New.
type Config struct {
	Authz         *auth.Authz
	Teams         auth.TeamDirectory
	AdminClientID string
	// Cluster is NAIS_CLUSTER_NAME.
	Cluster string
	// PilotTeams may write outside dev-*.
	PilotTeams []string
	// OperatorTeam may set the global job kill switch; empty, nobody can.
	OperatorTeam    string
	SlackWebhookURL string
}

// API is the wired HTTP layer. The database arrives later, through Connect.
type API struct {
	srv        *server
	handler    http.Handler
	ra         *runtimeAuth
	sweepEvery time.Duration
}

// New builds the handler. Until Connect succeeds, snapshot and write routes answer 503.
func New(cfg Config) (*API, error) {
	srv := newServer()
	srv.cluster = cfg.Cluster
	srv.pilotTeams = cfg.PilotTeams
	if !auth.DevCluster(cfg.Cluster) && len(srv.pilotTeams) == 0 {
		slog.Warn("FLEET_CONTROL_PILOT_TEAMS empty outside dev: every write is refused", "cluster", cfg.Cluster)
	}
	srv.notifier = newSlackNotifier(cfg.SlackWebhookURL)
	if _, ok := cfg.Teams.(auth.UnboundTeams); ok {
		srv.teamsUnbound = true
		slog.Warn("no Console service account bound: team lookups answer 503")
	}
	srv.operatorTeam = cfg.OperatorTeam
	ra, err := runtimeAuthFromEnv(cfg.Authz)
	if err != nil {
		return nil, err
	}
	sweepEvery, err := sweepIntervalFromEnv()
	if err != nil {
		return nil, err
	}
	return &API{srv: srv, handler: srv.routes(cfg.Authz, cfg.AdminClientID, cfg.Teams, ra), ra: ra, sweepEvery: sweepEvery}, nil
}

// Handler is the root handler for the HTTP server.
func (a *API) Handler() http.Handler { return a.handler }

// Connect retries the database until ctx ends, then serves from it and starts
// the job sweep when the runtime API is enabled.
func (a *API) Connect(ctx context.Context) {
	db, err := connect(ctx, a.srv.cluster)
	if err != nil {
		slog.Error("database setup failed; snapshots stay unavailable", "error", err)
		return
	}
	a.srv.store.Store(&storeBox{db})
	if a.ra != nil {
		go sweepLoop(ctx, db, a.sweepEvery)
	}
}

// NewHTTPServer wraps h in the server settings below.
func NewHTTPServer(port string, h http.Handler) *http.Server { return newHTTPServer(port, h) }

// newHTTPServer bounds header and body reads, so a caller cannot hold a
// connection open by stalling an upload. ReadTimeout covers reading the
// request only; there is no WriteTimeout, which would cut the claim
// long-poll (claimWait).
func newHTTPServer(port string, h http.Handler) *http.Server {
	return &http.Server{Addr: ":" + port, Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second}
}

// connect migrates and seeds as the owner role, then opens the runtime pool
// (Nais READWRITE_ role when present). It retries until ctx ends.
func connect(ctx context.Context, cluster string) (*DB, error) {
	for delay := time.Second; ; delay = min(delay*2, 30*time.Second) {
		db, err := setupDB(ctx, cluster)
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

func setupDB(ctx context.Context, cluster string) (*DB, error) {
	owner, err := OpenDB(ctx, os.Getenv("DB_URL"))
	if err != nil {
		return nil, err
	}
	defer owner.Close()
	if err := owner.Migrate(ctx); err != nil {
		return nil, err
	}
	if err := seedFromEnv(ctx, owner, cluster); err != nil {
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
func seedFromEnv(ctx context.Context, db *DB, cluster string) error {
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
	servers, err := seedServers(cluster, routes, allowlist)
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
	// pilotTeams may write outside dev-*; see writeAllowed.
	pilotTeams []string
	store      atomic.Pointer[storeBox]
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
