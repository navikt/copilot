-- #2763: every action keeps NAVident (actor) and Entra oid (actor_oid) in
-- separate columns. Rows written before this by job and agent-profile paths
-- have only the NAVident; fill the oid where another table pairs that
-- NAVident with exactly one oid. Ambiguous or unknown names stay NULL.
CREATE TEMP TABLE ident_oid ON COMMIT DROP AS
SELECT ident, min(oid::text)::uuid AS oid FROM (
    SELECT submitted_by AS ident, submitted_by_oid AS oid FROM server_versions
    UNION ALL SELECT approved_by, approved_by_oid FROM server_versions
    UNION ALL SELECT submitted_by, submitted_by_oid FROM agent_profile_generations
    UNION ALL SELECT granted_by, granted_by_oid FROM agent_profile_grants
    UNION ALL SELECT created_by, created_by_oid FROM runtime_jobs
    UNION ALL SELECT actor, actor_oid FROM audit_events
) p WHERE ident IS NOT NULL AND oid IS NOT NULL
GROUP BY ident HAVING count(DISTINCT oid) = 1;

-- Runs as the owner inside the migration transaction; the append-only
-- trigger is back on before commit.
ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_change;
UPDATE audit_events a SET actor_oid = m.oid
FROM ident_oid m WHERE a.actor_oid IS NULL AND a.actor = m.ident;
UPDATE audit_events SET actor_kind = 'System' WHERE actor_kind IS NULL AND actor = 'seed';
ALTER TABLE audit_events ENABLE TRIGGER audit_events_no_change;

UPDATE runtime_jobs j SET created_by_oid = m.oid
FROM ident_oid m WHERE j.created_by_oid IS NULL AND j.created_by = m.ident;

-- #2762: audit rows are kept 90 days. The trigger lets a DELETE through only
-- for rows past that age and only in a session that set
-- fleet.audit_retention = 'on'. The runtime role has no DELETE right (006),
-- so in practice only the owner role can run retention.
CREATE OR REPLACE FUNCTION audit_events_append_only() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' AND current_setting('fleet.audit_retention', true) = 'on'
       AND OLD.at < now() - interval '90 days' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'audit_events is append-only';
END;
$$ LANGUAGE plpgsql;
