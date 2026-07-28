package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// repoBackfillFetcher records which days were fetched and can return
// per-day results or errors.
type repoBackfillFetcher struct {
	fetched []string
	err     error
	records int
	scopeID string
}

func (f *repoBackfillFetcher) FetchDailyRepoMetrics(_ context.Context, day time.Time) (*FetchResult, error) {
	f.fetched = append(f.fetched, day.Format("2006-01-02"))
	if f.err != nil {
		return nil, f.err
	}
	records := make([]json.RawMessage, f.records)
	for i := range records {
		records[i] = json.RawMessage(`{"repo_name":"navikt/copilot"}`)
	}
	scopeID := f.scopeID
	if scopeID == "" {
		scopeID = "nav"
	}
	return &FetchResult{Records: records, Scope: "enterprise", ScopeID: scopeID}, nil
}

// repoBackfillStore tracks upserts per day and can report pre-existing days.
type repoBackfillStore struct {
	existing    map[string]bool
	inserted    map[string]int
	deleted     []string
	insertErr   error
	existsErr   error
	existsScope []string
	insertScope []string
}

func newRepoBackfillStore(existing ...string) *repoBackfillStore {
	s := &repoBackfillStore{existing: map[string]bool{}, inserted: map[string]int{}}
	for _, day := range existing {
		s.existing[day] = true
	}
	return s
}

func (s *repoBackfillStore) RepoMetricsDayExists(_ context.Context, day time.Time, scopeID string) (bool, error) {
	s.existsScope = append(s.existsScope, scopeID)
	if s.existsErr != nil {
		return false, s.existsErr
	}
	return s.existing[day.Format("2006-01-02")], nil
}

func (s *repoBackfillStore) DeleteRepoMetricsDay(_ context.Context, day time.Time, _ string) error {
	s.deleted = append(s.deleted, day.Format("2006-01-02"))
	return nil
}

func (s *repoBackfillStore) InsertRepoMetrics(_ context.Context, day time.Time, _, scopeID string, records []json.RawMessage) error {
	s.insertScope = append(s.insertScope, scopeID)
	if s.insertErr != nil {
		return s.insertErr
	}
	s.inserted[day.Format("2006-01-02")] = len(records)
	return nil
}

func withoutRepoMetricsDelay(t *testing.T) {
	t.Helper()
	original := repoMetricsRateLimitDelay
	repoMetricsRateLimitDelay = 0
	t.Cleanup(func() { repoMetricsRateLimitDelay = original })
}

func repoBackfillConfig() *Config {
	return &Config{EnterpriseSlug: "nav"}
}

// daysAgo returns the UTC day n days before today.
func daysAgo(n int) time.Time {
	return time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -n)
}

func TestRunRepoMetricsBackfill_FillsInteriorGap(t *testing.T) {
	withoutRepoMetricsDelay(t)

	// Days 3 and 2 already ingested by the nightly gap-fill; days 5 and 4
	// fell outside its 7-day window and must be filled.
	store := newRepoBackfillStore(daysAgo(3).Format("2006-01-02"), daysAgo(2).Format("2006-01-02"))
	fetcher := &repoBackfillFetcher{records: 7}

	if err := runRepoMetricsBackfill(context.Background(), fetcher, store, repoBackfillConfig(), daysAgo(5), false); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, day := range []int{5, 4, 1} {
		if store.inserted[daysAgo(day).Format("2006-01-02")] != 7 {
			t.Errorf("expected day -%d to be ingested with 7 records, got %d", day, store.inserted[daysAgo(day).Format("2006-01-02")])
		}
	}
	for _, day := range []int{3, 2} {
		if _, ok := store.inserted[daysAgo(day).Format("2006-01-02")]; ok {
			t.Errorf("expected existing day -%d to be skipped", day)
		}
	}
	if len(fetcher.fetched) != 3 {
		t.Errorf("expected 3 fetches (existing days skipped before fetch), got %d", len(fetcher.fetched))
	}
}

func TestRunRepoMetricsBackfill_ForceReingestsExistingDays(t *testing.T) {
	withoutRepoMetricsDelay(t)

	store := newRepoBackfillStore(daysAgo(2).Format("2006-01-02"), daysAgo(1).Format("2006-01-02"))
	fetcher := &repoBackfillFetcher{records: 3}

	if err := runRepoMetricsBackfill(context.Background(), fetcher, store, repoBackfillConfig(), daysAgo(2), true); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(fetcher.fetched) != 2 {
		t.Errorf("expected 2 fetches with force, got %d", len(fetcher.fetched))
	}
	if len(store.deleted) != 2 {
		t.Errorf("expected 2 deletes before re-insert, got %d", len(store.deleted))
	}
}

func TestRunRepoMetricsBackfill_ToleratesUnavailableReport(t *testing.T) {
	withoutRepoMetricsDelay(t)

	store := newRepoBackfillStore()
	fetcher := &repoBackfillFetcher{err: ErrReportNotAvailable}

	// Pre-GA days must not fail the run.
	if err := runRepoMetricsBackfill(context.Background(), fetcher, store, repoBackfillConfig(), daysAgo(3), false); err != nil {
		t.Fatalf("expected unavailable reports to be tolerated, got %v", err)
	}
	if len(store.inserted) != 0 {
		t.Errorf("expected no inserts, got %d", len(store.inserted))
	}
}

func TestRunRepoMetricsBackfill_EmptyResponseDoesNotDelete(t *testing.T) {
	withoutRepoMetricsDelay(t)

	store := newRepoBackfillStore(daysAgo(1).Format("2006-01-02"))
	fetcher := &repoBackfillFetcher{records: 0}

	if err := runRepoMetricsBackfill(context.Background(), fetcher, store, repoBackfillConfig(), daysAgo(1), true); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(store.deleted) != 0 {
		t.Errorf("expected empty response to leave existing rows intact, got %d deletes", len(store.deleted))
	}
}

func TestRunRepoMetricsBackfill_ExistsCheckErrorFallsThrough(t *testing.T) {
	withoutRepoMetricsDelay(t)

	// A failing existence check must not be treated as "already ingested" —
	// the day is fetched anyway and upsertReport re-checks authoritatively.
	store := newRepoBackfillStore()
	store.existsErr = errors.New("bigquery unavailable")
	fetcher := &repoBackfillFetcher{records: 4}

	err := runRepoMetricsBackfill(context.Background(), fetcher, store, repoBackfillConfig(), daysAgo(1), false)
	if err == nil {
		t.Fatal("expected the failing existence check inside upsertReport to surface as an error")
	}
	if len(fetcher.fetched) != 1 {
		t.Errorf("expected the day to be fetched despite the existence-check error, got %d fetches", len(fetcher.fetched))
	}
}

func TestRunRepoMetricsBackfill_PreCheckUsesEnterpriseSlugAndUpsertUsesFetchedScope(t *testing.T) {
	withoutRepoMetricsDelay(t)

	// The cheap pre-check queries the configured enterprise slug, while the
	// authoritative upsert uses whatever scope the fetch actually resolved to
	// (enterprise-first with org fallback).
	store := newRepoBackfillStore()
	fetcher := &repoBackfillFetcher{records: 2, scopeID: "navikt"}

	if err := runRepoMetricsBackfill(context.Background(), fetcher, store, repoBackfillConfig(), daysAgo(1), false); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(store.existsScope) == 0 || store.existsScope[0] != "nav" {
		t.Errorf("expected pre-check to use the enterprise slug %q, got %v", "nav", store.existsScope)
	}
	if len(store.insertScope) != 1 || store.insertScope[0] != "navikt" {
		t.Errorf("expected insert to use the fetched scope %q, got %v", "navikt", store.insertScope)
	}
}

func TestRunRepoMetricsBackfill_ReportsFailedDays(t *testing.T) {
	withoutRepoMetricsDelay(t)

	store := newRepoBackfillStore()
	fetcher := &repoBackfillFetcher{err: errors.New("upstream 500")}

	err := runRepoMetricsBackfill(context.Background(), fetcher, store, repoBackfillConfig(), daysAgo(2), false)
	if err == nil {
		t.Fatal("expected error for failed days, got nil")
	}
	if !contains(err.Error(), "2 failed day(s)") {
		t.Errorf("expected failed-day count in error, got: %v", err)
	}
}

func TestRunRepoMetricsBackfill_StreamingBufferIsNotFatal(t *testing.T) {
	withoutRepoMetricsDelay(t)

	store := newRepoBackfillStore()
	store.insertErr = ErrStreamingBuffer
	fetcher := &repoBackfillFetcher{records: 2}

	if err := runRepoMetricsBackfill(context.Background(), fetcher, store, repoBackfillConfig(), daysAgo(1), false); err != nil {
		t.Fatalf("expected streaming buffer to be non-fatal, got %v", err)
	}
}

func TestRunRepoMetricsBackfill_NothingToDo(t *testing.T) {
	withoutRepoMetricsDelay(t)

	store := newRepoBackfillStore()
	fetcher := &repoBackfillFetcher{records: 1}

	// Start date in the future relative to yesterday.
	if err := runRepoMetricsBackfill(context.Background(), fetcher, store, repoBackfillConfig(), daysAgo(0), false); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(fetcher.fetched) != 0 {
		t.Errorf("expected no fetches, got %d", len(fetcher.fetched))
	}
}

func TestRunRepoMetricsBackfill_CancelledContext(t *testing.T) {
	withoutRepoMetricsDelay(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	store := newRepoBackfillStore()
	fetcher := &repoBackfillFetcher{records: 1}

	if err := runRepoMetricsBackfill(ctx, fetcher, store, repoBackfillConfig(), daysAgo(3), false); err == nil {
		t.Fatal("expected context error, got nil")
	}
}

func TestRepoMetricsGADateIsParseable(t *testing.T) {
	if _, err := time.Parse("2006-01-02", repoMetricsGADate); err != nil {
		t.Fatalf("repoMetricsGADate must be a valid date: %v", err)
	}
}
