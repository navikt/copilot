-- API v1alpha1 (#2748). Additive only: an image from before this migration
-- keeps working against the new schema. 006 (PR 4) removes what is left over.
CREATE SEQUENCE fleet_resource_version;

ALTER TABLE servers
    ADD COLUMN uid UUID NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN resource_version BIGINT NOT NULL DEFAULT nextval('fleet_resource_version'),
    ADD COLUMN labels JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN annotations JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
-- dataCategory lives only in spec per generation. The published generation applies.

ALTER TABLE server_versions
    ADD COLUMN spec JSONB,                -- v1alpha1 spec, canonical
    ADD COLUMN spec_hash TEXT,            -- new contentHash; content_hash stays
    ADD COLUMN submitted_by_kind TEXT CHECK (submitted_by_kind IN ('User', 'Application', 'System')),
    ADD COLUMN submitted_by_oid UUID,
    ADD COLUMN approved_by_oid UUID,
    ADD COLUMN revoked_at TIMESTAMPTZ,    -- this generation was revoked
    -- Approved after 005 needs an oid for both and two different people.
    -- Rows approved before 005 have approved_by_oid NULL and pass.
    ADD CONSTRAINT server_versions_four_eyes_oid
        CHECK (approved_by_oid IS NULL
               OR (submitted_by_oid IS NOT NULL AND approved_by_oid <> submitted_by_oid));

-- Generations revoked or blocked before 005 are marked per row, so rollback
-- refuses them per generation.
UPDATE server_versions v SET revoked_at = s.updated_at
FROM servers s
WHERE s.namespace = v.namespace AND s.name = v.name AND s.status = 'revoked' AND s.published_version = v.version;
UPDATE server_versions SET revoked_at = COALESCE(revoked_at, created_at) WHERE blocked;

-- resource_version moves on every write, from old images too.
CREATE FUNCTION fleet_bump_resource_version() RETURNS trigger AS $$
BEGIN
    NEW.resource_version := nextval('fleet_resource_version');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER servers_resource_version BEFORE UPDATE ON servers
    FOR EACH ROW EXECUTE FUNCTION fleet_bump_resource_version();

-- Agent profiles get the same model as servers. The old table is frozen.
CREATE TABLE agent_profile_resources (
    namespace TEXT NOT NULL CHECK (namespace ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    name TEXT NOT NULL CHECK (name ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    uid UUID NOT NULL DEFAULT gen_random_uuid(),
    resource_version BIGINT NOT NULL DEFAULT nextval('fleet_resource_version'),
    labels JSONB NOT NULL DEFAULT '{}',
    annotations JSONB NOT NULL DEFAULT '{}',
    published_generation INT,
    pending_generation INT,
    revoked BOOLEAN NOT NULL DEFAULT false,
    -- Set on publish, cleared on revocation and expiry. One live profile per client id.
    published_client_id UUID UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (namespace, name)
);

CREATE TRIGGER agent_profile_resources_resource_version BEFORE UPDATE ON agent_profile_resources
    FOR EACH ROW EXECUTE FUNCTION fleet_bump_resource_version();

CREATE TABLE agent_profile_generations (
    namespace TEXT NOT NULL,
    name TEXT NOT NULL,
    generation INT NOT NULL CHECK (generation > 0),
    spec JSONB NOT NULL,
    spec_hash TEXT NOT NULL,
    entra_client_id UUID NOT NULL,
    submitted_by TEXT NOT NULL,
    submitted_by_kind TEXT NOT NULL CHECK (submitted_by_kind IN ('User', 'Application', 'System')),
    submitted_by_oid UUID,
    approved_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    identity_verified_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    blocked BOOLEAN NOT NULL DEFAULT false,
    -- Hash per grant, computed at submission: {"<ns>/<name>": "v1:sha256:..."}.
    -- approval compares against this before a row exists in agent_profile_grants.
    grant_hashes JSONB NOT NULL,
    PRIMARY KEY (namespace, name, generation),
    FOREIGN KEY (namespace, name) REFERENCES agent_profile_resources (namespace, name)
);

CREATE TABLE agent_profile_grants (
    namespace TEXT NOT NULL,
    name TEXT NOT NULL,
    generation INT NOT NULL,
    server TEXT NOT NULL,             -- <namespace>/<name>
    content_hash TEXT NOT NULL,       -- copy of grant_hashes at approval
    granted_by TEXT NOT NULL,
    granted_by_kind TEXT NOT NULL DEFAULT 'User' CHECK (granted_by_kind IN ('User', 'Application', 'System')),
    granted_by_oid UUID,              -- NULL only for rows copied from 003; the code requires an oid for new ones
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (namespace, name, generation, server),
    FOREIGN KEY (namespace, name, generation) REFERENCES agent_profile_generations
);

-- Copy live profiles from 003. Copied hashes use the prefix v0:, which no
-- v1alpha1 submission produces, so a copied draft can never be approved
-- through v1alpha1: a person submits it again and gets an oid and a v1 hash.
INSERT INTO agent_profile_resources (namespace, name, published_generation, pending_generation, published_client_id, created_at)
SELECT namespace, name,
       CASE WHEN status = 'approved' THEN 1 END,
       CASE WHEN status = 'draft' THEN 1 END,
       CASE WHEN status = 'approved' AND expires_at > now() AND identity_verified_at IS NOT NULL THEN entra_client_id END,
       created_at
FROM agent_profiles WHERE status IN ('draft', 'approved');

INSERT INTO agent_profile_generations (namespace, name, generation, spec, spec_hash, entra_client_id, submitted_by, submitted_by_kind,
                                       approved_at, expires_at, identity_verified_at, grant_hashes)
SELECT p.namespace, p.name, 1, spec.v,
       'v0:sha256:' || encode(sha256(convert_to(spec.v::text, 'UTF8')), 'hex'),
       p.entra_client_id, p.created_by,
       CASE WHEN p.created_by = 'seed' THEN 'System' ELSE 'User' END,
       p.approved_at, p.expires_at, p.identity_verified_at,
       (SELECT jsonb_object_agg(g->>'route', 'v0:sha256:' || encode(sha256(convert_to(g::text, 'UTF8')), 'hex'))
          FROM jsonb_array_elements(p.grants) g)
FROM agent_profiles p
CROSS JOIN LATERAL (SELECT jsonb_build_object(
    'identity', jsonb_build_object('app', p.app, 'cluster', p.cluster, 'entraClientID', p.entra_client_id::text),
    'mode', 'Delegated',
    'grants', (SELECT jsonb_agg(jsonb_build_object(
                   'serverRef', jsonb_build_object('group', 'fleet.nav.no', 'kind', 'MCPServer',
                                                   'namespace', split_part(g->>'route', '/', 1), 'name', split_part(g->>'route', '/', 2)),
                   'allowedTools', g->'tools',
                   'maxRisk', 'Read') ORDER BY g->>'route')
               FROM jsonb_array_elements(p.grants) g),
    'quota', jsonb_build_object('maxInFlight', p.max_in_flight, 'requestsPerMinute', p.rpm,
                                'dailyCallsPerUser', p.daily_calls_per_user, 'dailyCalls', p.daily_calls),
    'dpia', jsonb_build_object('owner', p.dpia_owner, 'reference', p.dpia_ref)) AS v) spec
WHERE p.status IN ('draft', 'approved');

INSERT INTO agent_profile_grants (namespace, name, generation, server, content_hash, granted_by, granted_at)
SELECT p.namespace, p.name, 1, a.route, g.grant_hashes->>a.route, a.approved_by, a.approved_at
FROM agent_profile_approvals a
JOIN agent_profiles p ON p.id = a.profile_id AND p.status IN ('draft', 'approved')
JOIN agent_profile_generations g ON g.namespace = p.namespace AND g.name = p.name AND g.generation = 1
WHERE g.grant_hashes ? a.route;

ALTER TABLE runtime_jobs
    ADD COLUMN resource_version BIGINT NOT NULL DEFAULT nextval('fleet_resource_version'),
    ADD COLUMN created_by_kind TEXT CHECK (created_by_kind IN ('User', 'Application', 'System')),
    ADD COLUMN created_by_client_id TEXT,
    ADD COLUMN created_by_oid UUID,
    ADD COLUMN name TEXT,             -- nullable until 006: old code does not set it
    ADD COLUMN submitted_spec JSONB;  -- spec as submitted, for retries
-- Partial index, so rows without a name from old code do not collide.
CREATE UNIQUE INDEX runtime_jobs_name ON runtime_jobs (team, name) WHERE name IS NOT NULL;

ALTER TABLE audit_events
    ADD COLUMN actor_kind TEXT CHECK (actor_kind IN ('User', 'Application', 'System')),
    ADD COLUMN actor_client_id TEXT,
    ADD COLUMN actor_oid UUID,
    ADD COLUMN resource_kind TEXT,
    ADD COLUMN generation INT;
