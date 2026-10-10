package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestLimitAtMonthEnd(t *testing.T) {
	for month, want := range map[string]float64{"2026-05": 0, "2026-06": 200, "2026-08": 400, "2026-09": 800, "2027-01": 800} {
		if got := limitAtMonthEnd(month); got != want {
			t.Errorf("%s: %v, want %v", month, got, want)
		}
	}
}

func TestLimitChanged(t *testing.T) {
	if !limitChanged("2026-09") || limitChanged("2026-08") || limitChanged("2026-10") {
		t.Error("only June, July and September 2026 have a change")
	}
}

func TestBandIndex(t *testing.T) {
	for _, tc := range []struct {
		net  float64
		want int
	}{{0, 0}, {-3, 0}, {0.004, 0}, {1, 1}, {100, 1}, {100.01, 2}, {300, 3}, {360, 4}, {400, 5}, {400.01, 6}} {
		if got := bandIndex(tc.net, 400); got != tc.want {
			t.Errorf("bandIndex(%v): %d, want %d", tc.net, got, tc.want)
		}
	}
}

func repeat(v float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func TestMergeBands(t *testing.T) {
	// 0 %: 6, 1–25: 10, 25–50: 5, 50–75: 3, 75–90: 0, 90–100: 1, over 100: 2.
	nets := append(append(append(append(append(repeat(0, 6), repeat(50, 10)...), repeat(150, 5)...), repeat(250, 3)...), repeat(380, 1)...), repeat(500, 2)...)
	got := mergeBands(nets, 400)
	want := []SpendBand{
		// 50–75 (3) joins 25–50; 90–100 (1) and over 100 (2) find no non-empty neighbour big enough and end up there too.
		{0, 0, "0\u00a0%", 6}, {1, 1, "1–25\u00a0%", 10}, {2, 6, "over 25\u00a0%", 11},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	for _, b := range got {
		if b.Users > 0 && b.Users < minBandUsers {
			t.Errorf("band %q has %d users", b.Label, b.Users)
		}
	}
	if mergeBands(repeat(10, 4), 400) != nil {
		t.Error("four users in total must give no bands")
	}
	// A small band at the top merges down.
	got = mergeBands(append(repeat(50, 5), repeat(390, 2)...), 400)
	if got[1].Label != "1–100\u00a0%" || got[1].Users != 7 || got[2].Label != "over 100\u00a0%" || got[2].Users != 0 {
		t.Errorf("unexpected bands %+v", got)
	}
	for _, b := range got {
		if b.Users > 0 && b.Users < minBandUsers {
			t.Errorf("band %q has %d users", b.Label, b.Users)
		}
	}
}

func TestForecastTotals(t *testing.T) {
	got := forecastTotals([]float64{30000, 60000, 66000}, "2026-10", 3)
	// Trend: slope 18000, intercept 34000 → 88000, 106000, 124000; low stays at 66000.
	want := []SpendForecastMonth{
		{"2026-11", 66000, 88000, 110000},
		{"2026-12", 66000, 106000, 146000},
		{"2027-01", 66000, 124000, 182000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	// A falling trend never goes below zero, and low is never above high.
	for _, f := range forecastTotals([]float64{1000, 0}, "2026-10", 3) {
		if f.LowUSD < 0 || f.MidUSD < 0 || f.LowUSD > f.HighUSD {
			t.Errorf("bad falling forecast %+v", f)
		}
	}
}

func TestBuildSpendBands(t *testing.T) {
	var hist []spendUserRow
	for i := 0; i < 10; i++ {
		hist = append(hist, spendUserRow{UserID: "a" + strconv.Itoa(i), Month: "2026-08", Net: 10, Gross: 20})
	}
	for i := 0; i < 10; i++ {
		hist = append(hist, spendUserRow{UserID: "u" + strconv.Itoa(i), Month: "2026-09", Net: 75, Gross: 100})
	}
	// 15 of 30 days, ratio 0.75, weight 0.5. u0–u5: 0.5×150 + 0.5×75 = 112.5.
	// u6–u9 have no usage yet but are counted from September: 37.5. n1 is new: 75.
	var cur []spendUserRow
	for i := 0; i < 6; i++ {
		cur = append(cur, spendUserRow{UserID: "u" + strconv.Itoa(i), Gross: 100})
	}
	cur = append(cur, spendUserRow{UserID: "n1", Gross: 100})
	got := buildSpendBands(hist, cur, "2026-11-15")
	if len(got.Months) != 2 || got.Months[0].LimitUSD != 400 || got.Months[1].LimitUSD != 800 || !got.Months[1].LimitChanged {
		t.Fatalf("months: %+v", got.Months)
	}
	if got.NetRatio != 0.75 || got.RunWeight != 0.5 || got.Days != 15 || got.DaysInMonth != 30 {
		t.Errorf("ratio %v weight %v days %d/%d", got.NetRatio, got.RunWeight, got.Days, got.DaysInMonth)
	}
	if got.Current == nil || got.Current.Users != 11 || got.Current.Bands[1].Users != 11 {
		t.Errorf("current: %+v", got.Current)
	}
	if last := got.Totals[len(got.Totals)-1]; last.Month != "2026-11" || last.MidUSD != 900 {
		t.Errorf("totals: %+v", got.Totals)
	}
	if len(got.Forecast) != 3 || got.Forecast[0].Month != "2026-12" {
		t.Errorf("forecast: %+v", got.Forecast)
	}
}

func TestBuildSpendBandsDampsEarlyBursts(t *testing.T) {
	var hist, cur []spendUserRow
	for i := 0; i < 5; i++ {
		hist = append(hist, spendUserRow{UserID: "u" + strconv.Itoa(i), Month: "2026-09", Net: 100, Gross: 100})
		// 3 of 30 days at 30 USD: the raw run rate is 300 USD, past nothing; the burst user is at 3000.
		cur = append(cur, spendUserRow{UserID: "u" + strconv.Itoa(i), Gross: 30})
	}
	cur[0].Gross = 300
	got := buildSpendBands(hist, cur, "2026-11-03")
	// u0: 0.1×3000 + 0.9×100 = 390 (undamped 3000, far over the 800 limit).
	for _, b := range got.Current.Bands {
		if b.First == 6 && b.Users > 0 {
			t.Errorf("burst user still over the limit: %+v", got.Current.Bands)
		}
	}
	if total := got.Totals[len(got.Totals)-1].MidUSD; total != 900 { // 390 + 4×(0.1×300 + 90)=480 → 870 → 900
		t.Errorf("total %v", total)
	}
}

func TestSpendBandsHandlerSendsOnlyAggregates(t *testing.T) {
	h := newBigQueryHandlers(&mockBigQueryClient{spendBands: buildSpendBands(
		[]spendUserRow{{Month: "2026-09", Net: 1, Gross: 2}, {Month: "2026-09", Net: 1, Gross: 2}, {Month: "2026-09", Net: 1, Gross: 2}, {Month: "2026-09", Net: 1, Gross: 2}, {Month: "2026-09", Net: 1, Gross: 2}}, nil, "")})
	rec := httptest.NewRecorder()
	h.handleSpendBands(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/spend-bands", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, banned := range []string{"user_id", "login", "gross\""} {
		if strings.Contains(body, banned) {
			t.Errorf("response contains %q: %s", banned, body)
		}
	}
	var got SpendBands
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Months[0].Bands[1].Users != 5 {
		t.Errorf("unexpected body %s (%v)", body, err)
	}
}
