package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate applies the embedded migrations once each, under an advisory lock.
func (d *Store) Migrate(ctx context.Context) error { return d.migrateTo(ctx, "") }

// migrateTo applies migrations up to and including last ("" for all), so a
// test can stand where an older image left the schema.
func (d *Store) migrateTo(ctx context.Context, last string) error {
	if _, err := d.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		if last != "" && name > last {
			break
		}
		err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", LockKey); err != nil {
				return err
			}
			var done bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", name).Scan(&done); err != nil || done {
				return err
			}
			sql, err := migrationFS.ReadFile("migrations/" + name)
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, string(sql)); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name)
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

// RevokeAuditChange re-runs 006 so a later re-grant to the runtime role is
// removed before CheckAuditLocked runs. Needs the owner pool.
func (d *Store) RevokeAuditChange(ctx context.Context) error {
	sql, err := migrationFS.ReadFile("migrations/006_audit_revoke.sql")
	if err != nil {
		return err
	}
	if _, err := d.pool.Exec(ctx, string(sql)); err != nil {
		return fmt.Errorf("revoke audit_events rights: %w", err)
	}
	if _, err := d.pool.Exec(ctx, revokeResourceStatus); err != nil {
		return fmt.Errorf("revoke resource_status rights: %w", err)
	}
	return nil
}

// revokeResourceStatus leaves only SELECT on resource_status to non-owners:
// the definer trigger on audit_events is its only writer, and a runtime
// write could forge or clear a platform revocation (009).
const revokeResourceStatus = `REVOKE INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER ON resource_status FROM PUBLIC;
DO $$
DECLARE r record;
BEGIN
    FOR r IN SELECT DISTINCT a.grantee::regrole AS grantee
             FROM pg_class c, aclexplode(c.relacl) a
             WHERE c.oid = 'resource_status'::regclass AND a.grantee <> 0 AND a.grantee <> c.relowner
               AND a.privilege_type <> 'SELECT'
             UNION
             SELECT a.grantee::regrole
             FROM pg_attribute att, aclexplode(att.attacl) a
             WHERE att.attrelid = 'resource_status'::regclass AND a.grantee <> 0
               AND a.privilege_type <> 'SELECT'
    LOOP
        EXECUTE format('REVOKE INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER ON resource_status FROM %s', r.grantee);
    END LOOP;
END $$;`

// CheckAuditLocked fails unless the connected role is not the owner of
// audit_events and holds none of the rights to change it (#2631), nor to
// write resource_status (009).
func (d *Store) CheckAuditLocked(ctx context.Context) error { return checkAuditLocked(ctx, d.pool) }

func checkAuditLocked(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) error {
	var owner, canChange bool
	err := q.QueryRow(ctx, `SELECT pg_has_role(current_user, c.relowner, 'USAGE'),
		has_table_privilege(c.oid, 'UPDATE') OR has_table_privilege(c.oid, 'DELETE')
		OR has_table_privilege(c.oid, 'TRUNCATE') OR has_table_privilege(c.oid, 'TRIGGER')
		OR has_any_column_privilege(c.oid, 'UPDATE')
		OR has_sequence_privilege('audit_events_id_seq', 'UPDATE')
		OR has_table_privilege('resource_status', 'INSERT') OR has_table_privilege('resource_status', 'UPDATE')
		OR has_table_privilege('resource_status', 'DELETE') OR has_table_privilege('resource_status', 'TRUNCATE')
		OR has_table_privilege('resource_status', 'TRIGGER')
		OR has_any_column_privilege('resource_status', 'INSERT') OR has_any_column_privilege('resource_status', 'UPDATE')
		FROM pg_class c WHERE c.oid = 'audit_events'::regclass`).Scan(&owner, &canChange)
	if err != nil {
		return fmt.Errorf("check audit_events rights: %w", err)
	}
	if owner || canChange {
		return fmt.Errorf("runtime role can change audit_events (owner=%t, change rights=%t); refusing to start", owner, canChange)
	}
	return nil
}

// PruneAudit deletes audit rows older than 90 days (#2762). Needs the owner
// pool: the runtime role has no DELETE right, and the trigger refuses
// younger rows and sessions without fleet.audit_retention.
// shortcut: deletes without export; the export target is not decided (#2762).
func (d *Store) PruneAudit(ctx context.Context) (int64, error) {
	var n int64
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET LOCAL fleet.audit_retention = 'on'`); err != nil {
			return err
		}
		// Status reads resource_status, not these rows (009).
		tag, err := tx.Exec(ctx, `DELETE FROM audit_events WHERE at < now() - interval '90 days'`)
		n = tag.RowsAffected()
		return err
	})
	return n, err
}

// BackfillIdentity fills {issuer, subject} on rows written before 012 or by
// an older pod, which writes only entra_client_id and the ” orderer
// sentinels. It runs on every start and changes nothing without an issuer.
func (d *Store) BackfillIdentity(ctx context.Context, issuer string) (int64, error) {
	if issuer == "" {
		return 0, nil
	}
	var n int64
	err := pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE agent_profile_generations SET identity_issuer = $1, identity_subject = entra_client_id::text
			WHERE identity_subject IS NULL AND entra_client_id IS NOT NULL`, issuer)
		if err != nil {
			return err
		}
		n = tag.RowsAffected()
		// A delegated job has both orderer_client_id and orderer_subject; a
		// person via my-copilot has neither and keeps NULL.
		tag, err = tx.Exec(ctx, `UPDATE runtime_jobs SET orderer_issuer = $1, orderer_client = orderer_client_id, orderer_sub = orderer_subject
			WHERE orderer_client IS NULL AND orderer_client_id <> '' AND orderer_subject <> ''`, issuer)
		n += tag.RowsAffected()
		return err
	})
	return n, err
}
