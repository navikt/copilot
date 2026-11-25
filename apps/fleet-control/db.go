package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/multitracer"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/navikt/copilot/fleet-control/internal/metrics"
	"github.com/navikt/copilot/fleet-control/internal/store"
)

// DB is the Postgres store. Server and snapshot queries live in store.Store.
type DB struct {
	*store.Store
	pool *pgxpool.Pool
}

// OpenDB connects to dsn, or to the libpq PG* variables Nais sets when dsn is empty.
func OpenDB(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	// Simple protocol avoids prepared statements incompatible with PgBouncer transaction mode.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	// pgx defaults to max(4, NumCPU), which is the node's cores. 10 per pod keeps
	// 3 pods (2 replicas plus a rolling surge) at 30, well under max_connections 100 (#2893).
	cfg.MaxConns = 10
	cfg.ConnConfig.Tracer = multitracer.New(store.PendingAudit, metrics.DBTracer{})
	cfg.BeforeClose = store.PendingAudit.Forget
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &DB{Store: store.New(pool), pool: pool}, nil
}

var pgEnvReplacer = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

// pgEnvDSN builds a DSN from prefixed PG* variables, e.g. Nais' READWRITE_PGHOST.
func pgEnvDSN(prefix string) string {
	var parts []string
	for _, kv := range [][2]string{{"PGHOST", "host"}, {"PGPORT", "port"}, {"PGDATABASE", "dbname"}, {"PGUSER", "user"}, {"PGPASSWORD", "password"}, {"PGSSLMODE", "sslmode"}, {"PGSSLCERT", "sslcert"}, {"PGSSLKEY", "sslkey"}, {"PGSSLROOTCERT", "sslrootcert"}} {
		if v := os.Getenv(prefix + kv[0]); v != "" {
			parts = append(parts, fmt.Sprintf("%s='%s'", kv[1], pgEnvReplacer.Replace(v)))
		}
	}
	return strings.Join(parts, " ")
}

// Close closes the pool.
func (d *DB) Close() { d.pool.Close() }

// Ping checks the connection.
func (d *DB) Ping(ctx context.Context) error { return d.pool.Ping(ctx) }

// entraIssuer is the Entra issuer (AZURE_OPENID_CONFIG_ISSUER) stored with
// Entra identities. Empty: writes that need it are refused (#2896).
var entraIssuer string
