package bigquery

import (
	"encoding/json"
	"testing"
)

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
	if result.Teams[0].AmountUSD+result.Teams[1].AmountUSD <= result.EnterpriseNetUSD {
		t.Fatal("test data must demonstrate that overlapping team amounts exceed enterprise total")
	}
	if teamNetOverview("2026-09", nil) != nil {
		t.Fatal("a month without completion marker must not appear as complete")
	}
}

func TestTeamOverviewsOmitTeamsBelowFive(t *testing.T) {
	gross := BuildTeamGrossOverview("2026-09", []TeamGrossRow{
		{TeamID: "big", TeamSlug: "big", Users: 5, Gross: 10.006, SmallTeams: 1, SmallUsers: 4, SmallGross: 9},
		{TeamID: "small", TeamSlug: "small", Users: 4, Gross: 9, SmallTeams: 1, SmallUsers: 4, SmallGross: 9},
	}, nil, nil)
	net := teamNetOverview("2026-09", []teamNetRow{
		{TeamID: "big", TeamSlug: "big", Users: 6, Net: 10},
		{TeamID: "small", TeamSlug: "small", Users: 1, Net: 3},
	})
	for _, teams := range [][]TeamSpend{gross.Teams, net.Teams} {
		if len(teams) != 1 || teams[0].TeamID != "big" {
			t.Fatalf("teams = %+v, want only big", teams)
		}
	}
	if gross.Teams[0].AmountUSD != 10.01 || gross.Teams[0].PerUserUSD != 2 || net.Teams[0].PerUserUSD != 1.67 {
		t.Fatalf("rounding: gross %+v net %+v", gross.Teams[0], net.Teams[0])
	}
	// Complementary suppression: no hidden-team sum or total reaches the client.
	body, _ := json.Marshal(gross)
	var fields map[string]any
	_ = json.Unmarshal(body, &fields)
	for key := range fields {
		switch key {
		case "usage", "month", "teams", "small_teams", "last_usage_day", "comparison":
		default:
			t.Errorf("gross overview leaks %q", key)
		}
	}
	body, _ = json.Marshal(net)
	fields = nil
	_ = json.Unmarshal(body, &fields)
	for key := range fields {
		switch key {
		case "month", "teams", "small_teams", "loaded_at", "comparison":
		default:
			t.Errorf("net overview leaks %q", key)
		}
	}
}

func TestWithChanges(t *testing.T) {
	cur := []TeamSpend{NewTeamSpend("a", "a", 5, 120), NewTeamSpend("b", "b", 5, 105), NewTeamSpend("c", "c", 5, 50), NewTeamSpend("d", "d", 5, 10)}
	prev := []TeamSpend{NewTeamSpend("a", "a", 5, 100), NewTeamSpend("b", "b", 5, 100), NewTeamSpend("c", "c", 4, 1)}
	got := WithChanges(cur, prev)
	if got[0].ChangeUSD == nil || *got[0].ChangeUSD != 20 || !got[0].Highlight {
		t.Errorf("a = %+v, want +20 highlighted", got[0])
	}
	if got[1].ChangeUSD == nil || *got[1].ChangeUSD != 5 || got[1].Highlight {
		t.Errorf("b = %+v, want +5 not highlighted", got[1])
	}
	if got[2].ChangeUSD != nil || got[3].ChangeUSD != nil {
		t.Errorf("c/d must have no change: %+v %+v", got[2], got[3])
	}
	// Change uses raw amounts: 10.004 - 0.004 is exactly 10, not 10.00 - 0.00 rounded twice.
	if c := WithChanges([]TeamSpend{NewTeamSpend("e", "e", 5, 10.006)}, []TeamSpend{NewTeamSpend("e", "e", 5, 0.004)}); *c[0].ChangeUSD != 10 {
		t.Errorf("e change = %v, want 10", *c[0].ChangeUSD)
	}
	if cur[0].ChangeUSD != nil {
		t.Error("WithChanges mutated its input")
	}
}
