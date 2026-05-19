CREATE OR REPLACE VIEW `%s.%s.v_team_adoption` AS
SELECT
  scan_date,
  JSON_VALUE(team, '$.slug') AS team_slug,
  JSON_VALUE(team, '$.name') AS team_name,
  COUNT(*) AS team_repos,
  COUNTIF(NOT is_archived) AS active_repos,
  COUNTIF(NOT is_archived AND default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY)) AS recently_active_repos,
  COUNTIF(has_any_customization AND NOT is_archived) AS repos_with_customizations,
  SAFE_DIVIDE(
    COUNTIF(has_any_customization AND NOT is_archived),
    COUNTIF(NOT is_archived)
  ) AS adoption_rate,
  SAFE_DIVIDE(
    COUNTIF(has_any_customization AND NOT is_archived AND default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY)),
    COUNTIF(NOT is_archived AND default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY))
  ) AS adoption_rate_active_only,
  COUNTIF(JSON_VALUE(customizations, '$.copilot_instructions.exists') = 'true' AND NOT is_archived) AS with_copilot_instructions,
  COUNTIF(JSON_VALUE(customizations, '$.agents_md.exists') = 'true' AND NOT is_archived) AS with_agents_md,
  COUNTIF(JSON_VALUE(customizations, '$.agents.exists') = 'true' AND NOT is_archived) AS with_agents,
  COUNTIF(JSON_VALUE(customizations, '$.instructions.exists') = 'true' AND NOT is_archived) AS with_instructions,
  COUNTIF(JSON_VALUE(customizations, '$.prompts.exists') = 'true' AND NOT is_archived) AS with_prompts,
  COUNTIF(JSON_VALUE(customizations, '$.skills.exists') = 'true' AND NOT is_archived) AS with_skills,
  COUNTIF(JSON_VALUE(customizations, '$.mcp_config.exists') = 'true' AND NOT is_archived) AS with_mcp_config,
  COUNTIF(JSON_VALUE(customizations, '$.copilot_setup_steps.exists') = 'true' AND NOT is_archived) AS with_copilot_setup_steps,
  COUNTIF(JSON_VALUE(customizations, '$.agentic_workflows.exists') = 'true' AND NOT is_archived) AS with_agentic_workflows,
  COUNTIF(JSON_VALUE(customizations, '$.agents_skills.exists') = 'true' AND NOT is_archived) AS with_agents_skills,
  COUNTIF(JSON_VALUE(customizations, '$.nav_pilot_state.exists') = 'true' AND NOT is_archived) AS with_nav_pilot_state,
  COUNTIF(JSON_VALUE(customizations, '$.cplt_toml.exists') = 'true' AND NOT is_archived) AS with_cplt_toml
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(teams)) AS team
WHERE teams IS NOT NULL
GROUP BY scan_date, team_slug, team_name
ORDER BY scan_date DESC, repos_with_customizations DESC
