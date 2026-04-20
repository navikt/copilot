CREATE OR REPLACE VIEW `%s.%s.v_customization_details` AS
SELECT
  scan_date,
  org,
  repo,
  primary_language,
  visibility,
  IFNULL(default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY), false) AS is_recently_active,
  'agents' AS category,
  JSON_VALUE(f) AS file_name
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(customizations, '$.agents.files')) AS f
WHERE JSON_VALUE(customizations, '$.agents.exists') = 'true'
  AND NOT is_archived

UNION ALL

SELECT
  scan_date,
  org,
  repo,
  primary_language,
  visibility,
  IFNULL(default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY), false) AS is_recently_active,
  'instructions' AS category,
  JSON_VALUE(f) AS file_name
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(customizations, '$.instructions.files')) AS f
WHERE JSON_VALUE(customizations, '$.instructions.exists') = 'true'
  AND NOT is_archived

UNION ALL

SELECT
  scan_date,
  org,
  repo,
  primary_language,
  visibility,
  IFNULL(default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY), false) AS is_recently_active,
  'prompts' AS category,
  JSON_VALUE(f) AS file_name
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(customizations, '$.prompts.files')) AS f
WHERE JSON_VALUE(customizations, '$.prompts.exists') = 'true'
  AND NOT is_archived

UNION ALL

SELECT
  scan_date,
  org,
  repo,
  primary_language,
  visibility,
  IFNULL(default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY), false) AS is_recently_active,
  'skills' AS category,
  JSON_VALUE(f) AS file_name
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(customizations, '$.skills.files')) AS f
WHERE JSON_VALUE(customizations, '$.skills.exists') = 'true'
  AND NOT is_archived

UNION ALL

SELECT
  scan_date,
  org,
  repo,
  primary_language,
  visibility,
  IFNULL(default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY), false) AS is_recently_active,
  'agentic_workflows' AS category,
  JSON_VALUE(f) AS file_name
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(customizations, '$.agentic_workflows.files')) AS f
WHERE JSON_VALUE(customizations, '$.agentic_workflows.exists') = 'true'
  AND NOT is_archived

UNION ALL

SELECT
  scan_date,
  org,
  repo,
  primary_language,
  visibility,
  IFNULL(default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY), false) AS is_recently_active,
  'agents_skills' AS category,
  JSON_VALUE(f) AS file_name
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(customizations, '$.agents_skills.files')) AS f
WHERE JSON_VALUE(customizations, '$.agents_skills.exists') = 'true'
  AND NOT is_archived
