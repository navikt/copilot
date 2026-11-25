-- Drops what only the pre-v1alpha1 code read or wrote (#2895). This is the
-- approved exception to "migrations only add": it runs before prod has data,
-- and after it no image from before this migration can run against the
-- database. See README, "Rollback".
DROP TABLE agent_profile_approvals;
DROP TABLE agent_profiles;
-- v1alpha1 reads spec_hash; content_hash was the /admin/v1 draft hash.
ALTER TABLE server_versions DROP COLUMN content_hash;
