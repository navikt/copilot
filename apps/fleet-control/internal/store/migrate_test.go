package store

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	testIssuer  = "https://login.microsoftonline.com/test-tenant/v2.0"
	appClientID = "55555555-5555-4555-8555-555555555555"
	oidA        = "00000000-0000-4000-8000-00000000000a"
	oidB1       = "00000000-0000-4000-8000-0000000000b1"
	oidB2       = "00000000-0000-4000-8000-0000000000b2"
)

// openTestDB connects to TEST_DATABASE_URL on a clean schema. Skips when unset.
func openTestDB(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" && os.Getenv("FLEET_CONTROL_REQUIRE_PG") == "1" {
		t.Fatal("FLEET_CONTROL_REQUIRE_PG=1 needs TEST_DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	return New(pool)
}

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

// TestIdentityBackfill: 009 fills actor_oid from a NAVident paired with
// exactly one oid, leaves ambiguous names NULL, and turns the append-only
// trigger back on (#2763).
func TestIdentityBackfill(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.migrateTo(ctx, "008_registry_name_ci.sql"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO audit_events (actor, actor_oid, action) VALUES ('A111111', '` + oidA + `', 'v1')`,
		`INSERT INTO audit_events (actor, action) VALUES ('A111111', 'legacy')`,
		`INSERT INTO audit_events (actor, actor_oid, action) VALUES ('B222222', '` + oidB1 + `', 'v1'), ('B222222', '` + oidB2 + `', 'v1')`,
		`INSERT INTO audit_events (actor, action) VALUES ('B222222', 'legacy'), ('seed', 'seed_published')`,
		`INSERT INTO audit_events (actor, action, resource_kind, namespace, name, detail, at) VALUES ('P1', 'revoked', 'MCPServer', 'ns', 'svc', '{"by":"platform"}', now() - interval '1 day'), ('P1', 'revoked', 'MCPServer', 'ns', 'svc', '{"by":"owner"}', now() - interval '2 days')`,
	} {
		if _, err := db.pool.Exec(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	rows, err := db.pool.Query(ctx, `SELECT actor, COALESCE(actor_oid::text, ''), COALESCE(actor_kind, '') FROM audit_events WHERE action IN ('legacy', 'seed_published')`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var a, o, k string
		if err := rows.Scan(&a, &o, &k); err != nil {
			t.Fatal(err)
		}
		got[a] = o + "/" + k
	}
	want := map[string]string{"A111111": oidA + "/", "B222222": "/", "seed": "/System"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	// 009 copies the newest status event per resource, without the NAVident.
	var by string
	if err := db.pool.QueryRow(ctx, `SELECT by_role FROM resource_status WHERE namespace = 'ns' AND name = 'svc' AND action = 'revoked'`).Scan(&by); err != nil || by != "platform" {
		t.Fatalf("resource_status backfill: %q, %v", by, err)
	}
	if _, err := db.pool.Exec(ctx, `UPDATE audit_events SET actor = 'x'`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("trigger must be back on after 009: %v", err)
	}
}

// TestPruneAudit: retention removes only rows past 90 days, and only in a
// session that opted in (#2762).
func TestPruneAudit(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `INSERT INTO audit_events (actor, action, at) VALUES ('A1', 'x', now() - interval '92 days'), ('A1', 'x', now() - interval '91 days'), ('A1', 'x', now() - interval '89 days')`); err != nil {
		t.Fatal(err)
	}
	// Without the session setting even the owner cannot delete.
	if _, err := db.pool.Exec(ctx, `DELETE FROM audit_events WHERE at < now() - interval '90 days'`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("delete without retention setting: %v", err)
	}
	// With the setting, a row younger than 90 days is still refused.
	err := pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL fleet.audit_retention = 'on'`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM audit_events WHERE at > now() - interval '90 days'`)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("delete of young row: %v", err)
	}
	n, err := db.PruneAudit(ctx)
	if err != nil || n != 2 {
		t.Fatalf("PruneAudit: n=%d err=%v", n, err)
	}
	var left string
	if err := db.pool.QueryRow(ctx, `SELECT count(*)::text FROM audit_events`).Scan(&left); err != nil || left != "1" {
		t.Fatalf("left %q, %v", left, err)
	}
}

// TestResourceStatusTracksNewest: revoke, unrevoke, revoke again leaves the
// newest revocation, and a future at in an audit insert does not pin the
// row, because the trigger stamps now() (009).
func TestResourceStatusTracksNewest(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	ins := func(action, by, offset string) {
		t.Helper()
		if _, err := db.pool.Exec(ctx, `INSERT INTO audit_events (actor, action, resource_kind, namespace, name, detail, at)
			VALUES ('A1', $1, 'MCPServer', 'ns', 'svc', jsonb_build_object('by', $2::text), now() + $3::interval)`, action, by, offset); err != nil {
			t.Fatal(err)
		}
	}
	ins("revoked", "platform", "1 year") // future at
	ins("rolled_back", "owner", "0")
	ins("revoked", "owner", "0")
	var by string
	var future bool
	if err := db.pool.QueryRow(ctx, `SELECT by_role, at > now() FROM resource_status WHERE namespace = 'ns' AND name = 'svc' AND action = 'revoked'`).Scan(&by, &future); err != nil {
		t.Fatal(err)
	}
	if by != "owner" || future {
		t.Fatalf("got %s, future=%t; want owner, not in the future", by, future)
	}
}

// TestMigration009LocksResourceStatus: default privileges that hand a new
// table to the runtime role are revoked by 009 itself, before any startup
// RevokeAuditChange runs.
func TestMigration009LocksResourceStatus(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.migrateTo(ctx, "008_registry_name_ci.sql"); err != nil {
		t.Fatal(err)
	}
	_, _ = db.pool.Exec(ctx, "DROP OWNED BY fleet_def_test")
	_, _ = db.pool.Exec(ctx, "DROP ROLE IF EXISTS fleet_def_test")
	for _, q := range []string{"CREATE ROLE fleet_def_test", "GRANT USAGE ON SCHEMA public TO fleet_def_test", "ALTER DEFAULT PRIVILEGES GRANT ALL ON TABLES TO fleet_def_test"} {
		if _, err := db.pool.Exec(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.pool.Exec(ctx, "DROP OWNED BY fleet_def_test")
		_, _ = db.pool.Exec(ctx, "DROP ROLE fleet_def_test")
	})
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var canWrite bool
	if err := db.pool.QueryRow(ctx, `SELECT has_table_privilege('fleet_def_test', 'resource_status', 'INSERT, UPDATE, DELETE, TRUNCATE, TRIGGER')`).Scan(&canWrite); err != nil || canWrite {
		t.Fatalf("runtime role can write resource_status after 009: %t, %v", canWrite, err)
	}
}

func TestRealPostgresMigration008RefusesCaseCollisions(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.migrateTo(ctx, "007_agentjob_orderer.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `INSERT INTO servers (namespace, name, registry_name, status) VALUES ('a', 'x', 'no.nav.a/x', 'draft'), ('b', 'x', 'No.Nav.A/x', 'draft')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err == nil || !strings.Contains(err.Error(), "no.nav.a/x") {
		t.Fatalf("want a clear refusal naming the name, got %v", err)
	}
}

// An image from before 007 keeps inserting jobs, before and after 007.
func TestMigration007OldImage(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.migrateTo(ctx, "005_v1alpha1.sql"); err != nil {
		t.Fatal(err)
	}
	oldInsert := `INSERT INTO runtime_jobs (team, kind, params, on_expiry, max_attempts, created_by) VALUES ('team-a', 'k', '{}', 'requeue', 3, 'A111111') RETURNING id`
	var before, after int64
	if err := db.pool.QueryRow(ctx, oldInsert).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, oldInsert).Scan(&after); err != nil {
		t.Fatalf("old-image insert after 007: %v", err)
	}
}

// Rows from an image before 012 get {issuer, subject} from BackfillIdentity,
// and a person's job keeps NULL, not ” (#2896).
func TestMigration012BackfillIdentity(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.migrateTo(ctx, "011_drop_legacy.sql"); err != nil {
		t.Fatal(err)
	}
	// The old image's profile insert.
	spec := `{"identity":{"app":"studio-api","cluster":"dev-gcp","entraClientID":"` + appClientID + `"},"mode":"Delegated","grants":[{"serverRef":{"group":"fleet.nav.no","kind":"MCPServer","namespace":"copilot","name":"dolly"},"allowedTools":["hent"],"maxRisk":"Read"}],"quota":{}}`
	if _, err := db.pool.Exec(ctx, `INSERT INTO agent_profile_resources (namespace, name, published_generation, published_client_id) VALUES ('team-a', 'studio', 1, $1::uuid)`, appClientID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `INSERT INTO agent_profile_generations (namespace, name, generation, spec, spec_hash, entra_client_id, submitted_by, submitted_by_kind,
		approved_at, expires_at, identity_verified_at, grant_hashes) VALUES ('team-a', 'studio', 1, $1::jsonb, 'v1:x', $2::uuid, 'A111111', 'User', now(), now() + interval '1 day', now(), '{}')`,
		spec, appClientID); err != nil {
		t.Fatal(err)
	}
	oldJob := `INSERT INTO runtime_jobs (team, kind, created_by, orderer_client_id, orderer_subject) VALUES ('team-a', 'k', 'B444444', $1, $2)`
	if _, err := db.pool.Exec(ctx, oldJob, appClientID, "66666666-6666-4666-8666-666666666666"); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	// Old image after 012: still inserts, new columns stay empty.
	if _, err := db.pool.Exec(ctx, oldJob, "", ""); err != nil {
		t.Fatalf("old-image insert after 012: %v", err)
	}
	if n, err := db.BackfillIdentity(ctx, ""); err != nil || n != 0 {
		t.Fatalf("backfill without issuer: %d %v", n, err)
	}
	if n, err := db.BackfillIdentity(ctx, testIssuer); err != nil || n != 2 {
		t.Fatalf("backfill: %d %v", n, err)
	}
	var iss, sub string
	if err := db.pool.QueryRow(ctx, `SELECT identity_issuer, identity_subject FROM agent_profile_generations`).Scan(&iss, &sub); err != nil || iss != testIssuer || sub != appClientID {
		t.Fatalf("profile identity: %q %q %v", iss, sub, err)
	}
	var withOrderer, persons int
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE orderer_issuer = $1 AND orderer_client = $2 AND orderer_sub <> ''),
		count(*) FILTER (WHERE orderer_issuer IS NULL AND orderer_client IS NULL AND orderer_sub IS NULL) FROM runtime_jobs`, testIssuer, appClientID).Scan(&withOrderer, &persons); err != nil || withOrderer != 1 || persons != 1 {
		t.Fatalf("jobs: delegated %d person %d %v", withOrderer, persons, err)
	}
}
