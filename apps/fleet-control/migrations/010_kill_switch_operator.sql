-- A team switch the operator team set is cleared only by the operator team
-- (#2854). Additive: an image from before 010 inserts with the default.
ALTER TABLE runtime_kill_switches ADD COLUMN set_by_operator BOOLEAN NOT NULL DEFAULT false;
