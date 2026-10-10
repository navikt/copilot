package main

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestSegmentsBigQuery runs the segment and cohort queries read-only against
// dev and checks invariants the mock-based handler tests cannot: groups add up
// to their total whenever none is hidden, and nothing below five leaks out.
func TestSegmentsBigQuery(t *testing.T) {
	if os.Getenv("VERIFY_SEGMENTS_BIGQUERY") != "true" {
		t.Skip("set VERIFY_SEGMENTS_BIGQUERY=true for the read-only dev check")
	}
	client, err := newBigQueryClient(&Config{GCPProjectID: "copilot-dev-e17a", CopilotMetricsDataset: "copilot_metrics"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	s, err := client.GetUserSegments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Intensity) == 0 || len(s.Mode) == 0 || len(s.Movement) == 0 || len(s.TeamAdoption) == 0 {
		t.Fatalf("empty segment series: %+v", s)
	}
	check := func(name, month string, total *int64, parts ...*int64) {
		var sum int64
		hidden := false
		for _, p := range append(parts, total) {
			if p == nil {
				hidden = true
				continue
			}
			if *p < minUsersForDistribution {
				t.Errorf("%s %s: %d is below the suppression limit", name, month, *p)
			}
		}
		for _, p := range parts {
			if p != nil {
				sum += *p
			}
		}
		if !hidden && sum != *total {
			t.Errorf("%s %s: parts sum to %d, total %d", name, month, sum, *total)
		}
	}
	for _, r := range s.Intensity {
		check("intensity", r.Month, r.ActiveUsers, r.Light, r.Medium, r.Heavy)
	}
	for _, r := range s.Mode {
		check("mode", r.Month, r.ActiveUsers, r.Completions, r.Chat, r.Agent, r.CLI)
	}
	for _, r := range s.Movement {
		check("movement", r.Month, r.Pairs, r.Up, r.Stay, r.Down)
	}
	for _, r := range s.TeamAdoption {
		check("team adoption", r.Month, r.Teams, r.Low, r.Medium, r.High)
	}

	cohorts, err := client.GetCohortRetention(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cohorts {
		if c.CohortSize < minUsersForDistribution {
			t.Errorf("cohort %s has %d users", c.CohortMonth, c.CohortSize)
		}
		for _, v := range []*int64{c.M1, c.M3, c.M6} {
			if v != nil && (*v < 0 || *v > 100) {
				t.Errorf("cohort %s share %d out of range", c.CohortMonth, *v)
			}
		}
	}
}
