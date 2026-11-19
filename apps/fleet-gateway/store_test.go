package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type storeFactory func(t *testing.T, now func() time.Time, caps map[string]int) SessionStore

func testKeyBase64(t *testing.T, rawURL bool) string {
	t.Helper()
	buf := make([]byte, sealKeyBytes)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	if rawURL {
		return base64.RawURLEncoding.EncodeToString(buf)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func runStoreContract(t *testing.T, factory storeFactory) {
	t.Helper()

	t.Run("put get pop single use and concurrent pop", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
		store := factory(t, clk.Now, map[string]int{kindCode: 10, kindSession: 10})
		defer store.Close()

		ctx := context.Background()
		want := []byte(`{"hello":"world"}`)
		if err := store.Put(ctx, kindCode, "code-1", want, time.Minute); err != nil {
			t.Fatalf("Put failed: %v", err)
		}

		got, ok, err := store.Get(ctx, kindCode, "code-1")
		if err != nil || !ok {
			t.Fatalf("Get failed: ok=%v err=%v", ok, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("Get mismatch: got %q, want %q", got, want)
		}

		popped, ok, err := store.Pop(ctx, kindCode, "code-1")
		if err != nil || !ok {
			t.Fatalf("Pop failed: ok=%v err=%v", ok, err)
		}
		if !bytes.Equal(popped, want) {
			t.Fatalf("Pop mismatch: got %q, want %q", popped, want)
		}

		if _, ok, err := store.Pop(ctx, kindCode, "code-1"); err != nil || ok {
			t.Fatalf("second Pop must miss: ok=%v err=%v", ok, err)
		}
		if _, ok, err := store.Get(ctx, kindCode, "code-1"); err != nil || ok {
			t.Fatalf("Get after Pop must miss: ok=%v err=%v", ok, err)
		}

		// Two concurrent Pops on the same key: exactly one wins.
		if err := store.Put(ctx, kindCode, "race-key", []byte("race-val"), time.Minute); err != nil {
			t.Fatalf("Put race-key failed: %v", err)
		}
		var wins atomic.Int32
		var startWG, doneWG sync.WaitGroup
		startWG.Add(1)
		for i := 0; i < 2; i++ {
			doneWG.Add(1)
			go func() {
				defer doneWG.Done()
				startWG.Wait()
				val, ok, popErr := store.Pop(ctx, kindCode, "race-key")
				if popErr != nil {
					t.Errorf("concurrent Pop error: %v", popErr)
					return
				}
				if ok && string(val) == "race-val" {
					wins.Add(1)
				}
			}()
		}
		startWG.Done()
		doneWG.Wait()
		if wins.Load() != 1 {
			t.Fatalf("expected exactly 1 winning concurrent Pop, got %d", wins.Load())
		}
	})

	t.Run("expiry with fake clock", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
		store := factory(t, clk.Now, map[string]int{kindSession: 10, kindCode: 10})
		defer store.Close()

		ctx := context.Background()
		if err := store.Put(ctx, kindSession, "s1", []byte("v1"), 30*time.Second); err != nil {
			t.Fatalf("Put s1 failed: %v", err)
		}
		if err := store.Put(ctx, kindCode, "c1", []byte("v2"), 30*time.Second); err != nil {
			t.Fatalf("Put c1 failed: %v", err)
		}

		clk.Advance(29 * time.Second)
		if _, ok, err := store.Get(ctx, kindSession, "s1"); err != nil || !ok {
			t.Fatalf("expected s1 live before expiry: ok=%v err=%v", ok, err)
		}

		clk.Advance(1 * time.Second)
		if _, ok, err := store.Get(ctx, kindSession, "s1"); err != nil || ok {
			t.Fatalf("expected s1 expired at exact TTL: ok=%v err=%v", ok, err)
		}
		if _, ok, err := store.Pop(ctx, kindCode, "c1"); err != nil || ok {
			t.Fatalf("expected c1 expired on Pop: ok=%v err=%v", ok, err)
		}
	})

	t.Run("cap returns errStoreFull and overwrite is allowed", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
		store := factory(t, clk.Now, map[string]int{kindSession: 2, kindCode: 2})
		defer store.Close()

		ctx := context.Background()
		if err := store.Put(ctx, kindSession, "k1", []byte("v1"), time.Minute); err != nil {
			t.Fatalf("Put k1 failed: %v", err)
		}
		if err := store.Put(ctx, kindSession, "k2", []byte("v2"), 2*time.Minute); err != nil {
			t.Fatalf("Put k2 failed: %v", err)
		}
		if err := store.Put(ctx, kindSession, "k3", []byte("v3"), time.Minute); !errors.Is(err, errStoreFull) {
			t.Fatalf("expected errStoreFull on k3, got %v", err)
		}

		// Overwriting existing live key at cap must succeed.
		if err := store.Put(ctx, kindSession, "k1", []byte("v1-updated"), time.Minute); err != nil {
			t.Fatalf("overwrite k1 at cap failed: %v", err)
		}
		got, ok, err := store.Get(ctx, kindSession, "k1")
		if err != nil || !ok || string(got) != "v1-updated" {
			t.Fatalf("expected overwritten value, got %q ok=%v err=%v", got, ok, err)
		}

		// Separate kind has independent cap.
		if err := store.Put(ctx, kindCode, "c1", []byte("cv1"), time.Minute); err != nil {
			t.Fatalf("Put in separate kind failed: %v", err)
		}

		// Once k1 expires, a new key k3 fits under cap.
		clk.Advance(61 * time.Second)
		if err := store.Put(ctx, kindSession, "k3", []byte("v3"), time.Minute); err != nil {
			t.Fatalf("Put k3 after expiry failed: %v", err)
		}
	})

	t.Run("delete removes key and ignores missing", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
		store := factory(t, clk.Now, map[string]int{kindSession: 10})
		defer store.Close()

		ctx := context.Background()
		if err := store.Put(ctx, kindSession, "s1", []byte("v1"), time.Minute); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
		if err := store.Delete(ctx, kindSession, "s1"); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		if _, ok, err := store.Get(ctx, kindSession, "s1"); err != nil || ok {
			t.Fatalf("expected s1 deleted: ok=%v err=%v", ok, err)
		}
		if err := store.Delete(ctx, kindSession, "missing"); err != nil {
			t.Fatalf("Delete missing key failed: %v", err)
		}
	})

	t.Run("revoke and isRevoked until expiry", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
		store := factory(t, clk.Now, map[string]int{kindSession: 10})
		defer store.Close()

		ctx := context.Background()
		rev, err := store.IsRevoked(ctx, "jti-1")
		if err != nil || rev {
			t.Fatalf("expected unrevoked jti-1: rev=%v err=%v", rev, err)
		}

		exp := clk.Now().Add(5 * time.Minute)
		if revErr := store.Revoke(ctx, "jti-1", exp); revErr != nil {
			t.Fatalf("Revoke failed: %v", revErr)
		}

		clk.Advance(4 * time.Minute)
		rev, err = store.IsRevoked(ctx, "jti-1")
		if err != nil || !rev {
			t.Fatalf("expected jti-1 revoked before expiry: rev=%v err=%v", rev, err)
		}

		clk.Advance(1 * time.Minute)
		rev, err = store.IsRevoked(ctx, "jti-1")
		if err != nil || rev {
			t.Fatalf("expected jti-1 expired: rev=%v err=%v", rev, err)
		}
	})

	t.Run("sweep removes expired items and revocations", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
		store := factory(t, clk.Now, map[string]int{kindSession: 10})
		defer store.Close()

		ctx := context.Background()
		if err := store.Put(ctx, kindSession, "short", []byte("a"), 10*time.Second); err != nil {
			t.Fatalf("Put short failed: %v", err)
		}
		if err := store.Put(ctx, kindSession, "long", []byte("b"), 10*time.Minute); err != nil {
			t.Fatalf("Put long failed: %v", err)
		}
		if err := store.Revoke(ctx, "rev-short", clk.Now().Add(10*time.Second)); err != nil {
			t.Fatalf("Revoke short failed: %v", err)
		}
		if err := store.Revoke(ctx, "rev-long", clk.Now().Add(10*time.Minute)); err != nil {
			t.Fatalf("Revoke long failed: %v", err)
		}

		clk.Advance(15 * time.Second)
		swept, err := store.Sweep(ctx)
		if err != nil {
			t.Fatalf("Sweep failed: %v", err)
		}
		if swept != 2 {
			t.Fatalf("expected 2 swept entries, got %d", swept)
		}

		if swept2, err := store.Sweep(ctx); err != nil || swept2 != 0 {
			t.Fatalf("expected 0 on second Sweep, got %d (err=%v)", swept2, err)
		}
		if _, ok, err := store.Get(ctx, kindSession, "long"); err != nil || !ok {
			t.Fatalf("expected live item preserved after sweep: ok=%v err=%v", ok, err)
		}
		if rev, err := store.IsRevoked(ctx, "rev-long"); err != nil || !rev {
			t.Fatalf("expected live revocation preserved after sweep: rev=%v err=%v", rev, err)
		}
	})

	t.Run("revokeOnce single use expiry and concurrent", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
		store := factory(t, clk.Now, map[string]int{kindSession: 10})
		defer store.Close()

		ctx := context.Background()
		exp := clk.Now().Add(5 * time.Minute)
		ok, err := store.RevokeOnce(ctx, "jti-once", exp)
		if err != nil || !ok {
			t.Fatalf("first RevokeOnce failed: ok=%v err=%v", ok, err)
		}
		ok, err = store.RevokeOnce(ctx, "jti-once", exp)
		if err != nil || ok {
			t.Fatalf("second RevokeOnce before expiry must return false: ok=%v err=%v", ok, err)
		}
		if rev, revErr := store.IsRevoked(ctx, "jti-once"); revErr != nil || !rev {
			t.Fatalf("expected jti-once revoked: rev=%v err=%v", rev, revErr)
		}

		// Once expired, existing row is treated as absent and RevokeOnce succeeds again.
		clk.Advance(5 * time.Minute)
		if rev, revErr := store.IsRevoked(ctx, "jti-once"); revErr != nil || rev {
			t.Fatalf("expected jti-once expired: rev=%v err=%v", rev, revErr)
		}
		ok, err = store.RevokeOnce(ctx, "jti-once", clk.Now().Add(5*time.Minute))
		if err != nil || !ok {
			t.Fatalf("RevokeOnce after expiry must return true: ok=%v err=%v", ok, err)
		}

		// Concurrent RevokeOnce on the same jti: exactly one returns true.
		const workers = 8
		var wins atomic.Int32
		var startWG, doneWG sync.WaitGroup
		startWG.Add(1)
		concExp := clk.Now().Add(10 * time.Minute)
		for i := 0; i < workers; i++ {
			doneWG.Add(1)
			go func() {
				defer doneWG.Done()
				startWG.Wait()
				won, revErr := store.RevokeOnce(ctx, "jti-race", concExp)
				if revErr != nil {
					t.Errorf("concurrent RevokeOnce error: %v", revErr)
					return
				}
				if won {
					wins.Add(1)
				}
			}()
		}
		startWG.Done()
		doneWG.Wait()
		if wins.Load() != 1 {
			t.Fatalf("expected exactly 1 winning concurrent RevokeOnce, got %d", wins.Load())
		}
	})

	t.Run("incr fixed window", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
		store := factory(t, clk.Now, nil)
		defer store.Close()
		ctx := context.Background()

		for want := 1; want <= 3; want++ {
			if n, err := store.Incr(ctx, "c", time.Hour); err != nil || n != want {
				t.Fatalf("Incr = %d, %v; want %d", n, err, want)
			}
		}
		clk.Advance(time.Hour)
		if n, err := store.Incr(ctx, "c", time.Hour); err != nil || n != 1 {
			t.Fatalf("Incr after window = %d, %v; want 1", n, err)
		}
	})

	t.Run("put evict frees a live slot atomically", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
		store := factory(t, clk.Now, map[string]int{kindDCRClient: 3})
		defer store.Close()
		ctx := context.Background()

		// An expired row that has not been swept must not be the one evicted:
		// it does not count toward the cap, so deleting it frees nothing.
		if err := store.Put(ctx, kindDCRClient, "expired", []byte("v"), time.Minute); err != nil {
			t.Fatalf("Put expired: %v", err)
		}
		clk.Advance(2 * time.Minute)
		for key, ttl := range map[string]time.Duration{"late": 3 * time.Hour, "early": time.Hour, "mid": 2 * time.Hour} {
			if err := store.Put(ctx, kindDCRClient, key, []byte("v"), ttl); err != nil {
				t.Fatalf("Put %s: %v", key, err)
			}
		}
		if err := store.Put(ctx, kindDCRClient, "new", []byte("v"), 4*time.Hour); !errors.Is(err, errStoreFull) {
			t.Fatalf("Put at cap = %v, want errStoreFull", err)
		}
		evicted, err := store.PutEvict(ctx, kindDCRClient, "new", []byte("v"), 4*time.Hour)
		if err != nil || !evicted {
			t.Fatalf("PutEvict = %v, %v; want evicted", evicted, err)
		}
		if _, ok, _ := store.Get(ctx, kindDCRClient, "early"); ok {
			t.Fatalf("earliest-expiring live item should be evicted")
		}
		for _, k := range []string{"mid", "late", "new"} {
			if _, ok, _ := store.Get(ctx, kindDCRClient, k); !ok {
				t.Fatalf("%s should remain", k)
			}
		}

		// Concurrent registrations at cap must all succeed.
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := store.PutEvict(ctx, kindDCRClient, fmt.Sprintf("c%d", i), []byte("v"), time.Duration(5+i)*time.Hour)
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("concurrent PutEvict at cap: %v", err)
			}
		}
	})

	t.Run("compare and swap", func(t *testing.T) {
		clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
		store := factory(t, clk.Now, nil)
		defer store.Close()
		ctx := context.Background()

		// Use an unsealed kind so SealedMemory also exercises the CAS contract.
		const kind = kindUserRevoked
		const key = "cas-key"

		type casStep struct {
			name     string
			advance  time.Duration
			seed     []byte
			seedTTL  time.Duration
			oldVal   []byte
			newVal   []byte
			wantSwap bool
			wantLive bool
			wantVal  []byte
		}

		steps := []casStep{
			{
				name:     "nil-to-nil on absent row",
				oldVal:   nil,
				newVal:   nil,
				wantSwap: true,
				wantLive: false,
			},
			{
				name:     "insert-if-absent",
				oldVal:   nil,
				newVal:   []byte("v1"),
				wantSwap: true,
				wantLive: true,
				wantVal:  []byte("v1"),
			},
			{
				name:     "nil-to-nil conflicts on live row",
				oldVal:   nil,
				newVal:   nil,
				wantSwap: false,
				wantLive: true,
				wantVal:  []byte("v1"),
			},
			{
				name:     "insert-if-absent conflict",
				oldVal:   nil,
				newVal:   []byte("v2"),
				wantSwap: false,
				wantLive: true,
				wantVal:  []byte("v1"),
			},
			{
				name:     "update conflict",
				oldVal:   []byte("wrong"),
				newVal:   []byte("v2"),
				wantSwap: false,
				wantLive: true,
				wantVal:  []byte("v1"),
			},
			{
				name:     "update match",
				oldVal:   []byte("v1"),
				newVal:   []byte("v2"),
				wantSwap: true,
				wantLive: true,
				wantVal:  []byte("v2"),
			},
			{
				name:     "delete conflict",
				oldVal:   []byte("v1"),
				newVal:   nil,
				wantSwap: false,
				wantLive: true,
				wantVal:  []byte("v2"),
			},
			{
				name:     "delete match",
				oldVal:   []byte("v2"),
				newVal:   nil,
				wantSwap: true,
				wantLive: false,
			},
			{
				name:     "update missing row",
				oldVal:   []byte("v2"),
				newVal:   []byte("v3"),
				wantSwap: false,
				wantLive: false,
			},
			{
				name:     "delete missing row",
				oldVal:   []byte("v2"),
				newVal:   nil,
				wantSwap: false,
				wantLive: false,
			},
			{
				name:     "update expired row fails",
				seed:     []byte("exp"),
				seedTTL:  10 * time.Second,
				advance:  10 * time.Second,
				oldVal:   []byte("exp"),
				newVal:   []byte("nope"),
				wantSwap: false,
				wantLive: false,
			},
			{
				name:     "delete expired row fails",
				seed:     []byte("exp"),
				seedTTL:  10 * time.Second,
				advance:  10 * time.Second,
				oldVal:   []byte("exp"),
				newVal:   nil,
				wantSwap: false,
				wantLive: false,
			},
			{
				name:     "insert-if-absent succeeds over expired row",
				seed:     []byte("exp2"),
				seedTTL:  10 * time.Second,
				advance:  10 * time.Second,
				oldVal:   nil,
				newVal:   []byte("fresh"),
				wantSwap: true,
				wantLive: true,
				wantVal:  []byte("fresh"),
			},
		}

		for _, tc := range steps {
			if tc.seed != nil {
				if err := store.Put(ctx, kind, key, tc.seed, tc.seedTTL); err != nil {
					t.Fatalf("%s: seed Put: %v", tc.name, err)
				}
			}
			if tc.advance > 0 {
				clk.Advance(tc.advance)
			}
			swapped, err := store.CompareAndSwap(ctx, kind, key, tc.oldVal, tc.newVal, time.Minute)
			if err != nil {
				t.Fatalf("%s: CompareAndSwap err: %v", tc.name, err)
			}
			if swapped != tc.wantSwap {
				t.Fatalf("%s: swapped=%v, want %v", tc.name, swapped, tc.wantSwap)
			}
			got, ok, err := store.Get(ctx, kind, key)
			if err != nil {
				t.Fatalf("%s: Get err: %v", tc.name, err)
			}
			if ok != tc.wantLive || (tc.wantLive && !bytes.Equal(got, tc.wantVal)) {
				t.Fatalf("%s: Get=(%q, %v), want (%q, %v)", tc.name, got, ok, tc.wantVal, tc.wantLive)
			}
		}
	})
}

func TestStoreContract_Memory(t *testing.T) {
	runStoreContract(t, func(_ *testing.T, now func() time.Time, caps map[string]int) SessionStore {
		return newMemoryStore(now, caps)
	})
}

func TestStoreContract_SealedMemory(t *testing.T) {
	kr, err := parseKeyRing("k1:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatalf("parseKeyRing failed: %v", err)
	}
	runStoreContract(t, func(_ *testing.T, now func() time.Time, caps map[string]int) SessionStore {
		return newSealedStore(newMemoryStore(now, caps), kr)
	})
}

func newTestPostgresStore(t *testing.T, pgURL string, now func() time.Time, caps map[string]int) SessionStore {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var randBuf [6]byte
	if _, err := rand.Read(randBuf[:]); err != nil {
		t.Fatalf("rand schema suffix: %v", err)
	}
	schema := "fgw_test_" + hex.EncodeToString(randBuf[:])

	adminPool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		t.Fatalf("connect postgres admin pool: %v", err)
	}
	if _, execErr := adminPool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA "%s"`, schema)); execErr != nil {
		adminPool.Close()
		t.Fatalf("create test schema %s: %v", schema, execErr)
	}
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dropCancel()
		_, _ = adminPool.Exec(dropCtx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema))
		adminPool.Close()
	})

	u, err := url.Parse(pgURL)
	if err != nil {
		t.Fatalf("parse FGW_TEST_PG_URL: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	store, err := newPostgresStore(ctx, u.String(), now, caps)
	if err != nil {
		t.Fatalf("newPostgresStore failed: %v", err)
	}
	return store
}

func TestStoreContract_Postgres(t *testing.T) {
	pgURL := os.Getenv("FGW_TEST_PG_URL")
	if pgURL == "" {
		t.Skip("FGW_TEST_PG_URL not set")
	}

	runStoreContract(t, func(t *testing.T, now func() time.Time, caps map[string]int) SessionStore {
		return newTestPostgresStore(t, pgURL, now, caps)
	})
}

func TestSealedStore_SecurityAndKeyRotation(t *testing.T) {
	key1 := testKeyBase64(t, false)
	key2 := testKeyBase64(t, true) // raw-url base64

	kr1, err := parseKeyRing("k1:" + key1)
	if err != nil {
		t.Fatalf("parseKeyRing k1 failed: %v", err)
	}
	krRotated, err := parseKeyRing("k2:" + key2 + ",k1:" + key1)
	if err != nil {
		t.Fatalf("parseKeyRing rotated failed: %v", err)
	}
	krOnlyK2, err := parseKeyRing("k2:" + key2)
	if err != nil {
		t.Fatalf("parseKeyRing k2 failed: %v", err)
	}

	clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
	inner := newMemoryStore(clk.Now, map[string]int{kindSession: 20, kindCode: 20})
	sealed1 := newSealedStore(inner, kr1)
	ctx := context.Background()

	secretPlaintext := []byte("super-secret-refresh-token-value")
	if putErr := sealed1.Put(ctx, kindSession, "sess-1", secretPlaintext, time.Hour); putErr != nil {
		t.Fatalf("Put sess-1 failed: %v", putErr)
	}

	// Raw stored bytes must not contain the plaintext.
	rawBlob, ok, err := inner.Get(ctx, kindSession, "sess-1")
	if err != nil || !ok {
		t.Fatalf("inner.Get failed: ok=%v err=%v", ok, err)
	}
	if bytes.Contains(rawBlob, secretPlaintext) {
		t.Fatal("raw stored blob leaked plaintext")
	}

	// Key rotation: rotated keyRing (k2 primary, k1 secondary) still decrypts k1 data.
	sealedRotated := newSealedStore(inner, krRotated)
	got, ok, err := sealedRotated.Get(ctx, kindSession, "sess-1")
	if err != nil || !ok || !bytes.Equal(got, secretPlaintext) {
		t.Fatalf("rotated Get failed: ok=%v err=%v got=%q", ok, err, got)
	}

	// New writes under rotated keyRing use primary key k2.
	if putErr := sealedRotated.Put(ctx, kindSession, "sess-2", []byte("new-secret"), time.Hour); putErr != nil {
		t.Fatalf("Put sess-2 failed: %v", putErr)
	}
	rawK2, ok, err := inner.Get(ctx, kindSession, "sess-2")
	if err != nil || !ok {
		t.Fatalf("inner.Get sess-2 failed: ok=%v err=%v", ok, err)
	}
	if int(rawK2[0]) != len("k2") || string(rawK2[1:1+len("k2")]) != "k2" {
		t.Fatalf("expected blob header kid k2, got %q", rawK2[1:1+int(rawK2[0])])
	}

	// Removing k1 causes k1-sealed blob to fail closed with an error (not not-found).
	sealedOnlyK2 := newSealedStore(inner, krOnlyK2)
	if _, ok, err := sealedOnlyK2.Get(ctx, kindSession, "sess-1"); err == nil || ok {
		t.Fatalf("expected error for unknown kid on Get, got ok=%v err=%v", ok, err)
	}
	if _, ok, err := sealedOnlyK2.Pop(ctx, kindSession, "sess-1"); err == nil || ok {
		t.Fatalf("expected error for unknown kid on Pop, got ok=%v err=%v", ok, err)
	}

	// Re-seed sess-1 and verify tamper & AAD swap fail closed.
	if err := sealed1.Put(ctx, kindSession, "sess-1", secretPlaintext, time.Hour); err != nil {
		t.Fatalf("re-Put sess-1 failed: %v", err)
	}
	origBlob, _, _ := inner.Get(ctx, kindSession, "sess-1")

	// Tamper ciphertext byte.
	tampered := append([]byte(nil), origBlob...)
	tampered[len(tampered)-1] ^= 0xff
	if err := inner.Put(ctx, kindSession, "sess-tampered", tampered, time.Hour); err != nil {
		t.Fatalf("inner Put tampered failed: %v", err)
	}
	// Note: kid+key AAD also differs, test exact-key tamper on sess-1.
	if err := inner.Put(ctx, kindSession, "sess-1", tampered, time.Hour); err != nil {
		t.Fatalf("inner Put tampered sess-1 failed: %v", err)
	}
	if _, ok, err := sealed1.Get(ctx, kindSession, "sess-1"); err == nil || ok {
		t.Fatalf("expected error on tampered ciphertext Get, got ok=%v err=%v", ok, err)
	}
	if _, ok, err := sealed1.Pop(ctx, kindSession, "sess-1"); err == nil || ok {
		t.Fatalf("expected error on tampered ciphertext Pop, got ok=%v err=%v", ok, err)
	}

	// AAD swap: copy valid sess-1 blob to another key and another kind.
	if err := inner.Put(ctx, kindSession, "sess-other-key", origBlob, time.Hour); err != nil {
		t.Fatalf("inner Put other-key failed: %v", err)
	}
	if _, ok, err := sealed1.Get(ctx, kindSession, "sess-other-key"); err == nil || ok {
		t.Fatalf("expected AAD mismatch error on key swap, got ok=%v err=%v", ok, err)
	}
	if err := inner.Put(ctx, kindCode, "sess-1", origBlob, time.Hour); err != nil {
		t.Fatalf("inner Put other-kind failed: %v", err)
	}
	if _, ok, err := sealed1.Pop(ctx, kindCode, "sess-1"); err == nil || ok {
		t.Fatalf("expected AAD mismatch error on kind swap, got ok=%v err=%v", ok, err)
	}

	// CompareAndSwap refuses sealed kinds.
	if swapped, casErr := sealed1.CompareAndSwap(ctx, kindSession, "sess-cas", nil, []byte("v"), time.Hour); casErr == nil || swapped {
		t.Fatalf("expected error for CompareAndSwap on sealed kind, got swapped=%v err=%v", swapped, casErr)
	}
}

func TestSealedStore_AuditOutboxLegacy(t *testing.T) {
	kr1, err := parseKeyRing("k1:" + testKeyBase64(t, false))
	if err != nil {
		t.Fatalf("parseKeyRing k1 failed: %v", err)
	}
	krOnlyK2, err := parseKeyRing("k2:" + testKeyBase64(t, true))
	if err != nil {
		t.Fatalf("parseKeyRing k2 failed: %v", err)
	}

	clk := newFakeClock(time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC))
	inner := newMemoryStore(clk.Now, nil)
	sealed1 := newSealedStore(inner, kr1)
	sealedOnlyK2 := newSealedStore(inner, krOnlyK2)
	ctx := context.Background()

	legacyOutboxPlain := []byte(`[{"id":"1"}]`)
	legacyOutboxBlob, err := kr1.Seal(sealAAD(kindAuditOutbox, lockoutAuditOutboxKey), legacyOutboxPlain)
	if err != nil {
		t.Fatalf("Seal legacy outbox failed: %v", err)
	}
	if putErr := inner.Put(ctx, kindAuditOutbox, lockoutAuditOutboxKey, legacyOutboxBlob, time.Hour); putErr != nil {
		t.Fatalf("inner Put legacy outbox failed: %v", putErr)
	}
	if _, ok, getErr := sealedOnlyK2.Get(ctx, kindAuditOutbox, lockoutAuditOutboxKey); getErr == nil || ok {
		t.Fatalf("expected error on legacy outbox Get with wrong key, got ok=%v err=%v", ok, getErr)
	}
	if swapped, casErr := sealedOnlyK2.CompareAndSwap(ctx, kindAuditOutbox, lockoutAuditOutboxKey, legacyOutboxPlain, []byte(`[]`), time.Hour); casErr == nil || swapped {
		t.Fatalf("expected error on legacy outbox CAS with wrong key, got swapped=%v err=%v", swapped, casErr)
	}
	if swapped, casErr := sealed1.CompareAndSwap(ctx, kindAuditOutbox, lockoutAuditOutboxKey, nil, []byte(`[]`), time.Hour); casErr != nil || swapped {
		t.Fatalf("expected conflict on legacy outbox CAS with oldVal=nil, got swapped=%v err=%v", swapped, casErr)
	}
	if swapped, casErr := sealed1.CompareAndSwap(ctx, kindAuditOutbox, lockoutAuditOutboxKey, []byte(`[{"id":"wrong"}]`), []byte(`[]`), time.Hour); casErr != nil || swapped {
		t.Fatalf("expected conflict on legacy outbox CAS with mismatched oldVal, got swapped=%v err=%v", swapped, casErr)
	}
	if popped, ok, popErr := sealed1.Pop(ctx, kindAuditOutbox, lockoutAuditOutboxKey); popErr != nil || !ok || !bytes.Equal(popped, legacyOutboxPlain) {
		t.Fatalf("expected legacy outbox Pop to unseal, got %q ok=%v err=%v", popped, ok, popErr)
	}
}

func TestParseKeyRing_Validation(t *testing.T) {
	validStd := testKeyBase64(t, false)
	validRawURL := testKeyBase64(t, true)
	shortKey := base64.StdEncoding.EncodeToString(make([]byte, 31))
	longKey := base64.StdEncoding.EncodeToString(make([]byte, 33))

	if _, err := parseKeyRing("k1:" + validStd + ",k2:" + validRawURL); err != nil {
		t.Fatalf("expected valid keyring with std+rawurl to parse, got %v", err)
	}

	invalid := []string{
		"",
		"   ",
		"k1:" + validStd + ",",
		"no-colon",
		": " + validStd,
		"bad/kid:" + validStd,
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:" + validStd, // 33 chars
		"dup:" + validStd + ",dup:" + validRawURL,
		"k1:not-base64!!!",
		"k1:" + shortKey,
		"k1:" + longKey,
	}
	for _, raw := range invalid {
		if _, err := parseKeyRing(raw); err == nil {
			t.Fatalf("expected parseKeyRing(%q) to fail", raw)
		}
	}
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{now: start}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
