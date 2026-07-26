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
WITH privacy_thresholds AS (
  -- min_repo_activity mirrors bigquery_stats.go:minUsersForDistribution (k = 5):
  -- below this many total PRs created over the window, a single contributor's
  -- behaviour dominates the repo's numbers, so the repo is suppressed.
  SELECT 5 AS min_repo_activity
),
daily AS (
  SELECT
    CAST(JSON_VALUE(raw_record, '$.repo_id') AS INT64)  AS repo_id,
    JSON_VALUE(raw_record, '$.repo_owner_name')         AS repo_owner,
    JSON_VALUE(raw_record, '$.repo_name')               AS repo_name,
    JSON_VALUE(raw_record, '$.repo_visibility')         AS repo_visibility,
    scope,
    scope_id,
    day,
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
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_copilot_suggestions') AS INT64) AS pr_copilot_suggestions,
    CAST(JSON_VALUE(raw_record, '$.pull_requests.total_copilot_applied_suggestions') AS INT64) AS pr_copilot_applied_suggestions
  FROM {{repository_metrics}}
  -- Privacy safeguard 1: never surface private repositories.
  WHERE JSON_VALUE(raw_record, '$.repo_visibility') IN ('PUBLIC', 'INTERNAL')
)
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
  -- Medians cannot be summed across days; expose the simple average of the daily
  -- median values (over days where the median is present) as an approximation of
  -- the window's typical time-to-merge. NULL when no day had a median.
  AVG(pr_median_minutes_to_merge) AS pr_avg_median_minutes_to_merge,
  AVG(pr_median_minutes_to_merge_copilot) AS pr_avg_median_minutes_to_merge_copilot,
  AVG(pr_median_minutes_to_merge_copilot_reviewed) AS pr_avg_median_minutes_to_merge_copilot_reviewed
FROM daily
GROUP BY repo_id, repo_owner, repo_name, scope_id
-- Privacy safeguard 2: suppress low-activity repos (k = 5 on total PRs created).
HAVING SUM(pr_total_created) >= (SELECT min_repo_activity FROM privacy_thresholds)
ORDER BY pr_created_by_copilot DESC;
