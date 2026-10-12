package main

import "testing"

func TestBuildBudgetDays(t *testing.T) {
	rows := []budgetBucketRow{
		// Day one: 20 in 0–10 %, 3 in 10–25 % (merged up), 6 over 100 %.
		{Day: "2026-10-01", Bucket: 0, Users: 20},
		{Day: "2026-10-01", Bucket: 1, Users: 3, Snapshots: 3},
		{Day: "2026-10-01", Bucket: 6, Users: 6},
		// Day two: too few users to show.
		{Day: "2026-10-02", Bucket: 0, Users: 4, Snapshots: 4},
	}
	got := buildBudgetDays(rows)
	if len(got) != 2 {
		t.Fatalf("days = %d, want 2", len(got))
	}
	d := got[0]
	if d.Users != 29 || d.Snapshots != 0 {
		t.Errorf("users, snapshots = %d, %d; want 29, 0 (snapshots under five hidden)", d.Users, d.Snapshots)
	}
	sum := 0
	for _, b := range d.Bands {
		if b.Users > 0 && b.Users < minBandUsers {
			t.Errorf("band %s has %d users", b.Label, b.Users)
		}
		sum += b.Users
	}
	if sum != d.Users {
		t.Errorf("bands sum to %d, want %d", sum, d.Users)
	}
	if d.Bands[0].Label != "0–25\u00a0%" || d.Bands[0].Users != 23 {
		t.Errorf("first band = %+v, want 0–25 %% with 23", d.Bands[0])
	}
	if g := buildBudgetDays([]budgetBucketRow{{Day: "2026-10-03", Users: 10, Snapshots: 8}}); g[0].Snapshots != 0 {
		t.Errorf("snapshots = %d, want 0 when only two estimates remain", g[0].Snapshots)
	}
	if got[1].Bands != nil || got[1].Users != 0 {
		t.Errorf("small day = %+v, want hidden", got[1])
	}
}
