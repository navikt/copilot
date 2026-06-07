package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockUsageReportFetcher struct {
	resp *OrganizationBillingUsageResponse
	err  error
}

func (m *mockUsageReportFetcher) FetchOrganizationUsage(_ context.Context, _ string, _ time.Time) (*OrganizationBillingUsageResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

type mockUsageReportStore struct {
	exists      bool
	existsErr   error
	deleteErr   error
	insertErr   error
	latestDay   time.Time
	latestErr   error
	deletedDays []time.Time
	inserted    int
}

func (m *mockUsageReportStore) BillingUsageReportDayExists(_ context.Context, _ time.Time, _ string) (bool, error) {
	return m.exists, m.existsErr
}

func (m *mockUsageReportStore) DeleteBillingUsageReportDay(_ context.Context, day time.Time, _ string) error {
	m.deletedDays = append(m.deletedDays, day)
	return m.deleteErr
}

func (m *mockUsageReportStore) InsertBillingUsageReportDay(_ context.Context, _ time.Time, _ string, items []OrganizationBillingUsageItem) error {
	m.inserted += len(items)
	return m.insertErr
}

func (m *mockUsageReportStore) GetLatestBillingUsageReportDay(_ context.Context, _ string) (time.Time, error) {
	return m.latestDay, m.latestErr
}

func TestIngestBillingUsageReportDay_SkipsWhenExists(t *testing.T) {
	store := &mockUsageReportStore{exists: true}
	fetcher := &mockUsageReportFetcher{
		resp: &OrganizationBillingUsageResponse{
			UsageItems: []OrganizationBillingUsageItem{{Product: "Copilot"}},
		},
	}
	cfg := &Config{OrganizationSlug: "navikt"}

	err := ingestBillingUsageReportDay(context.Background(), fetcher, store, cfg, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.inserted != 0 {
		t.Fatalf("expected no insert when already exists")
	}
}

func TestIngestBillingUsageReportDay_UpsertsRows(t *testing.T) {
	store := &mockUsageReportStore{}
	fetcher := &mockUsageReportFetcher{
		resp: &OrganizationBillingUsageResponse{
			UsageItems: []OrganizationBillingUsageItem{
				{Product: "Copilot", SKU: "Copilot Premium Request"},
				{Product: "Copilot", SKU: "Copilot AI Credits"},
			},
		},
	}
	cfg := &Config{OrganizationSlug: "navikt"}
	day := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)

	err := ingestBillingUsageReportDay(context.Background(), fetcher, store, cfg, day, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.deletedDays) != 1 {
		t.Fatalf("expected delete before insert")
	}
	if store.inserted != 2 {
		t.Fatalf("expected 2 inserted rows, got %d", store.inserted)
	}
}

func TestRunBillingUsageReportBackfill_AdjustsStartWithoutForce(t *testing.T) {
	prevDelay := billingUsageReportRateLimitDelay
	billingUsageReportRateLimitDelay = 0
	defer func() { billingUsageReportRateLimitDelay = prevDelay }()

	store := &mockUsageReportStore{
		latestDay: time.Now().UTC().AddDate(0, 0, -2).Truncate(24 * time.Hour),
	}
	fetcher := &mockUsageReportFetcher{
		resp: &OrganizationBillingUsageResponse{
			UsageItems: []OrganizationBillingUsageItem{{Product: "Copilot", SKU: "Copilot Premium Request"}},
		},
	}
	cfg := &Config{OrganizationSlug: "navikt"}

	err := runBillingUsageReportBackfill(
		context.Background(),
		fetcher,
		store,
		cfg,
		time.Now().UTC().AddDate(0, 0, -5).Truncate(24*time.Hour),
		false,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.inserted == 0 {
		t.Fatalf("expected at least one inserted row")
	}
	for _, d := range store.deletedDays {
		if d.Before(store.latestDay.AddDate(0, 0, 1)) {
			t.Fatalf("expected start adjusted to latest+1, got delete for %s", d.Format("2006-01-02"))
		}
	}
}

func TestRunBillingUsageReportBackfill_FailsWhenAllDaysFail(t *testing.T) {
	prevDelay := billingUsageReportRateLimitDelay
	billingUsageReportRateLimitDelay = 0
	defer func() { billingUsageReportRateLimitDelay = prevDelay }()

	store := &mockUsageReportStore{}
	fetcher := &mockUsageReportFetcher{err: errors.New("boom")}
	cfg := &Config{OrganizationSlug: "navikt"}
	start := time.Now().UTC().AddDate(0, 0, -2).Truncate(24 * time.Hour)

	err := runBillingUsageReportBackfill(context.Background(), fetcher, store, cfg, start, true)
	if err == nil {
		t.Fatalf("expected error when all days fail")
	}
}
