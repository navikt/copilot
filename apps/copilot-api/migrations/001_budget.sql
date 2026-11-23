-- Personal budget and raises (PRD kostnadsinnsikt/prd-budsjett-og-paafyll.md, chapter 5).
-- Identity is the GitHub login from the auth middleware; NAVident is never stored.

CREATE TABLE budget_profiles (
    github_login text PRIMARY KEY,
    team_id      text NOT NULL,
    role         text NOT NULL CHECK (role IN ('utvikler', 'designer', 'produkteier', 'leder', 'analytiker', 'annet')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE budget_raises (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    github_login    text NOT NULL,
    idempotency_key text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    month           date NOT NULL,
    amount_usd      integer NOT NULL,
    budget_set_usd  integer, -- absolute budget this raise sends to GitHub
    category        text NOT NULL CHECK (category IN ('koding', 'feilsoking', 'testing', 'dokumentasjon', 'analyse_data', 'laering', 'annet')),
    team_id         text NOT NULL,
    expected_value  text NOT NULL CHECK (expected_value IN ('spare_tid', 'levere_raskere', 'bedre_kvalitet', 'laere', 'utforske', 'annet')),
    free_text       text CHECK (char_length(free_text) <= 500),
    status          text NOT NULL CHECK (status IN ('pending', 'applied', 'failed', 'expired', 'dry_run')),
    admin_override  boolean NOT NULL DEFAULT false,
    UNIQUE (github_login, idempotency_key)
);
CREATE INDEX budget_raises_login_month ON budget_raises (github_login, month);

-- Budgets this service created in GitHub. github_budget_id is the only way we
-- recognise our own budgets; anything else is never patched or deleted.
CREATE TABLE budget_github (
    github_login     text NOT NULL,
    month            date NOT NULL,
    github_budget_id text NOT NULL UNIQUE,
    amount_usd       integer NOT NULL,
    expires_at       date NOT NULL,
    deleted_at       timestamptz,
    PRIMARY KEY (github_login, month)
);

-- One row per GitHub call (or dry-run). No free text here.
CREATE TABLE budget_audit (
    id                bigserial PRIMARY KEY,
    raise_id          uuid REFERENCES budget_raises (id) ON DELETE SET NULL,
    actor             text NOT NULL,
    at                timestamptz NOT NULL DEFAULT now(),
    action            text NOT NULL CHECK (action IN ('raise', 'retry', 'admin_override', 'expire_delete', 'retention')),
    budget_before_usd integer,
    budget_set_usd    integer,
    expires_at        date,
    github_status     integer,
    github_budget_id  text,
    dry_run           boolean NOT NULL
);
