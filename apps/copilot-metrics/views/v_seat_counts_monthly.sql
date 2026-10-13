CREATE OR REPLACE VIEW `%s.%s.v_seat_counts_monthly` AS
-- Last seat count per month and scope. History starts at first deploy (no backfill).
SELECT
  DATE_TRUNC(snapshot_date, MONTH) AS month,
  scope,
  scope_id,
  snapshot_date AS last_snapshot_date,
  total_seats,
  active_seats,
  pending_cancellation_seats
FROM {{seat_counts}}
WHERE TRUE
QUALIFY ROW_NUMBER() OVER (PARTITION BY DATE_TRUNC(snapshot_date, MONTH), scope_id ORDER BY snapshot_date DESC) = 1
ORDER BY month DESC, scope_id;
