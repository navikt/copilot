-- #2715 follow-up: let the schema hold the invariants the advisory lock keeps.

-- At most one raise in flight per user, whatever the locking does.
CREATE UNIQUE INDEX budget_raises_one_pending ON budget_raises (github_login) WHERE status = 'pending';

-- Per-user version, bumped by every locked write transaction. Replaces the
-- count of raise rows, which stops working once retention deletes rows.
CREATE TABLE budget_user_versions (
    github_login text PRIMARY KEY,
    version      bigint NOT NULL
);
