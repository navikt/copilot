package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestSegmentsBigQuery runs the segment and cohort queries read-only against
// dev and checks invariants the mock-based handler tests cannot: every shown
// month sums to 100.
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
	for name, c := range map[string]SegmentChart{"intensity": s.Intensity, "mode": s.Mode, "movement": s.Movement, "team adoption": s.TeamAdoption} {
		if len(c.Months) == 0 {
			t.Fatalf("%s: empty series", name)
		}
		for m, month := range c.Months {
			var sum int64
			hidden := 0
			for _, b := range c.Bands {
				if b.Shares[m] == nil {
					hidden++
				} else {
					sum += *b.Shares[m]
				}
			}
			if hidden < len(c.Bands) && sum != 100 {
				t.Errorf("%s %s: shares sum to %d", name, month, sum)
			}
		}
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

func shares(c SegmentChart) map[string][]any {
	out := map[string][]any{}
	for _, b := range c.Bands {
		for _, p := range b.Shares {
			if p == nil {
				out[b.Label] = append(out[b.Label], nil)
			} else {
				out[b.Label] = append(out[b.Label], *p)
			}
		}
	}
	return out
}

func TestBuildChartMergesAndHides(t *testing.T) {
	c := buildChart([]segmentRow{
		{Month: "2026-07", A: 50, B: 30, C: 20}, // nothing to merge
		{Month: "2026-08", A: 3, B: 40, C: 57},  // Lav into Middels
		{Month: "2026-09", A: 2, B: 2, C: 1},    // never reaches five: hidden
		{Month: "2026-10", A: 90, B: 1, C: 9},   // Middels into the smaller neighbour, Høy
	}, adoptionBands)
	want := map[string][]any{
		"Lav":        {int64(50), nil, nil, int64(90)},
		"Middels":    {int64(30), nil, nil, nil},
		"Høy":        {int64(20), int64(57), nil, nil},
		"Under 60 %": {nil, int64(43), nil, nil},
		"Minst 25 %": {nil, nil, nil, int64(10)},
	}
	if got := shares(c); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if len(c.Months) != 4 || c.Bands[0].Label != "Lav" {
		t.Errorf("months %v, bands %+v", c.Months, c.Bands)
	}
}

func TestWholePercentsLargestRemainder(t *testing.T) {
	for _, tc := range []struct{ in, want []int64 }{
		{[]int64{1, 1, 1}, []int64{34, 33, 33}},
		{[]int64{10, 10, 10, 70}, []int64{10, 10, 10, 70}},
		{[]int64{333, 333, 334}, []int64{33, 33, 34}},
		{[]int64{7, 6, 6}, []int64{37, 32, 31}},
	} {
		got := wholePercents(tc.in)
		var sum int64
		for _, p := range got {
			sum += p
		}
		if fmt.Sprint(got) != fmt.Sprint(tc.want) || sum != 100 {
			t.Errorf("wholePercents(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestMovementNet(t *testing.T) {
	c := withNet(buildChart([]segmentRow{{Month: "2026-08", A: 30, B: 50, C: 20}, {Month: "2026-09", A: 3, B: 50, C: 20}}, movementBands))
	if c.Net[0] == nil || *c.Net[0] != 10 || c.Net[1] != nil {
		t.Errorf("net %v", c.Net)
	}
}

// Every user-level query reads user_metrics through userDaysFrom, so a day
// stored under the organization scope counts and a user active in both
// scopes on one day counts once.
func TestUserDaysDeduplicatesScopes(t *testing.T) {
	q := userDaysFrom("t")
	for _, want := range []string{"scope IN ('enterprise', 'organization')", "PARTITION BY JSON_VALUE(raw_record, '$.user_id'), day", "IF(scope = 'enterprise', 0, 1)) = 1"} {
		if !strings.Contains(q, want) {
			t.Errorf("userDaysFrom lacks %q", want)
		}
	}
	src := ""
	for _, f := range []string{"segments.go", "monthly_trends.go"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src += string(b)
	}
	if strings.Count(src, "userDaysFrom(") < 4 {
		t.Error("a user query bypasses userDaysFrom")
	}
}
