package main

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pgMigrationLockID int64 = 0x666c656574677701

var pgMigrations = []string{
	`CREATE TABLE IF NOT EXISTS items (
		kind       text NOT NULL,
		key        text NOT NULL,
		value      bytea NOT NULL,
		expires_at timestamptz NOT NULL,
		created_at timestamptz NOT NULL,
		PRIMARY KEY (kind, key)
	);
	CREATE INDEX IF NOT EXISTS items_expires_at_idx ON items (expires_at);
	CREATE TABLE IF NOT EXISTS revoked (
		jti        text PRIMARY KEY,
		expires_at timestamptz NOT NULL
	);
	CREATE INDEX IF NOT EXISTS revoked_expires_at_idx ON revoked (expires_at);`,
	`CREATE TABLE IF NOT EXISTS counters (
		key        text PRIMARY KEY,
		n          bigint NOT NULL,
		expires_at timestamptz NOT NULL
	);
	CREATE INDEX IF NOT EXISTS counters_expires_at_idx ON counters (expires_at);`,
}

type pgxStore struct {
	pool *pgxpool.Pool
	caps map[string]int
	now  func() time.Time
}

func openPGPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	// PgBouncer in transaction mode: no server-side prepared statements, no session state.
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}

// newPostgresStore opens the store. With an empty databaseURL (Nais Postgres) it
// migrates with the unprefixed admin PG* credentials, then runs on a pool using
// the READWRITE_PG* runtime credentials (falling back to PG*).
func newPostgresStore(ctx context.Context, databaseURL string, now func() time.Time, caps map[string]int) (*pgxStore, error) {
	if now == nil {
		now = time.Now
	}
	copiedCaps := make(map[string]int, len(caps))
	for k, v := range caps {
		copiedCaps[k] = v
	}

	migPool, err := openPGPool(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	s := &pgxStore{pool: migPool, caps: copiedCaps, now: now}
	if err = s.migrate(ctx); err != nil {
		migPool.Close()
		return nil, fmt.Errorf("run postgres migrations: %w", err)
	}
	if databaseURL != "" || os.Getenv("READWRITE_PGHOST") == "" {
		return s, nil
	}
	migPool.Close()
	rtPool, err := openPGPool(ctx, pgEnvDSN("READWRITE_"))
	if err != nil {
		return nil, fmt.Errorf("runtime role: %w", err)
	}
	s.pool = rtPool
	// Fail at startup, not on first login, if the runtime role cannot use the tables.
	if _, err = rtPool.Exec(ctx, `SELECT 1 FROM items LIMIT 1`); err != nil {
		rtPool.Close()
		return nil, fmt.Errorf("runtime role cannot read items: %w", err)
	}
	if _, err = rtPool.Exec(ctx, `SELECT 1 FROM counters LIMIT 1`); err != nil {
		rtPool.Close()
		return nil, fmt.Errorf("runtime role cannot read counters: %w", err)
	}
	return s, nil
}

func (s *pgxStore) migrate(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, pgMigrationLockID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    integer PRIMARY KEY,
			applied_at timestamptz NOT NULL
		)`); err != nil {
		return err
	}

	appliedAt := s.now().UTC()
	for i, stmt := range pgMigrations {
		version := i + 1
		var applied bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migration %d: %w", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`, version, appliedAt); err != nil {
			return fmt.Errorf("record migration %d: %w", version, err)
		}
	}
	return tx.Commit(ctx)
}

func kindAdvisoryLockID(kind string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("fgw:kind:" + kind))
	// Mask to signed 63-bit positive space and avoid colliding with pgMigrationLockID.
	id := int64(h.Sum64() & 0x7fffffffffffffff)
	if id == pgMigrationLockID {
		id++
	}
	return id
}

func (s *pgxStore) Put(ctx context.Context, kind, key string, val []byte, ttl time.Duration) error {
	_, err := s.put(ctx, kind, key, val, ttl, false)
	return err
}

func (s *pgxStore) PutEvict(ctx context.Context, kind, key string, val []byte, ttl time.Duration) (bool, error) {
	return s.put(ctx, kind, key, val, ttl, true)
}

func (s *pgxStore) put(ctx context.Context, kind, key string, val []byte, ttl time.Duration, evict bool) (evicted bool, err error) {
	now := s.now().UTC()
	expiresAt := now.Add(ttl)

	capLimit, hasCap := s.caps[kind]
	if !hasCap || capLimit <= 0 {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO items (kind, key, value, expires_at, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (kind, key) DO UPDATE SET
				value = EXCLUDED.value,
				expires_at = EXCLUDED.expires_at,
				created_at = EXCLUDED.created_at`,
			kind, key, val, expiresAt, now,
		)
		return false, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, kindAdvisoryLockID(kind)); err != nil {
		return false, err
	}

	var liveCount int
	var keyLive bool
	if err := tx.QueryRow(ctx, `
		SELECT count(*), COALESCE(bool_or(key = $3), false)
		FROM items
		WHERE kind = $1 AND expires_at > $2`,
		kind, now, key,
	).Scan(&liveCount, &keyLive); err != nil {
		return false, err
	}
	if !keyLive && liveCount >= capLimit {
		if !evict || liveCount == 0 {
			return false, errStoreFull
		}
		// Only live rows count toward the cap, so only a live row frees a slot.
		tag, err := tx.Exec(ctx, `
			DELETE FROM items
			WHERE kind = $1 AND key = (
				SELECT key FROM items WHERE kind = $1 AND expires_at > $2
				ORDER BY expires_at LIMIT 1
			)`, kind, now)
		if err != nil {
			return false, err
		}
		evicted = tag.RowsAffected() == 1
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO items (kind, key, value, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (kind, key) DO UPDATE SET
			value = EXCLUDED.value,
			expires_at = EXCLUDED.expires_at,
			created_at = EXCLUDED.created_at`,
		kind, key, val, expiresAt, now,
	); err != nil {
		return false, err
	}
	return evicted, tx.Commit(ctx)
}

func (s *pgxStore) Get(ctx context.Context, kind, key string) ([]byte, bool, error) {
	var val []byte
	err := s.pool.QueryRow(ctx, `
		SELECT value
		FROM items
		WHERE kind = $1 AND key = $2 AND expires_at > $3`,
		kind, key, s.now().UTC(),
	).Scan(&val)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (s *pgxStore) Pop(ctx context.Context, kind, key string) ([]byte, bool, error) {
	var val []byte
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `
		DELETE FROM items
		WHERE kind = $1 AND key = $2
		RETURNING value, expires_at`,
		kind, key,
	).Scan(&val, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !s.now().UTC().Before(expiresAt) {
		return nil, false, nil
	}
	return val, true, nil
}

func (s *pgxStore) Delete(ctx context.Context, kind, key string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM items WHERE kind = $1 AND key = $2`, kind, key)
	return err
}

func (s *pgxStore) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO revoked (jti, expires_at)
		VALUES ($1, $2)
		ON CONFLICT (jti) DO UPDATE SET
			expires_at = GREATEST(revoked.expires_at, EXCLUDED.expires_at)`,
		jti, expiresAt.UTC(),
	)
	return err
}

func (s *pgxStore) RevokeOnce(ctx context.Context, jti string, expiresAt time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO revoked (jti, expires_at)
		VALUES ($1, $2)
		ON CONFLICT (jti) DO UPDATE SET
			expires_at = EXCLUDED.expires_at
		WHERE revoked.expires_at <= $3`,
		jti, expiresAt.UTC(), s.now().UTC(),
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *pgxStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	var one int
	err := s.pool.QueryRow(ctx, `
		SELECT 1
		FROM revoked
		WHERE jti = $1 AND expires_at > $2`,
		jti, s.now().UTC(),
	).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *pgxStore) Incr(ctx context.Context, key string, window time.Duration) (int, error) {
	now := s.now().UTC()
	var n int
	err := s.pool.QueryRow(ctx, `
		INSERT INTO counters (key, n, expires_at)
		VALUES ($1, 1, $2)
		ON CONFLICT (key) DO UPDATE SET
			n = CASE WHEN counters.expires_at <= $3 THEN 1 ELSE counters.n + 1 END,
			expires_at = CASE WHEN counters.expires_at <= $3 THEN EXCLUDED.expires_at ELSE counters.expires_at END
		RETURNING n`,
		key, now.Add(window), now,
	).Scan(&n)
	return n, err
}

func (s *pgxStore) Sweep(ctx context.Context) (int, error) {
	now := s.now().UTC()
	tag1, err := s.pool.Exec(ctx, `DELETE FROM items WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, err
	}
	tag2, err := s.pool.Exec(ctx, `DELETE FROM revoked WHERE expires_at <= $1`, now)
	if err != nil {
		return int(tag1.RowsAffected()), err
	}
	tag3, err := s.pool.Exec(ctx, `DELETE FROM counters WHERE expires_at <= $1`, now)
	if err != nil {
		return int(tag1.RowsAffected() + tag2.RowsAffected()), err
	}
	return int(tag1.RowsAffected() + tag2.RowsAffected() + tag3.RowsAffected()), nil
}

func (s *pgxStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *pgxStore) Close() {
	s.pool.Close()
}

var pgEnvReplacer = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

// pgEnvDSN builds a libpq keyword DSN from prefix+PG* variables (no password: TLS client certs).
func pgEnvDSN(prefix string) string {
	vars := []struct{ env, key string }{
		{"PGHOST", "host"}, {"PGPORT", "port"}, {"PGDATABASE", "dbname"}, {"PGUSER", "user"},
		{"PGSSLMODE", "sslmode"}, {"PGSSLCERT", "sslcert"}, {"PGSSLKEY", "sslkey"}, {"PGSSLROOTCERT", "sslrootcert"},
	}
	var parts []string
	for _, v := range vars {
		if val := os.Getenv(prefix + v.env); val != "" {
			parts = append(parts, fmt.Sprintf("%s='%s'", v.key, pgEnvReplacer.Replace(val)))
		}
	}
	return strings.Join(parts, " ")
}
