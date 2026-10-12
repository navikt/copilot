package main

import (
	"math"
	"testing"
	"time"
)

// Prod 2026, fakturert and active users per month (read 2026-10-12).
var (
	prodNet   = []float64{1268, 2402, 3310, 6406, 12828, 28014, 16664, 32295, 64559}
	prodUsers = []int64{386, 451, 508, 575, 602, 650, 568, 677, 719}
)

func prodMonths(through int) ([]AdminMonth, []ActiveUsersMonth) {
	var ms []AdminMonth
	var us []ActiveUsersMonth
	for i := 0; i < through; i++ {
		m := time.Date(2026, time.Month(i+1), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
		ms = append(ms, AdminMonth{Month: m, EnterpriseNetUSD: &prodNet[i]})
		us = append(us, ActiveUsersMonth{Month: m, ActiveUsers: prodUsers[i]})
	}
	return ms, us
}

func TestYearForecastLowMidHigh(t *testing.T) {
	ms, us := prodMonths(9)
	cur := ModelForecast{Model: modelTrend, NetMidUSD: 70000, NetLowUSD: 59500, NetHighUSD: 80500}
	pts, end := yearForecast(ms, us, cur, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if len(pts) != 12 || end == nil {
		t.Fatalf("want 12 points and a year end, got %d, %v", len(pts), end)
	}
	closed := pts[8].CumulativeNetUSD
	if pts[8].Forecast || !pts[9].Forecast || pts[8].Low != closed {
		t.Fatalf("closed months must be actual: %+v", pts[8:10])
	}
	// Low: flat at the current month's midpoint for Nov and Dec.
	if want := closed + 59500 + 2*70000; math.Abs(end.Low-want) > 1e-6 {
		t.Errorf("low %v, want %v", end.Low, want)
	}
	for i := 10; i < 12; i++ {
		step := pts[i].CumulativeNetUSD - pts[i-1].CumulativeNetUSD
		hiStep := pts[i].High - pts[i-1].High
		if step < 70000 || math.Abs(hiStep-step*(1+yearBandPct/100)) > 1e-6 {
			t.Errorf("%s: mid step %v below flat or high step %v off", pts[i].Month, step, hiStep)
		}
	}
	if !(end.Low <= end.Mid && end.Mid <= end.High) {
		t.Errorf("year end out of order: %+v", end)
	}
}

func TestYearGrowthCapsPerUser(t *testing.T) {
	net := []float64{100, 1000, 10000}
	ms := []AdminMonth{{Month: "2026-08", EnterpriseNetUSD: &net[0]}, {Month: "2026-09", EnterpriseNetUSD: &net[1]}, {Month: "2026-10", EnterpriseNetUSD: &net[2]}}
	g, ok := yearGrowth(ms, map[string]int64{"2026-08": 100, "2026-09": 100, "2026-10": 100})
	if !ok || math.Abs(g-yearDamping*yearMaxPerUserGrowth) > 1e-9 {
		t.Errorf("growth %v, want capped %v", g, yearDamping*yearMaxPerUserGrowth)
	}
	if _, ok := yearGrowth(ms[:1], map[string]int64{"2026-08": 100}); ok {
		t.Error("one month must not fit a trend")
	}
}

// TestYearBacktest records the backtest the parameters were picked on: a
// prediction from the end of a closed month, base holiday-adjusted, against
// what was billed. Few points; the test pins the errors so a change shows.
func TestYearBacktest(t *testing.T) {
	for _, c := range []struct{ cut, target int; wantPct float64 }{{7, 8, 31}, {7, 9, -8}, {8, 9, -38}} {
		ms, us := prodMonths(c.cut)
		au := map[string]int64{}
		for _, u := range us {
			au[u.Month] = u.ActiveUsers
		}
		g, _ := yearGrowth(ms, au)
		base := prodNet[c.cut-1]
		if c.cut == 7 {
			base /= holidayNetFactor
		}
		got := base * math.Exp(g*float64(c.target-c.cut))
		errPct := (got - prodNet[c.target-1]) / prodNet[c.target-1] * 100
		t.Logf("end of %d → month %d: %.0f vs %.0f (%+.0f %%)", c.cut, c.target, got, prodNet[c.target-1], errPct)
		if math.Abs(errPct-c.wantPct) > 1 {
			t.Errorf("error %+.1f %%, recorded %+.0f %%", errPct, c.wantPct)
		}
	}
}

func TestYearForecastFitsAcrossNewYear(t *testing.T) {
	net := []float64{1000, 1100, 1200}
	ms := []AdminMonth{{Month: "2026-11", EnterpriseNetUSD: &net[0]}, {Month: "2026-12", EnterpriseNetUSD: &net[1]}, {Month: "2027-01", EnterpriseNetUSD: &net[2]}}
	us := []ActiveUsersMonth{{Month: "2026-11", ActiveUsers: 10}, {Month: "2026-12", ActiveUsers: 10}, {Month: "2027-01", ActiveUsers: 10}}
	pts, end := yearForecast(ms, us, ModelForecast{NetMidUSD: 1300, NetLowUSD: 1200, NetHighUSD: 1400}, time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC))
	if end == nil || pts[0].Month != "2027-01" || pts[0].CumulativeNetUSD != 1200 || len(pts) != 12 {
		t.Fatalf("want 2027 only, fitted on 2026 too: %+v %v", pts[:1], end)
	}
}
