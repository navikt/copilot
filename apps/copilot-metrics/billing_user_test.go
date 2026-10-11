package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

type billingUserFetcherTest struct {
	items     []BillingUsageItem
	calls     int
	err       error
	notFound  map[string]bool   // logins that 404
	deleted   map[string]bool   // ids with no account
	current   map[string]string // id -> current login
	lookupErr error

	enterpriseCalls int
}

func (f *billingUserFetcherTest) AccountLogin(_ context.Context, id string) (string, error) {
	if f.deleted[id] {
		return "", f.lookupErr
	}
	return f.current[id], f.lookupErr
}

func TestUserBillingSkipsOnlyDeletedAccounts(t *testing.T) {
	month := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	items := []BillingUsageItem{{Product: "Copilot", SKU: "Copilot AI Credits", NetAmount: 2}}
	store := &billingUserStoreTest{users: map[string]string{"1": "gone", "2": "one"}, done: map[string]bool{}, rows: map[string]UserBillingRow{}}
	fetcher := &billingUserFetcherTest{items: items, notFound: map[string]bool{"gone": true}, deleted: map[string]bool{"1": true}}
	if err := ingestUserBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err != nil {
		t.Fatal(err)
	}
	if !store.complete || !store.done["1"] || len(store.rows) != 5 || store.rows["2:Copilot AI Credits"].NetAmount != 2 {
		t.Fatalf("deleted account not skipped with a marker only: %+v", store)
	}
	// Bot logins are not billed per user; GitHub answers with another name.
	store = &billingUserStoreTest{users: map[string]string{"1": "one", "2": "Copilot-SWE-Agent[Bot]"}, done: map[string]bool{}, rows: map[string]UserBillingRow{}}
	if err := ingestUserBillingMonth(context.Background(), &billingUserFetcherTest{items: items}, store, &Config{EnterpriseSlug: "nav"}, month); err != nil || store.done["2"] || !store.complete {
		t.Fatalf("bot account fetched: %v %+v", err, store)
	}
	// A 404 for an account that still has the same login: abort.
	store = &billingUserStoreTest{users: map[string]string{"1": "same"}, done: map[string]bool{}, rows: map[string]UserBillingRow{}}
	fetcher = &billingUserFetcherTest{items: items, notFound: map[string]bool{"same": true}, current: map[string]string{"1": "same"}}
	if err := ingestUserBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err == nil || store.complete || store.done["1"] {
		t.Fatalf("404 for an unchanged account skipped: %v %+v", err, store)
	}
	// A renamed account 404s on its old login: refetch with the current one.
	store = &billingUserStoreTest{users: map[string]string{"1": "old"}, done: map[string]bool{}, rows: map[string]UserBillingRow{}}
	fetcher = &billingUserFetcherTest{items: items, notFound: map[string]bool{"old": true}, current: map[string]string{"1": "New"}}
	if err := ingestUserBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err != nil || !store.complete {
		t.Fatalf("renamed account not refetched: %v %+v", err, store)
	}
	if row := store.rows["1:Copilot AI Credits"]; row.NetAmount != 2 || row.GitHubLogin != "new" {
		t.Fatalf("renamed account rows: %+v", store.rows)
	}
	// The refetch failing still aborts.
	store = &billingUserStoreTest{users: map[string]string{"1": "old"}, done: map[string]bool{}, rows: map[string]UserBillingRow{}}
	fetcher = &billingUserFetcherTest{items: items, notFound: map[string]bool{"old": true, "new": true}, current: map[string]string{"1": "new"}}
	if err := ingestUserBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err == nil || store.complete || store.done["1"] {
		t.Fatalf("failed refetch skipped: %v %+v", err, store)
	}
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
	if len(store.rows) != 3 || !store.done["1"] || !store.complete {
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

func TestUserBillingPremiumRequestMonths(t *testing.T) {
	for _, tc := range []struct {
		month time.Time
		path  string
	}{
		{time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "/enterprises/nav/settings/billing/premium_request/usage"},
		{time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), "/enterprises/nav/settings/billing/ai_credit/usage"},
	} {
		client := NewBillingClient("test", "nav")
		var requested []string
		client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requested = append(requested, req.URL.Path+"?user="+req.URL.Query().Get("user"))
			body := `{"user":"` + req.URL.Query().Get("user") + `","enterprise":"nav","timePeriod":{"year":2026,"month":` + tc.month.Format("1") + `},"usageItems":[
{"product":"Copilot","sku":"Copilot Premium Request","grossAmount":2,"netAmount":1},
{"product":"Copilot","sku":"Coding Agent Premium Request","grossAmount":4,"netAmount":3},
{"product":"Actions","sku":"Actions Linux","grossAmount":9,"netAmount":9}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		response, err := client.FetchUserBillingUsage(context.Background(), "one", tc.month)
		if err != nil {
			t.Fatal(err)
		}
		// The enterprise total must come from the same product as the user rows.
		if _, err := client.FetchEnterpriseAICreditUsage(context.Background(), tc.month); err != nil {
			t.Fatal(err)
		}
		if want := []string{tc.path + "?user=one", tc.path + "?user="}; !slices.Equal(requested, want) {
			t.Errorf("month %s requested %v, want %v", tc.month.Format("2006-01"), requested, want)
		}
		net := map[string]float64{}
		for _, row := range billingRows(response, tc.month, "nav", "1", "one") {
			net[row.SKU] = row.NetAmount
		}
		if len(net) != 3 || net["Copilot Premium Request"] != 1 || net["Coding Agent Premium Request"] != 3 {
			t.Fatalf("premium request rows: %v", net)
		}
	}
}

func (f *billingUserFetcherTest) FetchUserBillingUsage(_ context.Context, login string, month time.Time) (*BillingUsageResponse, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.notFound[login] {
		return nil, &billingHTTPError{Status: http.StatusNotFound}
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
	runs       [][3]float64 // users, enterprise net, user net
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
func (s *billingUserStoreTest) CompleteUserBilling(_ context.Context, _ time.Time, _ string, users int, enterpriseNet, userNet float64) error {
	s.complete = true
	s.runs = append(s.runs, [3]float64{float64(users), enterpriseNet, userNet})
	return nil
}

func (s *billingUserStoreTest) UserBillingNet(context.Context, time.Time, string) (float64, error) {
	net := 0.0
	for _, row := range s.rows {
		if row.UserID != "" {
			net += row.NetAmount
		}
	}
	return net, nil
}

func (f *billingUserFetcherTest) FetchEnterpriseAICreditUsage(_ context.Context, month time.Time) (*BillingUsageResponse, error) {
	f.enterpriseCalls++
	response := &BillingUsageResponse{Enterprise: "nav", UsageItems: []BillingUsageItem{{Product: "Copilot", SKU: "Copilot AI Credits", NetAmount: 8}}}
	response.TimePeriod.Year, response.TimePeriod.Month = month.Year(), int(month.Month())
	return response, nil
}

func TestManualBillingMonthPublishesEnterpriseRow(t *testing.T) {
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	store := &billingUserStoreTest{users: map[string]string{"1": "one"}, done: map[string]bool{}, rows: map[string]UserBillingRow{}}
	fetcher := &billingUserFetcherTest{items: []BillingUsageItem{{Product: "Copilot", SKU: "Copilot AI Credits", NetAmount: 3}}}
	if err := ingestUserBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err != nil {
		t.Fatal(err)
	}
	if store.rows[":Copilot AI Credits"].NetAmount != 8 || len(store.runs) != 1 || store.runs[0] != [3]float64{1, 8, 3} {
		t.Fatalf("manual month not published: rows=%v runs=%v", store.rows, store.runs)
	}
}

func TestRepublishBillingMonthIsIdempotent(t *testing.T) {
	month := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	store := &billingUserStoreTest{done: map[string]bool{}, rows: map[string]UserBillingRow{}}
	fetcher := &billingUserFetcherTest{}
	if err := republishBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err == nil || fetcher.enterpriseCalls != 0 {
		t.Fatal("published a month without a completion marker")
	}
	store.complete = true
	_ = store.ReplaceUserBilling(context.Background(), billingRows(&BillingUsageResponse{UsageItems: []BillingUsageItem{{Product: "Copilot", SKU: "Copilot AI Credits", NetAmount: 5}}}, month, "nav", "1", "one"))
	for range 2 {
		if err := republishBillingMonth(context.Background(), fetcher, store, &Config{EnterpriseSlug: "nav"}, month); err != nil {
			t.Fatal(err)
		}
	}
	// Second run replaces the enterprise row instead of adding to it, and the
	// enterprise "done" marker is not counted as a user.
	if len(store.rows) != 4 || store.rows[":Copilot AI Credits"].NetAmount != 8 || store.runs[1] != [3]float64{1, 8, 5} {
		t.Fatalf("republish not idempotent: rows=%v runs=%v", store.rows, store.runs)
	}
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
	if !store.complete || !store.done["1"] || len(store.rows) != 4 {
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

func TestUserBillingCompleteSQLIsTransactional(t *testing.T) {
	sql := userBillingCompleteSQL("runs")
	if !strings.HasPrefix(sql, "BEGIN TRANSACTION;") || !strings.HasSuffix(sql, "COMMIT TRANSACTION;") {
		t.Fatalf("publish must be atomic:\n%s", sql)
	}
	if strings.Index(sql, "'published'") > strings.Index(sql, "MERGE") {
		t.Fatalf("INSERT must precede MERGE:\n%s", sql)
	}
}
