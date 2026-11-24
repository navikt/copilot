package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

const (
	oidA  = "00000000-0000-4000-8000-00000000000a"
	oidB1 = "00000000-0000-4000-8000-0000000000b1"
	oidB2 = "00000000-0000-4000-8000-0000000000b2"
)

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
	if _, err := db.pool.Exec(ctx, `INSERT INTO audit_events (actor, action, at) VALUES ('A1', 'old', now() - interval '91 days'), ('A1', 'new', now() - interval '89 days')`); err != nil {
		t.Fatal(err)
	}
	// Without the session setting even the owner cannot delete.
	if _, err := db.pool.Exec(ctx, `DELETE FROM audit_events WHERE action = 'old'`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("delete without retention setting: %v", err)
	}
	// With the setting, a row younger than 90 days is still refused.
	err := pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL fleet.audit_retention = 'on'`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM audit_events WHERE action = 'new'`)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("delete of young row: %v", err)
	}
	n, err := db.PruneAudit(ctx)
	if err != nil || n != 1 {
		t.Fatalf("PruneAudit: n=%d err=%v", n, err)
	}
	var left string
	if err := db.pool.QueryRow(ctx, `SELECT string_agg(action, ',') FROM audit_events`).Scan(&left); err != nil || left != "new" {
		t.Fatalf("left %q, %v", left, err)
	}
}

// TestJobAuditWritesOid: a job action from /apis stores the oid next to the
// NAVident (#2763).
func TestJobAuditWritesOid(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	err := pgx.BeginFunc(ctx, db.pool, func(tx pgx.Tx) error {
		return jobAudit(ctx, tx, caller{Ident: "A111111", Kind: "User", Oid: oidA, ClientID: "app"}, "job_created", "team", 1, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	var a, o, k, c string
	if err := db.pool.QueryRow(ctx, `SELECT actor, actor_oid::text, actor_kind, actor_client_id FROM audit_events`).Scan(&a, &o, &k, &c); err != nil {
		t.Fatal(err)
	}
	if a != "A111111" || o != oidA || k != "User" || c != "app" {
		t.Fatalf("got %s %s %s %s", a, o, k, c)
	}
}
