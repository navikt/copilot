package main

import (
	"context"
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
	} {
		_, _ = db.pool.Exec(ctx, q) // DROP OWNED fails when the role does not exist yet
	}
	for _, q := range []string{
		"CREATE ROLE fleet_rw_test",
		"GRANT USAGE ON SCHEMA public TO fleet_rw_test",
		"GRANT ALL ON ALL TABLES IN SCHEMA public TO fleet_rw_test",
		"GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO fleet_rw_test",
		"GRANT ALL ON audit_events TO PUBLIC",
	} {
		if _, err := db.pool.Exec(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.pool.Exec(ctx, "DROP OWNED BY fleet_rw_test")
		_, _ = db.pool.Exec(ctx, "DROP ROLE fleet_rw_test")
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
		if _, err := tx.Exec(ctx, "INSERT INTO audit_events (actor, action) VALUES ('t', 't')"); err != nil {
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
