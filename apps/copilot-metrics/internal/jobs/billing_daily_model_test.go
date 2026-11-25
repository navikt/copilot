package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/config"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

type mockBillingDailyModelFetcher struct {
	resp *types.BillingUsageResponse
	err  error
}

func (m *mockBillingDailyModelFetcher) FetchDailyUsage(_ context.Context, _ time.Time) (*types.BillingUsageResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

type mockBillingDailyModelStore struct {
	deleteCalls   int
	insertCalls   int
	insertedItems []types.BillingUsageItem
}

func (m *mockBillingDailyModelStore) DeleteBillingUsageDailyModelDay(_ context.Context, _ time.Time, _ string) error {
	m.deleteCalls++
	return nil
}

func (m *mockBillingDailyModelStore) InsertBillingUsageDailyModelDay(_ context.Context, _ time.Time, _ string, items []types.BillingUsageItem) error {
	m.insertCalls++
	m.insertedItems = items
	return nil
}

func (m *mockBillingDailyModelStore) GetLatestBillingUsageDailyModelDay(_ context.Context, _ string) (time.Time, error) {
	return time.Time{}, nil
}

func TestIngestBillingModelDay_SkipsOverwriteOnEmptyResponse(t *testing.T) {
	fetcher := &mockBillingDailyModelFetcher{
		resp: &types.BillingUsageResponse{UsageItems: []types.BillingUsageItem{}},
	}
	store := &mockBillingDailyModelStore{}
	cfg := &config.Config{EnterpriseSlug: "nav"}

	err := ingestBillingModelDay(context.Background(), fetcher, store, cfg, time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.deleteCalls != 0 {
		t.Fatalf("expected no delete calls, got %d", store.deleteCalls)
	}
	if store.insertCalls != 0 {
		t.Fatalf("expected no insert calls, got %d", store.insertCalls)
	}
}

func TestIngestBillingModelDay_OverwritesWhenUsableRowsExist(t *testing.T) {
	fetcher := &mockBillingDailyModelFetcher{
		resp: &types.BillingUsageResponse{
			UsageItems: []types.BillingUsageItem{
				{Model: "gpt", GrossQuantity: 10},
				{Model: "zero", GrossQuantity: 0},
			},
		},
	}
	store := &mockBillingDailyModelStore{}
	cfg := &config.Config{EnterpriseSlug: "nav"}

	err := ingestBillingModelDay(context.Background(), fetcher, store, cfg, time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.deleteCalls != 1 {
		t.Fatalf("expected 1 delete call, got %d", store.deleteCalls)
	}
	if store.insertCalls != 1 {
		t.Fatalf("expected 1 insert call, got %d", store.insertCalls)
	}
	if len(store.insertedItems) != 1 {
		t.Fatalf("expected 1 inserted item, got %d", len(store.insertedItems))
	}
}
