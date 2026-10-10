package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"cloud.google.com/go/bigquery"
)

// Internal: the standard per-person spending limit (copilot-intern README,
// «Forbruksgrense per person»). Only this internal endpoint uses it; it never
// goes into my-copilot source. user_budget_snapshots is empty, so per-person
// exceptions are not known.
var spendLimits = []struct {
	from string // first day in force, YYYY-MM-DD
	usd  float64
}{
	{"2026-06-01", 200},
	{"2026-07-01", 400},
	{"2026-09-22", 800},
}

// limitChanged reports whether the limit changed during month.
func limitChanged(month string) bool {
	first, _ := time.Parse("2006-01", month)
	return limitAtMonthEnd(first.AddDate(0, -1, 0).Format("2006-01")) != limitAtMonthEnd(month)
}

// limitAtMonthEnd returns the limit in force on the last day of month (YYYY-MM).
// A month with a change uses the limit at month end.
func limitAtMonthEnd(month string) float64 {
	first, _ := time.Parse("2006-01", month)
	last := first.AddDate(0, 1, -1).Format("2006-01-02")
	limit := 0.0
	for _, l := range spendLimits {
		if l.from <= last {
			limit = l.usd
		}
	}
	return limit
}

// Base bands, as share of the limit: 0, (0,25], (25,50], (50,75], (75,90], (90,100], >100.
var bandEdges = []float64{0, 25, 50, 75, 90, 100}

func bandIndex(net, limit float64) int {
	if net < 0.005 {
		return 0
	}
	pct := net / limit * 100
	for i := 1; i < len(bandEdges); i++ {
		if pct <= bandEdges[i] {
			return i
		}
	}
	return len(bandEdges)
}

// SpendBand is one (possibly merged) band. First and Last index the base bands
// 0..6, so the page can colour and label merged bands.
type SpendBand struct {
	First int    `json:"first"`
	Last  int    `json:"last"`
	Label string `json:"label"`
	Users int    `json:"users"`
}

func bandLabel(first, last int) string {
	lo := []string{"0", "1", "25", "50", "75", "90", "100"}[first]
	switch {
	case last == 0:
		return "0\u00a0%"
	case first == 1 && last == 1:
		return "under 25\u00a0%"
	case last == len(bandEdges):
		return "over " + lo + "\u00a0%"
	default:
		return fmt.Sprintf("%s–%g\u00a0%%", lo, bandEdges[last])
	}
}

const minBandUsers = 5

// mergeBands counts users per base band, then merges any band with 1–4 users
// into its smaller non-empty neighbour (the lower one on a tie) until every band has
// zero or at least minBandUsers. Returns nil if all users together are too few.
func mergeBands(nets []float64, limit float64) []SpendBand {
	if len(nets) < minBandUsers {
		return nil
	}
	bands := make([]SpendBand, len(bandEdges)+1)
	for i := range bands {
		bands[i] = SpendBand{First: i, Last: i}
	}
	for _, n := range nets {
		bands[bandIndex(n, limit)].Users++
	}
	for {
		small := -1
		for i, b := range bands {
			if b.Users > 0 && b.Users < minBandUsers {
				small = i
				break
			}
		}
		if small < 0 {
			break
		}
		// Prefer the smaller non-empty neighbour, so empty bands are not swept in needlessly.
		weight := func(i int) int {
			if i < 0 || i >= len(bands) {
				return math.MaxInt
			}
			if bands[i].Users == 0 {
				return math.MaxInt - 1
			}
			return bands[i].Users
		}
		other := small - 1
		if weight(small+1) < weight(other) {
			other = small + 1
		}
		lo, hi := min(small, other), max(small, other)
		bands[lo] = SpendBand{First: bands[lo].First, Last: bands[hi].Last, Users: bands[lo].Users + bands[hi].Users}
		bands = append(bands[:hi], bands[hi+1:]...)
	}
	for i := range bands {
		bands[i].Label = bandLabel(bands[i].First, bands[i].Last)
	}
	return bands
}

type SpendBandMonth struct {
	Month    string  `json:"month"`
	LimitUSD float64 `json:"limit_usd"`
	// LimitChanged is true when the limit changed during the month; LimitUSD is the one at month end.
	LimitChanged bool        `json:"limit_changed"`
	Users        int         `json:"users"`
	Bands        []SpendBand `json:"bands"`
}

type SpendForecastMonth struct {
	Month   string  `json:"month"`
	LowUSD  float64 `json:"low_usd"`
	MidUSD  float64 `json:"mid_usd"`
	HighUSD float64 `json:"high_usd"`
}

type SpendBands struct {
	Months []SpendBandMonth `json:"months"`
	// Current is the current month projected to month end; nil without usage.
	Current *SpendBandMonth `json:"current"`
	// Days and DaysInMonth say how much of the current month the projection rests on.
	Days        int     `json:"days"`
	DaysInMonth int     `json:"days_in_month"`
	LastDay     string  `json:"last_day"`
	NetRatio    float64 `json:"net_ratio"`
	// RunWeight is the weight on this month's run rate; the rest is last month's net per user.
	RunWeight float64 `json:"run_weight"`
	// TotalsUSD are billed (complete months) or projected (current month) totals, rounded to 100 USD.
	Totals   []SpendForecastMonth `json:"totals"`
	Forecast []SpendForecastMonth `json:"forecast"`
}

type spendUserRow struct {
	UserID string  `bigquery:"user_id"`
	Month  string  `bigquery:"month"`
	Net    float64 `bigquery:"net"`
	Gross  float64 `bigquery:"gross"`
	// LastDay is set on current-month rows only: the last day with usage.
	LastDay string `bigquery:"last_day"`
}

func round100(v float64) float64 { return math.Round(v/100) * 100 }

// buildSpendBands turns per-user rows into band counts and forecasts. history
// holds billed net and gross per user for complete months; current holds gross
// so far per user in the current month, up to lastDay.
func buildSpendBands(history, current []spendUserRow, lastDay string) *SpendBands {
	out := &SpendBands{Months: []SpendBandMonth{}, Totals: []SpendForecastMonth{}, Forecast: []SpendForecastMonth{}}
	byMonth := map[string][]float64{}
	var order []string
	var totals []float64
	var net, gross float64
	for _, r := range history {
		if _, ok := byMonth[r.Month]; !ok {
			order = append(order, r.Month)
			totals = append(totals, 0)
			net, gross = 0, 0
		}
		byMonth[r.Month] = append(byMonth[r.Month], r.Net)
		totals[len(totals)-1] += r.Net
		net += r.Net
		gross += r.Gross
	}
	// The net share of gross in the last complete month: the allowance included in
	// each licence is spent first, so in-month net lags gross and cannot be used.
	if gross > 0 {
		out.NetRatio = net / gross
	}
	for i, m := range order {
		out.Months = append(out.Months, SpendBandMonth{Month: m, LimitUSD: limitAtMonthEnd(m), LimitChanged: limitChanged(m), Users: len(byMonth[m]), Bands: mergeBands(byMonth[m], limitAtMonthEnd(m))})
		t := round100(totals[i])
		out.Totals = append(out.Totals, SpendForecastMonth{Month: m, LowUSD: t, MidUSD: t, HighUSD: t})
	}

	day, err := time.Parse("2006-01-02", lastDay)
	if err != nil || len(current) == 0 || out.NetRatio == 0 {
		return out
	}
	month := day.Format("2006-01")
	// Until last month's invoice is loaded, the newest complete month is two
	// months back: neither its users nor its net fit, so no projection.
	if order[len(order)-1] != day.AddDate(0, -1, 1-day.Day()).Format("2006-01") {
		return out
	}
	out.Days, out.DaysInMonth, out.LastDay = day.Day(), day.AddDate(0, 0, 1-day.Day()).AddDate(0, 1, -1).Day(), lastDay
	scale := float64(out.DaysInMonth) / float64(out.Days) * out.NetRatio
	// No seat data in BigQuery: everyone billed last month is counted, so the 0 %
	// band compares with billed months. Each user blends the run rate with last
	// month's net, weighted by elapsed days, so early bursts are damped.
	prev := map[string]float64{}
	for _, r := range history {
		if r.Month == order[len(order)-1] {
			prev[r.UserID] = r.Net
		}
	}
	run := map[string]float64{}
	for _, r := range current {
		run[r.UserID] = r.Gross * scale
	}
	w := float64(out.Days) / float64(out.DaysInMonth)
	out.RunWeight = w
	projected := []float64{}
	total := 0.0
	add := func(id string) {
		v := w*run[id] + (1-w)*prev[id]
		projected = append(projected, v)
		total += v
	}
	for id := range prev {
		add(id)
	}
	for id := range run {
		if _, ok := prev[id]; !ok {
			add(id)
		}
	}
	limit := limitAtMonthEnd(month)
	out.Current = &SpendBandMonth{Month: month, LimitUSD: limit, LimitChanged: limitChanged(month), Users: len(projected), Bands: mergeBands(projected, limit)}
	out.Totals = append(out.Totals, SpendForecastMonth{Month: month, LowUSD: round100(total), MidUSD: round100(total), HighUSD: round100(total)})
	out.Forecast = forecastTotals(append(totals, total), month, 3)
	return out
}

// forecastTotals extends a least-squares line through the monthly totals ys,
// whose last month is last. Low holds the last total flat; high lies as far
// above the trend as the flat line lies from it. Values are rounded to 100 USD.
func forecastTotals(ys []float64, last string, months int) []SpendForecastMonth {
	n := float64(len(ys))
	var sx, sy, sxx, sxy float64
	for i, y := range ys {
		x := float64(i)
		sx, sy, sxx, sxy = sx+x, sy+y, sxx+x*x, sxy+x*y
	}
	slope := 0.0
	if d := n*sxx - sx*sx; d != 0 {
		slope = (n*sxy - sx*sy) / d
	}
	intercept := (sy - slope*sx) / n
	flat := ys[len(ys)-1]
	first, _ := time.Parse("2006-01", last)
	out := []SpendForecastMonth{}
	for k := 1; k <= months; k++ {
		mid := math.Max(0, intercept+slope*(n-1+float64(k)))
		low, high := flat, mid+math.Abs(mid-flat)
		out = append(out, SpendForecastMonth{Month: first.AddDate(0, k, 0).Format("2006-01"), LowUSD: round100(low), MidUSD: round100(mid), HighUSD: round100(high)})
	}
	return out
}

const spendBandsFirstMonth = "2026-08-01"

func (bq *BigQueryClient) GetSpendBands(ctx context.Context) (*SpendBands, error) {
	history := bq.client.Query(fmt.Sprintf(`
SELECT FORMAT_DATE('%%Y-%%m', month) month, user_id,
  SUM(IF(sku IN ('Copilot AI Credits','Copilot Cloud Agent'), net_amount, 0)) net,
  SUM(IF(sku IN ('Copilot AI Credits','Copilot Cloud Agent'), gross_amount, 0)) gross
FROM %s
WHERE month >= DATE(@first) AND scope_id='nav' AND user_id!=''
  AND month IN (SELECT month FROM %s WHERE scope_id='nav' AND status='complete')
GROUP BY month, user_id ORDER BY month`,
		bq.tableRef(bq.metricsDataset, "billing_user_monthly"),
		bq.tableRef(bq.metricsDataset, "billing_user_monthly_runs")))
	history.Parameters = []bigquery.QueryParameter{{Name: "first", Value: spendBandsFirstMonth}}
	it, err := history.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read spend band history: %w", err)
	}
	hist, err := readAllRows[spendUserRow](it)
	if err != nil {
		return nil, err
	}

	current := bq.client.Query(fmt.Sprintf(`
WITH u AS (
  SELECT day, JSON_VALUE(raw_record,'$.user_id') user_id,
    COALESCE(SAFE_CAST(JSON_VALUE(raw_record,'$.ai_credits_used') AS FLOAT64),0) * 0.01 gross
  FROM %s
  WHERE day >= DATE_TRUNC(CURRENT_DATE(), MONTH) AND scope='enterprise' AND scope_id='nav'
)
SELECT user_id, SUM(gross) gross, CAST((SELECT MAX(day) FROM u) AS STRING) last_day
FROM u GROUP BY user_id`, bq.tableRef(bq.metricsDataset, "user_metrics")))
	it, err = current.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read spend band current month: %w", err)
	}
	cur, err := readAllRows[spendUserRow](it)
	if err != nil {
		return nil, err
	}
	lastDay := ""
	if len(cur) > 0 {
		lastDay = cur[0].LastDay
	}
	return buildSpendBands(hist, cur, lastDay), nil
}

func (c *CachedBigQueryClient) GetSpendBands(ctx context.Context) (*SpendBands, error) {
	ctx, cancel := withQueryTimeout(ctx)
	defer cancel()
	return getCachedValue(c, "spend_bands", func() (*SpendBands, error) {
		return c.client.GetSpendBands(ctx)
	})
}

func (h *BigQueryHandlers) handleSpendBands(w http.ResponseWriter, r *http.Request) {
	bands, err := h.bqClient.GetSpendBands(r.Context())
	if err != nil {
		slog.Error("Failed to fetch spend bands")
		respondError(w, "internal_error", "Failed to fetch spend bands", http.StatusInternalServerError)
		return
	}
	cacheControl(w, 3600, false)
	respondJSON(w, bands, http.StatusOK)
}
