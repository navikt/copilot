package main

import "testing"

func TestTeamNetOverviewDoesNotAddOverlappingTeams(t *testing.T) {
	result := teamNetOverview("2026-09", []teamNetRow{
		{TeamID: "a", TeamSlug: "a", Users: 5, Net: 100, KnownNet: 100, EnterpriseNet: 102,
			SmallTeams: 2, SmallUsers: 3, SmallNet: 40, UnassignedNet: 10},
		{TeamID: "b", TeamSlug: "b", Users: 5, Net: 90, KnownNet: 100, EnterpriseNet: 102,
			SmallTeams: 2, SmallUsers: 3, SmallNet: 40, UnassignedNet: 10},
	})
	if result == nil || len(result.Teams) != 2 || result.KnownNetUSD != 100 || result.ResidualNetUSD != 2 {
		t.Fatalf("unexpected overlapping overview: %+v", result)
	}
	if result.Teams[0].NetUSD+result.Teams[1].NetUSD <= result.EnterpriseNetUSD {
		t.Fatal("test data must demonstrate that overlapping team amounts exceed enterprise total")
	}
	if teamNetOverview("2026-09", nil) != nil {
		t.Fatal("a month without completion marker must not appear as complete")
	}
}
