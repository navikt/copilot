package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type billingUserFetcherTest struct {
	items []BillingUsageItem
	calls int
	err   error
}

func (f *billingUserFetcherTest) FetchUserAICreditUsage(context.Context, string, time.Time) (*BillingUsageResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &BillingUsageResponse{UsageItems: f.items}, nil
}

type billingUserStoreTest struct {
	done     map[string]bool
	users    map[string]string
	rows     map[string]UserBillingRow
	complete bool
}

func (s *billingUserStoreTest) UserBillingRunComplete(context.Context, time.Time, string) (bool, error) {
	return s.complete, nil
}
func (s *billingUserStoreTest) GetBillingUsers(context.Context, time.Time, string) (map[string]string, error) {
	return s.users, nil
}
func (s *billingUserStoreTest) UserBillingDone(context.Context, time.Time, string) (map[string]bool, error) {
	return s.done, nil
}
func (s *billingUserStoreTest) UpsertUserBilling(_ context.Context, row UserBillingRow) error {
	s.rows[row.UserID+":"+row.SKU] = row
	s.done[row.UserID] = row.SKU == "done"
	return nil
}
func (s *billingUserStoreTest) CompleteUserBilling(context.Context, time.Time, string, int) error {
	s.complete = true
	return nil
}

func TestUserBillingMonthAggregatesAndResumes(t *testing.T) {
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	store := &billingUserStoreTest{users: map[string]string{"1": "one"}, done: map[string]bool{}, rows: map[string]UserBillingRow{}}
	fetcher := &billingUserFetcherTest{items: []BillingUsageItem{
		{Product: "Copilot", SKU: "Copilot AI Credits", GrossAmount: 4, NetAmount: 3},
		{Product: "Copilot", SKU: "Copilot AI Credits", GrossAmount: 2, NetAmount: 1},
		{Product: "Code Quality", SKU: "Code Quality AI Credits", GrossAmount: 8},
	}}
	if err := ingestUserBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err != nil {
		t.Fatal(err)
	}
	if row := store.rows["1:Copilot AI Credits"]; row.GrossAmount != 6 || row.NetAmount != 4 {
		t.Fatalf("wrong aggregated billing: %+v", row)
	}
	if !store.complete || !store.done["1"] || len(store.rows) != 2 {
		t.Fatalf("incomplete billing run: %+v", store)
	}
	if err := ingestUserBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err != nil {
		t.Fatal(err)
	}
	if fetcher.calls != 1 {
		t.Fatalf("completed month fetched again: %d calls", fetcher.calls)
	}
}

func TestUserBillingMonthDoesNotCompleteOnFailure(t *testing.T) {
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	store := &billingUserStoreTest{users: map[string]string{"1": "one"}, done: map[string]bool{}, rows: map[string]UserBillingRow{}}
	fetcher := &billingUserFetcherTest{err: errors.New("rate limited")}
	if err := ingestUserBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err == nil || store.complete {
		t.Fatalf("failed billing fetch marked complete: %v", err)
	}
}
