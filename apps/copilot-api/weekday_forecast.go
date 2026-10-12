package main

import (
	"math"
	"time"
)

// The weekday model projects enterprise gross for each remaining day as
//
//	base level × weekday factor × growth
//
// Weekday factors are each weekday's ratio to the weekly mean over the last
// factorWeeks complete weeks outside holidays. The base level is the
// weekday-adjusted daily mean this month so far, blended with last month's
// level grown by the month-on-month growth; this month weighs more as days
// pass. Growth is the mean month-on-month growth of the last growthMonths
// closed months, dampened and capped.
//
// Backtest 2026-10-12 on prod data, Aug and Sep at day 5, 10, 15, 20 and 25:
// mean absolute midpoint error 19.9 % against 8.7 % for the trend model. It
// runs low: usage grew 30–40 % a month, well past growthCap, and the
// month-to-date mean lags growth within the month. So trend stays shown;
// buildCostForecast switches when this model backtests better.
const (
	factorWeeks   = 8
	growthMonths  = 3
	growthDamping = 0.5
	growthCap     = 0.15 // per month, both ways
	// blendDays is how many days of this month it takes before the base
	// level rests on this month alone.
	blendDays = 10.0
)

// holidayPeriods are dates (inclusive, MM-DD) with holiday usage: left out of
// weekday factors and growth. Days in July and around Christmas.
var holidayPeriods = [][2]string{{"07-01", "07-31"}, {"12-20", "12-31"}, {"01-01", "01-03"}}

// publicHolidays are Norwegian public holidays on weekdays. A remaining
// holiday (these or holidayPeriods) is projected as a Sunday.
//
// ponytail: static list; TestPublicHolidaysCoverThisYear fails when it runs
// out. Compute Easter if extending it each year gets tedious.
var publicHolidays = map[string]bool{
	"2026-01-01": true, "2026-04-02": true, "2026-04-03": true, "2026-04-06": true,
	"2026-05-01": true, "2026-05-14": true, "2026-05-25": true, "2026-12-25": true,
	"2027-01-01": true, "2027-03-25": true, "2027-03-26": true, "2027-03-29": true,
	"2027-05-06": true, "2027-05-17": true,
}

func isHoliday(t time.Time) bool {
	if publicHolidays[t.Format("2006-01-02")] {
		return true
	}
	md := t.Format("01-02")
	for _, p := range holidayPeriods {
		if md >= p[0] && md <= p[1] {
			return true
		}
	}
	return false
}

// weekdayFactors returns each weekday's (time.Weekday index) ratio to the
// weekly mean over the last factorWeeks complete Mon–Sun weeks up to last,
// skipping weeks with a holiday. ok is false with fewer than two weeks.
func weekdayFactors(gross map[string]float64, last time.Time) (f [7]float64, ok bool) {
	// The Sunday on or before last ends the latest complete week.
	end := last.AddDate(0, 0, -int(last.Weekday()))
	weeks := 0
	for w := 0; weeks < factorWeeks && w < 4*factorWeeks; w++ {
		sun := end.AddDate(0, 0, -7*w)
		var vals [7]float64
		var sum float64
		skip := false
		for i := 0; i < 7 && !skip; i++ {
			d := sun.AddDate(0, 0, -i)
			v, has := gross[d.Format("2006-01-02")]
			skip = !has || isHoliday(d)
			vals[d.Weekday()] = v
			sum += v
		}
		if skip || sum <= 0 {
			continue
		}
		for i := range vals {
			f[i] += vals[i] / (sum / 7)
		}
		weeks++
	}
	if weeks < 2 {
		return f, false
	}
	for i := range f {
		f[i] /= float64(weeks)
	}
	return f, true
}

// levelOf is the weekday-adjusted daily mean gross of month's days up to
// last, leaving out holidays. ok is false without such days.
func levelOf(gross map[string]float64, f [7]float64, month, last time.Time) (float64, bool) {
	var sum float64
	n := 0
	for k := 1; k <= monthDays(month); k++ {
		t := dayOf(month, k)
		v, has := gross[t.Format("2006-01-02")]
		if t.After(last) || !has || isHoliday(t) || f[t.Weekday()] <= 0 {
			continue
		}
		sum += v / f[t.Weekday()]
		n++
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// monthGrowth is the dampened, capped mean month-on-month growth in level
// over the last growthMonths closed months before month; holiday months
// (July) and the months after them are left out. 0 without any pair.
func monthGrowth(gross map[string]float64, f [7]float64, month time.Time) float64 {
	var sum float64
	n := 0
	for i := 1; i <= growthMonths; i++ {
		m, p := month.AddDate(0, -i, 0), month.AddDate(0, -i-1, 0)
		if isHoliday(dayOf(m, 15)) || isHoliday(dayOf(p, 15)) {
			continue
		}
		lm, ok1 := levelOf(gross, f, m, dayOf(m, monthDays(m)))
		lp, ok2 := levelOf(gross, f, p, dayOf(p, monthDays(p)))
		if ok1 && ok2 && lp > 0 {
			sum += lm/lp - 1
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return math.Max(-growthCap, math.Min(growthCap, growthDamping*sum/float64(n)))
}

// blendBase weighs this month's level by days/blendDays (at most 1) against
// last month's level grown by growth.
func blendBase(cur float64, hasCur bool, prev float64, hasPrev bool, growth float64, days int) float64 {
	w := math.Min(1, float64(days)/blendDays)
	switch {
	case !hasPrev:
		return cur
	case !hasCur:
		return prev * (1 + growth)
	}
	return w*cur + (1-w)*prev*(1+growth)
}

// weekdayRemaining projects enterprise gross for each day after last with the
// weekday model.
func weekdayRemaining(days []CostDay, month, last time.Time) ([]float64, bool) {
	gross := map[string]float64{}
	for _, d := range days {
		if t, err := time.Parse("2006-01-02", d.Day); err == nil && !t.After(last) {
			gross[d.Day] = d.Gross
		}
	}
	f, ok := weekdayFactors(gross, last)
	if !ok {
		return nil, false
	}
	g := monthGrowth(gross, f, month)
	prevM := month.AddDate(0, -1, 0)
	cur, hasCur := levelOf(gross, f, month, last)
	prev, hasPrev := levelOf(gross, f, prevM, dayOf(prevM, monthDays(prevM)))
	if !hasCur && !hasPrev {
		return nil, false
	}
	base := blendBase(cur, hasCur, prev, hasPrev, g, last.Day())
	// Growth runs from the middle of the days the base level rests on.
	w := math.Min(1, float64(last.Day())/blendDays)
	if !hasPrev {
		w = 1
	} else if !hasCur {
		w = 0
	}
	mid := w*float64(last.Day()+1)/2 + (1-w)*float64(monthDays(month)+1)/2
	rem := make([]float64, 0, monthDays(month)-last.Day())
	for k := last.Day() + 1; k <= monthDays(month); k++ {
		t := dayOf(month, k)
		fk := f[t.Weekday()]
		if isHoliday(t) {
			fk = f[time.Sunday]
		}
		rem = append(rem, base*fk*math.Pow(1+g, (float64(k)-mid)/30))
	}
	return rem, true
}
