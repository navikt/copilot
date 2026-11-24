package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// lockKey serialises migrations and seeding across replicas ("fleetc").
const lockKey int64 = 0x666c65657463

// DB is the Postgres store.
type DB struct{ pool *pgxpool.Pool }

// OpenDB connects to dsn, or to the libpq PG* variables Nais sets when dsn is empty.
func OpenDB(ctx context.Context, dsn string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	// Simple protocol avoids prepared statements incompatible with PgBouncer transaction mode.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	cfg.ConnConfig.Tracer = pendingAudit
	cfg.BeforeClose = pendingAudit.forget
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &DB{pool: pool}, nil
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

// Migrate applies the embedded migrations once each, under an advisory lock.
func (d *DB) Migrate(ctx context.Context) error { return d.migrateTo(ctx, "") }

// migrateTo applies migrations up to and including last ("" for all), so a
// test can stand where an older image left the schema.
func (d *DB) migrateTo(ctx context.Context, last string) error {
	if _, err := d.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		if last != "" && name > last {
			break
		}
		err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
				return err
			}
			var done bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", name).Scan(&done); err != nil || done {
				return err
			}
			sql, err := migrationFS.ReadFile("migrations/" + name)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, string(sql)); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name)
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

// Seed imports servers as published version 1 in one audited transaction. It
// only runs on an empty servers table, so a second run, or a run after the
// database has become the source of truth, changes nothing. It reports
// whether it imported.
func (d *DB) Seed(ctx context.Context, servers []Server, source string) (bool, error) {
	seeded := false
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM servers)").Scan(&exists); err != nil || exists {
			return err
		}
		for _, s := range servers {
			var regName *string
			if s.RegistryName != "" {
				regName = &s.RegistryName
			}
			if _, err := tx.Exec(ctx, `INSERT INTO servers (namespace, name, registry_name, published_version, status) VALUES ($1, $2, $3, 1, 'published')`, s.Namespace, s.Name, regName); err != nil {
				return fmt.Errorf("insert %s: %w", s.Key(), err)
			}
			var registry *string
			if s.Registry != nil {
				r := string(s.Registry)
				registry = &r
			}
			var path, target, upstream *string
			var tools []string
			enabled := true
			if r := s.Route; r != nil {
				path, target, upstream, tools, enabled = &r.Path, &r.Target, &r.UpstreamURL, r.Tools, r.Enabled
			}
			if _, err := tx.Exec(ctx, `INSERT INTO server_versions (namespace, name, version, registry, route_path, target, upstream_url, enabled, tools, submitted_by, published_at)
				VALUES ($1, $2, 1, $3::jsonb, $4, $5, $6, $7, $8, 'seed', now())`, s.Namespace, s.Name, registry, path, target, upstream, enabled, tools); err != nil {
				return fmt.Errorf("insert version %s: %w", s.Key(), err)
			}
		}
		detail, err := json.Marshal(map[string]any{"source": source, "servers": len(servers)})
		if err != nil {
			return err
		}
		if err := insertAudit(ctx, tx, auditRow{Actor: "seed", Action: "seed_published", Detail: json.RawMessage(detail)}); err != nil {
			return err
		}
		seeded = true
		return nil
	})
	return seeded, err
}

// Servers returns every server at its published version, revoked ones included.
func (d *DB) Servers(ctx context.Context) ([]Server, error) { return queryServers(ctx, d.pool) }

// querier is a pool or a transaction.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func queryServers(ctx context.Context, q querier) ([]Server, error) {
	rows, err := q.Query(ctx, `
		SELECT s.namespace, s.name, COALESCE(s.registry_name, ''), s.status,
		       v.registry::text, v.route_path, v.target, v.upstream_url, v.enabled, v.tools
		FROM servers s JOIN server_versions v
		  ON v.namespace = s.namespace AND v.name = s.name AND v.version = s.published_version
		ORDER BY s.namespace, s.name`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Server, error) {
		var s Server
		var registry, path, target, upstream *string
		var enabled bool
		var tools []string
		if err := row.Scan(&s.Namespace, &s.Name, &s.RegistryName, &s.Status, &registry, &path, &target, &upstream, &enabled, &tools); err != nil {
			return s, err
		}
		if registry != nil {
			s.Registry = json.RawMessage(*registry)
		}
		if path != nil {
			s.Route = &Route{Path: *path, Target: *target, UpstreamURL: *upstream, Enabled: enabled, Tools: tools}
		}
		return s, nil
	})
}

// LatestSnapshot returns the newest stored snapshot of kind, or nil.
func (d *DB) LatestSnapshot(ctx context.Context, kind string) (*Snapshot, error) {
	return latestSnapshot(ctx, d.pool, kind)
}

func latestSnapshot(ctx context.Context, q querier, kind string) (*Snapshot, error) {
	s := Snapshot{Kind: kind}
	var body string
	err := q.QueryRow(ctx, `SELECT generation, etag, body::text FROM snapshots WHERE kind = $1 ORDER BY generation DESC LIMIT 1`, kind).Scan(&s.Generation, &s.ETag, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.Items = json.RawMessage(body)
	return &s, nil
}

// HasServers reports whether any server exists.
func (d *DB) HasServers(ctx context.Context) (bool, error) {
	var exists bool
	err := d.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM servers)").Scan(&exists)
	return exists, err
}

// snapshotLockKey is the advisory lock per snapshot kind: concurrent
// requests and replicas build one at a time, so a delayed request cannot
// store stale servers as a newer generation.
func snapshotLockKey(kind string) int64 {
	if kind == "routes" {
		return lockKey + 1
	}
	return lockKey + 2
}

// Publish implements store.
func (d *DB) Publish(ctx context.Context, kind string, build func(string, []Server, *Snapshot) (*Snapshot, error)) (*Snapshot, error) {
	var out *Snapshot
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", snapshotLockKey(kind)); err != nil {
			return err
		}
		servers, err := queryServers(ctx, tx)
		if err != nil {
			return err
		}
		prev, err := latestSnapshot(ctx, tx, kind)
		if err != nil {
			return err
		}
		next, err := build(kind, servers, prev)
		if err != nil {
			return err
		}
		if next != prev {
			if _, err := tx.Exec(ctx, `INSERT INTO snapshots (kind, generation, etag, body) VALUES ($1, $2, $3, $4::jsonb)`, next.Kind, next.Generation, next.ETag, string(next.Items)); err != nil {
				return err
			}
		}
		out = next
		return nil
	})
	return out, err
}

// RevokeAuditChange re-runs 006 so a later re-grant to the runtime role is
// removed before CheckAuditLocked runs. Needs the owner pool.
func (d *DB) RevokeAuditChange(ctx context.Context) error {
	sql, err := migrationFS.ReadFile("migrations/006_audit_revoke.sql")
	if err != nil {
		return err
	}
	if _, err := d.pool.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("revoke audit_events rights: %w", err)
	}
	return nil
}

// CheckAuditLocked fails unless the connected role is not the owner of
// audit_events and holds none of the rights to change it (#2631).
func (d *DB) CheckAuditLocked(ctx context.Context) error { return checkAuditLocked(ctx, d.pool) }

func checkAuditLocked(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) error {
	var owner, canChange bool
	err := q.QueryRow(ctx, `SELECT pg_has_role(current_user, c.relowner, 'USAGE'),
		has_table_privilege(c.oid, 'UPDATE') OR has_table_privilege(c.oid, 'DELETE')
		OR has_table_privilege(c.oid, 'TRUNCATE') OR has_table_privilege(c.oid, 'TRIGGER')
		OR has_any_column_privilege(c.oid, 'UPDATE')
		OR has_sequence_privilege('audit_events_id_seq', 'UPDATE')
		FROM pg_class c WHERE c.oid = 'audit_events'::regclass`).Scan(&owner, &canChange)
	if err != nil {
		return fmt.Errorf("check audit_events rights: %w", err)
	}
	if owner || canChange {
		return fmt.Errorf("runtime role can change audit_events (owner=%t, change rights=%t); refusing to start", owner, canChange)
	}
	return nil
}

// PruneAudit deletes audit rows older than 90 days (#2762), except the
// newest per resource and action. Needs the owner
// pool: the runtime role has no DELETE right, and the trigger refuses
// younger rows and sessions without fleet.audit_retention.
// shortcut: deletes without export; the export target is not decided (#2762).
func (d *DB) PruneAudit(ctx context.Context) (int64, error) {
	var n int64
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL fleet.audit_retention = 'on'`); err != nil {
			return err
		}
		// The newest event per resource and action stays: status reasons and
		// transition times (revoked by platform, rolled back) read it.
		tag, err := tx.Exec(ctx, `DELETE FROM audit_events WHERE at < now() - interval '90 days'
			AND id NOT IN (SELECT max(id) FROM audit_events GROUP BY namespace, name, resource_kind, action)`)
		n = tag.RowsAffected()
		return err
	})
	return n, err
}
