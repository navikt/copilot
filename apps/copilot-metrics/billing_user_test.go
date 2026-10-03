package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

type billingUserFetcherTest struct {
	items []BillingUsageItem
	calls int
	err   error
}

func TestUserBillingResponseBoundary(t *testing.T) {
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for _, body := range []string{
		`null`, `{}`, `{"user":"one","enterprise":"nav","timePeriod":{"year":2026,"month":9}}`,
		`{"user":"other","enterprise":"nav","timePeriod":{"year":2026,"month":9},"usageItems":[]}`,
		`{"user":"one","enterprise":"nav","timePeriod":{"year":2026,"month":8},"usageItems":[]}`,
		`{"user":"one","enterprise":"other","timePeriod":{"year":2026,"month":9},"usageItems":[]}`,
	} {
		client := NewBillingClient("test", "nav")
		client.httpClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		store := &billingUserStoreTest{users: map[string]string{"1": "one"}, done: map[string]bool{}, rows: map[string]UserBillingRow{}}
		if err := ingestUserBillingMonth(context.Background(), client, store, &Config{EnterpriseSlug: "nav"}, month); err == nil {
			t.Fatalf("accepted invalid billing response %s", body)
		}
		if len(store.rows) != 0 || len(store.done) != 0 || store.complete {
			t.Fatal("invalid response changed billing state")
		}
	}
}

func TestUserBillingResumeReplacesAbsentSKU(t *testing.T) {
	store := &billingUserStoreTest{
		users: map[string]string{"1": "one"}, done: map[string]bool{},
		rows: map[string]UserBillingRow{"1:Copilot AI Credits": {UserID: "1", SKU: "Copilot AI Credits", NetAmount: 99}},
	}
	fetcher := &billingUserFetcherTest{items: []BillingUsageItem{}}
	if err := ingestUserBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(store.rows) != 1 || !store.done["1"] || !store.complete {
		t.Fatalf("stale SKU preserved: %+v", store)
	}
}

func TestUserBillingSnapshotTransaction(t *testing.T) {
	if os.Getenv("VERIFY_TEAM_SPEND_BIGQUERY") != "true" {
		t.Skip("set VERIFY_TEAM_SPEND_BIGQUERY=true for the temporary-table transaction check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client, err := bigquery.NewClient(ctx, "copilot-dev-e17a")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	rows := []UserBillingRow{{
		Month: civil.Date{Year: 2026, Month: 9, Day: 1}, ScopeID: "nav", UserID: "synthetic", GitHubLogin: "synthetic",
		SKU: "done", LoadedAt: time.Now().UTC(),
	}}
	query := client.Query(`CREATE TEMP TABLE snapshot AS SELECT * FROM UNNEST(@rows);
INSERT INTO snapshot SELECT month,scope_id,user_id,github_login,'Copilot AI Credits',99,99,loaded_at FROM UNNEST(@rows);
` + userBillingReplacementSQL("snapshot") + `
BEGIN
BEGIN TRANSACTION;
DELETE FROM snapshot WHERE user_id=@uid;
ASSERT FALSE AS 'injected write failure';
COMMIT TRANSACTION;
EXCEPTION WHEN ERROR THEN ROLLBACK TRANSACTION;
END;
SELECT COUNT(*) n, SUM(net_amount) net FROM snapshot;`)
	query.Parameters = []bigquery.QueryParameter{
		{Name: "rows", Value: rows}, {Name: "month", Value: rows[0].Month},
		{Name: "scope", Value: "nav"}, {Name: "uid", Value: "synthetic"},
	}
	it, err := query.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		N   int64   `bigquery:"n"`
		Net float64 `bigquery:"net"`
	}
	if err := it.Next(&got); err != nil {
		t.Fatal(err)
	}
	if got.N != 1 || got.Net != 0 {
		t.Fatalf("stale billing snapshot survived transaction: %+v", got)
	}
}

func (f *billingUserFetcherTest) FetchUserAICreditUsage(_ context.Context, login string, month time.Time) (*BillingUsageResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	result := &BillingUsageResponse{User: login, Enterprise: "nav", UsageItems: f.items}
	result.TimePeriod.Year, result.TimePeriod.Month = month.Year(), int(month.Month())
	return result, nil
}

type billingUserStoreTest struct {
	done       map[string]bool
	users      map[string]string
	rows       map[string]UserBillingRow
	complete   bool
	replaceErr error
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
func (s *billingUserStoreTest) ReplaceUserBilling(_ context.Context, rows []UserBillingRow) error {
	if s.replaceErr != nil {
		return s.replaceErr
	}
	for key, row := range s.rows {
		if row.UserID == rows[0].UserID {
			delete(s.rows, key)
		}
	}
	for _, row := range rows {
		s.rows[row.UserID+":"+row.SKU] = row
		if row.SKU == "done" {
			s.done[row.UserID] = true
		}
	}
	return nil
}

func TestUserBillingPartialResumeAfterStoreFailure(t *testing.T) {
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	store := &billingUserStoreTest{
		users: map[string]string{"1": "one", "2": "two"}, done: map[string]bool{"1": true},
		rows: map[string]UserBillingRow{}, replaceErr: errors.New("write failed"),
	}
	fetcher := &billingUserFetcherTest{items: []BillingUsageItem{}}
	if err := ingestUserBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err == nil || store.complete || store.done["2"] {
		t.Fatalf("write failure marked user complete: %v", err)
	}
	store.replaceErr = nil
	if err := ingestUserBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err != nil {
		t.Fatal(err)
	}
	if !store.complete || !store.done["2"] || fetcher.calls != 2 {
		t.Fatal("retry did not resume only the unfinished user")
	}
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
