CREATE OR REPLACE VIEW `%s.%s.v_editor_stats` AS
WITH daily AS (
  SELECT day, raw_record
  FROM `%s.%s.%s`
  WHERE scope = 'enterprise' AND LOWER(scope_id) = 'nav'
  QUALIFY ROW_NUMBER() OVER (PARTITION BY day, scope, LOWER(scope_id) ORDER BY loaded_at DESC) = 1
)
SELECT
  day,
  JSON_VALUE(ide, '$.ide') AS editor,
  CAST(JSON_VALUE(ide, '$.code_generation_activity_count') AS INT64) AS generations,
  CAST(JSON_VALUE(ide, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
  CAST(JSON_VALUE(ide, '$.loc_suggested_to_add_sum') AS INT64) AS lines_suggested,
  CAST(JSON_VALUE(ide, '$.loc_added_sum') AS INT64) AS lines_accepted,
  CAST(JSON_VALUE(ide, '$.user_initiated_interaction_count') AS INT64) AS interactions
FROM daily,
  UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_ide')) AS ide
WHERE JSON_VALUE(ide, '$.ide') IS NOT NULL
ORDER BY day, generations DESC;
