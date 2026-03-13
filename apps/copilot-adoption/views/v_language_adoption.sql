CREATE OR REPLACE VIEW `%s.%s.v_language_adoption` AS
SELECT
  scan_date,
  IFNULL(primary_language, 'Unknown') AS language,
  COUNT(*) AS total_repos,
  COUNTIF(has_any_customization) AS repos_with_customizations,
  SAFE_DIVIDE(
    COUNTIF(has_any_customization),
    COUNT(*)
  ) AS adoption_rate,
  COUNTIF(JSON_VALUE(customizations, '$.copilot_instructions.exists') = 'true') AS with_copilot_instructions,
  COUNTIF(JSON_VALUE(customizations, '$.agents.exists') = 'true') AS with_agents,
  COUNTIF(JSON_VALUE(customizations, '$.instructions.exists') = 'true') AS with_instructions,
  COUNTIF(JSON_VALUE(customizations, '$.mcp_config.exists') = 'true') AS with_mcp_config
FROM `%s.%s.%s`
WHERE NOT is_archived
GROUP BY scan_date, language
HAVING total_repos >= 5
ORDER BY scan_date DESC, total_repos DESC
