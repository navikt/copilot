package api

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestSetupDBRevokesReGrant: a re-grant to the runtime role after 006 (as a
// later Nais reconcile could do) is revoked at the next start, so setupDB
// recovers instead of crash-looping.
func TestSetupDBRevokesReGrant(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.pool.Exec(ctx, "DROP OWNED BY fleet_login_test")
	_, _ = db.pool.Exec(ctx, "DROP ROLE IF EXISTS fleet_login_test")
	for _, q := range []string{
		"CREATE ROLE fleet_login_test LOGIN PASSWORD 'test'",
		"GRANT USAGE ON SCHEMA public TO fleet_login_test",
		"GRANT ALL ON ALL TABLES IN SCHEMA public TO fleet_login_test",
		"GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO fleet_login_test",
		"GRANT UPDATE (actor) ON audit_events TO fleet_login_test",
		"GRANT UPDATE (by_role) ON resource_status TO fleet_login_test",
	} {
		if _, execErr := db.pool.Exec(ctx, q); execErr != nil {
			t.Fatal(q, execErr)
		}
	}
	t.Cleanup(func() {
		_, _ = db.pool.Exec(ctx, "DROP OWNED BY fleet_login_test")
		_, _ = db.pool.Exec(ctx, "DROP ROLE fleet_login_test")
	})
	t.Setenv("DB_URL", dsn)
	t.Setenv("READWRITE_PGHOST", cfg.Host)
	t.Setenv("READWRITE_PGPORT", strconv.Itoa(int(cfg.Port)))
	t.Setenv("READWRITE_PGDATABASE", cfg.Database)
	t.Setenv("READWRITE_PGUSER", "fleet_login_test")
	t.Setenv("READWRITE_PGPASSWORD", "test")
	t.Setenv("READWRITE_PGSSLMODE", "disable")
	rt, err := setupDB(ctx, "prod-gcp")
	if err != nil {
		t.Fatalf("setupDB after re-grant: %v", err)
	}
	defer rt.Close()
	if err := rt.CheckAuditLocked(ctx); err != nil {
		t.Fatal(err)
	}
	// resource_status stays read-only for the runtime role (009).
	if _, err := rt.pool.Exec(ctx, "SELECT count(*) FROM resource_status"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"DELETE FROM resource_status", "UPDATE resource_status SET by_role = ''", "INSERT INTO resource_status VALUES ('a', 'b', '', 'revoked', now(), '')", "TRUNCATE resource_status"} {
		if _, err := rt.pool.Exec(ctx, q); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("%s as runtime role: %v", q, err)
		}
	}
}

// Without READWRITE_* the owner role would serve requests unchecked; in Nais
// setupDB refuses, outside Nais (local dev, tests) it falls back (#2854).
func TestSetupDBRefusesOwnerFallbackInNais(t *testing.T) {
	_ = openTestDB(t)
	t.Setenv("DB_URL", os.Getenv("TEST_DATABASE_URL"))
	t.Setenv("READWRITE_PGHOST", "")
	t.Setenv("NAIS_CLUSTER_NAME", "dev-gcp")
	if db, err := setupDB(context.Background(), "prod-gcp"); err == nil {
		db.Close()
		t.Fatal("setupDB in Nais without READWRITE_PGHOST must fail")
	}
	t.Setenv("NAIS_CLUSTER_NAME", "")
	db, err := setupDB(context.Background(), "prod-gcp")
	if err != nil {
		t.Fatalf("local owner fallback: %v", err)
	}
	db.Close()
}
