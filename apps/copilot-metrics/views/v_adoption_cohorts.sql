CREATE OR REPLACE VIEW `%s.%s.v_adoption_cohorts` AS
SELECT
  day,
  scope_id,
  SAFE_CAST(REGEXP_EXTRACT(JSON_VALUE(raw_record, '$.ai_adoption_phase.phase'), r'\d+') AS INT64) AS phase,
  JSON_VALUE(raw_record, '$.ai_adoption_phase.version') AS phase_version,
  COUNT(DISTINCT JSON_VALUE(raw_record, '$.user_id')) AS user_count,
  AVG(CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64)) AS avg_generations,
  AVG(CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64)) AS avg_acceptances,
  AVG(CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64)) AS avg_interactions,
  AVG(CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64)) AS avg_lines_added
FROM {{user_metrics}}
WHERE JSON_VALUE(raw_record, '$.ai_adoption_phase.phase') IS NOT NULL
GROUP BY day, scope_id, phase, phase_version
HAVING phase IS NOT NULL
ORDER BY day DESC, phase;
