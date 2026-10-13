package main

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"
)

func f(v float64) *float64 { return &v }

func TestAdminMonthlyOverview(t *testing.T) {
	cov := adminMonthRow{MembershipFrom: "2026-05-06", LastGrossDay: "2026-10-09", SeatsSnapshot: "2026-10-11"}
	row := func(month string, complete bool, enterprise, residual, assigned, team float64, gross *float64) adminMonthRow {
		r := cov
		r.Month, r.NetComplete, r.AssignedNet, r.TeamNet, r.Gross = month+"-01", complete, assigned, team, gross
		r.Seats, r.Licence, r.LoadedAt = 10, 190, "2026-10-11 11:32:45"
		if complete {
			r.EnterpriseNet, r.ResidualNet = f(enterprise), f(residual)
		}
		return r
	}
	rows := []adminMonthRow{
		row("2026-01", true, 1267.62, 152.79, 1114.83, 1000, f(2730.3)),
		row("2026-09", true, 64558.85, 170.51, 64388.34, 57000, f(79735.69)),
		row("2026-10", true, 9000, 1, 8999, 0, f(24517.32)), // run for the current month is ignored
		row("2026-11", false, 0, 0, 0, 0, nil),
	}
	rows[2].GrossNet = f(9290.14)
	now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	out := adminMonthlyOverview(2026, rows, now)
	if len(out.Months) != 3 {
		t.Fatalf("months = %d, want 3 (November is in the future)", len(out.Months))
	}
	jan, sep, oct := out.Months[0], out.Months[1], out.Months[2]
	if jan.Basis != basisPremium || sep.Basis != basisNet || oct.Basis != basisGross {
		t.Errorf("basis = %s/%s/%s, want premium/net/gross", jan.Basis, sep.Basis, oct.Basis)
	}
	for _, m := range []AdminMonth{jan, sep} {
		if d := *m.AssignedNetUSD + *m.ResidualNetUSD - *m.EnterpriseNetUSD; math.Abs(d) > 0.011 {
			t.Errorf("%s: assigned + residual - enterprise = %.2f", m.Month, d)
		}
		if m.GrossUSD != nil {
			t.Errorf("%s: net month has gross", m.Month)
		}
	}
	if *jan.ResidualPct != 12.05 {
		t.Errorf("January residual = %v%%, want 12.05", *jan.ResidualPct)
	}
	if jan.NoTeamNetUSD != nil || *sep.NoTeamNetUSD != 7388.34 {
		t.Errorf("no team = %v/%v, want nil before user_teams and 7388.34", jan.NoTeamNetUSD, sep.NoTeamNetUSD)
	}
	// The total for a month without net is the estimated billed amount, never gross.
	if oct.EnterpriseNetUSD != nil || *oct.GrossUSD != 24517.32 || *oct.EstimatedNetUSD != 9290.14 || *oct.TotalUSD != 9480.14 {
		t.Errorf("October = %+v, want estimated net plus licences", oct)
	}
	if *sep.TotalUSD != 64748.85 || *sep.Seats != 10 {
		t.Errorf("September total %v seats %v", *sep.TotalUSD, *sep.Seats)
	}
	if out.Coverage.LastNetMonth != "2026-09" || !out.Coverage.GrossStale {
		t.Errorf("coverage = %+v, want last net 2026-09 and stale gross", out.Coverage)
	}
	if adminMonthlyOverview(2026, rows, time.Date(2026, 10, 11, 23, 0, 0, 0, time.UTC)).Coverage.GrossStale {
		t.Error("gross two days old reported stale")
	}
}

func TestAdminMonthlyQueryReusesSharedCTEs(t *testing.T) {
	bqc, err := bigquery.NewClient(context.Background(), "p", option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	client := &BigQueryClient{client: bqc, projectID: "p", metricsDataset: "d"}
	q := client.adminMonthlyQuery(2026).Q
	for _, want := range []string{client.teamNetCTEs(), client.teamUsageCTEs(), client.seatCTEs(), "'enterprise',39.00,19.00"} {
		if !strings.Contains(q, want) {
			t.Errorf("query lacks %.60q", want)
		}
	}
	if strings.Contains(q, "%!") {
		t.Error("format verb left in query")
	}
}

// TestAdminMonthlyReconcilesBigQuery is the read-only check behind the PR
// evidence: VERIFY_ADMIN_MONTHLY_PROJECT=copilot-prod-c697 go test -run Reconciles.
func TestAdminMonthlyReconcilesBigQuery(t *testing.T) {
	project := os.Getenv("VERIFY_ADMIN_MONTHLY_PROJECT")
	if project == "" {
		t.Skip("set VERIFY_ADMIN_MONTHLY_PROJECT for the read-only check")
	}
	client, err := newBigQueryClient(&Config{GCPProjectID: project, CopilotMetricsDataset: "copilot_metrics"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	out, err := client.GetAdminMonthlyOverview(ctx, time.Now().UTC().Year())
	if err != nil {
		t.Fatal(err)
	}
	// Net per month straight from the enterprise usage table, independent of the run.
	q := client.client.Query(`SELECT FORMAT_DATE('%Y-%m',day) month,SUM(net_amount) net FROM ` +
		client.tableRef(client.metricsDataset, "billing_usage_daily_model") +
		` WHERE scope_id='nav' AND product='Copilot' AND sku IN (` + billedSKUs + `) GROUP BY month`)
	it, err := q.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	type usage struct {
		Month string  `bigquery:"month"`
		Net   float64 `bigquery:"net"`
	}
	rows, err := readAllRows[usage](it)
	if err != nil {
		t.Fatal(err)
	}
	usageNet := map[string]float64{}
	for _, r := range rows {
		usageNet[r.Month] = r.Net
	}
	net := 0
	for _, m := range out.Months {
		t.Logf("%s %-7s enterprise=%v assigned=%v residual=%v (%v%%) no_team=%v gross=%v seats=%v licence=%v",
			m.Month, m.Basis, deref(m.EnterpriseNetUSD), deref(m.AssignedNetUSD), deref(m.ResidualNetUSD), deref(m.ResidualPct),
			deref(m.NoTeamNetUSD), deref(m.GrossUSD), derefInt(m.Seats), deref(m.LicenceUSDEstimate))
		if m.EnterpriseNetUSD == nil {
			continue
		}
		net++
		if d := *m.AssignedNetUSD + *m.ResidualNetUSD - *m.EnterpriseNetUSD; math.Abs(d) > 0.011 {
			t.Errorf("%s: assigned + residual - enterprise = %.2f", m.Month, d)
		}
		if d := *m.EnterpriseNetUSD - usageNet[m.Month]; math.Abs(d) > 0.01 {
			t.Errorf("%s: enterprise net %.2f, billing_usage_daily_model %.2f", m.Month, *m.EnterpriseNetUSD, usageNet[m.Month])
		}
	}
	if net == 0 {
		t.Fatal("no net month compared")
	}
}

func deref(v *float64) any {
	if v == nil {
		return "-"
	}
	return *v
}

func derefInt(v *int64) any {
	if v == nil {
		return "-"
	}
	return *v
}

// The route must sit behind requireAdmin: Nav's bill is for admins only.
func TestAdminMonthlyRouteRequiresAdmin(t *testing.T) {
	const group = "11111111-2222-3333-4444-555555555555"
	t.Setenv(adminGroupEnv, group)
	router := makeAPIRouter(&Config{}, &BigQueryHandlers{bqClient: &mockBigQueryClient{}}, nil, nil, NewIdentityResolverChain())
	for _, tc := range []struct {
		groups []string
		want   int
	}{{[]string{group}, http.StatusOK}, {[]string{"other"}, http.StatusNotFound}} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/billing/monthly", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Groups: tc.groups}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("groups %v: got %d, want %d", tc.groups, rec.Code, tc.want)
		}
	}
}
