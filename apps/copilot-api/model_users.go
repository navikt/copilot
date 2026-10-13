package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
)

// ModelFamilyUsers is the number of distinct users of one model family in one
// month, for M4 on the admin cost page (cost per user per family). Admin only.
type ModelFamilyUsers struct {
	Month  string `bigquery:"month" json:"month"`
	Family string `bigquery:"family" json:"family"`
	Users  int    `bigquery:"users" json:"users"`
}

// modelFamilySQL maps a model name to the family keys of my-copilot's
// model-family.ts with costPage set. Keep the two in sync. Usage data also spells some models
// «claude-4.6-sonnet», so Claude families match anywhere in the name.
const modelFamilySQL = `CASE
  WHEN STARTS_WITH(n, 'auto') THEN 'auto'
  WHEN n = 'code-review-model' THEN 'kodegjennomgang'
  WHEN STRPOS(n, 'claude') > 0 AND STRPOS(n, 'opus') > 0 THEN 'claude_opus'
  WHEN STRPOS(n, 'claude') > 0 AND STRPOS(n, 'sonnet') > 0 THEN 'claude_sonnet'
  WHEN STRPOS(n, 'claude') > 0 AND STRPOS(n, 'haiku') > 0 THEN 'claude_haiku'
  WHEN STRPOS(n, 'claude') > 0 AND STRPOS(n, 'fable') > 0 THEN 'claude_fable'
  WHEN REGEXP_CONTAINS(n, r'^gpt-.*(mini|nano|luna)') THEN 'gpt_mini'
  WHEN STARTS_WITH(n, 'gpt-') THEN 'gpt'
  WHEN STARTS_WITH(n, 'gemini') THEN 'gemini'
  ELSE 'andre' END`

// GetModelFamilyUsers counts distinct users per model family per month, for
// the last three months including the current one.
func (bq *BigQueryClient) GetModelFamilyUsers(ctx context.Context) ([]ModelFamilyUsers, error) {
	q := bq.client.Query(fmt.Sprintf(`
SELECT month, %s family, COUNT(DISTINCT user_id) users FROM (
  SELECT FORMAT_DATE('%%Y-%%m', day) month, JSON_VALUE(raw_record, '$.user_id') user_id,
    REGEXP_REPLACE(LOWER(TRIM(JSON_VALUE(x, '$.model'))), r'[\s_]+', '-') n
  FROM %s, UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_model_feature')) x
  WHERE day >= DATE_SUB(DATE_TRUNC(CURRENT_DATE(), MONTH), INTERVAL 2 MONTH))
WHERE n IS NOT NULL AND n != 'unknown'
GROUP BY 1, 2 ORDER BY 1, 2`, modelFamilySQL, userDaysFrom(bq.tableRef(bq.metricsDataset, "user_metrics"))))
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read model family users: %w", err)
	}
	return readAllRows[ModelFamilyUsers](it)
}

func (c *CachedBigQueryClient) GetModelFamilyUsers(ctx context.Context) ([]ModelFamilyUsers, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "admin_model_family_users", func() ([]ModelFamilyUsers, error) {
		return c.client.GetModelFamilyUsers(ctx)
	})
}

func (h *BigQueryHandlers) handleModelFamilyUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetModelFamilyUsers(r.Context())
	if err != nil {
		slog.Error("Failed to fetch model family users", "error", err)
		respondError(w, "internal_error", "Failed to fetch model family users", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, rows, http.StatusOK)
}
