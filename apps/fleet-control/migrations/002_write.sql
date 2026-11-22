-- Write API (#2625, #2626). A server created through the API has no
-- published version until a second member of its team approves one.
ALTER TABLE servers ALTER COLUMN published_version DROP NOT NULL;
ALTER TABLE servers DROP CONSTRAINT servers_status_check;
ALTER TABLE servers ADD CONSTRAINT servers_status_check CHECK (status IN ('draft', 'published', 'revoked'));
ALTER TABLE servers ADD CONSTRAINT servers_published_status CHECK ((published_version IS NULL) = (status = 'draft'));

-- content_hash is what an approver reviewed. published_at marks every version
-- that was ever published, so rollback can find the previous one.
ALTER TABLE server_versions
    ADD COLUMN content_hash TEXT,
    ADD COLUMN approved_by TEXT,
    ADD COLUMN approved_at TIMESTAMPTZ,
    ADD COLUMN published_at TIMESTAMPTZ,
    ADD CONSTRAINT server_versions_four_eyes CHECK (approved_by IS NULL OR approved_by <> submitted_by);

UPDATE server_versions v SET published_at = v.created_at
FROM servers s
WHERE s.namespace = v.namespace AND s.name = v.name AND s.published_version = v.version;
