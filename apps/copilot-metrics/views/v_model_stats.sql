CREATE OR REPLACE VIEW `%s.%s.v_model_stats` AS
SELECT
  day,
  JSON_VALUE(model, '$.name') AS model_name,
  CAST(JSON_VALUE(model, '$.is_custom_model') AS BOOL) AS is_custom,
  'code_completion' AS feature,
  CAST(JSON_VALUE(model, '$.total_engaged_users') AS INT64) AS engaged_users
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_ide_code_completions.editors')) AS editor,
  UNNEST(JSON_QUERY_ARRAY(editor, '$.models')) AS model

UNION ALL

SELECT
  day,
  JSON_VALUE(model, '$.name') AS model_name,
  CAST(JSON_VALUE(model, '$.is_custom_model') AS BOOL) AS is_custom,
  'ide_chat' AS feature,
  CAST(JSON_VALUE(model, '$.total_engaged_users') AS INT64) AS engaged_users
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_ide_chat.editors')) AS editor,
  UNNEST(JSON_QUERY_ARRAY(editor, '$.models')) AS model

UNION ALL

SELECT
  day,
  JSON_VALUE(model, '$.name') AS model_name,
  CAST(JSON_VALUE(model, '$.is_custom_model') AS BOOL) AS is_custom,
  'dotcom_chat' AS feature,
  CAST(JSON_VALUE(model, '$.total_engaged_users') AS INT64) AS engaged_users
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_dotcom_chat.models')) AS model

ORDER BY day, model_name;
