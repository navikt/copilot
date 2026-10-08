WITH users AS (
  SELECT
    day,
    scope_id,
    JSON_VALUE(raw_record, '$.user_id') AS user_id,
    SAFE_CAST(JSON_VALUE(raw_record, '$.ai_credits_used') AS FLOAT64) * 0.01 AS gross_usd
  FROM `copilot-dev-e17a.copilot_metrics.user_metrics`
  WHERE day >= DATE '2026-09-01' AND day < DATE_ADD(DATE '2026-09-01', INTERVAL 1 MONTH)
    AND scope = 'enterprise' AND scope_id = 'nav'
),
teams AS (
  SELECT DISTINCT
    day,
    scope_id,
    JSON_VALUE(raw_record, '$.user_id') AS user_id,
    JSON_VALUE(raw_record, '$.team_id') AS team_id,
    JSON_VALUE(raw_record, '$.slug') AS team_slug
  FROM `copilot-dev-e17a.copilot_metrics.user_teams`
  WHERE day >= DATE '2026-09-01' AND day < DATE_ADD(DATE '2026-09-01', INTERVAL 1 MONTH)
    AND scope = 'enterprise' AND scope_id = 'nav'
    AND JSON_VALUE(raw_record, '$.slug') != 'nav-it-github-users'
),
memberships AS (
  SELECT day, scope_id, user_id, COUNT(*) AS team_count
  FROM teams
  GROUP BY day, scope_id, user_id
),
allocated AS (
  SELECT
    t.team_id,
    t.team_slug,
    u.user_id,
    IF(t.team_id IS NULL, u.gross_usd, u.gross_usd / m.team_count) AS gross_usd
  FROM users u
  LEFT JOIN memberships m USING (day, scope_id, user_id)
  LEFT JOIN teams t USING (day, scope_id, user_id)
)
SELECT
  COALESCE(team_slug, 'Unassigned') AS team,
  team_id,
  COUNT(DISTINCT user_id) AS users,
  ROUND(SUM(gross_usd), 2) AS allocated_gross_usd,
  ROUND(SUM(SUM(gross_usd)) OVER (), 2) AS all_allocated_gross_usd
FROM allocated
GROUP BY team_id, team_slug
ORDER BY team_id IS NULL, team;
