package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

// failingDayFetcher fails the entity fetch for one day, as long as fail is set.
type failingDayFetcher struct {
	supplementaryFetcher
	failDay string
	fail    bool
	fetched []string
}

func (f *failingDayFetcher) FetchDailyMetrics(ctx context.Context, day time.Time) (*FetchResult, error) {
	d := day.Format("2006-01-02")
	f.fetched = append(f.fetched, d)
	if f.fail && d == f.failDay {
		return nil, errors.New("boom")
	}
	return f.supplementaryFetcher.FetchDailyMetrics(ctx, day)
}

// A failed day must stop the loop: ingesting later days would move the
// high-water mark past it and the day would never be retried.
func TestIngestMissing_StopsAtFirstFailedDayAndRetriesIt(t *testing.T) {
	ctx := context.Background()
	cfg := supplementaryConfig()

	store := newScopedStore()
	store.seed(tableUsageMetrics, daysAgo(4), "nav", 1)

	failDay := daysAgo(2).Format("2006-01-02")
	fetcher := &failingDayFetcher{
		supplementaryFetcher: supplementaryFetcher{scope: "enterprise", scopeID: "nav", records: 1},
		failDay:              failDay,
		fail:                 true,
	}

	var alert slackMessage
	slack := &SlackNotifier{webhookURL: "http://test/webhook", client: mockClient(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&alert)
	}))}
	if err := ingestMissing(ctx, fetcher, store, cfg, slack); err != nil {
		t.Fatalf("partial success should not fail the run, got %v", err)
	}
	// Failed day plus the day it skipped: the alert covers the whole backlog.
	if !strings.Contains(alert.Text, "2 of 3") || !strings.Contains(alert.Text, daysAgo(1).Format("2006-01-02")) {
		t.Errorf("alert should report the skipped day too, got %q", alert.Text)
	}
	if got := store.count(tableUsageMetrics, daysAgo(3), "nav"); got != 1 {
		t.Errorf("day -3 should be ingested, got %d rows", got)
	}
	for _, day := range []int{2, 1} {
		if got := store.count(tableUsageMetrics, daysAgo(day), "nav"); got != 0 {
			t.Errorf("day -%d: expected no rows after the failed day, got %d", day, got)
		}
	}

	fetcher.fail, fetcher.fetched = false, nil
	if err := ingestMissing(ctx, fetcher, store, cfg, nil); err != nil {
		t.Fatalf("retry run: %v", err)
	}
	if len(fetcher.fetched) == 0 || fetcher.fetched[0] != failDay {
		t.Errorf("next run should start at the failed day %s, fetched %v", failDay, fetcher.fetched)
	}
	for _, day := range []int{2, 1} {
		if got := store.count(tableUsageMetrics, daysAgo(day), "nav"); got != 1 {
			t.Errorf("day -%d: expected 1 row after retry, got %d", day, got)
		}
	}
}
