package main

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Net is split over days in teamNetCTEs only, with no month special case.
func TestTeamNetCTEsHaveNoMonthCutoff(t *testing.T) {
	client := &BigQueryClient{projectID: "p", metricsDataset: "d"}
	q := client.teamNetCTEs()
	if strings.Contains(q, "2026-06-01") || !strings.Contains(q, "unspread AS") {
		t.Fatalf("net split must not special-case months:\n%s", q)
	}
}

// A rerun's duplicated load counts once (billingUserLatest).
// A user billed in June whose only usage was before ai_credits_used existed
// (2026-06-19) gets net spread evenly over those days, not «netto uten bruk».
// Runs the real CTEs on inline rows; set COPILOT_BQ_TEST_PROJECT to run it,
// and COPILOT_BQ_TEST_TOKEN (gcloud auth print-access-token) without ADC.
func TestTeamNetSpreadWithoutGross(t *testing.T) {
	project := os.Getenv("COPILOT_BQ_TEST_PROJECT")
	if project == "" {
		t.Skip("COPILOT_BQ_TEST_PROJECT not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var opts []option.ClientOption
	if tok := os.Getenv("COPILOT_BQ_TEST_TOKEN"); tok != "" {
		opts = append(opts, option.WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})))
	}
	bqc, err := bigquery.NewClient(ctx, project, opts...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = bqc.Close() }()
	client := &BigQueryClient{client: bqc, projectID: "p", metricsDataset: "d"}
	um := func(day, user, gross string) string {
		return `SELECT DATE '` + day + `' day,'enterprise' scope,'nav' scope_id,'{"user_id":"` + user + `","ai_credits_used":` + gross + `}' raw_record`
	}
	sql := "WITH " + client.teamUsageCTEs() + ", " + client.teamNetCTEs() + `
SELECT CONCAT('daily/',user_id,'/',CAST(day AS STRING)) k,net FROM daily
UNION ALL SELECT CONCAT('unspread/',user_id,'/',CAST(month AS STRING)),net FROM unspread`
	sql = strings.NewReplacer(
		"`p.d.user_metrics`", "("+strings.Join([]string{
			um("2026-05-10", "may", "0"), um("2026-05-11", "may", "0"),
			um("2026-06-05", "early", "0"), um("2026-06-10", "early", "0"),
			um("2026-06-20", "gross", "1"), um("2026-06-21", "gross", "3"), um("2026-06-22", "gross", "0"),
		}, " UNION ALL ")+")",
		"`p.d.user_teams`", "(SELECT DATE '2026-06-01' day,'enterprise' scope,'nav' scope_id,'{}' raw_record)",
		"`p.d.billing_user_monthly`", `(SELECT m month,'nav' scope_id,u user_id,s sku,n net_amount,TIMESTAMP(l) loaded_at
 FROM UNNEST([STRUCT(DATE '2026-06-01' AS m,'early' AS u,'Copilot AI Credits' AS s,10.0 AS n,'2026-07-03' AS l),
  (DATE '2026-06-01','gross','Copilot AI Credits',8.0,'2026-07-03'),(DATE '2026-06-01','none','Copilot AI Credits',4.0,'2026-07-03'),
  (DATE '2026-05-01','may','Copilot Premium Request',6.0,'2026-06-03'),
  -- An earlier load of the same rows that a rerun appended to: only the latest counts.
  (DATE '2026-06-01','early','Copilot AI Credits',10.0,'2026-07-02'),(DATE '2026-06-01','none','Copilot AI Credits',7.0,'2026-07-02')]))`,
	).Replace(sql)
	query := bqc.Query(sql)
	query.Parameters = monthRange(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), 2)
	it, err := query.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for {
		var row map[string]bigquery.Value
		if err := it.Next(&row); err == iterator.Done {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		got[row["k"].(string)] = row["net"].(float64)
	}
	want := map[string]float64{
		"daily/may/2026-05-10": 3, "daily/may/2026-05-11": 3,
		"daily/early/2026-06-05": 5, "daily/early/2026-06-10": 5,
		"daily/gross/2026-06-20": 2, "daily/gross/2026-06-21": 6, "daily/gross/2026-06-22": 0,
		"unspread/none/2026-06-01": 4,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if math.Abs(got[k]-v) > 1e-9 {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
