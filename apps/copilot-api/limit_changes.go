package main

import (
	"context"
	"fmt"
)

// LimitChange is a new default personal spending limit from Date (YYYY-MM-DD).
type LimitChange struct {
	Date string  `json:"date"`
	USD  float64 `json:"usd"`
}

// GetLimitSnapshots returns the enterprise default personal limit per
// snapshot day from budget_snapshots, oldest first.
func (bq *BigQueryClient) GetLimitSnapshots(ctx context.Context) ([]LimitChange, error) {
	q := bq.client.Query(fmt.Sprintf(`
SELECT FORMAT_DATE('%%Y-%%m-%%d', snapshot_date) date, MAX(budget_amount) usd
FROM %s
WHERE scope_id = 'nav' AND budget_scope = 'multi_user_customer'
GROUP BY snapshot_date ORDER BY snapshot_date`, bq.tableRef(bq.metricsDataset, "budget_snapshots")))
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read limit snapshots: %w", err)
	}
	return readAllRows[LimitChange](it)
}

func (c *CachedBigQueryClient) GetLimitSnapshots(ctx context.Context) ([]LimitChange, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "limit_snapshots", func() ([]LimitChange, error) {
		return c.client.GetLimitSnapshots(ctx)
	})
}

// limitChanges lists the days the default personal limit changed. Days
// before the first snapshot use the documented spendLimits, as budget_share
// does; from then on a change is a snapshot that differs from the one before.
func limitChanges(snaps []LimitChange) []LimitChange {
	out := []LimitChange{}
	first := ""
	for _, s := range snaps {
		if s.USD > 0 {
			first = s.Date
			break
		}
	}
	for _, l := range spendLimits {
		if first == "" || l.from < first {
			out = append(out, LimitChange{Date: l.from, USD: l.usd})
		}
	}
	for _, s := range snaps {
		if s.USD > 0 && (len(out) == 0 || out[len(out)-1].USD != s.USD) {
			out = append(out, s)
		}
	}
	return out
}
