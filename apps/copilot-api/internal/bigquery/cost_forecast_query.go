package bigquery

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
)

// costHistoryMonths is how far back the data goes: closed months to backtest,
// each with the month before it.
const costHistoryMonths = 4

// GetCostDays returns enterprise gross and net per day from costHistoryMonths
// before month up to yesterday (today is still being ingested).
func (bq *BigQueryClient) GetCostDays(ctx context.Context, month string) ([]CostDay, error) {
	q := bq.client.Query(fmt.Sprintf(`
SELECT FORMAT_DATE('%%Y-%%m-%%d', day) day, SUM(net_amount) net, SUM(gross_amount) gross
FROM %s
WHERE day >= DATE_SUB(DATE(@month || '-01'), INTERVAL @back MONTH)
  AND day < LEAST(DATE_ADD(DATE(@month || '-01'), INTERVAL 1 MONTH), CURRENT_DATE())
  AND LOWER(scope_id) = 'nav' AND product = 'Copilot' AND gross_quantity > 0
GROUP BY day ORDER BY day`, bq.tableRef(bq.metricsDataset, "billing_usage_daily_model")))
	q.Parameters = []bigquery.QueryParameter{{Name: "month", Value: month}, {Name: "back", Value: costHistoryMonths}}
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read cost days: %w", err)
	}
	return ReadAllRows[CostDay](it)
}

// GetCostUserDays returns gross per user and day for the same period as GetCostDays.
func (bq *BigQueryClient) GetCostUserDays(ctx context.Context, month string) ([]UserDay, error) {
	q := bq.client.Query(fmt.Sprintf(`
SELECT JSON_VALUE(raw_record, '$.user_id') user_id, FORMAT_DATE('%%Y-%%m-%%d', day) day,
  COALESCE(SAFE_CAST(JSON_VALUE(raw_record, '$.ai_credits_used') AS FLOAT64), 0) * 0.01 gross
FROM %s
WHERE day >= DATE_SUB(DATE(@month || '-01'), INTERVAL @back MONTH)
  AND day < LEAST(DATE_ADD(DATE(@month || '-01'), INTERVAL 1 MONTH), CURRENT_DATE())`,
		userDaysFrom(bq.tableRef(bq.metricsDataset, "user_metrics"))))
	q.Parameters = []bigquery.QueryParameter{{Name: "month", Value: month}, {Name: "back", Value: costHistoryMonths}}
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read cost user days: %w", err)
	}
	return ReadAllRows[UserDay](it)
}

func (c *CachedBigQueryClient) GetCostDays(ctx context.Context, month string) ([]CostDay, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "cost_days_"+month, func() ([]CostDay, error) {
		return c.client.GetCostDays(ctx, month)
	})
}

func (c *CachedBigQueryClient) GetCostUserDays(ctx context.Context, month string) ([]UserDay, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "cost_user_days_"+month, func() ([]UserDay, error) {
		return c.client.GetCostUserDays(ctx, month)
	})
}
