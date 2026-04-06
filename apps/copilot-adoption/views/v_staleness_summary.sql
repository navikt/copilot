CREATE OR REPLACE VIEW `%s.%s.v_staleness_summary` AS
WITH file_data AS (
  SELECT
    scan_date,
    org,
    repo,
    primary_language,
    visibility,
    IFNULL(default_branch_last_commit >= TIMESTAMP_SUB(TIMESTAMP(scan_date), INTERVAL 90 DAY), false) AS is_recently_active,
    item.category,
    item.file_name,
    item.oid,
    item.in_sync
  FROM `%s.%s.%s`,
    UNNEST(
      ARRAY_CONCAT(
        IF(JSON_VALUE(customizations, '$.agents.exists') = 'true',
          (SELECT ARRAY_AGG(STRUCT(
            'agents' AS category,
            JSON_VALUE(f) AS file_name,
            JSON_VALUE(o) AS oid,
            SAFE_CAST(JSON_VALUE(s) AS BOOL) AS in_sync
          ))
          FROM UNNEST(JSON_QUERY_ARRAY(customizations, '$.agents.files')) AS f WITH OFFSET pos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.agents.oids')) AS o WITH OFFSET opos ON pos = opos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.agents.in_sync')) AS s WITH OFFSET spos ON pos = spos),
          []),
        IF(JSON_VALUE(customizations, '$.instructions.exists') = 'true',
          (SELECT ARRAY_AGG(STRUCT(
            'instructions' AS category,
            JSON_VALUE(f) AS file_name,
            JSON_VALUE(o) AS oid,
            SAFE_CAST(JSON_VALUE(s) AS BOOL) AS in_sync
          ))
          FROM UNNEST(JSON_QUERY_ARRAY(customizations, '$.instructions.files')) AS f WITH OFFSET pos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.instructions.oids')) AS o WITH OFFSET opos ON pos = opos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.instructions.in_sync')) AS s WITH OFFSET spos ON pos = spos),
          []),
        IF(JSON_VALUE(customizations, '$.prompts.exists') = 'true',
          (SELECT ARRAY_AGG(STRUCT(
            'prompts' AS category,
            JSON_VALUE(f) AS file_name,
            JSON_VALUE(o) AS oid,
            SAFE_CAST(JSON_VALUE(s) AS BOOL) AS in_sync
          ))
          FROM UNNEST(JSON_QUERY_ARRAY(customizations, '$.prompts.files')) AS f WITH OFFSET pos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.prompts.oids')) AS o WITH OFFSET opos ON pos = opos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.prompts.in_sync')) AS s WITH OFFSET spos ON pos = spos),
          []),
        IF(JSON_VALUE(customizations, '$.skills.exists') = 'true',
          (SELECT ARRAY_AGG(STRUCT(
            'skills' AS category,
            JSON_VALUE(f) AS file_name,
            JSON_VALUE(o) AS oid,
            SAFE_CAST(JSON_VALUE(s) AS BOOL) AS in_sync
          ))
          FROM UNNEST(JSON_QUERY_ARRAY(customizations, '$.skills.files')) AS f WITH OFFSET pos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.skills.oids')) AS o WITH OFFSET opos ON pos = opos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.skills.in_sync')) AS s WITH OFFSET spos ON pos = spos),
          []),
        IF(JSON_VALUE(customizations, '$.copilot_instructions.exists') = 'true',
          (SELECT ARRAY_AGG(STRUCT(
            'copilot_instructions' AS category,
            JSON_VALUE(f) AS file_name,
            JSON_VALUE(o) AS oid,
            SAFE_CAST(JSON_VALUE(s) AS BOOL) AS in_sync
          ))
          FROM UNNEST(JSON_QUERY_ARRAY(customizations, '$.copilot_instructions.files')) AS f WITH OFFSET pos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.copilot_instructions.oids')) AS o WITH OFFSET opos ON pos = opos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.copilot_instructions.in_sync')) AS s WITH OFFSET spos ON pos = spos),
          []),
        IF(JSON_VALUE(customizations, '$.agents_md.exists') = 'true',
          (SELECT ARRAY_AGG(STRUCT(
            'agents_md' AS category,
            JSON_VALUE(f) AS file_name,
            JSON_VALUE(o) AS oid,
            SAFE_CAST(JSON_VALUE(s) AS BOOL) AS in_sync
          ))
          FROM UNNEST(JSON_QUERY_ARRAY(customizations, '$.agents_md.files')) AS f WITH OFFSET pos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.agents_md.oids')) AS o WITH OFFSET opos ON pos = opos
          LEFT JOIN UNNEST(JSON_QUERY_ARRAY(customizations, '$.agents_md.in_sync')) AS s WITH OFFSET spos ON pos = spos),
          [])
      )
    ) AS item
  WHERE NOT is_archived
    AND has_any_customization
)
SELECT
  scan_date,
  org,
  repo,
  primary_language,
  visibility,
  is_recently_active,
  category,
  file_name,
  oid,
  IFNULL(in_sync, false) AS in_sync
FROM file_data
WHERE file_name IS NOT NULL
ORDER BY scan_date DESC, org, repo, category, file_name
