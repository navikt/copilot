CREATE OR REPLACE VIEW `%s.%s.v_daily_summary` AS
SELECT
  day,
  scope,
  scope_id,
  CAST(JSON_VALUE(raw_record, '$.daily_active_users') AS INT64) AS daily_active_users,
  CAST(JSON_VALUE(raw_record, '$.weekly_active_users') AS INT64) AS weekly_active_users,
  CAST(JSON_VALUE(raw_record, '$.monthly_active_users') AS INT64) AS monthly_active_users,
  CAST(JSON_VALUE(raw_record, '$.monthly_active_chat_users') AS INT64) AS monthly_active_chat_users,
  CAST(JSON_VALUE(raw_record, '$.monthly_active_agent_users') AS INT64) AS monthly_active_agent_users,
  CAST(JSON_VALUE(raw_record, '$.daily_active_cli_users') AS INT64) AS daily_active_cli_users,
  CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64) AS total_generations,
  CAST(JSON_VALUE(raw_record, '$.code_acceptance_activity_count') AS INT64) AS total_acceptances,
  CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64) AS total_interactions,
  CAST(JSON_VALUE(raw_record, '$.loc_suggested_to_add_sum') AS INT64) AS lines_suggested,
  CAST(JSON_VALUE(raw_record, '$.loc_added_sum') AS INT64) AS lines_accepted,
  CAST(JSON_VALUE(raw_record, '$.loc_suggested_to_delete_sum') AS INT64) AS deletions_suggested,
  CAST(JSON_VALUE(raw_record, '$.loc_deleted_sum') AS INT64) AS deletions_accepted,
  -- Pull request metrics
  CAST(JSON_VALUE(raw_record, '$.pull_requests.total_created') AS INT64) AS pr_total_created,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.total_merged') AS INT64) AS pr_total_merged,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.total_reviewed') AS INT64) AS pr_total_reviewed,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.total_created_by_copilot') AS INT64) AS pr_created_by_copilot,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.total_reviewed_by_copilot') AS INT64) AS pr_reviewed_by_copilot,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.total_merged_created_by_copilot') AS INT64) AS pr_merged_copilot_authored,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.total_merged_reviewed_by_copilot') AS INT64) AS pr_merged_copilot_reviewed,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.median_minutes_to_merge') AS FLOAT64) AS pr_median_minutes_to_merge,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.median_minutes_to_merge_copilot_authored') AS FLOAT64) AS pr_median_minutes_to_merge_copilot,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.median_minutes_to_merge_copilot_reviewed') AS FLOAT64) AS pr_median_minutes_to_merge_copilot_reviewed,
  -- Code-review velocity (added to the usage API 2026-07-07). These live per AI
  -- adoption phase in totals_by_ai_adoption_phase[], so we roll them up to a daily
  -- figure weighted by each phase's merged-PR count. NULL for days ingested before
  -- the fields existed (the array or fields are simply absent).
  (
    SELECT SAFE_DIVIDE(
      SUM(CAST(JSON_VALUE(phase, '$.avg_pull_requests_minutes_to_review') AS FLOAT64)
          * CAST(JSON_VALUE(phase, '$.total_pull_requests_merged') AS INT64)),
      SUM(CAST(JSON_VALUE(phase, '$.total_pull_requests_merged') AS INT64))
    )
    FROM UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_ai_adoption_phase')) AS phase
  ) AS pr_avg_minutes_to_review,
  (
    SELECT SAFE_DIVIDE(
      SUM(CAST(JSON_VALUE(phase, '$.avg_pull_requests_review_cycles') AS FLOAT64)
          * CAST(JSON_VALUE(phase, '$.total_pull_requests_merged') AS INT64)),
      SUM(CAST(JSON_VALUE(phase, '$.total_pull_requests_merged') AS INT64))
    )
    FROM UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_ai_adoption_phase')) AS phase
  ) AS pr_avg_review_cycles,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.total_suggestions') AS INT64) AS pr_total_suggestions,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.total_copilot_suggestions') AS INT64) AS pr_copilot_suggestions,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.total_applied_suggestions') AS INT64) AS pr_applied_suggestions,
  CAST(JSON_VALUE(raw_record, '$.pull_requests.total_copilot_applied_suggestions') AS INT64) AS pr_copilot_applied_suggestions,
  -- CLI metrics
  CAST(JSON_VALUE(raw_record, '$.totals_by_cli.session_count') AS INT64) AS cli_session_count,
  CAST(JSON_VALUE(raw_record, '$.totals_by_cli.request_count') AS INT64) AS cli_request_count,
  CAST(JSON_VALUE(raw_record, '$.totals_by_cli.prompt_count') AS INT64) AS cli_prompt_count,
  CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.output_tokens_sum') AS INT64) AS cli_output_tokens,
  CAST(JSON_VALUE(raw_record, '$.totals_by_cli.token_usage.prompt_tokens_sum') AS INT64) AS cli_prompt_tokens
FROM `%s.%s.%s`
ORDER BY day;
