CREATE OR REPLACE VIEW `%s.%s.v_adoption_summary` AS
SELECT
  scan_date,
  COUNT(*) AS total_repos,
  COUNTIF(NOT is_archived) AS active_repos,
  COUNTIF(is_archived) AS archived_repos,
  COUNTIF(NOT is_archived AND default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY)) AS active_repos_with_recent_commits,
  COUNTIF(NOT is_archived AND default_branch_last_commit IS NOT NULL AND default_branch_last_commit < TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY)) AS dormant_repos,
  COUNTIF(NOT is_archived AND default_branch_last_commit IS NULL) AS unknown_last_commit_repos,
  COUNTIF(has_any_customization AND NOT is_archived) AS repos_with_any_customization,
  COUNTIF(NOT has_any_customization AND NOT is_archived) AS repos_without_customization,
  SAFE_DIVIDE(
    COUNTIF(has_any_customization AND NOT is_archived),
    COUNTIF(NOT is_archived)
  ) AS adoption_rate,
  SAFE_DIVIDE(
    COUNTIF(has_any_customization AND NOT is_archived AND default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY)),
    COUNTIF(NOT is_archived AND default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY))
  ) AS adoption_rate_active_only,
  COUNTIF(JSON_VALUE(customizations, '$.copilot_instructions.exists') = 'true' AND NOT is_archived) AS repos_with_copilot_instructions,
  COUNTIF(JSON_VALUE(customizations, '$.agents_md.exists') = 'true' AND NOT is_archived) AS repos_with_agents_md,
  COUNTIF(JSON_VALUE(customizations, '$.agents.exists') = 'true' AND NOT is_archived) AS repos_with_agents,
  COUNTIF(JSON_VALUE(customizations, '$.instructions.exists') = 'true' AND NOT is_archived) AS repos_with_instructions,
  COUNTIF(JSON_VALUE(customizations, '$.prompts.exists') = 'true' AND NOT is_archived) AS repos_with_prompts,
  COUNTIF(JSON_VALUE(customizations, '$.skills.exists') = 'true' AND NOT is_archived) AS repos_with_skills,
  COUNTIF(JSON_VALUE(customizations, '$.mcp_config.exists') = 'true' AND NOT is_archived) AS repos_with_mcp_config,
  COUNTIF(JSON_VALUE(customizations, '$.copilot_dir.exists') = 'true' AND NOT is_archived) AS repos_with_copilot_dir,
  -- Non-Copilot AI tools
  COUNTIF(JSON_VALUE(customizations, '$.cursorrules.exists') = 'true' AND NOT is_archived) AS repos_with_cursorrules,
  COUNTIF(JSON_VALUE(customizations, '$.cursor_rules_dir.exists') = 'true' AND NOT is_archived) AS repos_with_cursor_rules_dir,
  COUNTIF(JSON_VALUE(customizations, '$.claude_md.exists') = 'true' AND NOT is_archived) AS repos_with_claude_md,
  COUNTIF(JSON_VALUE(customizations, '$.windsurfrules.exists') = 'true' AND NOT is_archived) AS repos_with_windsurfrules,
  COUNTIF(JSON_VALUE(customizations, '$.cursorignore.exists') = 'true' AND NOT is_archived) AS repos_with_cursorignore,
  COUNTIF(JSON_VALUE(customizations, '$.claude_settings.exists') = 'true' AND NOT is_archived) AS repos_with_claude_settings,
  -- Aggregate: any non-Copilot AI tool
  COUNTIF((
    JSON_VALUE(customizations, '$.cursorrules.exists') = 'true' OR
    JSON_VALUE(customizations, '$.cursor_rules_dir.exists') = 'true' OR
    JSON_VALUE(customizations, '$.claude_md.exists') = 'true' OR
    JSON_VALUE(customizations, '$.claude_settings.exists') = 'true' OR
    JSON_VALUE(customizations, '$.cursorignore.exists') = 'true' OR
    JSON_VALUE(customizations, '$.windsurfrules.exists') = 'true'
  ) AND NOT is_archived) AS repos_with_any_non_copilot_ai,
  AVG(CASE WHEN NOT is_archived THEN customization_count END) AS avg_customization_count,
  MAX(CASE WHEN NOT is_archived THEN customization_count END) AS max_customization_count
FROM `%s.%s.%s`
GROUP BY scan_date
ORDER BY scan_date DESC
