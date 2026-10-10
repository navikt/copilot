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

func TestAdoptionCohortsSuppressesSmallAndComplementaryCells(t *testing.T) {
	d1 := civil.Date{Year: 2026, Month: 9, Day: 1}
	d2 := civil.Date{Year: 2026, Month: 9, Day: 2}
	h := &BigQueryHandlers{bqClient: &mockBigQueryClient{cohorts: []AdoptionCohortDay{
		// Day 1: phase 1 has 2 users; phase 2 (6) must go too, else 2 = total - 50 - 6 - 20.
		{Day: d1, Phase: 0, UserCount: 50}, {Day: d1, Phase: 1, UserCount: 2},
		{Day: d1, Phase: 2, UserCount: 6}, {Day: d1, Phase: 3, UserCount: 20},
		// Day 2: hidden cells already sum to 6, no extra cell needed.
		{Day: d2, Phase: 0, UserCount: 50}, {Day: d2, Phase: 1, UserCount: 3},
		{Day: d2, Phase: 2, UserCount: 3}, {Day: d2, Phase: 3, UserCount: 20},
	}}}
	rec := httptest.NewRecorder()
	h.handleAdoptionCohorts(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/cohorts", nil))
	var got []AdoptionCohortDay
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d rows, want 4: %+v", len(got), got)
	}
	for _, r := range got {
		if r.UserCount < minUsersForDistribution || r.Phase == 1 || r.Phase == 2 {
			t.Errorf("cell leaked: %+v", r)
		}
	}
}
