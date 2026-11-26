-- Owner decision 2026-11-26 (#2634, #2752): any team submits, only the
-- platform team approves, in every namespace, and its member may approve
-- what they submitted. The database cannot check team membership, so the
-- API enforces who approves. Expand only: older images keep writing rows
-- that pass these checks (their four-eyes code and the defaults below).
--
-- approved_via records how a generation was published: 'platform' (approved
-- by the platform team), 'auto' (an owner-team change that adds no access),
-- 'seed' (imported from git). NULL is a generation approved before this
-- migration by two members of the owner team; rollback refuses it.
ALTER TABLE server_versions
    ADD COLUMN self_approved BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN approved_via TEXT CHECK (approved_via IN ('platform', 'auto', 'seed')),
    DROP CONSTRAINT server_versions_four_eyes,
    DROP CONSTRAINT server_versions_four_eyes_oid,
    -- The self_approved mark matches the people on the row.
    ADD CONSTRAINT server_versions_self_approved_consistent
        CHECK (approved_by_oid IS NULL
               OR self_approved = (approved_by_oid = submitted_by_oid OR approved_by = submitted_by)),
    ADD CONSTRAINT server_versions_auto_has_no_approver
        CHECK (approved_via IS DISTINCT FROM 'auto' OR (approved_by = 'auto' AND approved_by_oid IS NULL AND NOT self_approved));

UPDATE server_versions SET approved_via = 'seed'
WHERE approved_by IS NULL AND published_at IS NOT NULL;
