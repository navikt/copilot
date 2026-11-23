-- Pull-based job API for fleet-runtime (#2689). fleet-runtime has no inbound
-- surface: it claims a job, holds a lease by heartbeat, and reads cancel and
-- kill from the heartbeat response.
CREATE TABLE runtime_jobs (
    id BIGSERIAL PRIMARY KEY,
    team TEXT NOT NULL,
    kind TEXT NOT NULL,
    params JSONB NOT NULL DEFAULT '{}',
    state TEXT NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'leased', 'succeeded', 'failed', 'cancelled', 'killed')),
    -- What an expired lease does: requeue (until max_attempts) or fail.
    on_expiry TEXT NOT NULL DEFAULT 'requeue' CHECK (on_expiry IN ('requeue', 'fail')),
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3 CHECK (max_attempts BETWEEN 1 AND 10),
    -- Delivered on the next heartbeat.
    command TEXT NOT NULL DEFAULT 'continue' CHECK (command IN ('continue', 'cancel', 'kill')),
    result JSONB,
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX runtime_jobs_queued ON runtime_jobs (id) WHERE state = 'queued';

CREATE TABLE runtime_leases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id BIGINT NOT NULL REFERENCES runtime_jobs (id),
    runtime TEXT NOT NULL, -- the caller's azp
    claim_token TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- A retried claim with the same token gets the same lease.
    UNIQUE (runtime, claim_token)
);
CREATE UNIQUE INDEX runtime_leases_one_active ON runtime_leases (job_id) WHERE ended_at IS NULL;

-- Kill switches: scope is 'global' (operator) or 'team:<team>'. A row means on.
CREATE TABLE runtime_kill_switches (
    scope TEXT PRIMARY KEY CHECK (scope = 'global' OR scope LIKE 'team:%'),
    reason TEXT NOT NULL,
    set_by TEXT NOT NULL,
    set_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
