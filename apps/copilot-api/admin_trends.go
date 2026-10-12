package main

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"slices"
	"time"
)

// Cost trends for the admin cost page (F3 and F8 in the cost insight PRD).
// Every response is an aggregate: no user names or IDs leave copilot-api.

// costTrendsFirstMonth is the first month with per-user billing (premium requests).
const costTrendsFirstMonth = "2026-01-01"

// basisForMonth names what a billed month's amount rests on: premium requests
// before June 2026, AI credits (net) from then.
func basisForMonth(month string) string {
	if month < "2026-06" {
		return "premium"
	}
	return "netto"
}

// ActiveUsersMonth is the number of users with at least one Copilot usage day
// in the month, the divisor for cost per active user (K9).
type ActiveUsersMonth struct {
	Month       string `bigquery:"month" json:"month"`
	ActiveUsers int64  `bigquery:"active_users" json:"active_users"`
}

// GetActiveUsersMonthly counts active users per month from January 2026 up to
// and including the current month.
func (bq *BigQueryClient) GetActiveUsersMonthly(ctx context.Context) ([]ActiveUsersMonth, error) {
	first, _ := time.Parse("2006-01-02", costTrendsFirstMonth)
	now := time.Now().UTC()
	months := (now.Year()-first.Year())*12 + int(now.Month()-first.Month()) + 1
	// userDaysFrom, not teamUsageCTEs: days the metrics job fetched from the
	// organization endpoint are stored under that scope and must count too.
	q := bq.client.Query(fmt.Sprintf(`
SELECT FORMAT_DATE('%%Y-%%m',DATE_TRUNC(day,MONTH)) month,
  COUNT(DISTINCT JSON_VALUE(raw_record,'$.user_id')) active_users
FROM %s WHERE day>=DATE(@from) AND day<DATE(@to)
GROUP BY month ORDER BY month`, userDaysFrom(bq.tableRef(bq.metricsDataset, "user_metrics"))))
	q.Parameters = monthRange(first, months)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read active users: %w", err)
	}
	return readAllRows[ActiveUsersMonth](it)
}

func (c *CachedBigQueryClient) GetActiveUsersMonthly(ctx context.Context) ([]ActiveUsersMonth, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "admin_active_users", func() ([]ActiveUsersMonth, error) {
		return c.client.GetActiveUsersMonthly(ctx)
	})
}

// The route must sit behind requireAdmin.
func (h *BigQueryHandlers) handleAdminActiveUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.bqClient.GetActiveUsersMonthly(r.Context())
	if err != nil {
		slog.Error("Failed to fetch active users", "error", err)
		respondError(w, "internal_error", "Failed to fetch active users", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, rows, http.StatusOK)
}

// topTiers are the high-consumer groups, as top percent of billed users (K22).
var topTiers = []int{1, 5, 10}

// budgetEdges are the owner's buckets for the share of the personal limit
// used (S15): 0–10, 10–25, 25–50, 50–75, 75–90, 90–100 and over 100 %.
var budgetEdges = []float64{10, 25, 50, 75, 90, 100}

// HighConsumerTier is the top TopPct % of billed users by net. Users and
// NetSharePct are hidden (0 and nil) when the group has fewer than five people.
type HighConsumerTier struct {
	TopPct      int      `json:"top_pct"`
	Users       int      `json:"users"`
	NetSharePct *float64 `json:"net_share_pct"`
}

type HighConsumerMonth struct {
	Month string `json:"month"`
	Basis string `json:"basis"`
	// Users is every user billed in the month, also those with zero net.
	Users int                `json:"users"`
	Tiers []HighConsumerTier `json:"tiers"`
	// LimitUSD is the personal limit at month end; 0 before limits existed.
	LimitUSD     float64 `json:"limit_usd"`
	LimitChanged bool    `json:"limit_changed"`
	// LastDay is set on the month-to-date entry only (Basis "hittil"): the day
	// the buckets are for. Snapshots counts users with GitHub's own figure, 0 below five.
	LastDay   string `json:"last_day,omitempty"`
	Snapshots int    `json:"snapshots,omitempty"`
	// BudgetBands count users by gross for the month as a share of LimitUSD.
	// Nil without a limit or with fewer than five users; small bands are merged.
	BudgetBands []SpendBand `json:"budget_bands"`
}

func budgetBandLabel(first, last int) string {
	lo := 0.0
	if first > 0 {
		lo = budgetEdges[first-1]
	}
	if last == len(budgetEdges) {
		return fmt.Sprintf("Over %g %%", lo)
	}
	return fmt.Sprintf("%g–%g %%", lo, budgetEdges[last])
}

// buildHighConsumers groups per-user rows by month. Rows must be ordered by month.
func buildHighConsumers(rows []spendUserRow) []HighConsumerMonth {
	out := []HighConsumerMonth{}
	for i := 0; i < len(rows); {
		j := i
		for j < len(rows) && rows[j].Month == rows[i].Month {
			j++
		}
		out = append(out, highConsumerMonth(rows[i].Month, rows[i:j]))
		i = j
	}
	return out
}

func highConsumerMonth(month string, rows []spendUserRow) HighConsumerMonth {
	m := HighConsumerMonth{Month: month, Basis: basisForMonth(month), Tiers: []HighConsumerTier{},
		LimitUSD: limitAtMonthEnd(month), LimitChanged: limitChanged(month)}
	nets := make([]float64, len(rows))
	gross := make([]float64, len(rows))
	total := 0.0
	for i, r := range rows {
		// Credits can make a user's net negative; count them as zero so shares stay in 0–100 %.
		nets[i], gross[i] = max(r.Net, 0), r.Gross
		total += nets[i]
	}
	if len(rows) < minBandUsers {
		return m
	}
	m.Users = len(rows)
	slices.SortFunc(nets, func(a, b float64) int { return cmp.Compare(b, a) })
	// A tier is shown only if it and its gap to the previous shown tier hold at
	// least minBandUsers, so differencing two tiers never isolates fewer people.
	shown := 0
	for _, p := range topTiers {
		k := int(math.Ceil(float64(len(nets)) * float64(p) / 100))
		t := HighConsumerTier{TopPct: p}
		if k-shown >= minBandUsers && total > 0 {
			shown = k
			sum := 0.0
			for _, v := range nets[:k] {
				sum += v
			}
			share := math.Round(sum/total*1000) / 10
			t.Users, t.NetSharePct = k, &share
		}
		m.Tiers = append(m.Tiers, t)
	}
	if m.LimitUSD > 0 {
		m.BudgetBands = mergeEdgeBands(gross, m.LimitUSD, budgetEdges)
		for i := range m.BudgetBands {
			m.BudgetBands[i].Label = budgetBandLabel(m.BudgetBands[i].First, m.BudgetBands[i].Last)
		}
	}
	return m
}

type budgetShareRow struct {
	Day    string  `bigquery:"day"`
	Share  float64 `bigquery:"share"`
	Source string  `bigquery:"source"`
}

// monthToDate buckets each user's month-to-date share of their own limit on
// the last day with data. No projection: that waits for the per-person model.
func monthToDate(rows []budgetShareRow) *HighConsumerMonth {
	if len(rows) == 0 {
		return nil
	}
	m := &HighConsumerMonth{Month: rows[0].Day[:7], Basis: "hittil", LastDay: rows[0].Day, Tiers: []HighConsumerTier{}}
	shares := make([]float64, len(rows))
	snaps := 0
	for i, r := range rows {
		shares[i] = r.Share
		if r.Source == "snapshot" {
			snaps++
		}
	}
	m.BudgetBands = mergeEdgeBands(shares, 1, budgetEdges)
	if m.BudgetBands == nil {
		return m
	}
	m.Users = len(rows)
	if snaps >= minBandUsers {
		m.Snapshots = snaps
	}
	for i := range m.BudgetBands {
		m.BudgetBands[i].Label = budgetBandLabel(m.BudgetBands[i].First, m.BudgetBands[i].Last)
	}
	return m
}

// GetHighConsumers returns high-consumer groups and personal budget bands per
// complete month, from the same per-user history as the spend bands, then
// the current month to date from budget_share.
func (bq *BigQueryClient) GetHighConsumers(ctx context.Context) ([]HighConsumerMonth, error) {
	rows, err := bq.spendUserHistory(ctx, costTrendsFirstMonth)
	if err != nil {
		return nil, err
	}
	out := buildHighConsumers(rows)
	it, err := bq.client.Query(`WITH ` + bq.budgetShareCTE() + `
SELECT CAST(day AS STRING) day, share, source FROM budget_share
WHERE share IS NOT NULL AND day = (SELECT MAX(day) FROM budget_share) AND day >= DATE_TRUNC(CURRENT_DATE(), MONTH)`).Read(ctx)
	var shares []budgetShareRow
	if err == nil {
		shares, err = readAllRows[budgetShareRow](it)
	}
	if err != nil {
		// The billed months still stand without the month to date.
		slog.Error("Failed to read budget share", "error", err)
		return out, nil
	}
	if m := monthToDate(shares); m != nil {
		out = append(out, *m)
	}
	return out, nil
}

func (c *CachedBigQueryClient) GetHighConsumers(ctx context.Context) ([]HighConsumerMonth, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "admin_high_consumers", func() ([]HighConsumerMonth, error) {
		return c.client.GetHighConsumers(ctx)
	})
}

// The route must sit behind requireAdmin.
func (h *BigQueryHandlers) handleAdminHighConsumers(w http.ResponseWriter, r *http.Request) {
	months, err := h.bqClient.GetHighConsumers(r.Context())
	if err != nil {
		slog.Error("Failed to fetch high consumers", "error", err)
		respondError(w, "internal_error", "Failed to fetch high consumers", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, months, http.StatusOK)
}
