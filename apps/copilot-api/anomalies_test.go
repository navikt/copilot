package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// days builds daily gross from start: weekdays get wd, weekends we.
func anomalyDays(start string, n int, wd, we float64) []anomalyDay {
	t, _ := time.Parse(time.DateOnly, start)
	out := make([]anomalyDay, n)
	for i := range out {
		d := t.AddDate(0, 0, i)
		g := wd
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			g = we
		}
		out[i] = anomalyDay{Day: d.Format(time.DateOnly), Gross: g}
	}
	return out
}

func rules(a []Anomaly) map[string]int {
	m := map[string]int{}
	for _, x := range a {
		m[x.Rule]++
	}
	return m
}

var anomalyToday = time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

func TestFindAnomaliesQuiet(t *testing.T) {
	in := anomalyInput{Days: anomalyDays("2026-08-01", 60, 2000, 200), FirstDataDay: "2026-01-01", NetComplete: true}
	if got := findAnomalies(in, anomalyToday); len(got) != 0 {
		t.Fatalf("steady data gave %v", got)
	}
}

func TestFindAnomaliesA1(t *testing.T) {
	in := anomalyInput{Days: anomalyDays("2026-08-01", 60, 2000, 200), FirstDataDay: "2026-01-01", NetComplete: true}
	in.Days[50].Gross = 4100 // 2026-09-20, Sunday: far above weekend median 200
	in.Days[51].Gross = 4100 // Monday: above 2 × 2000 and 100 over
	in.Days[52].Gross = 3900 // Tuesday: below 2 × median, no hit
	in.Days[53].Gross = 450  // Wednesday, low: no hit
	got := findAnomalies(in, anomalyToday)
	if r := rules(got); r["A1"] != 2 || len(got) != 2 {
		t.Fatalf("want two A1, got %v", got)
	}
	if got[0].Date != "2026-09-20" || got[0].Threshold != 200 || got[0].Value != 4100 {
		t.Fatalf("first A1 %+v", got[0])
	}
	// Weekend spike under the 100 USD floor is not raised.
	in = anomalyInput{Days: anomalyDays("2026-08-01", 60, 2000, 20), FirstDataDay: "2026-01-01", NetComplete: true}
	in.Days[50].Gross = 90
	if got := findAnomalies(in, anomalyToday); len(got) != 0 {
		t.Fatalf("small spike raised %v", got)
	}
}

func TestFindAnomaliesA3(t *testing.T) {
	in := anomalyInput{Days: anomalyDays("2026-08-01", 60, 2000, 200), FirstDataDay: "2026-01-01", NetComplete: true,
		NewModels: []anomalyModel{{Model: "new", FirstDay: "2026-09-15"}, {Model: "old", FirstDay: "2026-08-15"}}}
	got := findAnomalies(in, anomalyToday)
	if len(got) != 1 || got[0].Rule != "A3" || got[0].Model != "new" {
		t.Fatalf("want A3 for new only, got %v", got)
	}
	// The first 28 days of data say nothing about new models.
	in.FirstDataDay = "2026-09-01"
	if got := findAnomalies(in, anomalyToday); len(got) != 0 {
		t.Fatalf("model in warm-up raised %v", got)
	}
}

func TestFindAnomaliesA7(t *testing.T) {
	days := anomalyDays("2026-08-01", 57, 2000, 200) // last day 2026-09-26
	days = append(days[:40], days[42:]...)           // gap 2026-09-10 and 11
	in := anomalyInput{Days: days, FirstDataDay: "2026-01-01", NetComplete: false}
	got := findAnomalies(in, anomalyToday)
	if r := rules(got); r["A7"] != 4 || len(got) != 4 {
		t.Fatalf("want stale + 2 gaps + net, got %v", got)
	}
	if got[0].Date != "2026-09-26" || got[1].Date != "2026-09-10" || got[3].Date != "2026-08" {
		t.Fatalf("A7 order %v", got)
	}
	// Net is not due before day 5, and two days of lag is normal.
	in = anomalyInput{Days: anomalyDays("2026-08-01", 33, 2000, 200), FirstDataDay: "2026-01-01"}
	if got := findAnomalies(in, time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC)); len(got) != 0 {
		t.Fatalf("early month raised %v", got)
	}
	if got := findAnomalies(anomalyInput{NetComplete: true}, anomalyToday); len(got) != 1 || got[0].Rule != "A7" {
		t.Fatalf("no data must raise A7, got %v", got)
	}
}

func TestCostAnomaliesAdminOnly(t *testing.T) {
	t.Setenv(adminGroupEnv, "admins")
	h := newBigQueryHandlers(&mockBigQueryClient{anomalies: []Anomaly{{Rule: "A1", Date: "2026-09-21"}}})
	mux := makeAPIRouter(&Config{}, h, nil, nil, nil)
	for _, tc := range []struct {
		groups []string
		want   int
	}{{nil, http.StatusNotFound}, {[]string{"admins"}, http.StatusOK}} {
		req := httptest.NewRequest("GET", "/api/v1/copilot/billing/anomalies", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Groups: tc.groups}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("groups %v: status %d", tc.groups, rec.Code)
		}
		if tc.want == http.StatusOK {
			var got []Anomaly
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got) != 1 {
				t.Fatalf("body %s", rec.Body)
			}
		}
	}
}

// Days before the June 2026 basis switch are no baseline for days after it.
func TestFindAnomaliesBasisSwitch(t *testing.T) {
	in := anomalyInput{Days: append(anomalyDays("2026-05-01", 31, 700, 100), anomalyDays("2026-06-01", 20, 2100, 300)...), FirstDataDay: "2026-01-01", NetComplete: true}
	if got := findAnomalies(in, time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)); len(got) != 0 {
		t.Fatalf("basis switch raised %v", got)
	}
}

// A gap that began before the query window is still reported inside it.
func TestFindAnomaliesGapFromBeforeWindow(t *testing.T) {
	in := anomalyInput{Days: anomalyDays("2026-09-05", 25, 2000, 200), FirstDataDay: "2026-01-01", NetComplete: true}
	if r := rules(findAnomalies(in, anomalyToday)); r["A7"] != 5 { // 08-31 .. 09-04
		t.Fatalf("want 5 A7 gaps, got %v", r)
	}
}
