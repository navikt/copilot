CREATE OR REPLACE VIEW `%s.%s.v_user_credits_summary` AS
SELECT
  day,
  scope_id,
  JSON_VALUE(raw_record, '$.user_id') AS user_id,
  CAST(JSON_VALUE(raw_record, '$.ai_credits_used') AS FLOAT64) AS ai_credits_used,
  CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64) AS generations,
  CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
  CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64) AS interactions,
  CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64) AS lines_accepted,
  JSON_VALUE(raw_record, '$.ai_adoption_phase.phase') AS adoption_phase
FROM {{user_metrics}}
WHERE JSON_VALUE(raw_record, '$.ai_credits_used') IS NOT NULL
ORDER BY day DESC, ai_credits_used DESC;
