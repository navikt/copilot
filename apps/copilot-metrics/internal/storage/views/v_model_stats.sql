CREATE OR REPLACE VIEW `%s.%s.v_model_stats` AS
SELECT
  day,
  JSON_VALUE(mf, '$.model') AS model_name,
  JSON_VALUE(mf, '$.feature') AS feature,
  CAST(JSON_VALUE(mf, '$.code_generation_activity_count') AS INT64) AS generations,
  CAST(JSON_VALUE(mf, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
  CAST(JSON_VALUE(mf, '$.user_initiated_interaction_count') AS INT64) AS interactions
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_model_feature')) AS mf
WHERE JSON_VALUE(mf, '$.model') IS NOT NULL
ORDER BY day, model_name;
