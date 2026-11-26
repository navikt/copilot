-- Owner decision 2026-11-26 (#2634): the platform team may approve its own
-- MCPServer generations in namespace copilot. The four-eyes checks still hold
-- for every row that is not marked self_approved, and only namespace copilot
-- may mark one. Expand only: older images write the default false.
ALTER TABLE server_versions
    ADD COLUMN self_approved BOOLEAN NOT NULL DEFAULT false,
    DROP CONSTRAINT server_versions_four_eyes,
    DROP CONSTRAINT server_versions_four_eyes_oid,
    ADD CONSTRAINT server_versions_four_eyes
        CHECK (self_approved OR approved_by IS NULL OR approved_by <> submitted_by),
    ADD CONSTRAINT server_versions_four_eyes_oid
        CHECK (self_approved OR approved_by_oid IS NULL
               OR (submitted_by_oid IS NOT NULL AND approved_by_oid <> submitted_by_oid)),
    ADD CONSTRAINT server_versions_self_approved_platform
        CHECK (NOT self_approved OR namespace = 'copilot');
