package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	storeTimeout     = 2 * time.Second
	storeCleanupTick = time.Minute
	quotaKeepDays    = 2 // today and yesterday (UTC), at most 48 h
)

// errStore marks a store failure. create_order refuses and orders nothing.
var errStore = errors.New("order store is unavailable; nothing was ordered, try again later")

// orderStore holds the state create_order needs across requests: used
// confirmation tokens (single use) and the per-user quota.
type orderStore interface {
	// useToken marks a verified token as used. It returns errConfirm when the
	// token was already used, and errStore when the store cannot answer.
	useToken(ctx context.Context, userHash, sig string, exp, now time.Time) error
	// reserve charges antall test persons. ok is false when the quota is full.
	// release returns the reservation when the order failed and may be refunded.
	reserve(ctx context.Context, userHash string, antall int, now time.Time) (release func(), ok bool, err error)
	remaining(ctx context.Context, userHash string, now time.Time) (int, error)
	// sharedSingleUse reports whether used tokens are visible to every pod.
	sharedSingleUse() bool
}

// memStore is the per-pod store: rolling 24 h quota, in-memory used tokens.
type memStore struct {
	confirm *orderConfirm
	quota   *userQuota
}

func (m memStore) useToken(_ context.Context, userHash, sig string, exp, now time.Time) error {
	return m.confirm.markUsed(userHash, sig, exp, now)
}

func (m memStore) reserve(_ context.Context, userHash string, antall int, now time.Time) (func(), bool, error) {
	release, ok := m.quota.reserve(userHash, antall, now)
	return release, ok, nil
}

func (m memStore) remaining(_ context.Context, userHash string, now time.Time) (int, error) {
	return m.quota.remaining(userHash, now), nil
}

func (memStore) sharedSingleUse() bool { return false }

// pgStore shares state between replicas. The quota counts per UTC day.
type pgStore struct {
	pool *pgxpool.Pool

	mu       sync.Mutex
	migrated bool
}

// newPGStore connects lazily, so a database that is down at startup only
// blocks create_order, never the read tools.
func newPGStore(url string) (*pgStore, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	return &pgStore{pool: pool}, nil
}

const pgMigration = `
CREATE TABLE IF NOT EXISTS dolly_used_tokens (
	sig        text PRIMARY KEY,
	user_hash  text NOT NULL,
	expires_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS dolly_used_tokens_expires_at ON dolly_used_tokens (expires_at);
CREATE TABLE IF NOT EXISTS dolly_quota (
	user_hash text NOT NULL,
	day       date NOT NULL,
	used      integer NOT NULL CHECK (used >= 0),
	PRIMARY KEY (user_hash, day)
);`

// pgMigrationLock is the advisory lock key replicas take while migrating.
const pgMigrationLock = 0x646f6c6c79 // "dolly"

// migrate runs once per process; a failure is retried on the next call.
func (p *pgStore) migrate(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.migrated {
		return nil
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(pgMigrationLock)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, pgMigration); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	p.migrated = true
	return nil
}

func (p *pgStore) ready(ctx context.Context) (context.Context, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(ctx, storeTimeout)
	if err := p.migrate(ctx); err != nil {
		cancel()
		return nil, nil, p.fail("migrate", err)
	}
	return ctx, cancel, nil
}

func (p *pgStore) fail(op string, err error) error {
	slog.Error("dolly order store failed", "op", op, "error", err)
	return errStore
}

func (*pgStore) sharedSingleUse() bool { return true }

func utcDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (p *pgStore) useToken(ctx context.Context, userHash, sig string, exp, now time.Time) error {
	ctx, cancel, err := p.ready(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return p.fail("use token", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Per-user lock so concurrent replicas cannot both pass the cap.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('dolly-token:' || $1::text))`, userHash); err != nil {
		return p.fail("use token", err)
	}
	// Same cap as memory mode: reject when full, so a user or a looping agent
	// cannot mint unbounded state, and a used token is never evicted early.
	tag, err := tx.Exec(ctx, `
INSERT INTO dolly_used_tokens (sig, user_hash, expires_at)
SELECT $1::text, $2::text, $3::timestamptz
WHERE (SELECT count(*) FROM dolly_used_tokens WHERE user_hash = $2::text AND expires_at > $4::timestamptz) < $5::integer
ON CONFLICT DO NOTHING`,
		sig, userHash, exp, now, maxSeenPerUser)
	if err != nil {
		return p.fail("use token", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return p.fail("use token", err)
	}
	if tag.RowsAffected() == 0 {
		return errConfirm
	}
	return nil
}

func (p *pgStore) reserve(ctx context.Context, userHash string, antall int, now time.Time) (func(), bool, error) {
	ctx, cancel, err := p.ready(ctx)
	if err != nil {
		return nil, false, err
	}
	defer cancel()
	day := utcDay(now)
	// One statement, so concurrent replicas cannot both pass the limit.
	tag, err := p.pool.Exec(ctx, `
INSERT INTO dolly_quota (user_hash, day, used) SELECT $1::text, $2::date, $3::integer WHERE $3::integer <= $4::integer
ON CONFLICT (user_hash, day) DO UPDATE SET used = dolly_quota.used + EXCLUDED.used
WHERE dolly_quota.used + EXCLUDED.used <= $4::integer`,
		userHash, day, antall, dailyPersonQuota)
	if err != nil {
		return nil, false, p.fail("reserve quota", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, false, nil
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
		defer cancel()
		if _, err := p.pool.Exec(ctx,
			`UPDATE dolly_quota SET used = GREATEST(used - $3, 0) WHERE user_hash = $1 AND day = $2`,
			userHash, day, antall); err != nil {
			// Failing here keeps the charge: the user loses quota, never gains it.
			slog.Error("dolly order store failed", "op", "release quota", "error", err)
		}
	}, true, nil
}

func (p *pgStore) remaining(ctx context.Context, userHash string, now time.Time) (int, error) {
	ctx, cancel, err := p.ready(ctx)
	if err != nil {
		return 0, err
	}
	defer cancel()
	var used int
	err = p.pool.QueryRow(ctx, `SELECT COALESCE(SUM(used), 0) FROM dolly_quota WHERE user_hash = $1 AND day = $2`,
		userHash, utcDay(now)).Scan(&used)
	if err != nil {
		return 0, p.fail("read quota", err)
	}
	return max(dailyPersonQuota-used, 0), nil
}

// cleanup deletes expired tokens (they expire 5 min after issue) and quota
// rows older than yesterday (UTC).
func (p *pgStore) cleanup(ctx context.Context, now time.Time) error {
	ctx, cancel, err := p.ready(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	if _, err := p.pool.Exec(ctx, `DELETE FROM dolly_used_tokens WHERE expires_at < $1`, now); err != nil {
		return p.fail("cleanup tokens", err)
	}
	if _, err := p.pool.Exec(ctx, `DELETE FROM dolly_quota WHERE day < $1`, utcDay(now).AddDate(0, 0, 1-quotaKeepDays)); err != nil {
		return p.fail("cleanup quota", err)
	}
	return nil
}

func (p *pgStore) runCleanup(ctx context.Context) {
	t := time.NewTicker(storeCleanupTick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			_ = p.cleanup(ctx, now)
		}
	}
}

// newOrderStoreFromEnv picks the store. Default is memory, today's behaviour.
func newOrderStoreFromEnv(kind, url string, confirm *orderConfirm, quota *userQuota) (orderStore, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "memory":
		return memStore{confirm: confirm, quota: quota}, nil
	case "postgres":
		if strings.TrimSpace(url) == "" {
			return nil, errors.New("MCP_DOLLY_STORE=postgres needs MCP_DOLLY_DATABASE_URL")
		}
		return newPGStore(url)
	default:
		return nil, fmt.Errorf("invalid MCP_DOLLY_STORE %q: use memory or postgres", kind)
	}
}
