package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestRunBackfill_AbortsOnHighErrorRate(t *testing.T) {
	ctx := context.Background()

	fetcher := &countingFetcher{err: errors.New("API error")}
	store := &mockStore{}
	startDate := time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)

	err := runBackfill(ctx, fetcher, store, &Config{EnterpriseSlug: "nav"}, startDate)

	if err == nil {
		t.Fatal("expected backfill to abort, got nil")
	}
	if !contains(err.Error(), "error rate") {
		t.Errorf("expected error rate abort, got: %v", err)
	}
	if fetcher.calls > 15 {
		t.Errorf("expected abort after ~10 calls, got %d", fetcher.calls)
	}
}

func TestRunBackfill_AlreadyUpToDate(t *testing.T) {
	ctx := context.Background()

	store := &mockStore{
		latestDay: time.Now().UTC().AddDate(0, 0, -1), // yesterday = already caught up
	}

	fetcher := &countingFetcher{}

	err := runBackfill(ctx, fetcher, store, &Config{EnterpriseSlug: "nav"}, time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fetcher.calls != 0 {
		t.Errorf("expected 0 fetch calls when already up to date, got %d", fetcher.calls)
	}
}

func TestRunBackfill_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	store := &mockStore{}
	fetcher := &countingFetcher{}

	err := runBackfill(ctx, fetcher, store, &Config{EnterpriseSlug: "nav"}, time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}

// countingFetcher tracks calls and can return configurable results.
type countingFetcher struct {
	calls  int
	err    error
	result *FetchResult
}

func (f *countingFetcher) FetchDailyMetrics(_ context.Context, _ time.Time) (*FetchResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &FetchResult{
		Records: []json.RawMessage{json.RawMessage(`{"test":"data"}`)},
		Scope:   "enterprise",
		ScopeID: "nav",
	}, nil
}

func (f *countingFetcher) FetchLatest28DayReport(_ context.Context) (*FetchResult, error) {
	return f.result, f.err
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
