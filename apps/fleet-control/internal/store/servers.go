package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/navikt/copilot/fleet-control/internal/model"
)

// Store holds the server and snapshot queries on a pool.
type Store struct{ pool *pgxpool.Pool }

// New wraps pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// LockKey serialises migrations and seeding across replicas ("fleetc").
const LockKey int64 = 0x666c65657463

// Seed imports servers as published version 1 in one audited transaction. It
// only runs on an empty servers table, so a second run, or a run after the
// database has become the source of truth, changes nothing. It reports
// whether it imported.
func (d *Store) Seed(ctx context.Context, servers []model.Server, source string) (bool, error) {
	seeded := false
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", LockKey); err != nil {
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
		if err := InsertAudit(ctx, tx, AuditRow{Actor: "seed", Action: "seed_published", Detail: json.RawMessage(detail)}); err != nil {
			return err
		}
		seeded = true
		return nil
	})
	return seeded, err
}

// Servers returns every server at its published version, revoked ones included.
func (d *Store) Servers(ctx context.Context) ([]model.Server, error) {
	return QueryServers(ctx, d.pool)
}

// Querier is a pool or a transaction.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// QueryServers returns every server at its published version through q.
func QueryServers(ctx context.Context, q Querier) ([]model.Server, error) {
	rows, err := q.Query(ctx, `
		SELECT s.namespace, s.name, COALESCE(s.registry_name, ''), s.status,
		       v.registry::text, v.route_path, v.target, v.upstream_url, v.enabled, v.tools,
		       COALESCE(v.spec->>'dataCategory', ''), (v.spec->'toolRisk')::text, COALESCE(v.spec->>'defaultRisk', '')
		FROM servers s JOIN server_versions v
		  ON v.namespace = s.namespace AND v.name = s.name AND v.version = s.published_version
		ORDER BY s.namespace, s.name`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Server, error) {
		var s model.Server
		var registry, path, target, upstream *string
		var enabled bool
		var tools []string
		var toolRisk *string
		var defaultRisk string
		if err := row.Scan(&s.Namespace, &s.Name, &s.RegistryName, &s.Status, &registry, &path, &target, &upstream, &enabled, &tools, &s.DataClass, &toolRisk, &defaultRisk); err != nil {
			return s, err
		}
		if registry != nil {
			s.Registry = json.RawMessage(*registry)
		}
		if path != nil {
			s.Route = &model.Route{Path: *path, Target: *target, UpstreamURL: *upstream, Enabled: enabled, Tools: tools}
			if toolRisk != nil {
				if err := json.Unmarshal([]byte(*toolRisk), &s.Route.ToolRisk); err != nil {
					return s, fmt.Errorf("%s/%s: spec.toolRisk: %w", s.Namespace, s.Name, err)
				}
			}
			s.Route.ToolRisk = model.ResolveToolRisk(tools, s.Route.ToolRisk, defaultRisk)
		}
		return s, nil
	})
}

// LatestSnapshot returns the newest stored snapshot of kind, or nil.
func (d *Store) LatestSnapshot(ctx context.Context, kind string) (*model.Snapshot, error) {
	return latestSnapshot(ctx, d.pool, kind)
}

func latestSnapshot(ctx context.Context, q Querier, kind string) (*model.Snapshot, error) {
	s := model.Snapshot{Kind: kind}
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
func (d *Store) HasServers(ctx context.Context) (bool, error) {
	var exists bool
	err := d.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM servers)").Scan(&exists)
	return exists, err
}

// SnapshotLockKey is the advisory lock per snapshot kind: concurrent
// requests and replicas build one at a time, so a delayed request cannot
// store stale servers as a newer generation.
func SnapshotLockKey(kind string) int64 {
	if kind == "routes" {
		return LockKey + 1
	}
	return LockKey + 2
}

// Publish builds and stores the next snapshot of kind under its advisory lock.
func (d *Store) Publish(ctx context.Context, kind string, build func(string, []model.Server, *model.Snapshot) (*model.Snapshot, error)) (*model.Snapshot, error) {
	var out *model.Snapshot
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", SnapshotLockKey(kind)); err != nil {
			return err
		}
		servers, err := QueryServers(ctx, tx)
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
