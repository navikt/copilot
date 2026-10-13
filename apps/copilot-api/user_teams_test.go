package main

import (
	"slices"
	"testing"
)

func TestUserTeamSlugs(t *testing.T) {
	// roster + 30-day fallback rows, overlapping, with the all-users team.
	got := userTeamSlugs([]string{"team-b", "nav-it-github-users", "team-a", "team-b", "", "team-c"})
	if want := []string{"team-a", "team-b", "team-c"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// Roster-only teams (no usage, absent from the team table) are kept.
	got = userTeamSlugs([]string{"quiet-team-x", "quiet-team-y", "nav-it-github-users"})
	if want := []string{"quiet-team-x", "quiet-team-y"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := userTeamSlugs(nil); got == nil || len(got) != 0 {
		t.Fatalf("empty input must give empty non-nil slice, got %#v", got)
	}
}
