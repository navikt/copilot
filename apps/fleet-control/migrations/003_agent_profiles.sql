-- Agent profiles (#2582): which caller app may use which tools on which
-- routes, on behalf of a signed-in user. Phase 1 is delegated only; there is
-- no agent principal. Nothing here is enforced by fleet-gateway yet.
CREATE TABLE agent_profiles (
    id BIGSERIAL PRIMARY KEY,
    -- Owner team: the Nais namespace that owns the profile and the agent.
    namespace TEXT NOT NULL CHECK (namespace ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    name TEXT NOT NULL CHECK (name ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    -- The caller app. It lives in the owner team's namespace.
    cluster TEXT NOT NULL CHECK (cluster ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    app TEXT NOT NULL CHECK (app ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    entra_client_id UUID NOT NULL,
    -- Reserved for the Nais STS workload identity; unset until that issuer is chosen.
    sts_subject TEXT CHECK (sts_subject IS NULL),
    mode TEXT NOT NULL CHECK (mode = 'delegated'),
    -- [{"route": "<ns>/<name>", "tools": [...], "max_risk": "read"}], validated by the API.
    grants JSONB NOT NULL CHECK (jsonb_typeof(grants) = 'array' AND jsonb_array_length(grants) > 0),
    max_in_flight INT NOT NULL CHECK (max_in_flight BETWEEN 1 AND 64),
    rpm INT NOT NULL CHECK (rpm BETWEEN 1 AND 6000),
    daily_calls_per_user INT NOT NULL CHECK (daily_calls_per_user BETWEEN 1 AND 100000),
    daily_calls INT NOT NULL CHECK (daily_calls BETWEEN 1 AND 1000000),
    dpia_owner TEXT NOT NULL CHECK (dpia_owner ~ '^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$'),
    dpia_ref TEXT NOT NULL CHECK (length(dpia_ref) BETWEEN 1 AND 300),
    -- Set only when a verifier has matched entra_client_id to cluster/namespace/app.
    -- Unverified profiles never reach the gateway.
    identity_verified_at TIMESTAMPTZ,
    -- expired is set lazily when a newer profile needs the name or client id.
    status TEXT NOT NULL CHECK (status IN ('draft', 'approved', 'expired', 'revoked')),
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    approved_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    revoked_by TEXT,
    revoked_at TIMESTAMPTZ,
    CHECK (status <> 'approved' OR (approved_at IS NOT NULL AND expires_at IS NOT NULL)),
    CHECK (status <> 'revoked' OR (revoked_by IS NOT NULL AND revoked_at IS NOT NULL))
);

-- One live profile per name. Revoked and expired ones can be replaced.
CREATE UNIQUE INDEX agent_profiles_live ON agent_profiles (namespace, name) WHERE status IN ('draft', 'approved');
-- One verified live profile per client id. Unverified drafts reserve nothing,
-- so a team cannot squat on another team's client id.
CREATE UNIQUE INDEX agent_profiles_live_client ON agent_profiles (entra_client_id)
    WHERE status IN ('draft', 'approved') AND identity_verified_at IS NOT NULL;

-- One approval per granted route, by a person from the route's owning team.
CREATE TABLE agent_profile_approvals (
    profile_id BIGINT NOT NULL REFERENCES agent_profiles (id),
    route TEXT NOT NULL,
    approved_by TEXT NOT NULL,
    approved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (profile_id, route)
);
