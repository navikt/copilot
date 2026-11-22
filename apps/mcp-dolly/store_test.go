package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// pgTestStore returns a store on an empty schema. It skips without
// DOLLY_TEST_PG_URL, unless DOLLY_REQUIRE_PG=1 (set in the CI db-test job).
func pgTestStore(t *testing.T) *pgStore {
	t.Helper()
	url := os.Getenv("DOLLY_TEST_PG_URL")
	if url == "" {
		if os.Getenv("DOLLY_REQUIRE_PG") == "1" {
			t.Fatal("DOLLY_REQUIRE_PG=1 but DOLLY_TEST_PG_URL is not set")
		}
		t.Skip("DOLLY_TEST_PG_URL not set")
	}
	p, err := newPGStore(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.pool.Close)
	if _, err := p.pool.Exec(context.Background(), `DROP TABLE IF EXISTS dolly_used_tokens, dolly_quota`); err != nil {
		t.Fatal(err)
	}
	return p
}

var testNow = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

func TestPGStoreTokenSingleUse(t *testing.T) {
	p := pgTestStore(t)
	ctx := context.Background()
	exp := testNow.Add(confirmTTL)
	if err := p.useToken(ctx, "u", "sig-1", exp, testNow); err != nil {
		t.Fatalf("first use: %v", err)
	}
	if err := p.useToken(ctx, "u", "sig-1", exp, testNow); !errors.Is(err, errConfirm) {
		t.Fatalf("second use = %v, want errConfirm", err)
	}

	// Concurrent replicas: exactly one wins.
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if p.useToken(ctx, "u", "sig-2", exp, testNow) == nil {
				wins.Add(1)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("concurrent uses won %d times, want 1", wins.Load())
	}
}

func TestPGStoreQuotaLimit(t *testing.T) {
	p := pgTestStore(t)
	ctx := context.Background()

	var ok atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if _, granted, err := p.reserve(ctx, "u", 5, testNow); err == nil && granted {
				ok.Add(1)
			}
		})
	}
	wg.Wait()
	if ok.Load() != dailyPersonQuota/5 {
		t.Fatalf("granted %d reservations of 5, want %d", ok.Load(), dailyPersonQuota/5)
	}
	if left, err := p.remaining(ctx, "u", testNow); err != nil || left != 0 {
		t.Fatalf("remaining = %d, %v; want 0", left, err)
	}
	if _, granted, _ := p.reserve(ctx, "u", 1, testNow); granted {
		t.Fatal("reserve over the limit granted")
	}

	// Another user and the next UTC day are independent.
	if _, granted, _ := p.reserve(ctx, "other", 5, testNow); !granted {
		t.Fatal("other user refused")
	}
	tomorrow := utcDay(testNow).AddDate(0, 0, 1)
	if _, granted, _ := p.reserve(ctx, "u", 5, tomorrow); !granted {
		t.Fatal("next UTC day refused")
	}

	// A refund returns the reservation.
	release, granted, err := p.reserve(ctx, "u", 5, tomorrow)
	if err != nil || !granted {
		t.Fatalf("reserve: %v %v", granted, err)
	}
	release()
	if left, _ := p.remaining(ctx, "u", tomorrow); left != dailyPersonQuota-5 {
		t.Fatalf("remaining after refund = %d, want %d", left, dailyPersonQuota-5)
	}
}

func TestPGStoreCleanup(t *testing.T) {
	p := pgTestStore(t)
	ctx := context.Background()
	if err := p.useToken(ctx, "u", "old", testNow.Add(-time.Second), testNow); err != nil {
		t.Fatal(err)
	}
	if err := p.useToken(ctx, "u", "live", testNow.Add(confirmTTL), testNow); err != nil {
		t.Fatal(err)
	}
	for _, d := range []int{0, -1, -2} {
		if _, _, err := p.reserve(ctx, "u", 1, testNow.AddDate(0, 0, d)); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.cleanup(ctx, testNow); err != nil {
		t.Fatal(err)
	}
	var tokens, days int
	_ = p.pool.QueryRow(ctx, `SELECT count(*) FROM dolly_used_tokens`).Scan(&tokens)
	_ = p.pool.QueryRow(ctx, `SELECT count(*) FROM dolly_quota`).Scan(&days)
	if tokens != 1 || days != 2 {
		t.Fatalf("after cleanup: %d tokens, %d quota days; want 1 and 2", tokens, days)
	}
	// The live token is still spent.
	if err := p.useToken(ctx, "u", "live", testNow.Add(confirmTTL), testNow); !errors.Is(err, errConfirm) {
		t.Fatalf("live token reusable after cleanup: %v", err)
	}
}

// A database that cannot be reached fails closed. Needs no Postgres.
func TestPGStoreDownFailsClosed(t *testing.T) {
	p, err := newPGStore("postgres://u:p@127.0.0.1:1/x?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.pool.Close)
	ctx := context.Background()
	if err := p.useToken(ctx, "u", "s", testNow.Add(time.Minute), testNow); !errors.Is(err, errStore) {
		t.Errorf("useToken = %v, want errStore", err)
	}
	if _, granted, err := p.reserve(ctx, "u", 1, testNow); granted || !errors.Is(err, errStore) {
		t.Errorf("reserve = %v, %v; want refused with errStore", granted, err)
	}
	if _, err := p.remaining(ctx, "u", testNow); !errors.Is(err, errStore) {
		t.Errorf("remaining = %v, want errStore", err)
	}
}

// failingStore spends tokens but cannot reserve quota.
type failingStore struct{ memStore }

func (failingStore) reserve(context.Context, string, int, time.Time) (func(), bool, error) {
	return nil, false, errStore
}

func TestCreateOrderStoreDownOrdersNothing(t *testing.T) {
	call := func(srv *server, args map[string]any) map[string]any {
		_, out := callToolModern(t, srv.routes(), "in", "create_order", args)
		return out
	}
	srv, posts, _ := twoStepServer(t)
	tok := previewToken(t, call(srv, twoStepArgs))
	if tok == "" {
		t.Fatal("no preview token")
	}

	srv.store = failingStore{memStore{confirm: srv.confirm, quota: srv.quota}}
	args := map[string]any{"confirmationToken": tok}
	for k, v := range twoStepArgs {
		args[k] = v
	}
	txt, isErr := extractToolText(t, call(srv, args))
	if !isErr || !strings.Contains(txt, "unavailable") || posts() != 0 {
		t.Fatalf("store down: isErr=%v %q posts=%d", isErr, txt, posts())
	}

	srv.store, _ = newOrderStoreFromEnv("postgres", "postgres://u:p@127.0.0.1:1/x?sslmode=disable&connect_timeout=1", srv.confirm, srv.quota)
	t.Cleanup(srv.store.(*pgStore).pool.Close)
	if txt, isErr := extractToolText(t, call(srv, twoStepArgs)); !isErr || !strings.Contains(txt, "unavailable") {
		t.Fatalf("preview with database down: isErr=%v %q", isErr, txt)
	}
}

func TestNewOrderStoreFromEnv(t *testing.T) {
	if s, err := newOrderStoreFromEnv("", "", nil, nil); err != nil {
		t.Fatal(err)
	} else if _, ok := s.(memStore); !ok {
		t.Fatalf("default store = %T, want memStore", s)
	}
	if _, err := newOrderStoreFromEnv("postgres", "", nil, nil); err == nil {
		t.Fatal("postgres without URL accepted")
	}
	if _, err := newOrderStoreFromEnv("redis", "", nil, nil); err == nil {
		t.Fatal("unknown store accepted")
	}
}
