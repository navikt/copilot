package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"cloud.google.com/go/bigquery"
)

// CostForecast is the month-end forecast on the admin cost page (F4) and its
// comparison with the enterprise ai_credits budget (F5, anomaly A4). Two
// models run side by side until one has proved better. Spend and budget are
// internal: serve it only to admins and never log the amounts.
type CostForecast struct {
	Month       string `json:"month"`
	DaysElapsed int    `json:"days_elapsed"`
	DaysInMonth int    `json:"days_in_month"`
	// AllowanceUsedUp is true once the enterprise's included credits for the month are spent.
	AllowanceUsedUp bool `json:"allowance_used_up"`
	// AllowanceRunOutDate is the trend model's run-out day, actual or projected (F9).
	AllowanceRunOutDate string `json:"allowance_run_out_date,omitempty"`
	// NetToDateUSD is billed net so far this month.
	NetToDateUSD float64 `json:"net_to_date_usd"`
	// BudgetStatus is "set", "missing" (no budget in GitHub) or "error".
	BudgetStatus string          `json:"budget_status"`
	BudgetUSD    float64         `json:"budget_usd,omitempty"`
	Models       []ModelForecast `json:"models"`
	// UncertaintyLabel is earlyLabel in the first earlyDays days of the month.
	UncertaintyLabel string `json:"uncertainty_label,omitempty"`
	// Days is billed net accumulated per day this month: actual up to the
	// last day with data, then the trend model's midpoint.
	Days []CostForecastDay `json:"days"`
	// DailyForecast is the cumulative billed net per remaining day from the
	// trend model, for the month chart.
	DailyForecast []DailyForecastPoint `json:"daily_forecast"`
	// LimitChanges are the days the default personal limit changed, for chart markers.
	LimitChanges []LimitChange `json:"limit_changes"`
}

// CostForecastDay is cumulative enterprise net on one day of the month.
type CostForecastDay struct {
	Day      string  `json:"day"`
	NetUSD   float64 `json:"net_usd"`
	Forecast bool    `json:"forecast"`
}

// DailyForecastPoint is the projected cumulative net at the end of one day.
// Low and high widen from net to date to the model's range at month end.
type DailyForecastPoint struct {
	Date             string  `json:"date"`
	CumulativeNetUSD float64 `json:"cumulative_net_usd"`
	Low              float64 `json:"low"`
	High             float64 `json:"high"`
}

// ModelForecast is one model's forecast of billed net at month end.
type ModelForecast struct {
	Model       string  `json:"model"` // modelTrend or modelPerUser
	GrossMidUSD float64 `json:"gross_mid_usd"`
	NetLowUSD   float64 `json:"net_low_usd"`
	NetMidUSD   float64 `json:"net_mid_usd"`
	NetHighUSD  float64 `json:"net_high_usd"`
	// RangePct is the half-width of the range: the backtest error, at least rangeFloorPct.
	RangePct float64 `json:"range_pct"`
	// AllowanceRunsOut is the day the included credits ran out, or are projected to.
	AllowanceRunsOut string `json:"allowance_runs_out,omitempty"`
	// BacktestErrorPct is the mean absolute midpoint error on backtestDays
	// of BacktestMonths, the closed months in the data. 0 without any.
	BacktestErrorPct float64  `json:"backtest_error_pct"`
	BacktestMonths   []string `json:"backtest_months"`
	BudgetPct        float64  `json:"budget_pct"`
	// OverBudget is anomaly A4: the net midpoint is above the budget.
	OverBudget bool `json:"over_budget"`
	// cumNet is the projected cumulative net per remaining day.
	cumNet []float64
}

const (
	budgetSet     = "set"
	budgetMissing = "missing"
	budgetError   = "error"

	modelTrend   = "trend"
	modelPerUser = "per_user"

	// rangeFloorPct is the smallest range around the midpoint. The largest
	// backtest error in Aug and Sep 2026 was 9.7 %; the owner set 15 %.
	rangeFloorPct = 15.0
	// earlyRangePct replaces rangeFloorPct in the first earlyDays days of the
	// month, when the backtest errs most (owner decision 2026-10-12).
	earlyRangePct = 35.0
	earlyDays     = 7
	earlyLabel    = "tidlig i måneden, stor usikkerhet"
	// trendWeekdays is how many recent weekdays the gross trend is fitted on.
	trendWeekdays = 20
)

var backtestDays = []int{5, 10, 15, 20, 25}

// CostDay is one day of enterprise gross and net from billing_usage_daily_model.
type CostDay struct {
	Day   string  `bigquery:"day"`
	Net   float64 `bigquery:"net"`
	Gross float64 `bigquery:"gross"`
}

// UserDay is one user's gross (AI credits in USD) on one day, from user_metrics.
type UserDay struct {
	UserID string  `bigquery:"user_id"`
	Day    string  `bigquery:"day"`
	Gross  float64 `bigquery:"gross"`
}

// UserProjection is one user's gross this month: so far, projected to month
// end and the personal limit at month end. For aggregates only (F7): never
// send per-user rows to a client.
type UserProjection struct {
	UserID    string
	MTD       float64
	Projected float64
	Limit     float64
}

func monthDays(month time.Time) int { return month.AddDate(0, 1, -1).Day() }

func dayOf(month time.Time, d int) time.Time {
	return time.Date(month.Year(), month.Month(), d, 0, 0, 0, 0, time.UTC)
}

// trendRemaining projects enterprise gross for each day after last: a linear
// trend over the last trendWeekdays weekdays (usage grows within the month)
// and the mean of the last eight weekend days.
func trendRemaining(days []CostDay, month, last time.Time) ([]float64, bool) {
	type pt struct{ x, y float64 }
	var weekdays []pt
	var weekend []float64
	for i := len(days) - 1; i >= 0; i-- {
		t, err := time.Parse("2006-01-02", days[i].Day)
		if err != nil || t.After(last) {
			continue
		}
		if isWeekend(t) {
			if len(weekend) < 8 {
				weekend = append(weekend, days[i].Gross)
			}
		} else if len(weekdays) < trendWeekdays {
			weekdays = append(weekdays, pt{t.Sub(last).Hours() / 24, days[i].Gross})
		}
	}
	n := float64(len(weekdays))
	if n < 5 {
		return nil, false
	}
	var mx, my float64
	for _, p := range weekdays {
		mx += p.x / n
		my += p.y / n
	}
	var sxx, sxy float64
	for _, p := range weekdays {
		sxx += (p.x - mx) * (p.x - mx)
		sxy += (p.x - mx) * (p.y - my)
	}
	slope := 0.0
	if sxx > 0 {
		slope = sxy / sxx
	}
	intercept := my - slope*mx
	we := simpleAverage(weekend)
	rem := make([]float64, 0, monthDays(month)-last.Day())
	for k := last.Day() + 1; k <= monthDays(month); k++ {
		t := dayOf(month, k)
		if isWeekend(t) {
			rem = append(rem, we)
		} else {
			rem = append(rem, math.Max(0, intercept+slope*t.Sub(last).Hours()/24))
		}
	}
	return rem, true
}

// projectUsers projects each user's gross to month end. A user's remaining
// days follow their own profile in the reference month (last month, aligned on
// weekday), scaled by this month so far against the same days there. Users
// without a reference, and everyone when last month was July (holiday month,
// and June has no AI credits), use their own weekday and weekend means so
// far. Each day is capped at the personal limit in force that day; a user at
// the limit stops. daily is the projected total per remaining day.
//
// ponytail: the default limit only; per-person overrides in budget_snapshots
// are keyed by login, not user_id. Add them when there are more than a few.
func projectUsers(users []UserDay, month, last time.Time) (proj []UserProjection, daily []float64) {
	ref := month.AddDate(0, -1, 0)
	useRef := ref.Month() != time.July
	cur, refKey := month.Format("2006-01"), ref.Format("2006-01")
	type hist struct{ cur, ref map[int]float64 }
	byUser := map[string]*hist{}
	var order []string
	for _, u := range users {
		t, err := time.Parse("2006-01-02", u.Day)
		if err != nil || t.After(last) {
			continue
		}
		h := byUser[u.UserID]
		if h == nil {
			h = &hist{map[int]float64{}, map[int]float64{}}
			byUser[u.UserID] = h
			order = append(order, u.UserID)
		}
		switch u.Day[:7] {
		case cur:
			h.cur[t.Day()] += u.Gross
		case refKey:
			h.ref[t.Day()] += u.Gross
		}
	}
	// align maps each day this month to the day in ref with the same weekday,
	// nearest in day of month.
	align := map[int]int{}
	for d := 1; d <= monthDays(month); d++ {
		best := 0
		for k := 1; k <= monthDays(ref); k++ {
			if dayOf(ref, k).Weekday() == dayOf(month, d).Weekday() && (best == 0 || absInt(k-d) < absInt(best-d)) {
				best = k
			}
		}
		align[d] = best
	}
	rest := monthDays(month) - last.Day()
	daily = make([]float64, rest)
	monthEndLimit := limitOn(dayOf(month, monthDays(month)).Format("2006-01-02"))
	for _, id := range order {
		h := byUser[id]
		var mtd, expected, wdSum, weSum float64
		var wdN, weN int
		for k := 1; k <= last.Day(); k++ {
			mtd += h.cur[k]
			expected += h.ref[align[k]]
			if isWeekend(dayOf(month, k)) {
				weSum += h.cur[k]
				weN++
			} else {
				wdSum += h.cur[k]
				wdN++
			}
		}
		if mtd == 0 {
			continue
		}
		acc := mtd
		for i := 0; i < rest; i++ {
			k := last.Day() + 1 + i
			t := dayOf(month, k)
			var v float64
			switch {
			case useRef && expected > 0:
				v = h.ref[align[k]] * mtd / expected
			case isWeekend(t) && weN > 0:
				v = weSum / float64(weN)
			case !isWeekend(t) && wdN > 0:
				v = wdSum / float64(wdN)
			}
			v = math.Max(0, math.Min(v, limitOn(t.Format("2006-01-02"))-acc))
			acc += v
			daily[i] += v
		}
		proj = append(proj, UserProjection{UserID: id, MTD: mtd, Projected: acc, Limit: monthEndLimit})
	}
	return proj, daily
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// poolNet turns projected gross per remaining day into billed net. GitHub
// bills against a pooled enterprise allowance: every credit is free until the
// month's included credits are spent, then almost every credit is billed. So
//
//	net = net so far + max(0, remaining gross - allowance left) * (1 - trickle)
//
// While the allowance is not used up, allowance left is last month's
// allowance minus what is used so far (a change in seats shifts it). trickle
// is the small share still free after the allowance is used up (seats added
// during the month), from days 20+ of last month once its allowance was used.
func poolNet(days []CostDay, month, last time.Time, rem []float64) (gross, net float64, usedUp bool, runsOut string, cum []float64, ok bool) {
	cur, prev := month.Format("2006-01"), month.AddDate(0, -1, 0).Format("2006-01")
	var gm, nm, prevGross, prevNet, tailGross, tailNet float64
	for _, d := range days {
		t, err := time.Parse("2006-01-02", d.Day)
		if err != nil || t.After(last) {
			continue
		}
		switch d.Day[:7] {
		case cur:
			gm += d.Gross
			nm += d.Net
			if runsOut == "" && d.Net > 0.5*d.Gross {
				runsOut = d.Day
			}
		case prev:
			prevGross += d.Gross
			prevNet += d.Net
			if t.Day() >= 20 && d.Net > 0.5*d.Gross {
				tailGross += d.Gross
				tailNet += d.Net
			}
		}
	}
	if prevGross == 0 {
		return 0, 0, false, "", nil, false
	}
	usedUp = runsOut != ""
	left := 0.0
	if !usedUp {
		left = math.Max(0, (prevGross-prevNet)-(gm-nm))
	}
	trickle := 0.0 // no billed tail last month: assume none stays free
	if tailGross > 0 {
		trickle = 1 - tailNet/tailGross
	}
	var r float64
	cum = make([]float64, len(rem))
	for i, v := range rem {
		r += v
		if runsOut == "" && r >= left {
			runsOut = dayOf(month, last.Day()+1+i).Format("2006-01-02")
		}
		cum[i] = nm + math.Max(0, r-left)*(1-trickle)
	}
	return gm + r, nm + math.Max(0, r-left)*(1-trickle), usedUp, runsOut, cum, true
}

// lastDayIn returns the last day of month in days.
func lastDayIn(days []CostDay, month time.Time) (time.Time, bool) {
	var last time.Time
	key := month.Format("2006-01")
	for _, d := range days {
		if d.Day[:7] == key {
			if t, err := time.Parse("2006-01-02", d.Day); err == nil && t.After(last) {
				last = t
			}
		}
	}
	return last, !last.IsZero()
}

// forecastModels runs all models for month with data up to last. usedUp
// says whether the included credits are spent by last.
func forecastModels(days []CostDay, users []UserDay, month, last time.Time) (out map[string]ModelForecast, usedUp bool) {
	out = map[string]ModelForecast{}
	add := func(name string, rem []float64) {
		if g, n, u, runs, cum, ok := poolNet(days, month, last, rem); ok {
			out[name] = ModelForecast{Model: name, GrossMidUSD: g, NetMidUSD: n, AllowanceRunsOut: runs, cumNet: cum}
			usedUp = u
		}
	}
	if rem, ok := trendRemaining(days, month, last); ok {
		add(modelTrend, shapeByWeekday(days, month, last, rem))
	}
	if proj, rem := projectUsers(users, month, last); len(proj) > 0 {
		add(modelPerUser, rem)
	}
	return out, usedUp
}

// upTo keeps the rows on or before day.
func upTo[T any](rows []T, dayOfRow func(T) string, day string) []T {
	var out []T
	for _, r := range rows {
		if dayOfRow(r) <= day {
			out = append(out, r)
		}
	}
	return out
}

// backtest runs all models on each closed month before month that has the
// month before it in the data, as of backtestDays, against the month's
// net in the daily table. It returns the mean absolute midpoint error per
// model in percent, and the months used.
func backtest(days []CostDay, users []UserDay, month time.Time) (map[string]float64, []string) {
	sum, n := map[string]float64{}, map[string]int{}
	var months []string
	for m := month.AddDate(0, -1, 0); ; m = m.AddDate(0, -1, 0) {
		if _, ok := lastDayIn(days, m.AddDate(0, -1, 0)); !ok {
			break
		}
		key := m.Format("2006-01")
		var actual float64
		for _, d := range days {
			if d.Day[:7] == key {
				actual += d.Net
			}
		}
		if actual <= 0 {
			continue
		}
		used := false
		for _, cut := range backtestDays {
			day := dayOf(m, cut).Format("2006-01-02")
			f, _ := forecastModels(
				upTo(days, func(d CostDay) string { return d.Day }, day),
				upTo(users, func(u UserDay) string { return u.Day }, day),
				m, dayOf(m, cut))
			for k, v := range f {
				sum[k] += math.Abs(v.NetMidUSD-actual) / actual * 100
				n[k]++
				used = true
			}
		}
		if used {
			months = append([]string{key}, months...)
		}
	}
	out := map[string]float64{}
	for k, s := range sum {
		out[k] = s / float64(n[k])
	}
	return out, months
}

// buildCostForecast runs the models for month, sets each range from its
// backtest error (at least rangeFloorPct) and compares with the budget.
func buildCostForecast(days []CostDay, users []UserDay, month time.Time, budget float64, status string) CostForecast {
	out := CostForecast{Month: month.Format("2006-01"), DaysInMonth: monthDays(month), BudgetStatus: status, Models: []ModelForecast{}, Days: []CostForecastDay{}, DailyForecast: []DailyForecastPoint{}}
	if status == budgetSet && budget > 0 {
		out.BudgetUSD = budget
	}
	last, ok := lastDayIn(days, month)
	if !ok {
		return out
	}
	// Actual days first: the chart shows them before there is enough data for a forecast.
	for _, d := range days {
		if d.Day[:7] == out.Month {
			out.NetToDateUSD += d.Net
			out.Days = append(out.Days, CostForecastDay{Day: d.Day, NetUSD: out.NetToDateUSD})
		}
	}
	models, usedUp := forecastModels(days, users, month, last)
	if len(models) == 0 {
		return out
	}
	out.DaysElapsed, out.AllowanceUsedUp = last.Day(), usedUp
	errs, months := backtest(days, users, month)
	floor := rangeFloorPct
	if last.Day() <= earlyDays {
		floor, out.UncertaintyLabel = earlyRangePct, earlyLabel
	}
	for _, name := range []string{modelTrend, modelPerUser} {
		f, ok := models[name]
		if !ok {
			continue
		}
		f.BacktestErrorPct, f.BacktestMonths = errs[name], months
		f.RangePct = math.Max(floor, f.BacktestErrorPct)
		f.NetLowUSD = f.NetMidUSD * (1 - f.RangePct/100)
		f.NetHighUSD = f.NetMidUSD * (1 + f.RangePct/100)
		if out.BudgetUSD > 0 {
			f.BudgetPct = f.NetMidUSD / out.BudgetUSD * 100
			f.OverBudget = f.NetMidUSD > out.BudgetUSD
		}
		out.Models = append(out.Models, f)
		if name == modelTrend {
			out.AllowanceRunOutDate = f.AllowanceRunsOut
			out.DailyForecast = dailySeries(f, out.NetToDateUSD, month, last)
			for _, p := range out.DailyForecast {
				out.Days = append(out.Days, CostForecastDay{Day: p.Date, NetUSD: p.CumulativeNetUSD, Forecast: true})
			}
		}
	}
	return out
}

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
  AND LOWER(scope_id) = 'nav' AND gross_quantity > 0
GROUP BY day ORDER BY day`, bq.tableRef(bq.metricsDataset, "billing_usage_daily_model")))
	q.Parameters = []bigquery.QueryParameter{{Name: "month", Value: month}, {Name: "back", Value: costHistoryMonths}}
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("read cost days: %w", err)
	}
	return readAllRows[CostDay](it)
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
	return readAllRows[UserDay](it)
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

// handleCostForecast handles GET /api/v1/copilot/admin/cost-forecast.
func (h *BigQueryHandlers) handleCostForecast(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	days, err := h.bqClient.GetCostDays(r.Context(), month.Format("2006-01"))
	if err != nil {
		slog.Error("Failed to fetch cost days", "error", err)
		respondError(w, "internal_error", "Failed to fetch cost forecast", http.StatusInternalServerError)
		return
	}
	users, err := h.bqClient.GetCostUserDays(r.Context(), month.Format("2006-01"))
	if err != nil {
		slog.Error("Failed to fetch cost user days", "error", err)
		respondError(w, "internal_error", "Failed to fetch cost forecast", http.StatusInternalServerError)
		return
	}

	status, budget := budgetError, 0.0
	if h.budgetClient != nil {
		b, err := h.budgetClient.getGlobalBudget(r.Context())
		switch {
		case errors.Is(err, errBudgetNotFound):
			status = budgetMissing
		case err != nil:
			slog.Warn("Failed to fetch enterprise budget for cost forecast", "error", err)
		case b != nil && b.EnterpriseBudget > 0:
			status, budget = budgetSet, b.EnterpriseBudget
		default:
			status = budgetMissing
		}
	}

	out := buildCostForecast(days, users, month, budget, status)
	snaps, err := h.bqClient.GetLimitSnapshots(r.Context())
	if err != nil {
		// The markers are a nicety; the documented limits still give the history.
		slog.Warn("Failed to fetch limit snapshots", "error", err)
	}
	out.LimitChanges = limitChanges(snaps)
	cacheControl(w, 900, false)
	respondJSON(w, out, http.StatusOK)
}

// dailySeries turns f's cumulative net per remaining day into points whose
// range grows in step with the projected net, reaching f's low and high at
// month end.
func dailySeries(f ModelForecast, netToDate float64, month, last time.Time) []DailyForecastPoint {
	out := make([]DailyForecastPoint, 0, len(f.cumNet))
	for i, c := range f.cumNet {
		// Share of the way to month end: by projected net, or by day when
		// nothing more is billed (the allowance lasts the month).
		share := float64(i+1) / float64(len(f.cumNet))
		if f.NetMidUSD > netToDate {
			share = (c - netToDate) / (f.NetMidUSD - netToDate)
		}
		half := share * f.NetMidUSD * f.RangePct / 100
		out = append(out, DailyForecastPoint{
			Date:             dayOf(month, last.Day()+1+i).Format("2006-01-02"),
			CumulativeNetUSD: c, Low: c - half, High: c + half,
		})
	}
	return out
}
