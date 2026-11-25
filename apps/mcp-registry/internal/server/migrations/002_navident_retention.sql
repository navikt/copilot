-- Actors are NAVidents from now on, never email addresses (#2092).
-- registry_entries.owner_navident keeps ownership after version rows lose
-- submitted_by to the 90-day retention.
ALTER TABLE registry_entries ADD COLUMN IF NOT EXISTS owner_navident TEXT;
ALTER TABLE registry_entry_versions ALTER COLUMN submitted_by DROP NOT NULL;

-- Drop personal data written before this migration. Entries without an owner
-- can only be updated by an admin.
DELETE FROM registry_audit WHERE actor LIKE '%@%';
UPDATE registry_entry_versions SET submitted_by = NULL WHERE submitted_by LIKE '%@%';

CREATE INDEX IF NOT EXISTS idx_registry_audit_at ON registry_audit (at);
