package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
)

type EditorUsage struct {
	Editor       string `bigquery:"editor" json:"editor"`
	FirstDay     string `bigquery:"first_day" json:"first_day"`
	LastDay      string `bigquery:"last_day" json:"last_day"`
	Days         int64  `bigquery:"days" json:"days"`
	Generations  *int64 `bigquery:"generations" json:"generations"`
	Acceptances  *int64 `bigquery:"acceptances" json:"acceptances"`
	Interactions *int64 `bigquery:"interactions" json:"interactions"`
}

func editorUsageQuery(table string) string {
	return fmt.Sprintf(`WITH daily AS (
		SELECT day, raw_record FROM %s
		WHERE scope = 'enterprise' AND LOWER(scope_id) = 'nav'
		QUALIFY ROW_NUMBER() OVER (
			PARTITION BY day, scope, LOWER(scope_id) ORDER BY loaded_at DESC
		) = 1
	), editors AS (
		SELECT day, JSON_VALUE(ide, '$.ide') AS editor,
			SAFE_CAST(JSON_VALUE(ide, '$.code_generation_activity_count') AS INT64) AS generations,
			SAFE_CAST(JSON_VALUE(ide, '$.code_acceptance_activity_count') AS INT64) AS acceptances,
			SAFE_CAST(JSON_VALUE(ide, '$.user_initiated_interaction_count') AS INT64) AS interactions
		FROM daily, UNNEST(JSON_QUERY_ARRAY(raw_record, '$.totals_by_ide')) AS ide
		WHERE day > DATE_SUB((SELECT MAX(day) FROM daily), INTERVAL 28 DAY)
			AND JSON_VALUE(ide, '$.ide') IS NOT NULL
	)
	SELECT editor, CAST(MIN(day) AS STRING) AS first_day,
		CAST(MAX(day) AS STRING) AS last_day, COUNT(DISTINCT day) AS days,
		IF(COUNT(generations) = COUNT(*), SUM(generations), NULL) AS generations,
		IF(COUNT(acceptances) = COUNT(*), SUM(acceptances), NULL) AS acceptances,
		IF(COUNT(interactions) = COUNT(*), SUM(interactions), NULL) AS interactions
		FROM editors
		GROUP BY editor ORDER BY generations DESC, editor`, table)
}

func (bq *BigQueryClient) GetEditorUsage(ctx context.Context) ([]EditorUsage, error) {
	it, err := bq.client.Query(editorUsageQuery(bq.tableRef(bq.metricsDataset, bq.metricsTable))).Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute editor usage query: %w", err)
	}
	return readAllRows[EditorUsage](it)
}

func (c *CachedBigQueryClient) GetEditorUsage(ctx context.Context) ([]EditorUsage, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "editor_usage", func() ([]EditorUsage, error) { return c.client.GetEditorUsage(ctx) })
}

func (h *BigQueryHandlers) handleEditorUsage(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetEditorUsage(r.Context())
	if err != nil {
		slog.Error("Failed to fetch editor usage", "error", err)
		respondError(w, "internal_error", "Failed to fetch editor usage", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	if rows == nil {
		rows = []EditorUsage{}
	}
	respondJSON(w, rows, http.StatusOK)
}
