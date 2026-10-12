package main

import (
	"math"
	"time"
)

// Year model (owner decision 2026-10-12): a range from flat to continued
// growth. Parameters picked by a backtest from the cut points end of Jul
// and Aug 2026 (Jul→Aug, Jul→Sep, Aug→Sep): mean absolute error 26 %. Few
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
	// yearBandPct is the high end above the fitted trend: the backtest error, rounded up.
	yearBandPct = 30.0

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
// yearBandPct above mid. December gets christmasFactor on mid and high.
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
	mid, lo, hi := c+cur.NetMidUSD, c+cur.NetLowUSD, c+cur.NetHighUSD
	out = append(out, YearForecastPoint{Month: month.Format("2006-01"), CumulativeNetUSD: mid, Low: lo, High: hi, Forecast: true})
	for k := 1; month.AddDate(0, k, 0).Year() == month.Year(); k++ {
		m := month.AddDate(0, k, 0)
		trend := cur.NetMidUSD * math.Exp(g*float64(k))
		// Holidays pull mid and high down, but never below flat: low is the floor.
		switch m.Month() {
		case time.December:
			trend *= christmasFactor
		case holidayMonth:
			trend *= holidayNetFactor
		}
		trend = math.Max(trend, cur.NetMidUSD)
		mid, lo, hi = mid+trend, lo+cur.NetMidUSD, hi+trend*(1+yearBandPct/100)
		out = append(out, YearForecastPoint{Month: m.Format("2006-01"), CumulativeNetUSD: mid, Low: lo, High: hi, Forecast: true})
	}
	last := out[len(out)-1]
	return out, &YearEnd{Mid: last.CumulativeNetUSD, Low: last.Low, High: last.High}
}
