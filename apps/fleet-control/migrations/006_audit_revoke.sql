-- Only the owner role may change audit_events (#2631). The runtime role (Nais
-- READWRITE_) keeps SELECT and INSERT. A non-owner cannot disable or drop the
-- append-only triggers, so this closes the gap the triggers leave. Existing rows
-- are untouched; REVOKE is idempotent. setupDB checks the runtime role at every
-- start, so a later re-grant stops the app instead of passing silently.
REVOKE UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER ON audit_events FROM PUBLIC;
DO $$
DECLARE r record;
BEGIN
    FOR r IN SELECT DISTINCT grantee FROM information_schema.role_table_grants
             WHERE table_schema = current_schema() AND table_name = 'audit_events'
               AND privilege_type IN ('UPDATE', 'DELETE', 'TRUNCATE', 'REFERENCES', 'TRIGGER')
               AND grantee <> (SELECT tableowner FROM pg_tables
                               WHERE schemaname = current_schema() AND tablename = 'audit_events')
               AND grantee <> 'PUBLIC'
    LOOP
        EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER ON audit_events FROM %I', r.grantee);
    END LOOP;
END $$;
