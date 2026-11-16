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
	row := func(month string, complete bool, enterprise, residual, assigned, team, noUsage float64, gross *float64) adminMonthRow {
		r := cov
		r.Month, r.NetComplete, r.AssignedNet, r.TeamNet, r.NoUsageNet, r.Gross = month+"-01", complete, assigned, team, noUsage, gross
		r.Seats, r.Licence, r.LoadedAt = 10, 190, "2026-10-11 11:32:45"
		if complete {
			r.EnterpriseNet, r.ResidualNet = f(enterprise), f(residual)
		}
		return r
	}
	rows := []adminMonthRow{
		row("2026-01", true, 1267.62, 152.79, 1114.83, 1000, 50, f(2730.3)),           // unavailable membership (< MembershipFrom)
		row("2026-06", true, 50000, 1000, 49000, 40000, 4000, f(60000)),               // mixed allocation
		row("2026-07", true, 20000, 500, 19500, 0, 19500, f(25000)),                   // no observed usage (all unspread)
		row("2026-08", true, 30000, 500, 29500, 0, 0, f(35000)),                       // usage without team membership
		row("2026-09", true, 64558.85, 170.51, 64388.34, 57000, 1388.34, f(79735.69)), // mixed with cent precision
		row("2026-10", true, 9000, 1, 8999, 0, 0, f(24517.32)),                        // current provisional month
		row("2026-11", false, 0, 0, 0, 0, 0, nil),                                     // future month
	}
	rows[5].GrossNet = f(9290.14)
	now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	out := adminMonthlyOverview(2026, rows, now)
	if len(out.Months) != 6 {
		t.Fatalf("months = %d, want 6 (November is in the future)", len(out.Months))
	}
	jan, jun, jul, aug, sep, oct := out.Months[0], out.Months[1], out.Months[2], out.Months[3], out.Months[4], out.Months[5]
	if jan.Basis != basisPremium || sep.Basis != basisNet || oct.Basis != basisGross {
		t.Errorf("basis = %s/%s/%s, want premium/net/gross", jan.Basis, sep.Basis, oct.Basis)
	}
	for _, m := range []AdminMonth{jan, jun, jul, aug, sep} {
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
	// January is before MembershipFrom (2026-05-06): both must be nil
	if jan.NoTeamNetUSD != nil || jan.NoUsageNetUSD != nil {
		t.Errorf("January allocation: no_team=%v, no_usage=%v, want nil before user_teams", jan.NoTeamNetUSD, jan.NoUsageNetUSD)
	}
	// June: mixed allocation
	if *jun.NoUsageNetUSD != 4000 || *jun.NoTeamNetUSD != 5000 {
		t.Errorf("June: no_usage=%v, no_team=%v, want 4000 and 5000", *jun.NoUsageNetUSD, *jun.NoTeamNetUSD)
	}
	// July: no observed usage (all assigned lands on no_usage)
	if *jul.NoUsageNetUSD != 19500 || *jul.NoTeamNetUSD != 0 {
		t.Errorf("July: no_usage=%v, no_team=%v, want 19500 and 0", *jul.NoUsageNetUSD, *jul.NoTeamNetUSD)
	}
	// August: usage without membership (no_usage=0, all unassigned is no_team)
	if *aug.NoUsageNetUSD != 0 || *aug.NoTeamNetUSD != 29500 {
		t.Errorf("August: no_usage=%v, no_team=%v, want 0 and 29500", *aug.NoUsageNetUSD, *aug.NoTeamNetUSD)
	}
	// September: mixed cent precision (64388.34 - 57000 - 1388.34 = 6000.00)
	if *sep.NoUsageNetUSD != 1388.34 || *sep.NoTeamNetUSD != 6000.00 {
		t.Errorf("September: no_usage=%v, no_team=%v, want 1388.34 and 6000.00", *sep.NoUsageNetUSD, *sep.NoTeamNetUSD)
	}
	// Verify exact sum reconciliation for eligible months: TeamNet + NoTeamNet + NoUsageNet == AssignedNet
	for _, tc := range []struct {
		m       AdminMonth
		teamNet float64
	}{
		{jun, 40000},
		{jul, 0},
		{aug, 0},
		{sep, 57000},
	} {
		sum := tc.teamNet + *tc.m.NoTeamNetUSD + *tc.m.NoUsageNetUSD
		if d := sum - *tc.m.AssignedNetUSD; math.Abs(d) > 0.011 {
			t.Errorf("%s: team + no_team + no_usage - assigned = %.2f", tc.m.Month, d)
		}
	}
	// October (provisional): net allocation fields must be nil
	if oct.EnterpriseNetUSD != nil || oct.NoTeamNetUSD != nil || oct.NoUsageNetUSD != nil ||
		*oct.GrossUSD != 24517.32 || *oct.EstimatedNetUSD != 9290.14 || *oct.TotalUSD != 9480.14 {
		t.Errorf("October = %+v, want estimated net plus licences and nil net allocations", oct)
	}
	// Unavailable membership from empty MembershipFrom
	emptyOut := adminMonthlyOverview(2026, []adminMonthRow{
		{
			Month:          "2026-09-01",
			NetComplete:    true,
			EnterpriseNet:  f(100),
			ResidualNet:    f(10),
			AssignedNet:    90,
			TeamNet:        70,
			NoUsageNet:     10,
			MembershipFrom: "",
		},
	}, now)
	if len(emptyOut.Months) != 1 || emptyOut.Months[0].NoTeamNetUSD != nil || emptyOut.Months[0].NoUsageNetUSD != nil {
		t.Errorf("empty MembershipFrom: no_team=%v, no_usage=%v, want both nil",
			emptyOut.Months[0].NoTeamNetUSD, emptyOut.Months[0].NoUsageNetUSD)
	}
	// Negative adjustment (refund/correction)
	negOut := adminMonthlyOverview(2026, []adminMonthRow{
		{
			Month:          "2026-09-01",
			NetComplete:    true,
			EnterpriseNet:  f(80),
			ResidualNet:    f(10),
			AssignedNet:    70,
			TeamNet:        80,
			NoUsageNet:     -10, // adjustment in unspread
			MembershipFrom: "2026-05-06",
		},
	}, now)
	if len(negOut.Months) != 1 || *negOut.Months[0].NoUsageNetUSD != -10 || *negOut.Months[0].NoTeamNetUSD != 0 {
		t.Errorf("negative adjustment: no_usage=%v, no_team=%v, want -10 and 0",
			negOut.Months[0].NoUsageNetUSD, negOut.Months[0].NoTeamNetUSD)
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
	for _, want := range []string{
		client.teamNetCTEs(),
		client.teamUsageCTEs(),
		client.seatCTEs(),
		"unspread AS (\n SELECT b.month,b.user_id,b.net FROM billed b\n WHERE NOT EXISTS (SELECT 1 FROM usage_days u WHERE u.user_id=b.user_id AND u.month=b.month)\n)",
		"no_usage_m AS (\n SELECT month,SUM(net) net FROM unspread GROUP BY month\n),",
		"team_m AS (\n SELECT DATE_TRUNC(day,MONTH) month,SUM(net) net FROM daily w\n WHERE EXISTS (SELECT 1 FROM memberships m WHERE m.day=w.day AND m.user_id=w.user_id) GROUP BY month\n),",
		"IFNULL(nu.net,0) no_usage_net",
		"LEFT JOIN no_usage_m nu ON nu.month=t.month",
		"'enterprise',39.00,19.00",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query lacks %.60q", want)
		}
	}
	if strings.Contains(q, "%!") {
		t.Error("format verb left in query")
	}
}

func TestAdminMonthlySyntheticQuery(t *testing.T) {
	project := os.Getenv("VERIFY_ADMIN_MONTHLY_SYNTHETIC_PROJECT")
	if project == "" {
		t.Skip("set VERIFY_ADMIN_MONTHLY_SYNTHETIC_PROJECT for the opt-in synthetic BigQuery check")
	}

	client, err := newBigQueryClient(&Config{GCPProjectID: project, CopilotMetricsDataset: "copilot_metrics"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// 1. Synthetic runs: complete run for 2026-06 with enterprise_net=100, residual_net=10
	syntheticRuns := `(
		SELECT DATE '2026-06-01' AS month, TIMESTAMP '2026-07-02 08:00:00 UTC' AS loaded_at,
			100.0 AS enterprise_net, 10.0 AS residual_net, 'nav' AS scope_id, 'complete' AS status
	)`

	// 2. Synthetic billing_user_monthly: covers raw billed users, zero-gross user, unspread user, and refund
	syntheticBillingUserMonthly := `(
		SELECT DATE '2026-06-01' AS month, 'nav' AS scope_id, 'u1' AS user_id, 'Copilot AI Credits' AS sku, 30.0 AS net_amount, TIMESTAMP '2026-07-02 08:00:00 UTC' AS loaded_at
		UNION ALL
		SELECT DATE '2026-06-01' AS month, 'nav' AS scope_id, 'u2' AS user_id, 'Copilot AI Credits' AS sku, 25.0 AS net_amount, TIMESTAMP '2026-07-02 08:00:00 UTC' AS loaded_at
		UNION ALL
		SELECT DATE '2026-06-01' AS month, 'nav' AS scope_id, 'u3' AS user_id, 'Copilot AI Credits' AS sku, 20.0 AS net_amount, TIMESTAMP '2026-07-02 08:00:00 UTC' AS loaded_at
		UNION ALL
		SELECT DATE '2026-06-01' AS month, 'nav' AS scope_id, 'u4' AS user_id, 'Copilot AI Credits' AS sku, 20.0 AS net_amount, TIMESTAMP '2026-07-02 08:00:00 UTC' AS loaded_at
		UNION ALL
		SELECT DATE '2026-06-01' AS month, 'nav' AS scope_id, 'u5' AS user_id, 'Copilot AI Credits' AS sku, -5.0 AS net_amount, TIMESTAMP '2026-07-02 08:00:00 UTC' AS loaded_at
	)`

	// 3. Synthetic user_metrics: usage days for u1, u2, u4 (gross=0); u3 and u5 absent
	syntheticUserMetrics := `(
		SELECT DATE '2026-06-10' AS day, 'enterprise' AS scope, 'nav' AS scope_id, '{"user_id":"u1","ai_credits_used":"50"}' AS raw_record
		UNION ALL
		SELECT DATE '2026-06-11' AS day, 'enterprise' AS scope, 'nav' AS scope_id, '{"user_id":"u1","ai_credits_used":"50"}' AS raw_record
		UNION ALL
		SELECT DATE '2026-06-15' AS day, 'enterprise' AS scope, 'nav' AS scope_id, '{"user_id":"u2","ai_credits_used":"100"}' AS raw_record
		UNION ALL
		SELECT DATE '2026-06-20' AS day, 'enterprise' AS scope, 'nav' AS scope_id, '{"user_id":"u4","ai_credits_used":"0"}' AS raw_record
	)`

	// 4. Synthetic user_teams: initial coverage day, multi-team day (u1 in t1 and t2 on same day), missing membership day (u2 has no entry)
	syntheticUserTeams := `(
		SELECT DATE '2026-05-06' AS day, 'enterprise' AS scope, 'nav' AS scope_id, '{"user_id":"init","team_id":"t0","slug":"team-zero"}' AS raw_record
		UNION ALL
		SELECT DATE '2026-06-10' AS day, 'enterprise' AS scope, 'nav' AS scope_id, '{"user_id":"u1","team_id":"t1","slug":"team-alpha"}' AS raw_record
		UNION ALL
		SELECT DATE '2026-06-10' AS day, 'enterprise' AS scope, 'nav' AS scope_id, '{"user_id":"u1","team_id":"t2","slug":"team-beta"}' AS raw_record
		UNION ALL
		SELECT DATE '2026-06-11' AS day, 'enterprise' AS scope, 'nav' AS scope_id, '{"user_id":"u1","team_id":"t1","slug":"team-alpha"}' AS raw_record
		UNION ALL
		SELECT DATE '2026-06-20' AS day, 'enterprise' AS scope, 'nav' AS scope_id, '{"user_id":"u4","team_id":"t1","slug":"team-alpha"}' AS raw_record
	)`

	// 5. Synthetic billing_usage_daily_model: gross usage and coverage
	syntheticDailyModel := `(
		SELECT DATE '2026-06-15' AS day, 'nav' AS scope_id, 'Copilot' AS product, 'Copilot AI Credits' AS sku, 120.0 AS gross_amount, 90.0 AS net_amount
	)`

	// 6. Synthetic copilot_seats: 1 business seat
	syntheticCopilotSeats := `(
		SELECT DATE '2026-06-01' AS snapshot_date, TIMESTAMP '2026-01-01 00:00:00 UTC' AS created_at,
			CAST(NULL AS DATE) AS pending_cancellation_date, 'u1' AS user_id, 'u1' AS login, 'business' AS plan_type
	)`

	q := client.adminMonthlyQuery(2026)

	// Replace all 6 table references with synthetic inline fixtures
	replacements := map[string]string{
		client.tableRef(client.metricsDataset, "billing_user_monthly_runs"): syntheticRuns,
		client.tableRef(client.metricsDataset, "billing_user_monthly"):      syntheticBillingUserMonthly,
		client.tableRef(client.metricsDataset, "user_metrics"):              syntheticUserMetrics,
		client.tableRef(client.metricsDataset, "user_teams"):                syntheticUserTeams,
		client.tableRef(client.metricsDataset, "billing_usage_daily_model"): syntheticDailyModel,
		client.tableRef(client.metricsDataset, "copilot_seats"):             syntheticCopilotSeats,
	}

	for table, fixture := range replacements {
		if !strings.Contains(q.Q, table) {
			t.Fatalf("expected query to contain table reference %s", table)
		}
		q.Q = strings.ReplaceAll(q.Q, table, fixture)
	}

	// Strictly reject any remaining real table reference
	if strings.Contains(q.Q, "`") {
		t.Fatalf("synthetic query must not reference any real tables:\n%s", q.Q)
	}

	it, err := q.Read(ctx)
	if err != nil {
		t.Fatalf("execute synthetic query: %v", err)
	}
	rows, err := readAllRows[adminMonthRow](it)
	if err != nil {
		t.Fatalf("read synthetic rows: %v", err)
	}

	overview := adminMonthlyOverview(2026, rows, time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC))
	var jun *AdminMonth
	for i := range overview.Months {
		if overview.Months[i].Month == "2026-06" {
			jun = &overview.Months[i]
			break
		}
	}
	if jun == nil {
		t.Fatalf("missing June 2026 in overview: %+v", overview.Months)
	}

	// Verify exact amounts from real BigQuery SQL execution
	if jun.Basis != basisNet {
		t.Errorf("basis = %s, want net", jun.Basis)
	}
	if *jun.EnterpriseNetUSD != 100.0 {
		t.Errorf("EnterpriseNetUSD = %v, want 100.00", *jun.EnterpriseNetUSD)
	}
	if *jun.AssignedNetUSD != 90.0 {
		t.Errorf("AssignedNetUSD = %v, want 90.00", *jun.AssignedNetUSD)
	}
	if *jun.ResidualNetUSD != 10.0 {
		t.Errorf("ResidualNetUSD = %v, want 10.00", *jun.ResidualNetUSD)
	}
	// TeamNet: u1 (30.00) + u4 (20.00) = 50.00
	// NoUsageNet: u3 (20.00) + u5 (-5.00) = 15.00
	// NoTeamNet: AssignedNet (90.00) - TeamNet (50.00) - NoUsageNet (15.00) = 25.00 (u2)
	if *jun.NoUsageNetUSD != 15.0 {
		t.Errorf("NoUsageNetUSD = %v, want 15.00", *jun.NoUsageNetUSD)
	}
	if *jun.NoTeamNetUSD != 25.0 {
		t.Errorf("NoTeamNetUSD = %v, want 25.00", *jun.NoTeamNetUSD)
	}
	if *jun.Seats != 1 || *jun.LicenceUSDEstimate != 19.0 || *jun.TotalUSD != 119.0 {
		t.Errorf("seats=%v licence=%v total=%v; want 1, 19.00, 119.00", *jun.Seats, *jun.LicenceUSDEstimate, *jun.TotalUSD)
	}
	// Full reconciliation check
	if math.Abs(*jun.AssignedNetUSD+*jun.ResidualNetUSD-*jun.EnterpriseNetUSD) > 0.001 {
		t.Errorf("enterprise reconciliation failed: assigned %v + residual %v != enterprise %v",
			*jun.AssignedNetUSD, *jun.ResidualNetUSD, *jun.EnterpriseNetUSD)
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
