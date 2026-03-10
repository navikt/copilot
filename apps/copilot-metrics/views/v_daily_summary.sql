CREATE OR REPLACE VIEW `%s.%s.v_daily_summary` AS
SELECT
  day,
  CAST(JSON_VALUE(raw_record, '$.total_active_users') AS INT64) AS total_active_users,
  CAST(JSON_VALUE(raw_record, '$.total_engaged_users') AS INT64) AS total_engaged_users,
  CAST(JSON_VALUE(raw_record, '$.copilot_ide_code_completions.total_engaged_users') AS INT64) AS code_completion_users,
  CAST(JSON_VALUE(raw_record, '$.copilot_ide_chat.total_engaged_users') AS INT64) AS ide_chat_users,
  CAST(JSON_VALUE(raw_record, '$.copilot_dotcom_chat.total_engaged_users') AS INT64) AS dotcom_chat_users,
  CAST(JSON_VALUE(raw_record, '$.copilot_dotcom_pull_requests.total_engaged_users') AS INT64) AS pr_summary_users,
  (
    SELECT IFNULL(SUM(CAST(JSON_VALUE(lang, '$.total_code_suggestions') AS INT64)), 0)
    FROM UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_ide_code_completions.editors')) AS editor,
         UNNEST(JSON_QUERY_ARRAY(editor, '$.models')) AS model,
         UNNEST(JSON_QUERY_ARRAY(model, '$.languages')) AS lang
  ) AS total_suggestions,
  (
    SELECT IFNULL(SUM(CAST(JSON_VALUE(lang, '$.total_code_acceptances') AS INT64)), 0)
    FROM UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_ide_code_completions.editors')) AS editor,
         UNNEST(JSON_QUERY_ARRAY(editor, '$.models')) AS model,
         UNNEST(JSON_QUERY_ARRAY(model, '$.languages')) AS lang
  ) AS total_acceptances,
  (
    SELECT IFNULL(SUM(CAST(JSON_VALUE(lang, '$.total_code_lines_suggested') AS INT64)), 0)
    FROM UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_ide_code_completions.editors')) AS editor,
         UNNEST(JSON_QUERY_ARRAY(editor, '$.models')) AS model,
         UNNEST(JSON_QUERY_ARRAY(model, '$.languages')) AS lang
  ) AS total_lines_suggested,
  (
    SELECT IFNULL(SUM(CAST(JSON_VALUE(lang, '$.total_code_lines_accepted') AS INT64)), 0)
    FROM UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_ide_code_completions.editors')) AS editor,
         UNNEST(JSON_QUERY_ARRAY(editor, '$.models')) AS model,
         UNNEST(JSON_QUERY_ARRAY(model, '$.languages')) AS lang
  ) AS total_lines_accepted,
  (
    SELECT IFNULL(SUM(CAST(JSON_VALUE(model, '$.total_chats') AS INT64)), 0)
    FROM UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_ide_chat.editors')) AS editor,
         UNNEST(JSON_QUERY_ARRAY(editor, '$.models')) AS model
  ) AS total_ide_chats,
  (
    SELECT IFNULL(SUM(CAST(JSON_VALUE(model, '$.total_chats') AS INT64)), 0)
    FROM UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_dotcom_chat.models')) AS model
  ) AS total_dotcom_chats,
  (
    SELECT IFNULL(SUM(CAST(JSON_VALUE(model, '$.total_chat_copy_events') AS INT64)), 0)
    FROM UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_ide_chat.editors')) AS editor,
         UNNEST(JSON_QUERY_ARRAY(editor, '$.models')) AS model
  ) AS total_chat_copy_events,
  (
    SELECT IFNULL(SUM(CAST(JSON_VALUE(model, '$.total_chat_insertion_events') AS INT64)), 0)
    FROM UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_ide_chat.editors')) AS editor,
         UNNEST(JSON_QUERY_ARRAY(editor, '$.models')) AS model
  ) AS total_chat_insertion_events,
  (
    SELECT IFNULL(SUM(CAST(JSON_VALUE(model, '$.total_pr_summaries_created') AS INT64)), 0)
    FROM UNNEST(JSON_QUERY_ARRAY(raw_record, '$.copilot_dotcom_pull_requests.repositories')) AS repo,
         UNNEST(JSON_QUERY_ARRAY(repo, '$.models')) AS model
  ) AS total_pr_summaries
FROM `%s.%s.%s`
ORDER BY day;
