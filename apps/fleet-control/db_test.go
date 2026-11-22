package main

import (
	"context"
	"os"
	"testing"
)

// openTestDB connects to TEST_DATABASE_URL on a clean schema. Skips when unset.
func openTestDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" && os.Getenv("CI") != "" {
		t.Fatal("TEST_DATABASE_URL must be set in CI")
	}
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, err := OpenDB(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if _, err := db.pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestRealPostgresSeedAndSnapshot(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	for range 2 {
		if err := db.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	routes, allowlist := readSeedFiles(t, "prod")
	servers, err := seedServers(routes, allowlist)
	if err != nil {
		t.Fatal(err)
	}
	if seeded, seedErr := db.Seed(ctx, servers, "test"); seedErr != nil || !seeded {
		t.Fatalf("first seed: %v %v", seeded, seedErr)
	}
	if seeded, seedErr := db.Seed(ctx, servers[:1], "test"); seedErr != nil || seeded {
		t.Fatalf("second seed must be a no-op: %v %v", seeded, seedErr)
	}
	got, err := db.Servers(ctx)
	if err != nil || len(got) != len(servers) {
		t.Fatalf("servers: %d %v", len(got), err)
	}
	for i, s := range got {
		want := servers[i]
		if s.Key() != want.Key() || s.RegistryName != want.RegistryName || (s.Route == nil) != (want.Route == nil) {
			t.Errorf("round trip %s: got %+v", want.Key(), s)
		}
	}

	srv := newServer()
	srv.store.Store(&storeBox{db})
	for _, kind := range []string{"routes", "registry"} {
		a, aErr := srv.currentSnapshot(ctx, kind)
		if aErr != nil || a.Generation != 1 {
			t.Fatalf("%s: %v %+v", kind, aErr, a)
		}
		b, bErr := srv.currentSnapshot(ctx, kind)
		if bErr != nil || b.Generation != 1 || b.ETag != a.ETag {
			t.Errorf("%s rebuilt from the database must keep its generation: %+v", kind, b)
		}
	}

	if _, err = db.pool.Exec(ctx, "UPDATE servers SET status = 'revoked' WHERE name = 'dolly'"); err != nil {
		t.Fatal(err)
	}
	next, err := srv.currentSnapshot(ctx, "routes")
	if err != nil || next.Generation != 2 {
		t.Errorf("revoke must publish generation 2: %v %+v", err, next)
	}

	for _, stmt := range []string{"UPDATE audit_events SET actor = 'x'", "DELETE FROM audit_events", "TRUNCATE audit_events"} {
		if _, err = db.pool.Exec(ctx, stmt); err == nil {
			t.Errorf("audit_events must be append-only: %s succeeded", stmt)
		}
	}
}
