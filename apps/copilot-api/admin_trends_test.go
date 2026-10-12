package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHighConsumerMonth(t *testing.T) {
	// 200 users: 2 at 1000, 8 at 100, 190 at 10. Total 1000*2+800+1900 = 4700.
	var rows []spendUserRow
	add := func(n int, net, gross float64) {
		for range n {
			rows = append(rows, spendUserRow{UserID: fmt.Sprint(len(rows)), Month: "2026-09", Net: net, Gross: gross})
		}
	}
	add(2, 1000, 1000)
	add(8, 100, 500)
	add(190, 10, 20)
	m := buildHighConsumers(rows)[0]
	if m.Users != 200 || m.Basis != "netto" || m.LimitUSD != 800 || !m.LimitChanged {
		t.Fatalf("month = %+v", m)
	}
	// Top 1 % is 2 people: hidden. Top 5 % is 10: (2000+800)/4700. Top 10 % is 20: +100.
	if m.Tiers[0].NetSharePct != nil || m.Tiers[0].Users != 0 {
		t.Errorf("top 1 %% not hidden: %+v", m.Tiers[0])
	}
	if got := *m.Tiers[1].NetSharePct; m.Tiers[1].Users != 10 || got != 59.6 {
		t.Errorf("top 5 %% = %d users %v %%, want 10 and 59.6", m.Tiers[1].Users, got)
	}
	if got := *m.Tiers[2].NetSharePct; m.Tiers[2].Users != 20 || got != 61.7 {
		t.Errorf("top 10 %% = %d users %v %%, want 20 and 61.7", m.Tiers[2].Users, got)
	}
	// Gross against 800: 20 is 2.5 % (0–10), 500 is 62.5 % (50–75), 1000 is over 100 %
	// with 2 users, merged into its neighbour. Empty bands stay, as on /innsikt/forbruk.
	want := "0–10\u00a0%:190 10–25\u00a0%:0 25–50\u00a0%:0 Over 50\u00a0%:10"
	var got []string
	for _, b := range m.BudgetBands {
		got = append(got, fmt.Sprintf("%s:%d", b.Label, b.Users))
	}
	if g := strings.Join(got, " "); g != want {
		t.Errorf("budget bands = %q", g)
	}
}

func TestHighConsumerTierGap(t *testing.T) {
	// 81 users: top 5 % is 5 people, top 10 % is 9, only 4 more than top 5 %.
	var rows []spendUserRow
	for i := range 81 {
		rows = append(rows, spendUserRow{Month: "2026-09", Net: float64(i)})
	}
	tiers := buildHighConsumers(rows)[0].Tiers
	if tiers[1].NetSharePct == nil || tiers[2].NetSharePct != nil {
		t.Errorf("want top 5 %% shown and top 10 %% hidden: %+v", tiers)
	}
}

func TestHighConsumerMonthPremiumAndSmall(t *testing.T) {
	rows := []spendUserRow{{Month: "2026-01", Net: 5}, {Month: "2026-01", Net: 1}, {Month: "2026-06", Net: 1}}
	got := buildHighConsumers(rows)
	if len(got) != 2 || got[0].Basis != "premium" || got[0].Users != 0 || len(got[0].Tiers) != 0 || got[0].BudgetBands != nil {
		t.Fatalf("small months must hide everything: %+v", got)
	}
}

func TestBudgetBandLabel(t *testing.T) {
	for _, tc := range []struct {
		first, last int
		want        string
	}{{0, 0, "0–10 %"}, {1, 2, "10–50 %"}, {5, 5, "90–100 %"}, {6, 6, "Over 100 %"}, {4, 6, "Over 75 %"}} {
		if got := budgetBandLabel(tc.first, tc.last); got != tc.want {
			t.Errorf("budgetBandLabel(%d,%d) = %q, want %q", tc.first, tc.last, got, tc.want)
		}
	}
}

func TestAdminTrendRoutesNeedAdmin(t *testing.T) {
	const group = "11111111-2222-3333-4444-555555555555"
	t.Setenv(adminGroupEnv, group)
	router := makeAPIRouter(&Config{}, newBigQueryHandlers(&mockBigQueryClient{}), nil, nil, nil)
	for _, path := range []string{"/api/v1/copilot/billing/admin-active-users", "/api/v1/copilot/billing/admin-high-consumers", "/api/v1/copilot/billing/admin-budget-days"} {
		for _, tc := range []struct {
			groups []string
			want   int
		}{{nil, http.StatusNotFound}, {[]string{group}, http.StatusOK}} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Groups: tc.groups}))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("%s groups=%v: %d, want %d", path, tc.groups, rec.Code, tc.want)
			}
			if tc.want == http.StatusOK && !strings.HasPrefix(rec.Header().Get("Cache-Control"), "private") {
				t.Errorf("%s: cache-control %q, want private", path, rec.Header().Get("Cache-Control"))
			}
		}
	}
}

func TestMonthToDate(t *testing.T) {
	if monthToDate(nil) != nil {
		t.Fatal("no rows must give no month")
	}
	var rows []budgetShareRow
	for i := range 12 {
		src := "estimate"
		if i < 3 { // under five: not reported
			src = "snapshot"
		}
		share := 0.05 // 0–10 %
		if i >= 8 {
			share = 1.5 // over 100 %, 4 users: merged into a neighbour
		}
		rows = append(rows, budgetShareRow{Day: "2026-10-12", Share: share, Source: src})
	}
	m := monthToDate(rows)
	if m.Month != "2026-10" || m.Basis != "hittil" || m.Users != 12 || m.Snapshots != 0 || m.LastDay != "2026-10-12" {
		t.Fatalf("month = %+v", m)
	}
	for _, b := range m.BudgetBands {
		if b.Users > 0 && b.Users < minBandUsers {
			t.Errorf("band %s has %d users", b.Label, b.Users)
		}
	}
	if small := monthToDate(rows[:4]); small.Users != 0 || small.BudgetBands != nil {
		t.Errorf("fewer than five users must hide everything: %+v", small)
	}
}
