-- Only the owner role may change audit_events (#2631). The runtime role (Nais
-- READWRITE_) keeps SELECT and INSERT, and USAGE on the id sequence. A non-owner
-- cannot disable or drop the append-only triggers, so this closes the gap the
-- triggers leave. Existing rows are untouched; REVOKE is idempotent. setupDB
-- checks the runtime role at every start, so a later re-grant stops the app
-- instead of passing silently.
REVOKE UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER ON audit_events FROM PUBLIC;
REVOKE UPDATE ON SEQUENCE audit_events_id_seq FROM PUBLIC;
DO $$
DECLARE r record;
BEGIN
    -- pg_class.relacl holds table and column-independent grants for every
    -- grantee, including group roles such as Nais app_readwrite.
    FOR r IN SELECT DISTINCT c.relname, c.relkind, a.grantee::regrole AS grantee
             FROM pg_class c, aclexplode(c.relacl) a
             WHERE c.oid IN ('audit_events'::regclass, 'audit_events_id_seq'::regclass)
               AND a.grantee <> 0 AND a.grantee <> c.relowner
               AND a.privilege_type IN ('UPDATE', 'DELETE', 'TRUNCATE', 'REFERENCES', 'TRIGGER')
    LOOP
        IF r.relkind = 'S' THEN
            EXECUTE format('REVOKE UPDATE ON SEQUENCE %I FROM %s', r.relname, r.grantee);
        ELSE
            EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER ON %I FROM %s', r.relname, r.grantee);
        END IF;
    END LOOP;
    -- Column-level UPDATE grants live in pg_attribute.attacl.
    FOR r IN SELECT DISTINCT att.attname, a.grantee::regrole AS grantee
             FROM pg_attribute att, aclexplode(att.attacl) a
             WHERE att.attrelid = 'audit_events'::regclass AND a.privilege_type IN ('UPDATE', 'REFERENCES')
    LOOP
        EXECUTE format('REVOKE UPDATE, REFERENCES (%I) ON audit_events FROM %s',
                       r.attname, CASE WHEN r.grantee::oid = 0 THEN 'PUBLIC' ELSE r.grantee::text END);
    END LOOP;
END $$;
