CREATE OR REPLACE VIEW `%s.%s.v_language_stats` AS
SELECT
  day,
  JSON_VALUE(lang, '$.name') AS language,
  CAST(JSON_VALUE(lang, '$.total_engaged_users') AS INT64) AS engaged_users,
  CAST(JSON_VALUE(lang, '$.total_code_suggestions') AS INT64) AS suggestions,
  CAST(JSON_VALUE(lang, '$.total_code_acceptances') AS INT64) AS acceptances,
  CAST(JSON_VALUE(lang, '$.total_code_lines_suggested') AS INT64) AS lines_suggested,
  CAST(JSON_VALUE(lang, '$.total_code_lines_accepted') AS INT64) AS lines_accepted
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_ide_code_completions.editors')) AS editor,
  UNNEST(JSON_QUERY_ARRAY(editor, '$.models')) AS model,
  UNNEST(JSON_QUERY_ARRAY(model, '$.languages')) AS lang
WHERE JSON_VALUE(lang, '$.name') IS NOT NULL
ORDER BY day, engaged_users DESC;
