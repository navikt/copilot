CREATE OR REPLACE VIEW `%s.%s.v_customization_details` AS
SELECT
  scan_date,
  org,
  repo,
  primary_language,
  visibility,
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
  'skills' AS category,
  JSON_VALUE(f) AS file_name
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(customizations, '$.skills.files')) AS f
WHERE JSON_VALUE(customizations, '$.skills.exists') = 'true'
  AND NOT is_archived
