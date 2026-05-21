CREATE OR REPLACE VIEW `%s.%s.v_team_daily_summary` AS
WITH user_teams AS (
  SELECT
    day,
    scope_id,
    JSON_VALUE(raw_record, '$.user_id') AS user_id,
    JSON_VALUE(raw_record, '$.user_login') AS user_login,
    JSON_VALUE(raw_record, '$.team_id') AS team_id,
    JSON_VALUE(raw_record, '$.slug') AS team_slug,
    COALESCE(
      JSON_VALUE(raw_record, '$.organization_id'),
      JSON_VALUE(raw_record, '$.enterprise_id')
    ) AS entity_id
  FROM {{user_teams}}
),
user_metrics AS (
  SELECT
    day,
    scope_id,
    JSON_VALUE(raw_record, '$.user_id') AS user_id,
    CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64) AS generations,
    CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
    CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64) AS interactions,
    CAST(JSON_VALUE(raw_record, '$.loc_suggested_to_add_sum') AS INT64) AS lines_suggested,
    CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64) AS lines_accepted
  FROM {{user_metrics}}
)
SELECT
  ut.day,
  ut.team_id,
  ut.team_slug,
  ut.entity_id,
  ut.scope_id,
  COUNT(DISTINCT ut.user_id) AS total_users,
  COUNTIF(COALESCE(um.acceptances, 0) + COALESCE(um.interactions, 0) > 0) AS active_users,
  SUM(COALESCE(um.generations, 0)) AS total_generations,
  SUM(COALESCE(um.acceptances, 0)) AS total_acceptances,
  SUM(COALESCE(um.interactions, 0)) AS total_interactions,
  SUM(COALESCE(um.lines_suggested, 0)) AS total_lines_suggested,
  SUM(COALESCE(um.lines_accepted, 0)) AS total_lines_accepted
FROM user_teams ut
LEFT JOIN user_metrics um
  ON ut.user_id = um.user_id
  AND ut.day = um.day
  AND ut.scope_id = um.scope_id
GROUP BY ut.day, ut.team_id, ut.team_slug, ut.entity_id, ut.scope_id
ORDER BY ut.day DESC, active_users DESC;
