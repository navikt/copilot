CREATE OR REPLACE VIEW `%s.%s.v_client_stats` AS
SELECT
  day,
  JSON_VALUE(client, '$.client') AS client,
  CAST(JSON_VALUE(client, '$.request_count') AS INT64) AS requests,
  CAST(JSON_VALUE(client, '$.session_count') AS INT64) AS sessions
FROM `%s.%s.%s`,
  UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_client')) AS client
WHERE scope = 'enterprise'
  AND JSON_VALUE(client, '$.client') IS NOT NULL
ORDER BY day, requests DESC;
