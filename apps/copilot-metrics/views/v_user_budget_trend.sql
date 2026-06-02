CREATE OR REPLACE VIEW `%s.%s.v_user_budget_trend` AS
SELECT
  snapshot_date,
  scope_id,
  COUNT(*) AS total_users,
  COUNTIF(is_override) AS override_users,
  COUNTIF(NOT is_override) AS standard_users,
  COUNTIF(consumed_amount > 0) AS users_with_consumption,
  ROUND(SUM(consumed_amount), 2) AS total_consumed_usd,
  ROUND(AVG(consumed_amount), 2) AS avg_consumed_usd,
  ROUND(MAX(consumed_amount), 2) AS max_consumed_usd,
  COUNTIF(consumed_amount >= budget_amount * 0.75) AS users_above_75pct,
  COUNTIF(consumed_amount >= budget_amount * 0.90) AS users_above_90pct,
  COUNTIF(consumed_amount >= budget_amount) AS users_at_limit
FROM {{user_budget_snapshots}}
GROUP BY 1, 2
ORDER BY snapshot_date DESC, scope_id;
