package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"cloud.google.com/go/civil"
)

// Nav-wide phase counts name no one: small cells are averaged per week and
// shown as is, and a phase with no users that week is 0.
func TestAdoptionCohortsWeeklyKeepsSmallCells(t *testing.T) {
	mon := civil.Date{Year: 2026, Month: 9, Day: 7}
	tue := civil.Date{Year: 2026, Month: 9, Day: 8}
	h := &BigQueryHandlers{bqClient: &mockBigQueryClient{cohorts: []AdoptionCohortDay{
		{Day: mon, Phase: 0, UserCount: 50}, {Day: mon, Phase: 1, UserCount: 2},
		{Day: tue, Phase: 0, UserCount: 50}, {Day: tue, Phase: 1, UserCount: 4},
	}}}
	rec := httptest.NewRecorder()
	h.handleAdoptionCohorts(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/cohorts", nil))
	var got []AdoptionCohortWeek
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := []AdoptionCohortWeek{{Week: mon, Phase: 0, UserCount: 50}, {Week: mon, Phase: 1, UserCount: 3}, {Week: mon, Phase: 2}, {Week: mon, Phase: 3}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %+v, want %+v", got, want)
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

// Credits per user and cohort retention are Nav-wide and name no one, so their
// queries carry no minimum group size.
func TestAnonymousTrendQueriesHaveNoMinimum(t *testing.T) {
	b, err := os.ReadFile("monthly_trends.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "@min_users") {
		t.Error("monthly_trends.go still filters on @min_users")
	}
}
