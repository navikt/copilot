-- v_repository_usage — per-repository GitHub Copilot pull-request activity,
-- aggregated across all available days, with privacy safeguards baked in.
--
-- The repos-1-day report is PR-only (coding agent + code review); the
-- pull_requests.* sub-object is the same shape already extracted per entity in
-- v_daily_summary.sql, so the field mapping is a known quantity — but here it is
-- read per repository from the repository_metrics raw-JSON table.
--
-- WHY AGGREGATED (never raw per-day per-repo):
-- Repo-level data creates a re-identification risk the user-level k-anonymity
-- guard (bigquery_stats.go:minUsersForDistribution = 5) does not cover: a repo
-- with only one or two contributors makes Copilot PR counts effectively
-- attributable to a named individual, and a single active day could be singled
-- out. This view therefore exposes ONLY per-repo rollups, never raw per-day
-- per-repo rows.
--
-- PRIVACY SAFEGUARDS (see docs/repository-level-metrics-integration.md §7):
--   1. Private repos are excluded — only repo_visibility IN ('PUBLIC','INTERNAL').
--   2. Low-activity repos are suppressed: a repo is surfaced only when its total
--      PRs created (all authors, over the window) is >= min_repo_activity (5),
--      matching the k = 5 used for user distributions so the two guards stay
--      visibly related.
--   3. Aggregation is per-repo across ALL available days; the API/query layer
--      bounds the trailing window. This mirrors the existing views
--      (v_daily_summary / v_team_daily_summary) which leave time windowing to the
--      query layer rather than hard-coding a window in the view.
--
-- All pull_requests.* metrics are NULL for pre-GA (2026-07-17) days — the
-- repos-1-day report did not exist before GA, so those days contribute nothing.
CREATE OR REPLACE VIEW `%s.%s.v_repository_usage` AS
WITH scoped_latest AS (
  SELECT * FROM {{repository_metrics}}
  QUALIFY ROW_NUMBER() OVER (
    PARTITION BY scope, scope_id, JSON_VALUE(raw_record, '$.repo_id'), day
    ORDER BY loaded_at DESC
  ) = 1
), ambiguous_repos AS (
  -- Without parent enterprise identity, overlapping scopes cannot be reconciled.
  SELECT JSON_VALUE(raw_record, '$.repo_id') AS repo_id
  FROM scoped_latest
  GROUP BY repo_id, day
  HAVING COUNTIF(NULLIF(JSON_VALUE(raw_record, '$.enterprise_id'), '') IS NULL) > 0
    AND COUNT(DISTINCT CONCAT(scope, ':', scope_id)) > 1
), source AS (
  SELECT * FROM scoped_latest AS candidate
  -- Withhold the whole rollup, not just ambiguous days that would hide missing counts.
  WHERE NOT EXISTS (
    SELECT 1 FROM ambiguous_repos
    WHERE ambiguous_repos.repo_id = JSON_VALUE(candidate.raw_record, '$.repo_id')
  )
  QUALIFY ROW_NUMBER() OVER (
    PARTITION BY COALESCE(NULLIF(JSON_VALUE(raw_record, '$.enterprise_id'), ''), CONCAT(scope, ':', scope_id)),
      JSON_VALUE(raw_record, '$.repo_id'), day
    ORDER BY IF(scope = 'enterprise', 0, 1), loaded_at DESC
  ) = 1
), daily AS (
  SELECT
    CAST(JSON_VALUE(raw_record, '$.repo_id') AS INT64)  AS repo_id,
    JSON_VALUE(raw_record, '$.repo_owner_name')         AS repo_owner,
    JSON_VALUE(raw_record, '$.repo_name')               AS repo_name,
    JSON_VALUE(raw_record, '$.repo_visibility')         AS repo_visibility,
    scope,
    scope_id,
    day,
    raw_record,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_created') AS INT64) AS pr_total_created,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_merged') AS INT64) AS pr_total_merged,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_reviewed') AS INT64) AS pr_total_reviewed,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_created_by_copilot') AS INT64) AS pr_created_by_copilot,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_reviewed_by_copilot') AS INT64) AS pr_reviewed_by_copilot,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_merged_created_by_copilot') AS INT64) AS pr_merged_copilot_authored,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_merged_reviewed_by_copilot') AS INT64) AS pr_merged_copilot_reviewed,
    NULLIF(CAST(JSON_VALUE(raw_record, '$.pull_requests.median_minutes_to_merge') AS FLOAT64), 0) AS pr_median_minutes_to_merge,
    NULLIF(CAST(JSON_VALUE(raw_record, '$.pull_requests.median_minutes_to_merge_copilot_authored') AS FLOAT64), 0) AS pr_median_minutes_to_merge_copilot,
    NULLIF(CAST(JSON_VALUE(raw_record, '$.pull_requests.median_minutes_to_merge_copilot_reviewed') AS FLOAT64), 0) AS pr_median_minutes_to_merge_copilot_reviewed,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_copilot_suggestions') AS INT64) AS pr_copilot_suggestions,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_copilot_applied_suggestions') AS INT64) AS pr_copilot_applied_suggestions,
    JSON_TYPE(JSON_QUERY(raw_record, '$.copilot_code_review')) = 'object' AS has_review_outcomes,
    SAFE_CAST(JSON_VALUE(raw_record, '$.copilot_code_review.total_comments_evaluated') AS INT64) AS review_comments_evaluated,
    SAFE_CAST(JSON_VALUE(raw_record, '$.copilot_code_review.total_comments_addressed') AS INT64) AS review_comments_addressed,
    SAFE_CAST(JSON_VALUE(raw_record, '$.copilot_code_review.total_merged_no_human_change_request_after_review') AS INT64) AS review_merged_no_human_change_request
  FROM source
  -- Privacy safeguard 1: never surface private repositories.
  WHERE JSON_VALUE(raw_record, '$.repo_visibility') IN ('PUBLIC', 'INTERNAL')
), dimension_counts AS (
  SELECT repo_id, repo_owner, repo_name, scope_id, dimension,
    IFNULL(CASE dimension
      WHEN 'severity' THEN JSON_VALUE(entry, '$.severity')
      WHEN 'review_tier' THEN JSON_VALUE(entry, '$.review_tier')
      WHEN 'comment_type' THEN JSON_VALUE(entry, '$.comment_type')
      WHEN 'customization' THEN JSON_VALUE(entry, '$.customization')
    END, 'unknown') AS label,
    IF(COUNT(SAFE_CAST(JSON_VALUE(entry, '$.total_comments_evaluated') AS INT64)) = COUNT(*),
      SUM(SAFE_CAST(JSON_VALUE(entry, '$.total_comments_evaluated') AS INT64)), NULL) AS evaluated,
    IF(COUNT(SAFE_CAST(JSON_VALUE(entry, '$.total_comments_addressed') AS INT64)) = COUNT(*),
      SUM(SAFE_CAST(JSON_VALUE(entry, '$.total_comments_addressed') AS INT64)), NULL) AS addressed
  FROM daily,
  UNNEST([
    STRUCT('severity' AS dimension, JSON_QUERY_ARRAY(raw_record, '$.copilot_code_review.review_comments_by_severity') AS entries),
    STRUCT('review_tier', JSON_QUERY_ARRAY(raw_record, '$.copilot_code_review.review_comments_by_review_tier')),
    STRUCT('comment_type', JSON_QUERY_ARRAY(raw_record, '$.copilot_code_review.review_comments_by_comment_type')),
    STRUCT('customization', JSON_QUERY_ARRAY(raw_record, '$.copilot_code_review.review_comments_by_customization'))
  ]), UNNEST(entries) AS entry
  GROUP BY repo_id, repo_owner, repo_name, scope_id, dimension, label
), breakdowns AS (
  SELECT repo_id, repo_owner, repo_name, scope_id,
    ARRAY_AGG(STRUCT(dimension, label, evaluated, addressed) ORDER BY dimension, label) AS review_breakdowns
  FROM dimension_counts
  GROUP BY repo_id, repo_owner, repo_name, scope_id
), totals AS (
SELECT
  repo_id,
  repo_owner,
  repo_name,
  ANY_VALUE(repo_visibility) AS repo_visibility,
  scope_id,
  COUNT(DISTINCT day) AS days_with_data,
  MIN(day) AS first_day,
  MAX(day) AS last_day,
  SUM(pr_total_created) AS pr_total_created,
  SUM(pr_total_merged) AS pr_total_merged,
  SUM(pr_total_reviewed) AS pr_total_reviewed,
  SUM(pr_created_by_copilot) AS pr_created_by_copilot,
  SUM(pr_reviewed_by_copilot) AS pr_reviewed_by_copilot,
  SUM(pr_merged_copilot_authored) AS pr_merged_copilot_authored,
  SUM(pr_merged_copilot_reviewed) AS pr_merged_copilot_reviewed,
  SUM(pr_copilot_suggestions) AS pr_copilot_suggestions,
  SUM(pr_copilot_applied_suggestions) AS pr_copilot_applied_suggestions,
  -- Missing historical objects remain NULL; zero is a reported observation.
  SUM(review_comments_evaluated) AS review_comments_evaluated,
  SUM(review_comments_addressed) AS review_comments_addressed,
  SUM(review_merged_no_human_change_request) AS review_merged_no_human_change_request,
  COUNT(DISTINCT IF(has_review_outcomes, day, NULL)) AS review_outcome_days,
  COUNT(DISTINCT IF(review_comments_evaluated IS NOT NULL AND review_comments_addressed IS NOT NULL, day, NULL)) AS review_comment_days,
  CAST(MIN(IF(has_review_outcomes, day, NULL)) AS STRING) AS review_outcome_first_day,
  CAST(MAX(IF(has_review_outcomes, day, NULL)) AS STRING) AS review_outcome_last_day,
  -- Medians cannot be summed across days; expose the simple average of the daily
  -- median values (over days where the median is present) as an approximation of
  -- the window's typical time-to-merge. NULL when no day had a median.
  AVG(pr_median_minutes_to_merge) AS pr_avg_median_minutes_to_merge,
  AVG(pr_median_minutes_to_merge_copilot) AS pr_avg_median_minutes_to_merge_copilot,
  AVG(pr_median_minutes_to_merge_copilot_reviewed) AS pr_avg_median_minutes_to_merge_copilot_reviewed
FROM daily
GROUP BY repo_id, repo_owner, repo_name, scope_id
-- Privacy safeguard 2: suppress low-activity repos. The threshold 5 mirrors
-- bigquery_stats.go:minUsersForDistribution (k = 5) — below this many total PRs
-- created over the window, a single contributor's behaviour dominates the
-- repo's numbers, so the repo is suppressed.
--
-- Referenced as the SELECT alias, not SUM(pr_total_created): in HAVING the alias
-- shadows the source column, so wrapping it in SUM() again is read as
-- SUM(SUM(...)) and BigQuery rejects it with "Aggregations of aggregations are
-- not allowed".
HAVING pr_total_created >= 5
)
SELECT totals.*, IFNULL(breakdowns.review_breakdowns, []) AS review_breakdowns
FROM totals LEFT JOIN breakdowns USING (repo_id, repo_owner, repo_name, scope_id)
ORDER BY pr_created_by_copilot DESC;
