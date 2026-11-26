-- Pending generations expire 14 days after submission (#2731). server_versions
-- already has created_at; agent profile generations get one. Rows that exist
-- now take the migration time, so a draft pending today gets a full window.
ALTER TABLE agent_profile_generations
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
