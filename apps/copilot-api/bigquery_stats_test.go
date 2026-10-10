package main

import "testing"

func TestInteriorDeciles(t *testing.T) {
	q := []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 100}
	got := interiorDeciles(q, 10*minUsersForDistribution)
	if len(got) != 9 || got[0] != 1 || got[8] != 9 {
		t.Fatalf("interiorDeciles = %v, want [1..9] without min and max", got)
	}
	if got := interiorDeciles(q, 10*minUsersForDistribution-1); got != nil {
		t.Errorf("below n>=%d per decile: got %v, want nil", minUsersForDistribution, got)
	}
	if got := interiorDeciles([]int64{1, 2}, 1000); got != nil {
		t.Errorf("unexpected length: got %v, want nil", got)
	}
}
