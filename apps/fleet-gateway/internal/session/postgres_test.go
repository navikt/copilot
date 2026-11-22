package session

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// pgTestURL returns a DSN scoped to a fresh schema. CI sets FGW_TEST_PG_URL
// (see .github/workflows/fleet-gateway.yaml); locally the tests skip without it.
func pgTestURL(t *testing.T) (string, *pgxpool.Pool) {
	t.Helper()
	raw := os.Getenv("FGW_TEST_PG_URL")
	if raw == "" {
		// The Postgres CI job sets FGW_REQUIRE_PG, so losing the URL there fails instead of passing green.
		if os.Getenv("FGW_REQUIRE_PG") == "1" {
			t.Fatal("FGW_TEST_PG_URL is empty but FGW_REQUIRE_PG=1")
		}
		t.Skip("FGW_TEST_PG_URL not set")
	}
	ctx := context.Background()
	var b [6]byte
	_, _ = rand.Read(b[:])
	schema := "fgw_session_" + hex.EncodeToString(b[:])
	admin, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatalf("admin pool: %v", err)
	}
	if _, execErr := admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); execErr != nil {
		admin.Close()
		t.Fatalf("create schema: %v", execErr)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
		admin.Close()
	})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse FGW_TEST_PG_URL: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	scoped, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("scoped pool: %v", err)
	}
	t.Cleanup(scoped.Close)
	return u.String(), scoped
}

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) Now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *testClock) Add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

func newPGStore(t *testing.T, dsn string, clk *testClock, caps map[string]int) *PostgresStore {
	t.Helper()
	s, err := NewPostgresStore(context.Background(), dsn, clk.Now, caps)
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestPostgresStore_RevokeNeverShortensDenylist(t *testing.T) {
	dsn, _ := pgTestURL(t)
	clk := &testClock{t: time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)}
	s := newPGStore(t, dsn, clk, nil)
	ctx := context.Background()

	if err := s.Revoke(ctx, "fam:f1", clk.Now().Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	// A later, shorter revoke (e.g. from a second logout) must not lift the family ban early.
	if err := s.Revoke(ctx, "fam:f1", clk.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	clk.Add(time.Hour)
	if revoked, err := s.IsRevoked(ctx, "fam:f1"); err != nil || !revoked {
		t.Fatalf("IsRevoked after shorter re-revoke = %v, %v; want true", revoked, err)
	}
	clk.Add(2 * time.Hour)
	if revoked, err := s.IsRevoked(ctx, "fam:f1"); err != nil || revoked {
		t.Fatalf("IsRevoked after expiry = %v, %v; want false", revoked, err)
	}
	if revoked, err := s.IsRevoked(ctx, "never-seen"); err != nil || revoked {
		t.Fatalf("IsRevoked unknown = %v, %v; want false", revoked, err)
	}
}

func TestPostgresStore_CompareAndSwapRejectsStaleAndExpired(t *testing.T) {
	dsn, _ := pgTestURL(t)
	clk := &testClock{t: time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)}
	s := newPGStore(t, dsn, clk, nil)
	ctx := context.Background()

	if ok, err := s.CompareAndSwap(ctx, KindSession, "k", nil, []byte("v1"), time.Minute); err != nil || !ok {
		t.Fatalf("insert = %v, %v", ok, err)
	}
	if ok, err := s.CompareAndSwap(ctx, KindSession, "k", nil, []byte("other"), time.Minute); err != nil || ok {
		t.Fatalf("insert over live row = %v, %v; want false", ok, err)
	}
	if ok, err := s.CompareAndSwap(ctx, KindSession, "k", []byte("wrong"), []byte("v2"), time.Minute); err != nil || ok {
		t.Fatalf("swap with stale old = %v, %v; want false", ok, err)
	}
	if ok, err := s.CompareAndSwap(ctx, KindSession, "k", []byte("v1"), []byte("v2"), time.Minute); err != nil || !ok {
		t.Fatalf("swap = %v, %v; want true", ok, err)
	}

	clk.Add(2 * time.Minute) // row expired but not swept
	if ok, err := s.CompareAndSwap(ctx, KindSession, "k", []byte("v2"), []byte("v3"), time.Minute); err != nil || ok {
		t.Fatalf("swap on expired row = %v, %v; want false", ok, err)
	}
	if ok, err := s.CompareAndSwap(ctx, KindSession, "k", []byte("v2"), nil, time.Minute); err != nil || ok {
		t.Fatalf("delete on expired row = %v, %v; want false", ok, err)
	}
	if ok, err := s.CompareAndSwap(ctx, KindSession, "k", nil, []byte("fresh"), time.Minute); err != nil || !ok {
		t.Fatalf("insert over expired row = %v, %v; want true", ok, err)
	}
	if v, ok, err := s.Get(ctx, KindSession, "k"); err != nil || !ok || !bytes.Equal(v, []byte("fresh")) {
		t.Fatalf("Get = %q, %v, %v", v, ok, err)
	}
}

func TestPostgresStore_OutageFailsClosed(t *testing.T) {
	dsn, _ := pgTestURL(t)
	clk := &testClock{t: time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)}
	s, err := NewPostgresStore(context.Background(), dsn, clk.Now, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.Revoke(ctx, "jti-1", clk.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	s.Close() // database gone

	if revoked, err := s.IsRevoked(ctx, "jti-1"); err == nil {
		t.Fatalf("IsRevoked during outage = %v, nil; want error", revoked)
	}
	if _, ok, err := s.Get(ctx, KindSession, "k"); err == nil || ok {
		t.Fatalf("Get during outage = %v, %v; want error", ok, err)
	}
	if ok, err := s.CompareAndSwap(ctx, KindSession, "k", nil, []byte("v"), time.Minute); err == nil || ok {
		t.Fatalf("CompareAndSwap during outage = %v, %v; want error", ok, err)
	}
	if ok, err := s.RevokeOnce(ctx, "jti-2", clk.Now().Add(time.Hour)); err == nil || ok {
		t.Fatalf("RevokeOnce during outage = %v, %v; want error", ok, err)
	}
	if err := s.Revoke(ctx, "jti-3", clk.Now().Add(time.Hour)); err == nil {
		t.Fatal("Revoke during outage = nil; want error")
	}

	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := NewPostgresStore(tctx, "postgres://nobody:x@127.0.0.1:1/none?sslmode=disable", clk.Now, nil); err == nil {
		t.Fatal("NewPostgresStore with unreachable DSN = nil error")
	}
}

// TestPostgresStore_ReadsRowsFromOlderVersion seeds a schema as a pod with only
// migration 1 left it, then opens the current store over it.
func TestPostgresStore_ReadsRowsFromOlderVersion(t *testing.T) {
	dsn, pool := pgTestURL(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	seed := `CREATE TABLE schema_migrations (version integer PRIMARY KEY, applied_at timestamptz NOT NULL);
		INSERT INTO schema_migrations VALUES (1, now());` + pgMigrations[0]
	if _, err := pool.Exec(ctx, seed); err != nil {
		t.Fatalf("seed v1 schema: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO items (kind, key, value, expires_at, created_at) VALUES
		($1, 'legacy', '\x6f6c64', $3, $2),
		($4, 'gone', '\x6f6c64', $2, $2)`,
		KindPendingEntraLegacy, now, now.Add(time.Hour), KindSession); err != nil {
		t.Fatalf("seed items: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO revoked (jti, expires_at) VALUES ('old-jti', $1)`, now.Add(time.Hour)); err != nil {
		t.Fatalf("seed revoked: %v", err)
	}

	clk := &testClock{t: now}
	s := newPGStore(t, dsn, clk, nil)
	newPGStore(t, dsn, clk, nil) // second pod: migrations are idempotent

	if revoked, err := s.IsRevoked(ctx, "old-jti"); err != nil || !revoked {
		t.Fatalf("old revocation = %v, %v; want true", revoked, err)
	}
	if v, ok, err := s.Get(ctx, KindPendingEntraLegacy, "legacy"); err != nil || !ok || string(v) != "old" {
		t.Fatalf("legacy item = %q, %v, %v", v, ok, err)
	}
	if _, ok, err := s.Pop(ctx, KindSession, "gone"); err != nil || ok {
		t.Fatalf("expired old item popped = %v, %v; want not found", ok, err)
	}
	if n, err := s.Incr(ctx, "rl:x", time.Minute); err != nil || n != 1 {
		t.Fatalf("Incr after migration 2 = %d, %v", n, err)
	}
}
