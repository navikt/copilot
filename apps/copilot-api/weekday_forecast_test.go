package main

import (
	"math"
	"testing"
	"time"
)

// weekGross fills gross for every day from start to end: weekdays wd, weekends we.
func weekGross(start, end time.Time, wd, we float64) map[string]float64 {
	g := map[string]float64{}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		v := wd
		if isWeekend(d) {
			v = we
		}
		g[d.Format("2006-01-02")] = v
	}
	return g
}

func TestWeekdayFactors(t *testing.T) {
	last := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC) // Wednesday
	g := weekGross(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), last, 120, 50)
	g["2026-10-13"] = 1e6 // incomplete week: must not count
	f, ok := weekdayFactors(g, last)
	if !ok {
		t.Fatal("no factors")
	}
	mean := (5*120.0 + 2*50) / 7
	if math.Abs(f[time.Monday]-120/mean) > 1e-9 || math.Abs(f[time.Sunday]-50/mean) > 1e-9 {
		t.Fatalf("factors %v", f)
	}

	// Weeks in July are holiday weeks: with only July before, no factors.
	jul := weekGross(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC), 120, 50)
	if _, ok := weekdayFactors(jul, time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)); ok {
		t.Fatal("July weeks must be skipped")
	}
}

func TestBlendBase(t *testing.T) {
	if b := blendBase(200, true, 100, true, 0.1, 5); math.Abs(b-(0.5*200+0.5*110)) > 1e-9 {
		t.Fatalf("day 5: %v", b)
	}
	if b := blendBase(200, true, 100, true, 0.1, 12); b != 200 {
		t.Fatalf("after blendDays this month alone: %v", b)
	}
	if b := blendBase(0, false, 100, true, 0.1, 0); math.Abs(b-110) > 1e-9 {
		t.Fatalf("no days yet: %v", b)
	}
}

func TestMonthGrowthDampenedAndCapped(t *testing.T) {
	var f [7]float64
	for i := range f {
		f[i] = 1
	}
	g := map[string]float64{}
	level := map[time.Month]float64{time.August: 100, time.September: 110, time.October: 1000}
	for m, v := range level {
		for d := time.Date(2026, m, 1, 0, 0, 0, 0, time.UTC); d.Month() == m; d = d.AddDate(0, 0, 1) {
			g[d.Format("2006-01-02")] = v
		}
	}
	// Sep over Aug is +10 %, dampened to +5 %; July pairs are left out.
	if gr := monthGrowth(g, f, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)); math.Abs(gr-0.05) > 1e-9 {
		t.Fatalf("growth %v", gr)
	}
	// Oct over Sep is +809 %: capped.
	if gr := monthGrowth(g, f, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)); gr != growthCap {
		t.Fatalf("growth %v, want cap", gr)
	}
}

func TestWeekdayRemainingShape(t *testing.T) {
	days := synthDays(14, 1000, 1000)
	rem, ok := weekdayRemaining(days, oct, dayOf(oct, 14))
	if !ok || len(rem) != 17 {
		t.Fatalf("rem %v %v", ok, rem)
	}
	// Oct 17 is a Saturday, Oct 19 a Monday.
	if rem[3]*5 > rem[5] {
		t.Fatalf("weekend %v not well below Monday %v", rem[3], rem[5])
	}
}

func TestDailyForecastSeries(t *testing.T) {
	f := buildCostForecast(synthDays(15, 1000, 1000), nil, oct, 0, budgetMissing)
	if len(f.DailyForecast) != 16 {
		t.Fatalf("points %d", len(f.DailyForecast))
	}
	first, end := f.DailyForecast[0], f.DailyForecast[15]
	if first.Date != "2026-10-16" || end.Date != "2026-10-31" {
		t.Fatalf("dates %s %s", first.Date, end.Date)
	}
	m := f.Models[0]
	if math.Abs(end.CumulativeNetUSD-m.NetMidUSD) > 1e-6 || math.Abs(end.Low-m.NetLowUSD) > 1e-6 || math.Abs(end.High-m.NetHighUSD) > 1e-6 {
		t.Fatalf("end %+v model %+v", end, m)
	}
	prev := f.NetToDateUSD
	for _, p := range f.DailyForecast {
		if p.CumulativeNetUSD < prev || p.Low > p.CumulativeNetUSD || p.High < p.CumulativeNetUSD {
			t.Fatalf("point %+v after %v", p, prev)
		}
		prev = p.CumulativeNetUSD
	}
}

func TestPickShown(t *testing.T) {
	both := map[string]ModelForecast{modelTrend: {}, modelWeekday: {}}
	cases := []struct {
		models map[string]ModelForecast
		errs   map[string]float64
		want   string
	}{
		{both, map[string]float64{modelTrend: 9, modelWeekday: 8}, modelWeekday},
		{both, map[string]float64{modelTrend: 8, modelWeekday: 9}, modelTrend},
		{both, map[string]float64{modelTrend: 8}, modelTrend}, // weekday not backtested
		{both, nil, modelTrend},
		{map[string]ModelForecast{modelWeekday: {}}, nil, modelWeekday},
	}
	for i, c := range cases {
		if got, _ := pickShown(c.models, c.errs); got != c.want {
			t.Errorf("case %d: %s, want %s", i, got, c.want)
		}
	}
}

// TestPublicHolidaysCoverThisYear fails once the static calendar runs out.
func TestPublicHolidaysCoverThisYear(t *testing.T) {
	year := time.Now().Format("2006")
	for d := range publicHolidays {
		if d[:4] >= year {
			return
		}
	}
	t.Fatalf("publicHolidays has no dates in %s: add them", year)
}

func TestDailyForecastWithinAllowance(t *testing.T) {
	f := ModelForecast{NetMidUSD: 100, RangePct: 15, cumNet: []float64{100, 100}}
	pts := dailySeries(f, 100, oct, dayOf(oct, 29))
	if math.Abs(pts[1].High-115) > 1e-9 || math.Abs(pts[0].High-107.5) > 1e-9 {
		t.Fatalf("points %+v", pts)
	}
}
