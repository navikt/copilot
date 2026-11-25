package session

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

// PostgresStore persists gateway state in PostgreSQL.
type PostgresStore struct {
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
	// pgx defaults to max(4, NumCPU), which is the node's cores. 10 per pod keeps
	// 3 pods (2 replicas plus a rolling surge) at 30, well under max_connections 100 (#2893).
	poolCfg.MaxConns = 10
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

// NewPostgresStore opens the store. With an empty databaseURL (Nais Postgres) it
// migrates with the unprefixed admin PG* credentials, then runs on a pool using
// the READWRITE_PG* runtime credentials (falling back to PG*).
func NewPostgresStore(ctx context.Context, databaseURL string, now func() time.Time, caps map[string]int) (*PostgresStore, error) {
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
	s := &PostgresStore{pool: migPool, caps: copiedCaps, now: now}
	if err = s.migrate(ctx); err != nil {
		migPool.Close()
		return nil, fmt.Errorf("run postgres migrations: %w", err)
	}
	if databaseURL != "" || os.Getenv("READWRITE_PGHOST") == "" {
		return s, nil
	}
	migPool.Close()
	rtPool, err := openPGPool(ctx, PGEnvDSN("READWRITE_"))
	if err != nil {
		return nil, fmt.Errorf("runtime role: %w", err)
	}
	s.pool = rtPool
	// Fail at startup, not on first login, if the runtime role cannot use the tables.
	if err = CheckPGRuntimeTables(ctx, func(ctx context.Context, q string) error {
		_, execErr := rtPool.Exec(ctx, q)
		return execErr
	}); err != nil {
		rtPool.Close()
		return nil, err
	}
	return s, nil
}

var pgRuntimeTableChecks = []struct {
	table string
	query string
}{
	{"items", `SELECT 1 FROM items LIMIT 1`},
	{"revoked", `SELECT 1 FROM revoked LIMIT 1`},
	{"counters", `SELECT 1 FROM counters LIMIT 1`},
}

// CheckPGRuntimeTables verifies the runtime role can read all store tables.
func CheckPGRuntimeTables(ctx context.Context, exec func(context.Context, string) error) error {
	for _, c := range pgRuntimeTableChecks {
		if err := exec(ctx, c.query); err != nil {
			return fmt.Errorf("runtime role cannot read %s: %w", c.table, err)
		}
	}
	return nil
}

func (s *PostgresStore) migrate(ctx context.Context) error {
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

// Put stores a value with the given TTL, failing with ErrStoreFull if kind is at cap.
func (s *PostgresStore) Put(ctx context.Context, kind, key string, val []byte, ttl time.Duration) error {
	_, err := s.put(ctx, kind, key, val, ttl, false)
	return err
}

// PutEvict stores a value, evicting the earliest-expiring live item if kind is at cap.
func (s *PostgresStore) PutEvict(ctx context.Context, kind, key string, val []byte, ttl time.Duration) (bool, error) {
	return s.put(ctx, kind, key, val, ttl, true)
}

func (s *PostgresStore) put(ctx context.Context, kind, key string, val []byte, ttl time.Duration, evict bool) (evicted bool, err error) {
	now := s.now().UTC()
	expiresAt := now.Add(ttl)

	capLimit, hasCap := s.caps[kind]
	if !hasCap || capLimit <= 0 {
		_, err = s.pool.Exec(ctx, `
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
			return false, ErrStoreFull
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

// Get returns the live value for (kind, key), if any.
func (s *PostgresStore) Get(ctx context.Context, kind, key string) ([]byte, bool, error) {
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

// Pop atomically removes and returns the live value for (kind, key), if any.
func (s *PostgresStore) Pop(ctx context.Context, kind, key string) ([]byte, bool, error) {
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

// Delete removes (kind, key) if present.
func (s *PostgresStore) Delete(ctx context.Context, kind, key string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM items WHERE kind = $1 AND key = $2`, kind, key)
	return err
}

// CompareAndSwap atomically replaces oldVal with newVal for (kind, key).
func (s *PostgresStore) CompareAndSwap(ctx context.Context, kind, key string, oldVal, newVal []byte, ttl time.Duration) (bool, error) {
	now := s.now().UTC()
	expiresAt := now.Add(ttl)

	if oldVal == nil {
		if newVal == nil {
			var live bool
			err := s.pool.QueryRow(ctx, `
				SELECT EXISTS(SELECT 1 FROM items WHERE kind = $1 AND key = $2 AND expires_at > $3)`,
				kind, key, now,
			).Scan(&live)
			if err != nil {
				return false, err
			}
			return !live, nil
		}

		if capLimit, hasCap := s.caps[kind]; hasCap && capLimit > 0 {
			return s.compareAndSwapInsertCapped(ctx, kind, key, newVal, expiresAt, now, capLimit)
		}

		tag, err := s.pool.Exec(ctx, `
			INSERT INTO items (kind, key, value, expires_at, created_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (kind, key) DO UPDATE SET
				value = EXCLUDED.value,
				expires_at = EXCLUDED.expires_at,
				created_at = EXCLUDED.created_at
			WHERE items.expires_at <= $5`,
			kind, key, newVal, expiresAt, now,
		)
		if err != nil {
			return false, err
		}
		return tag.RowsAffected() == 1, nil
	}

	if newVal == nil {
		tag, err := s.pool.Exec(ctx, `
			DELETE FROM items
			WHERE kind = $1 AND key = $2 AND value = $3 AND expires_at > $4`,
			kind, key, oldVal, now,
		)
		if err != nil {
			return false, err
		}
		return tag.RowsAffected() == 1, nil
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE items
		SET value = $4, expires_at = $5, created_at = $6
		WHERE kind = $1 AND key = $2 AND value = $3 AND expires_at > $6`,
		kind, key, oldVal, newVal, expiresAt, now,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *PostgresStore) compareAndSwapInsertCapped(ctx context.Context, kind, key string, newVal []byte, expiresAt, now time.Time, capLimit int) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, execErr := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, kindAdvisoryLockID(kind)); execErr != nil {
		return false, execErr
	}

	var liveCount int
	var keyLive bool
	if scanErr := tx.QueryRow(ctx, `
		SELECT count(*), COALESCE(bool_or(key = $3), false)
		FROM items
		WHERE kind = $1 AND expires_at > $2`,
		kind, now, key,
	).Scan(&liveCount, &keyLive); scanErr != nil {
		return false, scanErr
	}
	if keyLive {
		return false, nil
	}
	if liveCount >= capLimit {
		return false, ErrStoreFull
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO items (kind, key, value, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (kind, key) DO UPDATE SET
			value = EXCLUDED.value,
			expires_at = EXCLUDED.expires_at,
			created_at = EXCLUDED.created_at
		WHERE items.expires_at <= $5`,
		kind, key, newVal, expiresAt, now,
	)
	if err != nil {
		return false, err
	}
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return false, commitErr
	}
	return tag.RowsAffected() == 1, nil
}

// Revoke marks jti as revoked until at least expiresAt.
func (s *PostgresStore) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO revoked (jti, expires_at)
		VALUES ($1, $2)
		ON CONFLICT (jti) DO UPDATE SET
			expires_at = GREATEST(revoked.expires_at, EXCLUDED.expires_at)`,
		jti, expiresAt.UTC(),
	)
	return err
}

// RevokeOnce revokes jti if it is not already live-revoked, reporting whether this call won.
func (s *PostgresStore) RevokeOnce(ctx context.Context, jti string, expiresAt time.Time) (bool, error) {
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

// IsRevoked reports whether jti has a live revocation entry.
func (s *PostgresStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
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

// Incr increments a fixed-window counter and returns the new count.
func (s *PostgresStore) Incr(ctx context.Context, key string, window time.Duration) (int, error) {
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

// CountPrefix counts live items of kind whose key starts with prefix.
func (s *PostgresStore) CountPrefix(ctx context.Context, kind, prefix string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM items
		WHERE kind = $1 AND starts_with(key, $2) AND expires_at > $3`,
		kind, prefix, s.now().UTC(),
	).Scan(&n)
	return n, err
}

// Sweep deletes expired items, revocations and counters, returning the total removed.
func (s *PostgresStore) Sweep(ctx context.Context) (int, error) {
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

// Ping checks database connectivity.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Close closes the underlying connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// Pool returns the active connection pool.
func (s *PostgresStore) Pool() *pgxpool.Pool {
	return s.pool
}

var pgEnvReplacer = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

// PGEnvDSN builds a libpq keyword DSN from prefix+PG* variables (no password: TLS client certs).
func PGEnvDSN(prefix string) string {
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
