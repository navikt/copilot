package api

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

const oidA = "00000000-0000-4000-8000-00000000000a"

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

// ageAudit moves every audit row past the retention limit, as the owner.
func ageAudit(t *testing.T, db *DB) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{
		`ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_change`,
		`UPDATE audit_events SET at = at - interval '91 days'`,
		`ALTER TABLE audit_events ENABLE TRIGGER audit_events_no_change`,
	} {
		if _, err := db.pool.Exec(ctx, q); err != nil {
			t.Fatal(q, err)
		}
	}
	if _, err := db.PruneAudit(ctx); err != nil {
		t.Fatal(err)
	}
}
