package main

import (
	"reflect"
	"testing"
)

func TestLimitChanges(t *testing.T) {
	got := limitChanges([]LimitChange{
		{"2026-07-03", 400}, {"2026-07-04", 400}, {"2026-09-21", 400}, {"2026-09-22", 800}, {"2026-09-23", 0},
	})
	want := []LimitChange{{"2026-06-01", 200}, {"2026-07-01", 400}, {"2026-09-22", 800}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := limitChanges(nil); len(got) != len(spendLimits) {
		t.Fatalf("without snapshots: got %v", got)
	}
}
