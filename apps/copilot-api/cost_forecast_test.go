package main

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

// synthDays builds last month and this month up to upTo: weekday gross g on
// day k is base+k, weekends 10 % of base. The allowance pool is used up on the
// first days of each month (prevPool last month, pool this month); after that
// 2 % of gross stays free.
func synthDays(until int, prevPool, pool float64) []CostDay {
	pools := []float64{prevPool, pool}
	var out []CostDay
	for i, m := range []time.Time{time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)} {
		left := pools[i]
		for d := m; d.Month() == m.Month(); d = d.AddDate(0, 0, 1) {
			if d.Month() == 10 && d.Day() > until {
				break
			}
			g := 100.0 + float64(d.YearDay())
			if isWeekend(d) {
				g = 10
			}
			free := math.Min(left, g)
			left -= free
			net := (g - free) * 0.98
			out = append(out, CostDay{Day: d.Format("2006-01-02"), Gross: g, Net: net})
		}
	}
	return out
}

var oct = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

func trendForecast(t *testing.T, days []CostDay) ModelForecast {
	t.Helper()
	last, ok := lastDayIn(days, oct)
	if !ok {
		t.Fatal("no days")
	}
	f, _ := forecastModels(days, nil, oct, last)
	m, ok := f[modelTrend]
	if !ok {
		t.Fatal("no trend forecast")
	}
	return m
}

func TestTrendWithPool(t *testing.T) {
	var actual float64
	for _, d := range synthDays(31, 1000, 1000) {
		if d.Day[:7] == "2026-10" {
			actual += d.Net
		}
	}

	t.Run("allowance used up", func(t *testing.T) {
		f := trendForecast(t, synthDays(15, 1000, 1000))
		if math.Abs(f.NetMidUSD-actual)/actual > 0.01 {
			t.Fatalf("mid %.1f, actual %.1f", f.NetMidUSD, actual)
		}
		if f.AllowanceRunsOut != "2026-10-06" { // first day mostly billed in synthDays
			t.Fatalf("runs out %q", f.AllowanceRunsOut)
		}
	})

	t.Run("allowance left is subtracted and its end projected", func(t *testing.T) {
		f := trendForecast(t, synthDays(10, 4000, 4000))
		// Allowance left is last month's free gross (pool plus trickle) minus
		// what is used so far, so net lands just under 98 % of gross over pool.
		want := 0.98 * (f.GrossMidUSD - 4000)
		if math.Abs(f.NetMidUSD-want)/want > 0.03 {
			t.Fatalf("mid %.1f, want %.1f", f.NetMidUSD, want)
		}
		if f.AllowanceRunsOut <= "2026-10-10" {
			t.Fatalf("runs out %q, want a projected day after the 10th", f.AllowanceRunsOut)
		}
	})
}

func TestProjectUsersCapsAtLimitInForce(t *testing.T) {
	sep := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var users []UserDay
	for d := 1; d <= 31; d++ {
		users = append(users, UserDay{"heavy", dayOf(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), d).Format("2006-01-02"), 30})
		users = append(users, UserDay{"light", dayOf(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), d).Format("2006-01-02"), 1})
	}
	for d := 1; d <= 10; d++ {
		users = append(users, UserDay{"heavy", dayOf(sep, d).Format("2006-01-02"), 30})
		users = append(users, UserDay{"light", dayOf(sep, d).Format("2006-01-02"), 1})
	}
	proj, daily := projectUsers(users, sep, dayOf(sep, 10))
	got := map[string]UserProjection{}
	for _, p := range proj {
		got[p.UserID] = p
	}
	// heavy: 30 a day stops at 400 until the limit rises to 800 on the 22nd,
	// then continues 30 a day for the 9 days left: 400 + 270.
	if h := got["heavy"]; math.Abs(h.Projected-670) > 1e-6 || h.MTD != 300 || h.Limit != 800 {
		t.Fatalf("heavy %+v", h)
	}
	if l := got["light"]; math.Abs(l.Projected-30) > 1e-6 {
		t.Fatalf("light %+v", l)
	}
	var sum float64
	for _, v := range daily {
		sum += v
	}
	if math.Abs(sum-(370+20)) > 1e-6 {
		t.Fatalf("daily sum %.1f", sum)
	}
}

func TestProjectUsersSkipsJulyReference(t *testing.T) {
	aug := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	users := []UserDay{{"u", "2026-07-15", 100}}
	for d := 1; d <= 10; d++ {
		if !isWeekend(dayOf(aug, d)) {
			users = append(users, UserDay{"u", dayOf(aug, d).Format("2006-01-02"), 2})
		}
	}
	proj, _ := projectUsers(users, aug, dayOf(aug, 10))
	// Own weekday mean (2) on the 15 weekdays left, weekends 0.
	if len(proj) != 1 || math.Abs(proj[0].Projected-(proj[0].MTD+2*15)) > 1e-6 {
		t.Fatalf("got %+v", proj)
	}
}

func TestBuildCostForecastBudget(t *testing.T) {
	days := synthDays(15, 1000, 1000)
	f := buildCostForecast(days, nil, oct, 0, budgetMissing)
	if len(f.Models) != 1 || f.Models[0].OverBudget || f.BudgetUSD != 0 {
		t.Fatalf("no budget must give no A4: %+v", f)
	}
	m := f.Models[0]
	if f.NetToDateUSD <= 0 || f.AllowanceRunOutDate != m.AllowanceRunsOut {
		t.Fatalf("to date %v run out %q", f.NetToDateUSD, f.AllowanceRunOutDate)
	}
	if m.RangePct != rangeFloorPct || math.Abs(m.NetHighUSD-m.NetMidUSD*1.15) > 1e-6 {
		t.Fatalf("range %+v", m)
	}
	mid := m.NetMidUSD
	if g := buildCostForecast(days, nil, oct, mid*2, budgetSet).Models[0]; math.Abs(g.BudgetPct-50) > 1e-9 || g.OverBudget {
		t.Fatalf("under budget %+v", g)
	}
	// The high end over budget is not enough: A4 follows the midpoint.
	if buildCostForecast(days, nil, oct, mid*1.1, budgetSet).Models[0].OverBudget {
		t.Fatal("A4 must follow the midpoint")
	}
	if !buildCostForecast(days, nil, oct, mid*0.9, budgetSet).Models[0].OverBudget {
		t.Fatal("want A4")
	}
}

func TestGlobalBudgetEnterpriseAICredits(t *testing.T) {
	amount := 5.0
	// A fresh cache stands in for the GitHub API.
	c := newBudgetClient("token", "nav")
	c.cachedAt = time.Now()
	c.cachedData = []BudgetEntry{
		{BudgetScope: "enterprise", BudgetProductSKU: "actions", BudgetAmount: 99},
		{BudgetScope: "enterprise", BudgetProductSKU: "ai_credits", BudgetAmount: 1234},
		{BudgetScope: "multi_user_customer", BudgetProductSKU: "ai_credits", BudgetAmount: 400},
		{BudgetScope: "user", BudgetProductSKU: "ai_credits", BudgetAmount: 400, ConsumedAmount: &amount},
	}

	b, err := c.getGlobalBudget(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if b.EnterpriseBudget != 1234 || b.PerUserBudget != 400 {
		t.Fatalf("got %+v", b)
	}
	raw, _ := json.Marshal(b)
	if string(raw) != `{"totalConsumed":5,"perUserBudget":400,"activeUsers":1}` {
		t.Fatalf("enterprise budget leaked into public JSON: %s", raw)
	}
}

func TestBuildCostForecastDays(t *testing.T) {
	f := buildCostForecast(synthDays(15, 1000, 1000), nil, oct, 0, budgetMissing)
	if len(f.Days) != 31 || f.Days[14].Forecast || !f.Days[15].Forecast || f.Days[30].Day != "2026-10-31" {
		t.Fatalf("days %+v", f.Days)
	}
	if math.Abs(f.Days[14].NetUSD-f.NetToDateUSD) > 1e-9 || math.Abs(f.Days[30].NetUSD-f.Models[0].NetMidUSD) > 1e-6 {
		t.Fatalf("to date %v mid %v days %+v", f.NetToDateUSD, f.Models[0].NetMidUSD, f.Days)
	}
	for i := 1; i < len(f.Days); i++ {
		if f.Days[i].NetUSD < f.Days[i-1].NetUSD {
			t.Fatalf("not cumulative at %d: %+v", i, f.Days)
		}
	}
}

func TestBuildCostForecastDaysWithoutTrend(t *testing.T) {
	// October only: no history, so neither model can run.
	oct2 := upTo(synthDays(2, 1000, 1000), func(d CostDay) string { return d.Day }, "2026-10-31")[30:]
	f := buildCostForecast(oct2, nil, oct, 0, budgetMissing)
	if len(f.Models) != 0 || len(f.Days) != 2 || f.Days[1].Forecast {
		t.Fatalf("actual days must come without a trend: %+v", f)
	}
}
