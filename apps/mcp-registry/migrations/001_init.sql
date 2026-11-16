CREATE TABLE IF NOT EXISTS registry_entries (
    name TEXT PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    entry JSONB NOT NULL,
    version INT NOT NULL,
    pending_version INT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS registry_entry_versions (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    version INT NOT NULL,
    entry JSONB NOT NULL,
    submitted_by TEXT NOT NULL,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name, version)
);

CREATE TABLE IF NOT EXISTS registry_audit (
    id BIGSERIAL PRIMARY KEY,
    at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    name TEXT NOT NULL,
    version INT,
    reason TEXT
);

CREATE INDEX IF NOT EXISTS idx_registry_audit_actor_action_at
    ON registry_audit (actor, action, at);
