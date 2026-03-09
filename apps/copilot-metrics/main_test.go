package main

import (
	"context"
	"encoding/json"
	"errors"
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
