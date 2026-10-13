package main

import (
	"time"
)

// Weekday shape: the trend model's remaining gross is spread over the
// remaining days by each weekday's share, so the daily line dips at weekends
// while the month-end total stays the trend's. Factors are each weekday's
// ratio to the weekly mean over the last factorWeeks complete weeks outside
// holidays.
const factorWeeks = 8

// holidayPeriods are dates (inclusive, MM-DD) with holiday usage: left out of
// weekday factors. Days in July and around Christmas.
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

// shapeByWeekday spreads the total of rem over the same days in proportion
// to each day's weekday factor (holidays as Sundays). rem is returned as is
// without factors.
func shapeByWeekday(days []CostDay, month, last time.Time, rem []float64) []float64 {
	gross := map[string]float64{}
	for _, d := range days {
		gross[d.Day] = d.Gross
	}
	f, ok := weekdayFactors(gross, last)
	if !ok {
		return rem
	}
	var total, weights float64
	w := make([]float64, len(rem))
	for i, v := range rem {
		t := dayOf(month, last.Day()+1+i)
		w[i] = f[t.Weekday()]
		if isHoliday(t) {
			w[i] = f[time.Sunday]
		}
		total += v
		weights += w[i]
	}
	if weights <= 0 {
		return rem
	}
	out := make([]float64, len(rem))
	for i := range w {
		out[i] = total * w[i] / weights
	}
	return out
}
