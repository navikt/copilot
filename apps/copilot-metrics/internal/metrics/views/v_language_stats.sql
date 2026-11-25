CREATE OR REPLACE VIEW `%s.%s.v_language_stats` AS
SELECT
  day,
  JSON_VALUE(lf, '$.language') AS language,
  JSON_VALUE(lf, '$.feature') AS feature,
  CAST(JSON_VALUE(lf, '$.code_generation_activity_count') AS INT64) AS generations,
  CAST(JSON_VALUE(lf, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
  CAST(JSON_VALUE(lf, '$.loc_suggested_to_add_sum') AS INT64) AS lines_suggested,
  CAST(JSON_VALUE(lf, '$.loc_added_sum') AS INT64) AS lines_accepted,
  CAST(JSON_VALUE(lf, '$.loc_suggested_to_delete_sum') AS INT64) AS deletions_suggested,
  CAST(JSON_VALUE(lf, '$.loc_deleted_sum') AS INT64) AS deletions_accepted
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_language_feature')) AS lf
WHERE JSON_VALUE(lf, '$.language') IS NOT NULL
ORDER BY day, generations DESC;
