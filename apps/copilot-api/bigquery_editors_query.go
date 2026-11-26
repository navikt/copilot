package main

import (
	"context"
	"fmt"

	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
)

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
	return bqstore.ReadAllRows[EditorUsage](it)
}

func (c *CachedBigQueryClient) GetEditorUsage(ctx context.Context) ([]EditorUsage, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "editor_usage", func() ([]EditorUsage, error) { return c.client.GetEditorUsage(ctx) })
}
