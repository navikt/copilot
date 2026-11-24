-- AgentJob names are unique per orderer (#2748 PR 3). Additive: an image
-- from before this migration inserts without these columns and gets ''.
-- orderer_client_id is the ordering app's azp; '' is a person via my-copilot.
-- orderer_subject is the person's oid for a delegated job; '' otherwise.
ALTER TABLE runtime_jobs
    ADD COLUMN orderer_client_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN orderer_subject TEXT NOT NULL DEFAULT '',
    ADD COLUMN agent_profile_generation INT; -- bound at creation, NULL for old rows

DROP INDEX runtime_jobs_name;
CREATE UNIQUE INDEX runtime_jobs_name ON runtime_jobs
    (team, orderer_client_id, orderer_subject, name) WHERE name IS NOT NULL;

-- resourceVersion moves on every write, from old images and the runtime too.
CREATE TRIGGER runtime_jobs_resource_version BEFORE UPDATE ON runtime_jobs
    FOR EACH ROW EXECUTE FUNCTION fleet_bump_resource_version();
