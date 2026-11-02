package main

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"
)

func TestInteriorDeciles(t *testing.T) {
	q := []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 100}
	got := interiorDeciles(q, 10*minUsersForDistribution)
	if len(got) != 9 || got[0] != 1 || got[8] != 9 {
		t.Fatalf("interiorDeciles = %v, want [1..9] without min and max", got)
	}
	if got := interiorDeciles(q, 10*minUsersForDistribution-1); got != nil {
		t.Errorf("below n>=%d per decile: got %v, want nil", minUsersForDistribution, got)
	}
	if got := interiorDeciles([]int64{1, 2}, 1000); got != nil {
		t.Errorf("unexpected length: got %v, want nil", got)
	}
}

func TestBillingModelDailyQueryConstruction(t *testing.T) {
	bqc, err := bigquery.NewClient(context.Background(), "p", option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bqc.Close() })
	client := &BigQueryClient{client: bqc, projectID: "p", metricsDataset: "d"}
	q := client.billingModelDailyQuery("2026-06")

	for _, want := range []string{
		"FROM " + client.tableRef(client.metricsDataset, "billing_usage_daily_model"),
		"product = 'Copilot'",
		"LOWER(scope_id) = 'nav'",
		"gross_quantity > 0",
		"FORMAT_DATE('%Y-%m-%d', day) AS day",
		"day >= DATE(@month || '-01')",
		"day < DATE_ADD(DATE(@month || '-01'), INTERVAL 1 MONTH)",
	} {
		if !strings.Contains(q.Q, want) {
			t.Errorf("query lacks %q:\n%s", want, q.Q)
		}
	}
	if strings.Contains(q.Q, "%!") {
		t.Error("format verb left in query")
	}
	if len(q.Parameters) != 1 || q.Parameters[0].Name != "month" || q.Parameters[0].Value != "2026-06" {
		t.Errorf("parameters = %+v, want month=2026-06", q.Parameters)
	}
}

func TestBillingModelDailySyntheticMixedProduct(t *testing.T) {
	project := os.Getenv("VERIFY_BILLING_MODEL_DAILY_PROJECT")
	if project == "" {
		t.Skip("set VERIFY_BILLING_MODEL_DAILY_PROJECT for the opt-in synthetic BigQuery check")
	}

	client, err := newBigQueryClient(&Config{GCPProjectID: project, CopilotMetricsDataset: "copilot_metrics"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	fixture := `(
		SELECT DATE('2026-06-01') AS day, 'nav' AS scope_id, 'Copilot' AS product, 'gpt-4o' AS model, 10.0 AS gross_quantity, 10.0 AS net_quantity, 5.0 AS gross_amount, 4.0 AS net_amount
		UNION ALL
		SELECT DATE('2026-06-01') AS day, 'nav' AS scope_id, 'Code Quality' AS product, 'gpt-4o' AS model, 20.0 AS gross_quantity, 20.0 AS net_quantity, 15.0 AS gross_amount, 12.0 AS net_amount
		UNION ALL
		SELECT DATE('2026-05-31') AS day, 'nav' AS scope_id, 'Copilot' AS product, 'gpt-4o' AS model, 5.0 AS gross_quantity, 5.0 AS net_quantity, 2.5 AS gross_amount, 2.0 AS net_amount
		UNION ALL
		SELECT DATE('2026-07-01') AS day, 'nav' AS scope_id, 'Copilot' AS product, 'gpt-4o' AS model, 5.0 AS gross_quantity, 5.0 AS net_quantity, 2.5 AS gross_amount, 2.0 AS net_amount
		UNION ALL
		SELECT DATE('2026-06-02') AS day, 'other-scope' AS scope_id, 'Copilot' AS product, 'gpt-4o' AS model, 10.0 AS gross_quantity, 10.0 AS net_quantity, 5.0 AS gross_amount, 4.0 AS net_amount
		UNION ALL
		SELECT DATE('2026-06-02') AS day, 'nav' AS scope_id, 'Copilot' AS product, 'gpt-4o' AS model, 0.0 AS gross_quantity, 0.0 AS net_quantity, 0.0 AS gross_amount, 0.0 AS net_amount
	)`

	q := client.billingModelDailyQuery("2026-06")
	source := client.tableRef(client.metricsDataset, "billing_usage_daily_model")
	if strings.Count(q.Q, source) != 1 {
		t.Fatal("expected exactly one billing table reference to replace")
	}
	// Exercise the runtime SQL without reading billing records.
	q.Q = strings.Replace(q.Q, source, fixture, 1)
	if strings.Contains(q.Q, "`") {
		t.Fatal("synthetic query must not reference another table")
	}
	it, err := q.Read(ctx)
	if err != nil {
		t.Fatalf("execute query with synthetic fixture: %v", err)
	}

	rows, err := readAllRows[BillingModelDailyCost](it)
	if err != nil {
		t.Fatalf("read rows: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(rows), rows)
	}

	r := rows[0]
	if r.Day != "2026-06-01" || r.Model != "gpt-4o" {
		t.Errorf("row = %+v, want day=2026-06-01 model=gpt-4o", r)
	}
	if r.GrossRequests != 10 || r.NetRequests != 10 {
		t.Errorf("requests = %d gross, %d net; want 10 each (Code Quality leaked if 30)", r.GrossRequests, r.NetRequests)
	}
	if math.Abs(r.GrossAmount-5.0) > 0.001 || math.Abs(r.NetAmount-4.0) > 0.001 {
		t.Errorf("amounts = gross %.2f, net %.2f; want 5.00 and 4.00 (Code Quality leaked if 20.00 and 16.00)", r.GrossAmount, r.NetAmount)
	}
}
