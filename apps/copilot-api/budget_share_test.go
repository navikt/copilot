package main

import (
	"os"
	"strings"
	"testing"
)

func TestSpendLimitSQL(t *testing.T) {
	got := spendLimitSQL("d")
	want := "CASE WHEN d >= DATE '2026-09-22' THEN 800 WHEN d >= DATE '2026-07-01' THEN 400 WHEN d >= DATE '2026-06-01' THEN 200 END"
	if got != want {
		t.Fatalf("got %s", got)
	}
}

// BUDGET_SHARE_SQL=path writes the full query for a dry run with bq.
func TestBudgetShareCTE(t *testing.T) {
	bq := &BigQueryClient{projectID: "copilot-prod-c697", metricsDataset: "copilot_metrics"}
	cte := bq.budgetShareCTE()
	for _, s := range []string{"budget_share AS (", "'snapshot'", "'estimate'", "user_budget_snapshots", "DATE '2026-06-19'", "budget_scope='multi_user_customer'"} {
		if !strings.Contains(cte, s) {
			t.Errorf("missing %q", s)
		}
	}
	if p := os.Getenv("BUDGET_SHARE_SQL"); p != "" {
		q := "WITH " + cte + "\nSELECT DATE_TRUNC(day, MONTH) month, source, COUNT(*) user_days, COUNT(DISTINCT user_id) users, COUNTIF(limit_usd IS NULL) no_limit, ROUND(AVG(share), 3) avg_share FROM budget_share GROUP BY 1, 2 ORDER BY 1, 2"
		if err := os.WriteFile(p, []byte(q), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
