CREATE OR REPLACE VIEW `%s.%s.v_code_generation` AS
SELECT
  day,
  JSON_VALUE(f, '$.feature') AS feature,
  CASE
    WHEN JSON_VALUE(f, '$.feature') IN ('code_completion', 'chat_panel_ask_mode', 'chat_inline')
      THEN 'user_initiated'
    WHEN JSON_VALUE(f, '$.feature') IN ('agent_edit', 'chat_panel_agent_mode', 'chat_panel_edit_mode', 'chat_panel_custom_mode')
      THEN 'agent_initiated'
    ELSE 'other'
  END AS generation_mode,
  CAST(JSON_VALUE(f, '$.code_generation_activity_count') AS INT64) AS generations,
  CAST(JSON_VALUE(f, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
  CAST(JSON_VALUE(f, '$.loc_added_sum') AS INT64) AS lines_added,
  CAST(JSON_VALUE(f, '$.loc_deleted_sum') AS INT64) AS lines_deleted,
  CAST(JSON_VALUE(f, '$.loc_suggested_to_add_sum') AS INT64) AS lines_suggested_to_add,
  CAST(JSON_VALUE(f, '$.loc_suggested_to_delete_sum') AS INT64) AS lines_suggested_to_delete,
  CAST(JSON_VALUE(f, '$.user_initiated_interaction_count') AS INT64) AS interactions
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_feature')) AS f
WHERE JSON_VALUE(f, '$.feature') IS NOT NULL
  AND JSON_VALUE(f, '$.feature') != 'others'
  AND JSON_VALUE(f, '$.feature') != 'chat_panel_unknown_mode'
ORDER BY day, generations DESC;
