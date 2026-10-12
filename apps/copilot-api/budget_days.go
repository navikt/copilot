package main

import (
	"context"
	"log/slog"
	"net/http"
)

// F7: users per bucket of the share of their personal limit used, per day
// (K20). Aggregates only: buckets under minBandUsers are merged.

// BudgetDay holds the budget bands for one day. Bands is nil below five users.
// Snapshots counts users with GitHub's own figure (0 below five); the rest are estimates.
type BudgetDay struct {
	Day       string      `json:"day"`
	Users     int         `json:"users"`
	Snapshots int         `json:"snapshots"`
	Bands     []SpendBand `json:"bands"`
}

type budgetBucketRow struct {
	Day       string `bigquery:"day"`
	Bucket    int    `bigquery:"bucket"`
	Users     int    `bigquery:"users"`
	Snapshots int    `bigquery:"snapshots"`
}

// buildBudgetDays turns bucket counts, ordered by day, into one BudgetDay per day.
func buildBudgetDays(rows []budgetBucketRow) []BudgetDay {
	out := []BudgetDay{}
	for i := 0; i < len(rows); {
		d := BudgetDay{Day: rows[i].Day}
		bands := make([]SpendBand, len(budgetEdges)+1)
		for k := range bands {
			bands[k] = SpendBand{First: k, Last: k}
		}
		for ; i < len(rows) && rows[i].Day == d.Day; i++ {
			bands[rows[i].Bucket].Users += rows[i].Users
			d.Users += rows[i].Users
			d.Snapshots += rows[i].Snapshots
		}
		if d.Users < minBandUsers {
			d.Users, d.Snapshots = 0, 0
		} else {
			d.Bands = mergeSmallBands(bands)
			for k := range d.Bands {
				d.Bands[k].Label = budgetBandLabel(d.Bands[k].First, d.Bands[k].Last)
			}
		}
		// Hide snapshots when either it or the estimates it leaves are under five.
		if d.Snapshots < minBandUsers || d.Users-d.Snapshots > 0 && d.Users-d.Snapshots < minBandUsers {
			d.Snapshots = 0
		}
		out = append(out, d)
	}
	return out
}

// GetBudgetDays buckets budget_share per day from budgetShareFrom. RANGE_BUCKET
// counts the lower edge in: exactly 10 % lands in 10–25 %.
func (bq *BigQueryClient) GetBudgetDays(ctx context.Context) ([]BudgetDay, error) {
	it, err := bq.client.Query(`WITH ` + bq.budgetShareCTE() + `
SELECT CAST(day AS STRING) day,
  RANGE_BUCKET(ROUND(share * 100, 6), ARRAY<FLOAT64>[10, 25, 50, 75, 90, 100]) bucket,
  COUNT(*) users, COUNTIF(source = 'snapshot') snapshots
FROM budget_share WHERE share IS NOT NULL
GROUP BY 1, 2 ORDER BY 1, 2`).Read(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := readAllRows[budgetBucketRow](it)
	if err != nil {
		return nil, err
	}
	return buildBudgetDays(rows), nil
}

func (c *CachedBigQueryClient) GetBudgetDays(ctx context.Context) ([]BudgetDay, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "admin_budget_days", func() ([]BudgetDay, error) {
		return c.client.GetBudgetDays(ctx)
	})
}

// The route must sit behind requireAdmin.
func (h *BigQueryHandlers) handleAdminBudgetDays(w http.ResponseWriter, r *http.Request) {
	days, err := h.bqClient.GetBudgetDays(r.Context())
	if err != nil {
		slog.Error("Failed to fetch budget days", "error", err)
		respondError(w, "internal_error", "Failed to fetch budget days", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, days, http.StatusOK)
}
