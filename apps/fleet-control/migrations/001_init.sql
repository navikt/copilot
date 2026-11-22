-- One row per MCP server, keyed by Nais namespace and name (#2334).
-- published_version is what snapshots serve; pending_version is a draft or a
-- version awaiting approval and never touches the published one.
CREATE TABLE servers (
    namespace TEXT NOT NULL CHECK (namespace ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    name TEXT NOT NULL CHECK (name ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    registry_name TEXT UNIQUE,
    published_version INT NOT NULL,
    pending_version INT,
    status TEXT NOT NULL CHECK (status IN ('published', 'revoked')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (namespace, name)
);

-- Every version is kept. registry is the public server.json entry; the route
-- columns are set only for servers behind fleet-gateway.
CREATE TABLE server_versions (
    namespace TEXT NOT NULL,
    name TEXT NOT NULL,
    version INT NOT NULL CHECK (version > 0),
    registry JSONB,
    route_path TEXT,
    target TEXT,
    upstream_url TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    tools TEXT[],
    submitted_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (namespace, name, version),
    FOREIGN KEY (namespace, name) REFERENCES servers (namespace, name),
    CHECK (registry IS NOT NULL OR route_path IS NOT NULL),
    CHECK ((route_path IS NULL) = (target IS NULL)
       AND (route_path IS NULL) = (upstream_url IS NULL)
       AND (route_path IS NULL) = (tools IS NULL)),
    CHECK (tools IS NULL OR cardinality(tools) > 0)
);

-- Append-only audit trail.
CREATE TABLE audit_events (
    id BIGSERIAL PRIMARY KEY,
    at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    namespace TEXT,
    name TEXT,
    version INT,
    detail JSONB
);

CREATE FUNCTION audit_events_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_events_no_change BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_append_only();

CREATE TRIGGER audit_events_no_truncate BEFORE TRUNCATE ON audit_events
    FOR EACH STATEMENT EXECUTE FUNCTION audit_events_append_only();

-- shortcut: the app role owns this table and can still disable or drop the
-- triggers. Real protection needs a separate owner role or REVOKE (open Nais question, #2334).

-- Published snapshots per kind. generation only grows; consumers accept only
-- a higher generation than the one they hold.
CREATE TABLE snapshots (
    kind TEXT NOT NULL CHECK (kind IN ('routes', 'registry')),
    generation BIGINT NOT NULL CHECK (generation > 0),
    etag TEXT NOT NULL,
    body JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, generation)
);
