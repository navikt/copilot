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
// code generations. Months with fewer than minUsersForDistribution users are
// left out (k-anonymity).
func (bq *BigQueryClient) GetCreditsPerUserMonthly(ctx context.Context) ([]CreditsPerUserMonth, error) {
	metricsRef := bq.tableRef(bq.metricsDataset, "user_metrics")
	query := bq.client.Query(fmt.Sprintf(`
      WITH per_user AS (
        SELECT
          FORMAT_DATE('%%Y-%%m', day) AS month,
          JSON_VALUE(raw_record, '$.user_login') AS user_login,
          SUM(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.ai_credits_used') AS FLOAT64), 0.0)) AS credits,
          SUM(COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.user_initiated_interaction_count') AS INT64), 0)
            + COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.code_generation_activity_count') AS INT64), 0)) AS activity
        FROM %s
        WHERE day >= DATE(@start) AND scope = 'enterprise'
        GROUP BY month, user_login
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
