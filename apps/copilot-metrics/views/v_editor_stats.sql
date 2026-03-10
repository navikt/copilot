CREATE OR REPLACE VIEW `%s.%s.v_editor_stats` AS
SELECT
  day,
  JSON_VALUE(editor, '$.name') AS editor,
  CAST(JSON_VALUE(editor, '$.total_engaged_users') AS INT64) AS engaged_users,
  IFNULL(
    (SELECT SUM(CAST(JSON_VALUE(lang, '$.total_code_suggestions') AS INT64))
     FROM UNNEST(JSON_QUERY_ARRAY(model, '$.languages')) AS lang), 0
  ) AS suggestions,
  IFNULL(
    (SELECT SUM(CAST(JSON_VALUE(lang, '$.total_code_acceptances') AS INT64))
     FROM UNNEST(JSON_QUERY_ARRAY(model, '$.languages')) AS lang), 0
  ) AS acceptances
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_ide_code_completions.editors')) AS editor,
  UNNEST(JSON_QUERY_ARRAY(editor, '$.models')) AS model
WHERE JSON_VALUE(editor, '$.name') IS NOT NULL
ORDER BY day, engaged_users DESC;
