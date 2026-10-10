package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"cloud.google.com/go/bigquery"
)

// Monthly history for /innsikt/trender. Both queries return aggregates only,
// never per-person or per-repository rows.

// creditsHistoryStart is the first month with ai_credits_used in user_metrics:
// usage-based billing started on 2026-06-01.
const creditsHistoryStart = "2026-06-01"

// prHistoryStart is the first day with pull_requests.* in repository_metrics
// (the repos-1-day report went GA on 2026-07-17).
const prHistoryStart = "2026-07-17"

// CreditsPerUserMonth is the AI Credits used per active user in one month.
// Deliberately no max or high percentile: the top of the distribution is one
// or a few people.
type CreditsPerUserMonth struct {
	Month       string  `bigquery:"month" json:"month"`
	Median      float64 `bigquery:"median" json:"median"`
	Mean        float64 `bigquery:"mean" json:"mean"`
	ActiveUsers int64   `bigquery:"active_users" json:"active_users"`
}

// CopilotPRMonth is the number of pull requests Copilot coding agent created
// and Copilot code review reviewed in one month, across all repositories.
type CopilotPRMonth struct {
	Month             string `bigquery:"month" json:"month"`
	CreatedByCopilot  int64  `bigquery:"created_by_copilot" json:"created_by_copilot"`
	ReviewedByCopilot int64  `bigquery:"reviewed_by_copilot" json:"reviewed_by_copilot"`
	Days              int64  `bigquery:"days" json:"days"`
}

// GetCreditsPerUserMonthly returns median and mean AI Credits per active user
// per month. A user is active in a month with any credits, interactions or
// code generations, counted by the stable user_id. Months with fewer than minUsersForDistribution users are
// left out (k-anonymity).
func (bq *BigQueryClient) GetCreditsPerUserMonthly(ctx context.Context) ([]CreditsPerUserMonth, error) {
	metricsRef := bq.tableRef(bq.metricsDataset, "user_metrics")
	query := bq.client.Query(fmt.Sprintf(`
      WITH per_user AS (
        SELECT
          FORMAT_DATE('%%Y-%%m', day) AS month,
          JSON_VALUE(raw_record, '$.user_id') AS user_id,
          SUM(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.ai_credits_used') AS FLOAT64), 0.0)) AS credits,
          SUM(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64), 0)
            + COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64), 0)) AS activity
        FROM %s
        WHERE day >= DATE(@start) AND scope = 'enterprise'
        GROUP BY month, user_id
      )
      SELECT
        month,
        APPROX_QUANTILES(credits, 2)[OFFSET(1)] AS median,
        AVG(credits) AS mean,
        COUNT(*) AS active_users
      FROM per_user
      WHERE credits > 0 OR activity > 0
      GROUP BY month
      HAVING active_users >= @min_users
      ORDER BY month
    `, metricsRef))
	query.Parameters = []bigquery.QueryParameter{
		{Name: "start", Value: creditsHistoryStart},
		{Name: "min_users", Value: minUsersForDistribution},
	}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[CreditsPerUserMonth](it)
}

// GetCopilotPRsMonthly sums Copilot-created and Copilot-reviewed PRs per month
// from repository_metrics. Private repositories are excluded, as in
// v_repository_usage. A day is stored under one scope only, so summing across
// scopes does not double count.
func (bq *BigQueryClient) GetCopilotPRsMonthly(ctx context.Context) ([]CopilotPRMonth, error) {
	repoRef := bq.tableRef(bq.metricsDataset, "repository_metrics")
	query := bq.client.Query(fmt.Sprintf(`
      SELECT
        FORMAT_DATE('%%Y-%%m', day) AS month,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.pull_requests.total_created_by_copilot') AS INT64)), 0) AS created_by_copilot,
        COALESCE(SUM(SAFE_CAST(JSON_VALUE(raw_record, '$.pull_requests.total_reviewed_by_copilot') AS INT64)), 0) AS reviewed_by_copilot,
        COUNT(DISTINCT day) AS days
      FROM %s
      WHERE day >= DATE(@start)
        AND JSON_VALUE(raw_record, '$.repo_visibility') IN ('PUBLIC', 'INTERNAL')
      GROUP BY month
      ORDER BY month
    `, repoRef))
	query.Parameters = []bigquery.QueryParameter{{Name: "start", Value: prHistoryStart}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[CopilotPRMonth](it)
}

func (c *CachedBigQueryClient) GetCreditsPerUserMonthly(ctx context.Context) ([]CreditsPerUserMonth, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "credits_per_user_monthly", func() ([]CreditsPerUserMonth, error) {
		return c.client.GetCreditsPerUserMonthly(ctx)
	})
}

func (c *CachedBigQueryClient) GetCopilotPRsMonthly(ctx context.Context) ([]CopilotPRMonth, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "copilot_prs_monthly", func() ([]CopilotPRMonth, error) {
		return c.client.GetCopilotPRsMonthly(ctx)
	})
}

// handleCreditsPerUserMonthly handles GET /api/v1/copilot/usage/credits-per-user
func (h *BigQueryHandlers) handleCreditsPerUserMonthly(w http.ResponseWriter, r *http.Request) {
	months, err := h.bqClient.GetCreditsPerUserMonthly(r.Context())
	if err != nil {
		slog.Error("Failed to fetch credits per user", "error", err)
		respondError(w, "internal_error", "Failed to fetch credits per user", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, months, http.StatusOK)
}

// handleCopilotPRsMonthly handles GET /api/v1/copilot/usage/copilot-prs
func (h *BigQueryHandlers) handleCopilotPRsMonthly(w http.ResponseWriter, r *http.Request) {
	months, err := h.bqClient.GetCopilotPRsMonthly(r.Context())
	if err != nil {
		slog.Error("Failed to fetch Copilot PRs per month", "error", err)
		respondError(w, "internal_error", "Failed to fetch Copilot PRs per month", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, months, http.StatusOK)
}

// CohortRetention is one monthly cohort: users whose first active day in
// user_metrics falls in CohortMonth, and the share of them, in whole percent,
// active again in month +1, +3 and +6. A share is null while that month is
// not yet complete.
type CohortRetention struct {
	CohortMonth string             `bigquery:"cohort_month" json:"cohort_month"`
	CohortSize  int64              `bigquery:"cohort_size" json:"cohort_size"`
	M1          *int64 `bigquery:"m1" json:"m1"`
	M3          *int64 `bigquery:"m3" json:"m3"`
	M6          *int64 `bigquery:"m6" json:"m6"`
}

// GetCohortRetention groups users by the month of their first active day and
// returns retention per cohort. Counted per user_id in the enterprise scope
// (active as in GetCreditsPerUserMonthly), returned only as aggregates.
// Cohorts with fewer than minUsersForDistribution users are left out. The
// first data month (October 2025) is left-censored: data starts 2025-10-10,
// so it mixes earlier users with new ones and cannot tell them apart.
func (bq *BigQueryClient) GetCohortRetention(ctx context.Context) ([]CohortRetention, error) {
	metricsRef := bq.tableRef(bq.metricsDataset, "user_metrics")
	query := bq.client.Query(fmt.Sprintf(`
      WITH active AS (
        SELECT DISTINCT
          JSON_VALUE(raw_record, '$.user_id') AS user_id,
          DATE_TRUNC(day, MONTH) AS month
        FROM %s
        WHERE scope = 'enterprise'
          AND JSON_VALUE(raw_record, '$.user_id') IS NOT NULL
          AND (COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.ai_credits_used') AS FLOAT64), 0) > 0
            OR COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64), 0)
              + COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64), 0) > 0)
      ),
      cohorts AS (
        SELECT user_id, MIN(month) AS cohort FROM active GROUP BY user_id
      ),
      agg AS (
        SELECT
          c.cohort,
          COUNT(DISTINCT c.user_id) AS cohort_size,
          COUNT(DISTINCT IF(DATE_DIFF(a.month, c.cohort, MONTH) = 1, c.user_id, NULL)) AS n1,
          COUNT(DISTINCT IF(DATE_DIFF(a.month, c.cohort, MONTH) = 3, c.user_id, NULL)) AS n3,
          COUNT(DISTINCT IF(DATE_DIFF(a.month, c.cohort, MONTH) = 6, c.user_id, NULL)) AS n6
        FROM cohorts c JOIN active a USING (user_id)
        GROUP BY c.cohort
      )
      SELECT
        FORMAT_DATE('%%Y-%%m', cohort) AS cohort_month,
        cohort_size,
        -- Only complete months get a share; later ones stay NULL, not 0.
        IF(DATE_ADD(cohort, INTERVAL 1 MONTH) < DATE_TRUNC(CURRENT_DATE(), MONTH), CAST(ROUND(100 * n1 / cohort_size) AS INT64), NULL) AS m1,
        IF(DATE_ADD(cohort, INTERVAL 3 MONTH) < DATE_TRUNC(CURRENT_DATE(), MONTH), CAST(ROUND(100 * n3 / cohort_size) AS INT64), NULL) AS m3,
        IF(DATE_ADD(cohort, INTERVAL 6 MONTH) < DATE_TRUNC(CURRENT_DATE(), MONTH), CAST(ROUND(100 * n6 / cohort_size) AS INT64), NULL) AS m6
      FROM agg
      WHERE cohort_size >= @min_users
      ORDER BY cohort_month
    `, metricsRef))
	query.Parameters = []bigquery.QueryParameter{{Name: "min_users", Value: minUsersForDistribution}}
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	return readAllRows[CohortRetention](it)
}

func (c *CachedBigQueryClient) GetCohortRetention(ctx context.Context) ([]CohortRetention, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "cohort_retention", func() ([]CohortRetention, error) {
		return c.client.GetCohortRetention(ctx)
	})
}

// handleCohortRetention handles GET /api/v1/copilot/usage/cohort-retention
func (h *BigQueryHandlers) handleCohortRetention(w http.ResponseWriter, r *http.Request) {
	cohorts, err := h.bqClient.GetCohortRetention(r.Context())
	if err != nil {
		slog.Error("Failed to fetch cohort retention", "error", err)
		respondError(w, "internal_error", "Failed to fetch cohort retention", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, cohorts, http.StatusOK)
}

