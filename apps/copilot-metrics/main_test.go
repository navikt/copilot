package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

// mockFetcher is a test double for MetricsFetcher.
type mockFetcher struct {
	result *FetchResult
	err    error
}

func (m *mockFetcher) FetchDailyMetrics(_ context.Context, _ time.Time) (*FetchResult, error) {
	return m.result, m.err
}

func (m *mockFetcher) FetchLatest28DayReport(_ context.Context) (*FetchResult, error) {
	return m.result, m.err
}

// mockStore is a test double for MetricsStore.
type mockStore struct {
	dayExists      bool
	dayExistsErr   error
	insertErr      error
	deleteErr      error
	latestDay      time.Time
	latestDayErr   error
	insertedDay    time.Time
	insertedScope  string
	insertedCount  int
	deletedDay     time.Time
	tableExistsErr error
}

func (m *mockStore) EnsureTableExists(_ context.Context) error {
	return m.tableExistsErr
}

func (m *mockStore) InsertMetrics(_ context.Context, day time.Time, scope, _ string, records []json.RawMessage) error {
	m.insertedDay = day
	m.insertedScope = scope
	m.insertedCount = len(records)
	return m.insertErr
}

func (m *mockStore) DayExists(_ context.Context, _ time.Time, _ string) (bool, error) {
	return m.dayExists, m.dayExistsErr
}

func (m *mockStore) DeleteDay(_ context.Context, day time.Time, _ string) error {
	m.deletedDay = day
	return m.deleteErr
}

func (m *mockStore) GetLatestDay(_ context.Context, _ string) (time.Time, error) {
	return m.latestDay, m.latestDayErr
}

func (m *mockStore) Close() error {
	return nil
}

func TestIngestDay_Success(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC)

	fetcher := &mockFetcher{
		result: &FetchResult{
			Records: []json.RawMessage{json.RawMessage(`{"test": "data"}`)},
			Scope:   "enterprise",
			ScopeID: "nav",
		},
	}

	store := &mockStore{
		dayExists: false,
	}

	err := ingestDay(ctx, fetcher, store, &Config{EnterpriseSlug: "nav"}, day)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !store.insertedDay.Equal(day) {
		t.Errorf("expected insertedDay %v, got %v", day, store.insertedDay)
	}
	if store.insertedScope != "enterprise" {
		t.Errorf("expected scope enterprise, got %s", store.insertedScope)
	}
	if store.insertedCount != 1 {
		t.Errorf("expected 1 record, got %d", store.insertedCount)
	}
}

func TestIngestDay_ExistingDataDeleted(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC)

	fetcher := &mockFetcher{
		result: &FetchResult{
			Records: []json.RawMessage{json.RawMessage(`{"test": "data"}`)},
			Scope:   "enterprise",
			ScopeID: "nav",
		},
	}

	store := &mockStore{
		dayExists: true, // Day already exists
	}

	err := ingestDay(ctx, fetcher, store, &Config{EnterpriseSlug: "nav"}, day)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify delete was called
	if store.deletedDay.IsZero() {
		t.Error("expected delete to be called, but it wasn't")
	}
}

func TestIngestDay_FetchError(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC)

	fetcher := &mockFetcher{
		err: errors.New("API error"),
	}

	store := &mockStore{}

	err := ingestDay(ctx, fetcher, store, &Config{EnterpriseSlug: "nav"}, day)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestIngestDay_NoRecords(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC)

	fetcher := &mockFetcher{
		result: &FetchResult{
			Records: []json.RawMessage{}, // Empty
			Scope:   "enterprise",
			ScopeID: "nav",
		},
	}

	store := &mockStore{}

	err := ingestDay(ctx, fetcher, store, &Config{EnterpriseSlug: "nav"}, day)
	if err != nil {
		t.Fatalf("expected no error for empty records, got %v", err)
	}

	// Verify insert was NOT called (zero day means not set)
	if !store.insertedDay.IsZero() {
		t.Error("expected insert NOT to be called for empty records")
	}
}

func TestIngestDay_OrgFallback(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC)

	// Simulate org fallback scenario
	fetcher := &mockFetcher{
		result: &FetchResult{
			Records: []json.RawMessage{json.RawMessage(`{"test": "data"}`)},
			Scope:   "organization", // Fell back to org
			ScopeID: "navikt",
		},
	}

	store := &mockStore{
		dayExists: false,
	}

	err := ingestDay(ctx, fetcher, store, &Config{EnterpriseSlug: "nav", OrganizationSlug: "navikt"}, day)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify org scope was used
	if store.insertedScope != "organization" {
		t.Errorf("expected scope organization, got %s", store.insertedScope)
	}
}

func TestConfigValidate_MissingRequired(t *testing.T) {
	cfg := &Config{
		GitHubAppID:             0,  // Missing
		GitHubAppPrivateKey:     "", // Missing
		GitHubAppInstallationID: 0,  // Missing
		BigQueryProjectID:       "", // Missing
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	configErr, ok := err.(*ConfigError)
	if !ok {
		t.Fatalf("expected ConfigError, got %T", err)
	}

	if len(configErr.MissingVars) != 4 {
		t.Errorf("expected 4 missing vars, got %d: %v", len(configErr.MissingVars), configErr.MissingVars)
	}
}

func TestConfigValidate_AllPresent(t *testing.T) {
	cfg := &Config{
		GitHubAppID:             12345,
		GitHubAppPrivateKey:     "test-key",
		GitHubAppInstallationID: 67890,
		BigQueryProjectID:       "my-project",
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestIngestDay_ReportNotAvailable(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC)

	fetcher := &mockFetcher{
		err: fmt.Errorf("%w for 2025-10-15: enterprise report not generated yet and org endpoint also failed: forbidden",
			ErrReportNotAvailable),
	}

	store := &mockStore{}

	err := ingestDay(ctx, fetcher, store, &Config{EnterpriseSlug: "nav"}, day)
	if err != nil {
		t.Fatalf("expected no error for report not available, got %v", err)
	}

	// Verify insert was NOT called
	if !store.insertedDay.IsZero() {
		t.Error("expected insert NOT to be called when report is not available")
	}
}

func TestIsClientError(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{errors.New("API returned status 404: not found"), true},
		{errors.New("API returned status 401: unauthorized"), true},
		{errors.New("API returned status 500: internal error"), false},
		{errors.New("connection timeout"), false},
	}

	for _, tt := range tests {
		result := isClientError(tt.err)
		if result != tt.expected {
			t.Errorf("isClientError(%q) = %v, want %v", tt.err, result, tt.expected)
		}
	}
}

func TestIsReportNotAvailable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "report not available message",
			err:      errors.New(`API returned status 404: {"message":"No report available for this enterprise on 2026-03-12"}`),
			expected: true,
		},
		{
			name:     "wrapped ErrReportNotAvailable",
			err:      fmt.Errorf("No report available: %w", ErrReportNotAvailable),
			expected: true,
		},
		{
			name:     "generic 404",
			err:      errors.New("API returned status 404: not found"),
			expected: false,
		},
		{
			name:     "403 forbidden",
			err:      errors.New("API returned status 403: Resource not accessible by integration"),
			expected: false,
		},
		{
			name:     "connection error",
			err:      errors.New("connection timeout"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReportNotAvailable(tt.err)
			if result != tt.expected {
				t.Errorf("isReportNotAvailable(%q) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIngestMissing_FillsGaps(t *testing.T) {
	ctx := context.Background()

	// Latest day in BigQuery is 3 days ago — should fill 2 missing days
	threeDaysAgo := time.Now().UTC().AddDate(0, 0, -3)

	fetcher := &countingFetcher{
		result: &FetchResult{
			Records: []json.RawMessage{json.RawMessage(`{"test":"data"}`)},
			Scope:   "enterprise",
			ScopeID: "nav",
		},
	}

	store := &mockStore{
		latestDay: threeDaysAgo,
	}

	cfg := &Config{EnterpriseSlug: "nav"}
	err := ingestMissing(ctx, fetcher, store, cfg, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should have fetched 2 days: day-2 and day-1 (yesterday)
	if fetcher.calls != 2 {
		t.Errorf("expected 2 fetch calls, got %d", fetcher.calls)
	}
}

func TestIngestMissing_AlreadyUpToDate(t *testing.T) {
	ctx := context.Background()

	// Latest day is yesterday — nothing to fill
	yesterday := time.Now().UTC().AddDate(0, 0, -1)

	fetcher := &countingFetcher{}
	store := &mockStore{latestDay: yesterday}

	cfg := &Config{EnterpriseSlug: "nav"}
	err := ingestMissing(ctx, fetcher, store, cfg, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fetcher.calls != 0 {
		t.Errorf("expected 0 fetch calls when up to date, got %d", fetcher.calls)
	}
}

func TestIngestMissing_NoExistingData(t *testing.T) {
	ctx := context.Background()

	// No data in BigQuery — should ingest yesterday only
	fetcher := &countingFetcher{
		result: &FetchResult{
			Records: []json.RawMessage{json.RawMessage(`{"test":"data"}`)},
			Scope:   "enterprise",
			ScopeID: "nav",
		},
	}

	store := &mockStore{} // latestDay is zero

	cfg := &Config{EnterpriseSlug: "nav"}
	err := ingestMissing(ctx, fetcher, store, cfg, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fetcher.calls != 1 {
		t.Errorf("expected 1 fetch call for yesterday, got %d", fetcher.calls)
	}
}

func TestIngestMissing_ContinuesOnPartialFailure(t *testing.T) {
	ctx := context.Background()

	// 3 days to fill, middle one will fail
	fourDaysAgo := time.Now().UTC().AddDate(0, 0, -4)

	callCount := 0
	fetcher := &callbackFetcher{
		fn: func() (*FetchResult, error) {
			callCount++
			if callCount == 2 {
				return nil, errors.New("transient error")
			}
			return &FetchResult{
				Records: []json.RawMessage{json.RawMessage(`{"test":"data"}`)},
				Scope:   "enterprise",
				ScopeID: "nav",
			}, nil
		},
	}

	store := &mockStore{latestDay: fourDaysAgo}

	cfg := &Config{EnterpriseSlug: "nav"}
	err := ingestMissing(ctx, fetcher, store, cfg, nil)
	if err != nil {
		t.Fatalf("expected no error (partial success), got %v", err)
	}

	// All 3 days attempted
	if callCount != 3 {
		t.Errorf("expected 3 fetch calls, got %d", callCount)
	}
}

func TestIngestMissing_AllDaysFail(t *testing.T) {
	ctx := context.Background()

	twoDaysAgo := time.Now().UTC().AddDate(0, 0, -2)

	fetcher := &countingFetcher{err: errors.New("API error")}
	store := &mockStore{latestDay: twoDaysAgo}

	cfg := &Config{EnterpriseSlug: "nav"}
	err := ingestMissing(ctx, fetcher, store, cfg, nil)
	if err == nil {
		t.Fatal("expected error when all days fail, got nil")
	}
}

// callbackFetcher calls a function for each fetch, allowing per-call behavior.
type callbackFetcher struct {
	fn func() (*FetchResult, error)
}

func (f *callbackFetcher) FetchDailyMetrics(_ context.Context, _ time.Time) (*FetchResult, error) {
	return f.fn()
}

func (f *callbackFetcher) FetchLatest28DayReport(_ context.Context) (*FetchResult, error) {
	return f.fn()
}
