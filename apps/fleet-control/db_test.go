package main

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/navikt/copilot/fleet-control/internal/model"
)

const testIssuer = "https://login.microsoftonline.com/test-tenant/v2.0"

// openTestDB connects to TEST_DATABASE_URL on a clean schema. Skips when unset.
func openTestDB(t *testing.T) *DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" && os.Getenv("FLEET_CONTROL_REQUIRE_PG") == "1" {
		t.Fatal("FLEET_CONTROL_REQUIRE_PG=1 needs TEST_DATABASE_URL")
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
	entraIssuer = testIssuer
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
		a, aErr := srv.resolve(ctx, kind)
		if aErr != nil || a.Generation != 1 {
			t.Fatalf("%s: %v %+v", kind, aErr, a)
		}
		b, bErr := srv.resolve(ctx, kind)
		if bErr != nil || b.Generation != 1 || b.ETag != a.ETag {
			t.Errorf("%s rebuilt from the database must keep its generation: %+v", kind, b)
		}
	}

	if _, err = db.pool.Exec(ctx, "UPDATE servers SET status = 'revoked' WHERE name = 'dolly'"); err != nil {
		t.Fatal(err)
	}
	next, err := srv.resolve(ctx, "routes")
	if err != nil || next.Generation != 2 {
		t.Errorf("revoke must publish generation 2: %v %+v", err, next)
	}

	// A running instance does not read the seed files again.
	t.Setenv("SEED_ROUTES_FILE", "/nonexistent/routes.yaml")
	t.Setenv("SEED_ALLOWLIST_FILE", "/nonexistent/allowlist.json")
	if err = seedFromEnv(ctx, db); err != nil {
		t.Errorf("seed with a populated database must not read the files: %v", err)
	}

	// Concurrent publishes are serialised: each gets its own generation.
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := range 10 {
		wg.Go(func() {
			_, pubErr := db.Publish(ctx, "registry", func(kind string, servers []model.Server, prev *model.Snapshot) (*model.Snapshot, error) {
				next, buildErr := buildSnapshot(kind, servers, prev)
				if buildErr != nil {
					return nil, buildErr
				}
				return &model.Snapshot{Kind: kind, Generation: prev.Generation + 1, ETag: fmt.Sprint(next.ETag, i), Items: next.Items}, nil
			})
			errs <- pubErr
		})
	}
	wg.Wait()
	close(errs)
	for pubErr := range errs {
		if pubErr != nil {
			t.Errorf("concurrent publish: %v", pubErr)
		}
	}
	if latest, _ := db.LatestSnapshot(ctx, "registry"); latest == nil || latest.Generation != 11 {
		t.Errorf("want generation 11 after 10 publishes, got %+v", latest)
	}

	for _, stmt := range []string{"UPDATE audit_events SET actor = 'x'", "DELETE FROM audit_events", "TRUNCATE audit_events"} {
		if _, err = db.pool.Exec(ctx, stmt); err == nil {
			t.Errorf("audit_events must be append-only: %s succeeded", stmt)
		}
	}
}
