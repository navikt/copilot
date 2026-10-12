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

func TestShapeKeepsTrendTotalAndDipsAtWeekends(t *testing.T) {
	days := synthDays(14, 1000, 1000)
	last := dayOf(oct, 14)
	rem, _ := trendRemaining(days, oct, last)
	shaped := shapeByWeekday(days, oct, last, rem)
	var a, b float64
	for i := range rem {
		a += rem[i]
		b += shaped[i]
	}
	if math.Abs(a-b) > 1e-6 {
		t.Fatalf("total %v, trend %v", b, a)
	}
	// Oct 17 is a Saturday, Oct 19 a Monday.
	if shaped[3]*5 > shaped[5] {
		t.Fatalf("weekend %v not well below Monday %v", shaped[3], shaped[5])
	}
	f := buildCostForecast(days, nil, oct, 0, budgetMissing)
	end := f.DailyForecast[len(f.DailyForecast)-1].CumulativeNetUSD
	_, net, _, _, _, _ := poolNet(days, oct, last, rem)
	if math.Abs(end-net) > 1e-6 || math.Abs(f.Models[0].NetMidUSD-net) > 1e-6 {
		t.Fatalf("end %v mid %v, trend net %v", end, f.Models[0].NetMidUSD, net)
	}
}

func TestRangeSwitchesAtDay8(t *testing.T) {
	for _, c := range []struct {
		day   int
		pct   float64
		label string
	}{{7, earlyRangePct, earlyLabel}, {8, rangeFloorPct, ""}} {
		f := buildCostForecast(synthDays(c.day, 1000, 1000), nil, oct, 0, budgetMissing)
		if len(f.Models) == 0 || f.Models[0].RangePct != c.pct || f.UncertaintyLabel != c.label {
			t.Fatalf("day %d: %+v", c.day, f)
		}
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
