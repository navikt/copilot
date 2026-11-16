package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
)

// UsageQualityDay contains independent daily counts, not unique users over a period.
type UsageQualityDay struct {
	Day                      string `bigquery:"day" json:"day"`
	Scope                    string `bigquery:"scope" json:"scope"`
	ScopeID                  string `bigquery:"scope_id" json:"scope_id"`
	MissingClientTelemetry   *int64 `bigquery:"missing_client_telemetry_user_count" json:"missing_client_telemetry_user_count"`
	BelowUsageMetricsMinimum *int64 `bigquery:"below_usage_metrics_minimum_user_count" json:"below_usage_metrics_minimum_user_count"`
}

func usageQualitySQL(table string) string {
	// Select the Nav source before extraction; unknown enterprise counts never borrow organization counts.
	return fmt.Sprintf(`
WITH selected AS (
  SELECT day, scope, scope_id, raw_record FROM %s
  WHERE day >= DATE_SUB(CURRENT_DATE(), INTERVAL 27 DAY)
    AND ((scope = 'enterprise' AND scope_id = 'nav')
      OR (scope = 'organization' AND scope_id = 'navikt'))
  QUALIFY ROW_NUMBER() OVER (
    PARTITION BY day ORDER BY IF(scope = 'enterprise', 0, 1), loaded_at DESC, scope_id,
      TO_JSON_STRING(raw_record) DESC
  ) = 1
), extracted AS (
SELECT day, scope, scope_id,
  IF(JSON_TYPE(JSON_QUERY(raw_record, '$.missing_client_telemetry_user_count')) = 'number',
    SAFE_CAST(JSON_VALUE(raw_record, '$.missing_client_telemetry_user_count') AS BIGNUMERIC), NULL) missing,
  IF(JSON_TYPE(JSON_QUERY(raw_record, '$.below_usage_metrics_minimum_user_count')) = 'number',
    SAFE_CAST(JSON_VALUE(raw_record, '$.below_usage_metrics_minimum_user_count') AS BIGNUMERIC), NULL) below
FROM selected
)
SELECT CAST(day AS STRING) day, scope, scope_id,
  IF(missing >= 0 AND missing = TRUNC(missing), SAFE_CAST(missing AS INT64), NULL)
    missing_client_telemetry_user_count,
  IF(below >= 0 AND below = TRUNC(below), SAFE_CAST(below AS INT64), NULL)
    below_usage_metrics_minimum_user_count
FROM extracted ORDER BY day`, table)
}

func (bq *BigQueryClient) GetUsageQuality(ctx context.Context) ([]UsageQualityDay, error) {
	it, err := bq.client.Query(usageQualitySQL(bq.tableRef(bq.metricsDataset, bq.metricsTable))).Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read usage quality: %w", err)
	}
	return readAllRows[UsageQualityDay](it)
}

func (c *CachedBigQueryClient) GetUsageQuality(ctx context.Context) ([]UsageQualityDay, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "usage_quality", func() ([]UsageQualityDay, error) {
		return c.client.GetUsageQuality(ctx)
	})
}

func (h *BigQueryHandlers) handleUsageQuality(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetUsageQuality(r.Context())
	if err != nil {
		slog.Error("Failed to fetch usage quality", "error", err)
		respondError(w, "internal_error", "Failed to fetch usage quality", http.StatusInternalServerError)
		return
	}
	if rows == nil {
		rows = []UsageQualityDay{}
	}
	cacheControl(w, 3600, false)
	respondJSON(w, rows, http.StatusOK)
}
