package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/civil"
)

func TestTeamSummaryDropsTeamsBelowFive(t *testing.T) {
	h := &BigQueryHandlers{bqClient: &mockBigQueryClient{teamUsage: []TeamUsageSummary{
		{TeamSlug: "small", AvgActiveUsers: 4},
		{TeamSlug: "big", AvgActiveUsers: 5},
	}}}
	rec := httptest.NewRecorder()
	h.handleTeamUsageSummary(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-summary", nil))
	var got []TeamUsageSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TeamSlug != "big" {
		t.Fatalf("got %+v, want only big", got)
	}
}

func TestAdoptionCohortsWeeklySuppressesSmallAndComplementaryCells(t *testing.T) {
	// 2026-09-07 is a Monday; Tue and Sun of the same week fold into it.
	mon := civil.Date{Year: 2026, Month: 9, Day: 7}
	tue := civil.Date{Year: 2026, Month: 9, Day: 8}
	sun := civil.Date{Year: 2026, Month: 9, Day: 13}
	next := civil.Date{Year: 2026, Month: 9, Day: 14}
	h := &BigQueryHandlers{bqClient: &mockBigQueryClient{cohorts: []AdoptionCohortDay{
		// Week 1 averages: phase 0 = 50, phase 1 = 2, phase 2 = (6+6+7)/3 = 6, phase 3 = 20.
		// Phase 1 (2) is small; phase 2 (6) must go too, else 2 = total - 50 - 6 - 20.
		{Day: mon, Phase: 0, UserCount: 50}, {Day: mon, Phase: 1, UserCount: 6},
		{Day: mon, Phase: 2, UserCount: 6}, {Day: mon, Phase: 3, UserCount: 20},
		{Day: tue, Phase: 0, UserCount: 50}, {Day: tue, Phase: 2, UserCount: 6},
		{Day: tue, Phase: 3, UserCount: 20},
		{Day: sun, Phase: 0, UserCount: 50}, {Day: sun, Phase: 2, UserCount: 7},
		{Day: sun, Phase: 3, UserCount: 20},
		// Week 2: hidden cells already sum to 6, no extra cell needed.
		{Day: next, Phase: 0, UserCount: 50}, {Day: next, Phase: 1, UserCount: 3},
		{Day: next, Phase: 2, UserCount: 3}, {Day: next, Phase: 3, UserCount: 20},
	}}}
	rec := httptest.NewRecorder()
	h.handleAdoptionCohorts(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/cohorts", nil))
	var got []AdoptionCohortWeek
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := []AdoptionCohortWeek{
		{Week: mon, Phase: 0, UserCount: 50}, {Week: mon, Phase: 3, UserCount: 20},
		{Week: next, Phase: 0, UserCount: 50}, {Week: next, Phase: 3, UserCount: 20},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].Week != want[i].Week || got[i].Phase != want[i].Phase || got[i].UserCount != want[i].UserCount {
			t.Errorf("row %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestAdoptionCohortsSuppressesBeforeRounding(t *testing.T) {
	mon := civil.Date{Year: 2026, Month: 9, Day: 7}
	tue := civil.Date{Year: 2026, Month: 9, Day: 8}
	// Phase 1 averages 4.5, which rounds to 5 but is below the threshold.
	got := suppressSmallCohorts(weeklyCohorts([]AdoptionCohortDay{
		{Day: mon, Phase: 0, UserCount: 50}, {Day: mon, Phase: 1, UserCount: 4},
		{Day: tue, Phase: 0, UserCount: 50}, {Day: tue, Phase: 1, UserCount: 5},
	}))
	for _, r := range got {
		if r.Phase == 1 {
			t.Errorf("cell below threshold leaked after rounding: %+v", r)
		}
	}
}

func TestTeamAdoptionOmitsSmallTeamsAndCountsOnlyVisible(t *testing.T) {
	got := teamAdoptionOverview([]TeamAdoption{
		{TeamSlug: "big", ActiveRepos: 10, ReposWithCustomizations: 4, AdoptionRate: 0.4, RecentlyActiveRepos: 3, AdoptionRateActiveOnly: 0.666},
		{TeamSlug: "none", ActiveRepos: 5},
		{TeamSlug: "small", ActiveRepos: 4, ReposWithCustomizations: 4, AdoptionRate: 1},
		{TeamSlug: "inactive", ActiveRepos: 0},
	})
	if len(got.Teams) != 2 || got.Teams[0].TeamSlug != "big" || got.SmallTeams != 1 {
		t.Fatalf("got %+v", got)
	}
	// Totals cover visible teams only, so total minus visible rows reveals nothing.
	if got.TotalTeams != 2 || got.TeamsWithAdoption != 1 || got.AdoptionPct != 50 {
		t.Fatalf("summary %+v", got)
	}
	if got.Teams[0].AdoptionPct != 40 || *got.Teams[0].AdoptionActivePct != 67 || got.Teams[1].AdoptionActivePct != nil {
		t.Fatalf("rates %+v %+v", got.Teams[0], got.Teams[1])
	}
}
