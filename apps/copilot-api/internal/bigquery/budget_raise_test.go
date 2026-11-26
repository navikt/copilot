package bigquery

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"github.com/navikt/copilot/apps/copilot-api/internal/cache"
)

func TestRosterTeamsNoSnapshotIsUnknown(t *testing.T) {
	now := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	snap := func(daysAgo int) bigquery.NullDate {
		return bigquery.NullDate{Date: civil.DateOf(now).AddDays(-daysAgo), Valid: true}
	}
	if _, err := rosterTeams([]rosterRow{{}}, now); err == nil {
		t.Error("no snapshot must be an error, not an empty membership")
	}
	if _, err := rosterTeams([]rosterRow{{SnapshotDate: snap(rosterMaxAgeDays + 1)}}, now); err == nil {
		t.Error("a stale snapshot must count as unknown")
	}
	if got, err := rosterTeams([]rosterRow{{SnapshotDate: snap(rosterMaxAgeDays)}}, now); err != nil || len(got) != 0 {
		t.Errorf("recent snapshot without the user: %v %v", got, err)
	}
	if got, _ := rosterTeams([]rosterRow{{SnapshotDate: snap(0), TeamSlug: bigquery.NullString{StringVal: "team-b", Valid: true}}}, now); len(got) != 1 {
		t.Errorf("member: %v", got)
	}
}

func TestCachedRosterRechecksSnapshotAge(t *testing.T) {
	day := time.Date(2026, 11, 23, 23, 55, 0, 0, time.UTC)
	now := day
	c := &CachedBigQueryClient{cache: cache.New(time.Hour), now: func() time.Time { return now }}
	c.cache.Set("roster_rows_ola", []rosterRow{{
		SnapshotDate: bigquery.NullDate{Date: civil.DateOf(day).AddDays(-rosterMaxAgeDays), Valid: true},
		TeamSlug:     bigquery.NullString{StringVal: "team-b", Valid: true},
	}})
	if got, err := c.GetRosterTeams(context.Background(), "ola"); err != nil || len(got) != 1 {
		t.Fatalf("fresh cached roster: %v %v", got, err)
	}
	now = day.Add(10 * time.Minute) // past midnight: the cached snapshot is now too old
	if _, err := c.GetRosterTeams(context.Background(), "ola"); err == nil {
		t.Error("a cache hit must re-check the snapshot age")
	}
}
