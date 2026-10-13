package main

import (
	"math"
	"time"
)

// Year model (owner decisions 2026-10-12): a wide, honest range from flat to
// continued growth, its high end calibrated by the backtest on every request.
// Window, damping and cap were picked by a backtest from the cut points end of
// Jul and Aug 2026 (Jul→Aug, Jul→Sep, Aug→Sep): mean absolute error 26 %. Few
// data points; treat them as a starting guess, not a calibration.
const (
	// yearFitMonths is how many closed months the log-linear trend is fitted on.
	yearFitMonths = 4
	// yearDamping scales the fitted monthly log growth before it compounds.
	yearDamping = 0.75
	// yearMaxPerUserGrowth caps the log growth of fakturert per active user per
	// month (≈ +49 %). Users at the personal limit cannot grow further while
	// the limit stays at 800 USD (from 2026-09-22).
	// ponytail: fixed cap, derive from the share near the limit in budget_bands when it moves the error.
	yearMaxPerUserGrowth = 0.4
	// yearBandFloorPct is the smallest high end above the trend; above it the
	// mean absolute backtest error of the closed AI-credits months sets it.
	yearBandFloorPct = 30.0
	// yearRoughMonths: below this many closed AI-credits months the range is labelled yearRoughLabel.
	yearRoughMonths = 6
	yearRoughLabel  = "grovt anslag, få måneder å bygge på"

	// holidayMonth is July, the only holiday observed. Its fakturert was 0.55 of
	// the geometric mean of June and August, its active users 0.86.
	holidayMonth      = time.July
	holidayNetFactor  = 0.55
	holidayUserFactor = 0.86
	// christmasFromDay: December runs at the July rate from about 20 December.
	christmasFromDay = 20
)

// christmasFactor is December's share of a normal month: 12 of 31 days at the July rate.
var christmasFactor = 1 - float64(31-christmasFromDay+1)/31*(1-holidayNetFactor)

// YearForecastPoint is cumulative fakturert from 1 January to the end of month.
type YearForecastPoint struct {
	Month            string  `json:"month"`
	CumulativeNetUSD float64 `json:"cumulative_net_usd"`
	Low              float64 `json:"low"`
	High             float64 `json:"high"`
	Forecast         bool    `json:"forecast"`
}

// YearEnd is the forecast for 31 December.
type YearEnd struct {
	Mid  float64 `json:"mid"`
	Low  float64 `json:"low"`
	High float64 `json:"high"`
	// RangePct is how far high lies above the trend: the backtest error, at least yearBandFloorPct.
	RangePct float64 `json:"range_pct"`
	// BacktestErrorPct is the mean absolute error one and two months ahead; 0 without a backtest.
	BacktestErrorPct float64 `json:"backtest_error_pct"`
	Label            string  `json:"label,omitempty"`
}

// seasonal is the share of a normal month that month is billed.
func seasonal(m time.Month) float64 {
	switch m {
	case time.December:
		return christmasFactor
	case holidayMonth:
		return holidayNetFactor
	}
	return 1
}

// yearBacktest predicts each closed AI-credits month one and two months ahead
// from the months before it, as the model would have, and returns the mean
// absolute error in percent. Limits: horizons of one and two months only,
// while the forecast looks further; and the holiday factors were derived from
// June and August, so predictions of August see a little of it.
func yearBacktest(closed []AdminMonth, au map[string]int64) (float64, int) {
	var sum float64
	var n int
	for i := range closed {
		g, ok := yearGrowth(closed[:i+1], au)
		if !ok || *closed[i].EnterpriseNetUSD <= 0 {
			continue
		}
		cut, _ := time.Parse("2006-01", closed[i].Month)
		base := *closed[i].EnterpriseNetUSD / seasonal(cut.Month())
		for h := 1; h <= 2 && i+h < len(closed); h++ {
			t := closed[i+h]
			if t.Month < aiCreditStart || *t.EnterpriseNetUSD <= 0 {
				continue
			}
			tm, _ := time.Parse("2006-01", t.Month)
			// The same floor as the forecast: never below flat.
			pred := math.Max(base*math.Exp(g*float64(h)), base) * seasonal(tm.Month())
			sum += math.Abs(pred-*t.EnterpriseNetUSD) / *t.EnterpriseNetUSD * 100
			n++
		}
	}
	if n == 0 {
		return 0, 0
	}
	return sum / float64(n), n
}

// yearGrowth fits log(active users) and log(fakturert per active user) on the
// last yearFitMonths closed months, holiday-adjusted, and returns the damped
// monthly log growth of fakturert. ok is false with fewer than two months.
func yearGrowth(months []AdminMonth, users map[string]int64) (g float64, ok bool) {
	var xs, lu, lp []float64
	for _, m := range months {
		if m.EnterpriseNetUSD == nil || *m.EnterpriseNetUSD <= 0 || users[m.Month] <= 0 {
			continue
		}
		t, err := time.Parse("2006-01", m.Month)
		if err != nil {
			continue
		}
		net, u := *m.EnterpriseNetUSD, float64(users[m.Month])
		if t.Month() == holidayMonth {
			net, u = net/holidayNetFactor, u/holidayUserFactor
		}
		xs, lu, lp = append(xs, float64(t.Year()*12+int(t.Month()))), append(lu, math.Log(u)), append(lp, math.Log(net/u))
	}
	if len(xs) > yearFitMonths {
		xs, lu, lp = xs[len(xs)-yearFitMonths:], lu[len(lu)-yearFitMonths:], lp[len(lp)-yearFitMonths:]
	}
	if len(xs) < 2 {
		return 0, false
	}
	return yearDamping * (slope(xs, lu) + math.Min(slope(xs, lp), yearMaxPerUserGrowth)), true
}

func slope(xs, ys []float64) float64 {
	var mx, my float64
	for i := range xs {
		mx, my = mx+xs[i], my+ys[i]
	}
	mx, my = mx/float64(len(xs)), my/float64(len(xs))
	var num, den float64
	for i := range xs {
		num += (xs[i] - mx) * (ys[i] - my)
		den += (xs[i] - mx) * (xs[i] - mx)
	}
	return num / den
}

// yearForecast builds cumulative fakturert per month to 31 December: closed
// months as billed, the current month from cur, then each remaining month low
// flat at cur's midpoint, mid on the damped trend (never below flat) and high
// the backtest error (at least yearBandFloorPct) above mid. December gets christmasFactor on mid and high.
func yearForecast(months []AdminMonth, users []ActiveUsersMonth, cur ModelForecast, month time.Time) ([]YearForecastPoint, *YearEnd) {
	au := map[string]int64{}
	for _, u := range users {
		au[u.Month] = u.ActiveUsers
	}
	var closed []AdminMonth
	out := []YearForecastPoint{}
	var c float64
	for _, m := range months {
		if m.Month >= month.Format("2006-01") || m.EnterpriseNetUSD == nil {
			continue
		}
		// Last year's months only feed the fit.
		closed = append(closed, m)
		if m.Month[:4] != month.Format("2006") {
			continue
		}
		c += *m.EnterpriseNetUSD
		out = append(out, YearForecastPoint{Month: m.Month, CumulativeNetUSD: c, Low: c, High: c})
	}
	g, ok := yearGrowth(closed, au)
	if !ok || cur.NetMidUSD <= 0 {
		return out, nil
	}
	errPct, _ := yearBacktest(closed, au)
	end := YearEnd{BacktestErrorPct: errPct, RangePct: math.Max(yearBandFloorPct, errPct)}
	credits := 0
	for _, m := range closed {
		if m.Month >= aiCreditStart && *m.EnterpriseNetUSD > 0 {
			credits++
		}
	}
	if credits < yearRoughMonths {
		end.Label = yearRoughLabel
	}
	mid, lo, hi := c+cur.NetMidUSD, c+cur.NetLowUSD, c+cur.NetHighUSD
	out = append(out, YearForecastPoint{Month: month.Format("2006-01"), CumulativeNetUSD: mid, Low: lo, High: hi, Forecast: true})
	for k := 1; month.AddDate(0, k, 0).Year() == month.Year(); k++ {
		m := month.AddDate(0, k, 0)
		trend := cur.NetMidUSD * math.Exp(g*float64(k))
		// Holidays pull mid and high down, but never below flat: low is the floor.
		trend = math.Max(trend*seasonal(m.Month()), cur.NetMidUSD)
		mid, lo, hi = mid+trend, lo+cur.NetMidUSD, hi+trend*(1+end.RangePct/100)
		out = append(out, YearForecastPoint{Month: m.Format("2006-01"), CumulativeNetUSD: mid, Low: lo, High: hi, Forecast: true})
	}
	last := out[len(out)-1]
	end.Mid, end.Low, end.High = last.CumulativeNetUSD, last.Low, last.High
	return out, &end
}
