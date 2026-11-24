package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestAuditEventsLockedForRuntimeRole: a runtime role granted everything
// before 006 (as Nais does) loses the rights to change audit_events (#2631).
func TestAuditEventsLockedForRuntimeRole(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.migrateTo(ctx, "005_v1alpha1.sql"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"DROP OWNED BY fleet_rw_test",
		"DROP ROLE IF EXISTS fleet_rw_test",
		"DROP OWNED BY fleet_grp_test",
		"DROP ROLE IF EXISTS fleet_grp_test",
	} {
		_, _ = db.pool.Exec(ctx, q) // DROP OWNED fails when the role does not exist yet
	}
	for _, q := range []string{
		"CREATE ROLE fleet_rw_test",
		"GRANT USAGE ON SCHEMA public TO fleet_rw_test",
		"GRANT ALL ON ALL TABLES IN SCHEMA public TO fleet_rw_test",
		"GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO fleet_rw_test",
		"GRANT ALL ON audit_events TO PUBLIC",
		"GRANT UPDATE (actor) ON audit_events TO fleet_rw_test, PUBLIC",
		// Nais grants through a group role (app_readwrite) the runtime role inherits.
		"CREATE ROLE fleet_grp_test",
		"GRANT ALL ON audit_events, audit_events_id_seq TO fleet_grp_test",
		"GRANT fleet_grp_test TO fleet_rw_test",
	} {
		if _, err := db.pool.Exec(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.pool.Exec(ctx, "DROP OWNED BY fleet_rw_test")
		_, _ = db.pool.Exec(ctx, "DROP ROLE fleet_rw_test")
		_, _ = db.pool.Exec(ctx, "DROP OWNED BY fleet_grp_test")
		_, _ = db.pool.Exec(ctx, "DROP ROLE fleet_grp_test")
	})
	asRuntime := func() error {
		return pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SET LOCAL ROLE fleet_rw_test"); err != nil {
				return err
			}
			return checkAuditLocked(ctx, tx)
		})
	}
	if err := asRuntime(); err == nil || !strings.Contains(err.Error(), "refusing to start") {
		t.Fatalf("control: want refusal before 006, got %v", err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := asRuntime(); err != nil {
		t.Fatalf("after 006: %v", err)
	}
	// The real statements fail on privilege, not on the trigger.
	for _, q := range []string{"DELETE FROM audit_events", "UPDATE audit_events SET actor = 'x'", "TRUNCATE audit_events", "ALTER TABLE audit_events DISABLE TRIGGER ALL"} {
		err := pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SET LOCAL ROLE fleet_rw_test"); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, q)
			return err
		})
		if err == nil || (!strings.Contains(err.Error(), "permission denied") && !strings.Contains(err.Error(), "must be owner")) {
			t.Errorf("%s as runtime role: %v", q, err)
		}
	}
	// INSERT and SELECT still work.
	err := pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SET LOCAL ROLE fleet_rw_test"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO audit_events (actor, action, namespace, name) VALUES ('t', 't', 'ns', 'n')"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "SELECT count(*) FROM audit_events")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	// The owner itself fails the check, so a misconfigured runtime pool stops the app.
	if err := db.CheckAuditLocked(ctx); err == nil {
		t.Fatal("owner role passed the check")
	}
}

// TestAuditCheckRefusesSingleRight: each change right on its own fails the
// startup check, including column-level UPDATE and setval on the id sequence.
func TestAuditCheckRefusesSingleRight(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, grant := range []string{
		"GRANT TRIGGER ON audit_events TO fleet_one_test",
		"GRANT UPDATE (actor) ON audit_events TO fleet_one_test",
		"GRANT UPDATE ON SEQUENCE audit_events_id_seq TO fleet_one_test",
		"GRANT INSERT ON resource_status TO fleet_one_test",
		"GRANT UPDATE ON resource_status TO fleet_one_test",
		"GRANT DELETE ON resource_status TO fleet_one_test",
		"GRANT TRUNCATE ON resource_status TO fleet_one_test",
		"GRANT TRIGGER ON resource_status TO fleet_one_test",
		"GRANT UPDATE (by_role) ON resource_status TO fleet_one_test",
		"GRANT INSERT (by_role) ON resource_status TO fleet_one_test",
	} {
		t.Run(grant, func(t *testing.T) {
			_, _ = db.pool.Exec(ctx, "DROP OWNED BY fleet_one_test")
			_, _ = db.pool.Exec(ctx, "DROP ROLE IF EXISTS fleet_one_test")
			for _, q := range []string{"CREATE ROLE fleet_one_test", "GRANT USAGE ON SCHEMA public TO fleet_one_test", grant} {
				if _, err := db.pool.Exec(ctx, q); err != nil {
					t.Fatal(q, err)
				}
			}
			t.Cleanup(func() {
				_, _ = db.pool.Exec(ctx, "DROP OWNED BY fleet_one_test")
				_, _ = db.pool.Exec(ctx, "DROP ROLE fleet_one_test")
			})
			err := pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, "SET LOCAL ROLE fleet_one_test"); err != nil {
					return err
				}
				return checkAuditLocked(ctx, tx)
			})
			if err == nil || !strings.Contains(err.Error(), "refusing to start") {
				t.Fatalf("want refusal, got %v", err)
			}
		})
	}
}

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
	rt, err := setupDB(ctx)
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
