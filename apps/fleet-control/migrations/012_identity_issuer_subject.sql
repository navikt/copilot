-- Generic identities {issuer, subject} (#2896), expand step. Adds only:
-- images from before this migration keep writing entra_client_id and the
-- '' sentinels, and BackfillIdentity fills the new columns on each start.
-- A later migration may drop the old columns once no image reads them.

-- The agent's workload identity. Entra: issuer is the tenant's issuer and
-- subject the client id. entra_client_id becomes optional, so a non-Entra
-- identity needs no placeholder.
ALTER TABLE agent_profile_generations
    ADD COLUMN identity_issuer TEXT,
    ADD COLUMN identity_subject TEXT,
    ALTER COLUMN entra_client_id DROP NOT NULL,
    ADD CONSTRAINT agent_profile_generations_identity_pair
        CHECK ((identity_issuer IS NULL) = (identity_subject IS NULL)),
    -- No row may lose its identity: NULL entra_client_id needs the new pair.
    ADD CONSTRAINT agent_profile_generations_identity_present
        CHECK (entra_client_id IS NOT NULL OR identity_subject IS NOT NULL);

-- The orderer of a delegated job: issuer, client (azp) and subject
-- (oid). All NULL for a person via my-copilot, instead of ''.
ALTER TABLE runtime_jobs
    ADD COLUMN orderer_issuer TEXT,
    ADD COLUMN orderer_client TEXT,
    ADD COLUMN orderer_sub TEXT,
    ADD CONSTRAINT runtime_jobs_orderer_complete
        CHECK ((orderer_issuer IS NULL) = (orderer_client IS NULL) AND (orderer_client IS NULL) = (orderer_sub IS NULL));
