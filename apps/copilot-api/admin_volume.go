package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// VolumeMonth is Nav's Copilot volume for one month, for the innsyn answer
// (question 1b). Credits and premium requests come from the same billed SKUs
// as the admin monthly overview. GitHub reports tokens only for Copilot CLI
// and the Copilot app, for the whole enterprise and without a model split.
type VolumeMonth struct {
	Month           string `bigquery:"month" json:"month"`
	AICredits       int64  `bigquery:"ai_credits" json:"ai_credits"`
	PremiumRequests int64  `bigquery:"premium_requests" json:"premium_requests"`
	CLITokens       int64  `bigquery:"cli_tokens" json:"cli_tokens"`
	AppTokens       int64  `bigquery:"app_tokens" json:"app_tokens"`
}

func (bq *BigQueryClient) GetVolumeMonthly(ctx context.Context, year int) ([]VolumeMonth, error) {
	tokens := func(path string) string {
		return fmt.Sprintf(`SUM(IFNULL(SAFE_CAST(JSON_VALUE(raw_record,'$.%[1]s.token_usage.prompt_tokens_sum') AS INT64),0)
  +IFNULL(SAFE_CAST(JSON_VALUE(raw_record,'$.%[1]s.token_usage.output_tokens_sum') AS INT64),0))`, path)
	}
	q := bq.client.Query(fmt.Sprintf(`
WITH v AS (
 SELECT DATE_TRUNC(day,MONTH) month,
  CAST(ROUND(SUM(IF(unit_type='ai-credits',gross_quantity,0))) AS INT64) ai_credits,
  CAST(ROUND(SUM(IF(unit_type='requests',gross_quantity,0))) AS INT64) premium_requests
 FROM %[1]s WHERE day>=DATE(@from) AND day<DATE(@to) AND scope_id='nav' AND product='Copilot' AND sku IN (`+billedSKUs+`)
 GROUP BY month
), t AS (
 SELECT DATE_TRUNC(day,MONTH) month,%[3]s cli_tokens,%[4]s app_tokens
 FROM %[2]s WHERE day>=DATE(@from) AND day<DATE(@to) AND scope='enterprise' AND scope_id='nav'
 GROUP BY month
)
SELECT FORMAT_DATE('%%Y-%%m',month) month,IFNULL(v.ai_credits,0) ai_credits,IFNULL(v.premium_requests,0) premium_requests,
 IFNULL(t.cli_tokens,0) cli_tokens,IFNULL(t.app_tokens,0) app_tokens
FROM v FULL JOIN t USING (month) ORDER BY month`,
		bq.tableRef(bq.metricsDataset, "billing_usage_daily_model"),
		bq.tableRef(bq.metricsDataset, bq.metricsTable),
		tokens("totals_by_cli"), tokens("totals_by_copilot_app")))
	q.Parameters = monthRange(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), 12)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read volume: %w", err)
	}
	return readAllRows[VolumeMonth](it)
}

func (c *CachedBigQueryClient) GetVolumeMonthly(ctx context.Context, year int) ([]VolumeMonth, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "admin_volume_"+strconv.Itoa(year), func() ([]VolumeMonth, error) {
		return c.client.GetVolumeMonthly(ctx, year)
	})
}

// handleAdminVolume handles GET /api/v1/admin/billing/volume?year=.
func (h *BigQueryHandlers) handleAdminVolume(w http.ResponseWriter, r *http.Request) {
	thisYear := time.Now().UTC().Year()
	year, ok := optionalIntParam(r, "year", thisYear, 2026, thisYear)
	if !ok {
		respondError(w, "invalid_parameter", fmt.Sprintf("year must be between 2026 and %d", thisYear), http.StatusBadRequest)
		return
	}
	rows, err := h.bqClient.GetVolumeMonthly(r.Context(), year)
	if err != nil {
		slog.Error("Failed to fetch volume", "error", err)
		respondError(w, "internal_error", "Failed to fetch volume", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, rows, http.StatusOK)
}
