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
		if got[i] != want[i] {
			t.Errorf("row %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
