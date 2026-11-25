package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/config"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/fetchers"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/storage"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

type sourceRepairStoreTest struct {
	days   map[string]bool
	writes int
}

func (s *sourceRepairStoreTest) BillingSourceDays(context.Context, time.Time, string) (map[string]bool, error) {
	return s.days, nil
}

func (s *sourceRepairStoreTest) ReplaceUserMetrics(context.Context, time.Time, *types.FetchResult) error {
	s.writes++
	return nil
}

func (s *sourceRepairStoreTest) ReplaceUserTeams(context.Context, time.Time, *types.FetchResult) error {
	s.writes++
	return nil
}

func TestRepairBillingSourcesStopsOnTerminalStatus(t *testing.T) {
	for _, status := range []int{401, 403, 429} {
		for _, source := range []string{"users", "teams", "download", "org", "org teams", "partial teams"} {
			t.Run(fmt.Sprintf("%d/%s", status, source), func(t *testing.T) {
				calls := 0
				store := &sourceRepairStoreTest{days: map[string]bool{}}
				if source == "teams" || source == "org teams" || source == "partial teams" {
					store.days["2026-10-01:user_metrics"] = true
				}
				client := &fetchers.GitHubClient{Enterprise: "nav", Org: "navikt"}
				client.HTTPClient = mockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if (source == "org" || source == "org teams" || source == "partial teams") && strings.HasPrefix(r.URL.Path, "/enterprises/") {
						if source != "partial teams" {
							w.WriteHeader(http.StatusNoContent)
						} else {
							_, _ = w.Write([]byte(`{"report_day":"2026-10-01","download_links":["https://download.test/first"]}`))
						}
						return
					}
					if source == "download" {
						_, _ = w.Write([]byte(`{"report_day":"2026-10-01","download_links":["https://download.test/first","https://download.test/second"]}`))
						return
					}
					w.WriteHeader(status)
					_, _ = w.Write([]byte("No report available; status 204; CANCEL"))
				}))
				client.OrgHTTPClient = client.HTTPClient
				client.DownloadClient = mockClient(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls++
					if source == "partial teams" {
						_, _ = w.Write([]byte(`{"user_id":1,"team_id":2}`))
						return
					}
					w.WriteHeader(status)
				}))
				cfg := &config.Config{EnterpriseSlug: "nav", BigQueryUserMetricsTable: "user_metrics", BigQueryUserTeamsTable: "user_teams"}
				err := repairBillingSources(context.Background(), client, store, cfg, types.BillingSyncStart)
				var got *fetchers.GithubHTTPError
				wantCalls := 1
				switch source {
				case "download", "org", "org teams":
					wantCalls = 2
				case "partial teams":
					wantCalls = 3
				}
				if !errors.As(err, &got) || got.Status != status || !stopBillingSync(err) || calls != wantCalls || store.writes != 0 {
					t.Fatalf("err=%v calls=%d writes=%d", err, calls, store.writes)
				}
			})
		}
	}
}

func TestRepairBillingSourcesPreservesEarlierFailure(t *testing.T) {
	for _, earlier := range []int{http.StatusNoContent, http.StatusNotFound} {
		calls := 0
		client := &fetchers.GitHubClient{Enterprise: "nav", Org: "navikt"}
		client.HTTPClient = mockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if strings.Contains(r.URL.Path, "/users-1-day") {
				w.WriteHeader(earlier)
				return
			}
			w.WriteHeader(http.StatusForbidden)
		}))
		err := repairBillingSources(context.Background(), client, &sourceRepairStoreTest{}, &config.Config{EnterpriseSlug: "nav", BigQueryUserMetricsTable: "user_metrics", BigQueryUserTeamsTable: "user_teams"}, types.BillingSyncStart)
		if !stopBillingSync(err) || (earlier == http.StatusNoContent && !errors.Is(err, fetchers.ErrReportNotAvailable)) || !strings.Contains(err.Error(), "user_metrics 2026-10-01") || !strings.Contains(err.Error(), "status 403") || calls != 3 {
			t.Fatalf("earlier failure lost or repair continued: err=%v calls=%d", err, calls)
		}
	}
}

type syncFetcherTest struct {
	billingUserFetcherTest
	identity      map[string]string
	identityErr   error
	enterpriseErr error
}

type checkpointFetcherTest struct{ syncFetcherTest }

func (f *checkpointFetcherTest) FetchUserID(_ context.Context, login string) (string, error) {
	if login == "two" {
		return "", fetchers.ErrBillingBudget
	}
	return "reused", nil
}

func TestBillingCheckpointPreservesEarlierFailure(t *testing.T) {
	store := &billingUserStoreTest{users: map[string]string{"1": "one", "2": "two"}, done: map[string]bool{}, rows: map[string]types.UserBillingRow{}}
	err := syncBillingMonth(context.Background(), &checkpointFetcherTest{}, store, &config.Config{EnterpriseSlug: "nav"}, types.BillingSyncStart, 0)
	if err == fetchers.ErrBillingBudget || !errors.Is(err, fetchers.ErrBillingBudget) || !strings.Contains(err.Error(), "identity") || store.complete {
		t.Fatalf("checkpoint hid failure: %v", err)
	}
}

func TestClosedBillingMonths(t *testing.T) {
	if got := closedBillingMonths(types.BillingSyncStart); len(got) != 0 {
		t.Fatal("current month selected")
	}
	if got := closedBillingMonths(time.Date(2026, 11, billingSettleDay-1, 23, 0, 0, 0, time.UTC)); len(got) != 0 {
		t.Fatalf("month closed before settling: %v", got)
	}
	if got := closedBillingMonths(time.Date(2026, 11, billingSettleDay, 0, 0, 0, 0, time.UTC)); len(got) != 1 {
		t.Fatalf("month not closed after settling: %v", got)
	}
	got := closedBillingMonths(time.Date(2027, 1, billingSettleDay, 0, 0, 0, 0, time.UTC))
	if len(got) != 3 || got[0].Format("2006-01") != "2026-12" || got[2].Format("2006-01") != "2026-10" {
		t.Fatalf("backlog not discovered: %v", got)
	}
}

func (s *billingUserStoreTest) BillingSourceDays(_ context.Context, month time.Time, _ string) (map[string]bool, error) {
	days := map[string]bool{}
	for day := month; day.Before(month.AddDate(0, 1, 0)); day = day.AddDate(0, 0, 1) {
		for _, report := range []string{"user_metrics", "user_teams", ""} {
			days[day.Format("2006-01-02")+":"+report] = true
		}
	}
	return days, nil
}

func (f *syncFetcherTest) FetchUserID(_ context.Context, login string) (string, error) {
	return f.identity[login], f.identityErr
}

func (f *syncFetcherTest) FetchEnterpriseAICreditUsage(_ context.Context, month time.Time) (*types.BillingUsageResponse, error) {
	response := &types.BillingUsageResponse{Enterprise: "nav", UsageItems: []types.BillingUsageItem{{Product: "Copilot", SKU: "Copilot AI Credits", NetAmount: 8}}}
	response.TimePeriod.Year, response.TimePeriod.Month = month.Year(), int(month.Month())
	return response, f.enterpriseErr
}

func TestBillingSyncIdentityAndResume(t *testing.T) {
	month := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	store := &billingUserStoreTest{users: map[string]string{"1": "one", "2": "two"}, done: map[string]bool{}, rows: map[string]types.UserBillingRow{}}
	fetcher := &syncFetcherTest{identity: map[string]string{"one": "reused", "two": "2"}, billingUserFetcherTest: billingUserFetcherTest{items: []types.BillingUsageItem{{Product: "Copilot", SKU: "Copilot AI Credits", NetAmount: 3}}}}
	if err := syncBillingMonth(context.Background(), fetcher, store, &config.Config{EnterpriseSlug: "nav"}, month, 0); err == nil {
		t.Fatal("identity mismatch completed month")
	}
	if store.complete || store.done["1"] || !store.done["2"] || fetcher.calls != 1 {
		t.Fatalf("unsafe checkpoints: %+v", store)
	}
	if _, ok := store.rows["1:identity_unresolved"]; !ok {
		t.Fatal("unresolved identity was not recorded")
	}
	fetcher.identity["one"] = "1"
	if err := syncBillingMonth(context.Background(), fetcher, store, &config.Config{EnterpriseSlug: "nav"}, month, 0); err != nil {
		t.Fatal(err)
	}
	if !store.complete || fetcher.calls != 2 || store.rows[":Copilot AI Credits"].NetAmount != 8 || len(store.runs) != 1 || store.runs[0][1] != 8 {
		t.Fatal("resume did not publish enterprise snapshot")
	}
	if _, ok := store.rows["1:identity_unresolved"]; ok {
		t.Fatal("identity failure survived successful replacement")
	}
}

func TestBillingSyncDoesNotPublishFailedReconciliation(t *testing.T) {
	store := &billingUserStoreTest{users: map[string]string{"1": "one"}, done: map[string]bool{}, rows: map[string]types.UserBillingRow{}}
	fetcher := &syncFetcherTest{identity: map[string]string{"one": "1"}, enterpriseErr: errors.New("failed"), billingUserFetcherTest: billingUserFetcherTest{items: []types.BillingUsageItem{}}}
	if err := syncBillingMonth(context.Background(), fetcher, store, &config.Config{EnterpriseSlug: "nav"}, types.BillingSyncStart, 0); err == nil || store.complete || !store.done["1"] {
		t.Fatal("failed reconciliation published or lost user checkpoint")
	}
	// A budget hit at the enterprise fetch is a clean checkpoint, not an alert.
	fetcher.enterpriseErr = fetchers.ErrBillingBudget
	if err := syncBillingMonth(context.Background(), fetcher, store, &config.Config{EnterpriseSlug: "nav"}, types.BillingSyncStart, 0); err != fetchers.ErrBillingBudget || store.complete {
		t.Fatalf("budget hit at publish was not a bare checkpoint: %v", err)
	}
}

func TestBillingSyncCancelledDoesNotWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := &billingUserStoreTest{users: map[string]string{"1": "one"}, done: map[string]bool{}, rows: map[string]types.UserBillingRow{}}
	fetcher := &syncFetcherTest{identity: map[string]string{"one": "1"}}
	if err := syncBillingMonth(ctx, fetcher, store, &config.Config{EnterpriseSlug: "nav"}, types.BillingSyncStart, 0); !errors.Is(err, context.Canceled) || len(store.rows) != 0 {
		t.Fatal("cancelled run wrote data")
	}
}

func TestBillingRequestBudgetAndRateLimit(t *testing.T) {
	client := fetchers.NewBillingClient(&http.Client{}, "test", "nav")
	client.Budgeted, client.Remaining = true, 501
	calls := 0
	client.HTTPClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"3600"}, "X-Ratelimit-Remaining": []string{"500"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	_, err := client.FetchUserID(context.Background(), "one")
	var status *fetchers.BillingHTTPError
	if !errors.As(err, &status) || !status.RetryAt.After(time.Now()) || calls != 1 {
		t.Fatalf("rate limit not preserved: %v", err)
	}
	if _, err := client.FetchUserID(context.Background(), "one"); !errors.Is(err, fetchers.ErrBillingBudget) || calls != 1 {
		t.Fatal("request budget did not stop further calls")
	}
}

func TestAllocationReportValidation(t *testing.T) {
	day := types.BillingSyncStart
	valid := json.RawMessage(`{"day":"2026-10-01","user_id":"1","user_login":"one","team_id":"2","slug":"team"}`)
	if err := storage.ValidateAllocationReport(day, &types.FetchResult{Records: []json.RawMessage{json.RawMessage(`{"day":"2026-10-01","user_id":1,"user_login":"one","team_id":2,"slug":"team"}`)}}, true); err != nil {
		t.Fatal(err)
	}
	for _, records := range [][]json.RawMessage{{valid}, {}, {valid, valid}, {json.RawMessage(`{"day":"2026-09-01","user_id":"1","user_login":"one"}`)}, {json.RawMessage(`{"day":"2026-10-01","user_id":"1","user_login":"one"}`)}} {
		err := storage.ValidateAllocationReport(day, &types.FetchResult{Records: records}, true)
		wantErr := len(records) > 1 || (len(records) == 1 && string(records[0]) != string(valid))
		if (err != nil) != wantErr {
			t.Fatalf("records %s: %v", records, err)
		}
	}
	for _, raw := range []string{
		`{"day":"2026-10-01","user_id":1,"user_login":"one"}`,
		`{"day":"2026-10-01","user_id":1,"user_login":"one","ai_credits_used":"bad"}`,
		`{"day":"2026-10-01","user_id":1,"user_login":"one","ai_credits_used":-1}`,
	} {
		if err := storage.ValidateAllocationReport(day, &types.FetchResult{Records: []json.RawMessage{json.RawMessage(raw)}}, false); err == nil {
			t.Fatalf("invalid weight accepted: %s", raw)
		}
	}
	if err := storage.ValidateAllocationReport(day, &types.FetchResult{Records: []json.RawMessage{json.RawMessage(`{"day":"2026-10-01","user_id":1,"user_login":"one","ai_credits_used":0}`)}}, false); err != nil {
		t.Fatal(err)
	}
}

func TestManualBillingCannotPublishAutomaticMonths(t *testing.T) {
	store := &billingUserStoreTest{}
	if err := ingestUserBillingMonth(context.Background(), &billingUserFetcherTest{}, store, &config.Config{}, types.BillingSyncStart); err == nil || store.complete {
		t.Fatal("manual billing bypassed validated worker")
	}
}

func TestTransportErrorDoesNotExposeURL(t *testing.T) {
	err := fetchers.SafeTransportError(&url.Error{Op: "Get", URL: "https://example.test/secret?token=secret", Err: errors.New("connection failed")})
	if strings.Contains(err.Error(), "secret") || err.Error() != "connection failed" {
		t.Fatal(err)
	}
}

func TestDailyReportRejectsWrongDayAndPartialMembership(t *testing.T) {
	client := &fetchers.GitHubClient{Enterprise: "nav", Org: "navikt"}
	client.HTTPClient = mockClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/orgs/") {
			w.WriteHeader(403)
			return
		}
		_, _ = w.Write([]byte(`{"report_day":"2026-10-01","download_links":[]}`))
	}))
	client.DownloadClient = &http.Client{}
	result, err := client.FetchDailyUserTeams(context.Background(), types.BillingSyncStart)
	if err == nil || result != nil {
		t.Fatalf("partial membership certified: %+v %v", result, err)
	}
	if _, err := client.FetchDailyUserMetrics(context.Background(), types.BillingSyncStart.AddDate(0, 0, 1)); err == nil {
		t.Fatal("wrong report day accepted")
	}
}

type checkpointStoreTest struct {
	billingUserStoreTest
	failSKU string
}

func (s *checkpointStoreTest) ReplaceUserBilling(ctx context.Context, rows []types.UserBillingRow) error {
	if rows[0].SKU == s.failSKU {
		return context.DeadlineExceeded
	}
	return s.billingUserStoreTest.ReplaceUserBilling(ctx, rows)
}

func TestBillingWriteCheckpointRetainsIdentityFailure(t *testing.T) {
	for _, sku := range []string{"identity_unresolved", "done"} {
		store := &checkpointStoreTest{billingUserStoreTest: billingUserStoreTest{users: map[string]string{"1": "one", "2": "two"}, done: map[string]bool{}, rows: map[string]types.UserBillingRow{}}, failSKU: sku}
		fetcher := &syncFetcherTest{identity: map[string]string{"one": "reused", "two": "2"}, billingUserFetcherTest: billingUserFetcherTest{items: []types.BillingUsageItem{}}}
		err := syncBillingMonth(context.Background(), fetcher, store, &config.Config{EnterpriseSlug: "nav"}, types.BillingSyncStart, 0)
		if err == context.DeadlineExceeded || !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "identity") || store.complete {
			t.Fatalf("%s write hid failure: %v", sku, err)
		}
	}
}

func TestAllocationReplacementTransaction(t *testing.T) {
	if os.Getenv("VERIFY_TEAM_SPEND_BIGQUERY") != "true" {
		t.Skip("set VERIFY_TEAM_SPEND_BIGQUERY=true for temporary-table SQL verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client, err := bigquery.NewClient(ctx, "copilot-dev-e17a")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	for _, records := range [][]string{{`{"day":"2026-10-01","user_id":1,"user_login":"one"}`}, {}} {
		q := client.Query(`CREATE TEMP TABLE source(day DATE,scope STRING,scope_id STRING,raw_record JSON,loaded_at TIMESTAMP);
CREATE TEMP TABLE receipts(day DATE,report STRING,scope_id STRING,records INT64,loaded_at TIMESTAMP);
INSERT INTO source VALUES (@day,@scope,@scopeID,JSON '{"stale":true}',@loaded);
` + storage.AllocationReplacementSQL("source", "receipts") + `
BEGIN
BEGIN TRANSACTION;
DELETE FROM source WHERE day=@day;
DELETE FROM receipts WHERE day=@day;
ASSERT FALSE AS 'injected failure';
COMMIT TRANSACTION;
EXCEPTION WHEN ERROR THEN ROLLBACK TRANSACTION;
END;
SELECT (SELECT COUNT(*) FROM source) row_count, (SELECT COUNT(*) FROM receipts) receipts, (SELECT MAX(records) FROM receipts) recorded;`)
		q.Parameters = []bigquery.QueryParameter{{Name: "day", Value: civil.DateOf(types.BillingSyncStart)}, {Name: "scope", Value: "enterprise"}, {Name: "scopeID", Value: "nav"}, {Name: "loaded", Value: time.Now().UTC()}, {Name: "report", Value: "user_metrics"}, {Name: "records", Value: records}, {Name: "complete", Value: true}}
		it, err := q.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Rows     int64 `bigquery:"row_count"`
			Receipts int64
			Recorded int64
		}
		if err := it.Next(&got); err != nil {
			t.Fatal(err)
		}
		if got.Rows != int64(len(records)) || got.Receipts != 1 || got.Recorded != int64(len(records)) {
			t.Fatalf("replacement or rollback failed: %+v", got)
		}
	}
}

func TestLiveMonthlyBillingContract(t *testing.T) {
	if os.Getenv("VERIFY_TEAM_SPEND_BIGQUERY") != "true" || os.Getenv("GITHUB_BILLING_TOKEN") == "" {
		t.Skip("requires opt-in BigQuery verification and GITHUB_BILLING_TOKEN")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	bq, err := storage.NewBigQueryClient(ctx, &config.Config{BigQueryProjectID: "copilot-dev-e17a", BigQueryDataset: "copilot_metrics"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = bq.Close() }()
	q := bq.Client.Query("SELECT JSON_VALUE(raw_record,'$.user_id') id,JSON_VALUE(raw_record,'$.user_login') login FROM `copilot-dev-e17a.copilot_metrics.user_metrics` WHERE day='2026-09-30' AND scope='enterprise' AND scope_id='nav' ORDER BY FARM_FINGERPRINT(JSON_VALUE(raw_record,'$.user_id')) LIMIT 1")
	it, err := q.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var user struct {
		ID    string
		Login string
	}
	if err := it.Next(&user); err != nil {
		t.Fatal(err)
	}
	billing := fetchers.NewBillingClient(&http.Client{}, os.Getenv("GITHUB_BILLING_TOKEN"), "nav")
	billing.Budgeted, billing.Remaining, billing.RequestLimit = true, 5000, 3
	id, err := billing.FetchUserID(ctx, user.Login)
	if err != nil {
		t.Fatal(err)
	}
	if id != user.ID {
		t.Fatal("sample historical identity no longer resolves to its source ID")
	}
	month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if _, err := billing.FetchUserBillingUsage(ctx, user.Login, month); err != nil {
		t.Fatal(err)
	}
	t.Logf("After user billing: requests=%d reported_remaining=%d", billing.Requests, billing.Remaining)
	response, err := billing.FetchEnterpriseAICreditUsage(ctx, month)
	if err != nil {
		t.Fatal(err)
	}
	rows := billingRows(response, month, "nav", "", "")
	if len(rows) < 2 {
		t.Fatal("enterprise response contains no supported Copilot SKUs")
	}
	t.Logf("Validated identity, monthly user response and enterprise response in %d requests", billing.Requests)
}

func TestBillingSyncDeletedAccountAndFailedLookup(t *testing.T) {
	month := types.BillingSyncStart
	gone := func(lookupErr error) (*billingUserStoreTest, error) {
		store := &billingUserStoreTest{users: map[string]string{"1": "one"}, done: map[string]bool{}, rows: map[string]types.UserBillingRow{}}
		fetcher := &syncFetcherTest{identityErr: &fetchers.BillingHTTPError{Status: 404}, billingUserFetcherTest: billingUserFetcherTest{deleted: map[string]bool{"1": true}, lookupErr: lookupErr}}
		return store, syncBillingMonth(context.Background(), fetcher, store, &config.Config{EnterpriseSlug: "nav"}, month, 0)
	}
	if store, err := gone(nil); err != nil || !store.done["1"] || !store.complete {
		t.Fatalf("deleted account not skipped: %v %+v", err, store)
	}
	// Renamed: the identity lookup 404s on the old login, usage uses the new one.
	store := &billingUserStoreTest{users: map[string]string{"1": "old"}, done: map[string]bool{}, rows: map[string]types.UserBillingRow{}}
	fetcher := &syncFetcherTest{identityErr: &fetchers.BillingHTTPError{Status: 404}, billingUserFetcherTest: billingUserFetcherTest{
		items: []types.BillingUsageItem{{Product: "Copilot", SKU: "Copilot AI Credits", NetAmount: 3}}, notFound: map[string]bool{"old": true}, current: map[string]string{"1": "new"}}}
	if err := syncBillingMonth(context.Background(), fetcher, store, &config.Config{EnterpriseSlug: "nav"}, month, 0); err != nil || !store.complete || store.rows["1:Copilot AI Credits"].GitHubLogin != "new" {
		t.Fatalf("renamed account not refetched: %v %+v", err, store)
	}
	for _, lookupErr := range []error{fetchers.ErrBillingBudget, &fetchers.BillingHTTPError{Status: 403}} {
		store, err := gone(lookupErr)
		if err == nil || !errors.Is(err, lookupErr) || !stopBillingSync(err) || store.done["1"] || store.complete || len(store.rows) != 0 {
			t.Fatalf("lookup failure %v not propagated: %v %+v", lookupErr, err, store)
		}
	}
}
