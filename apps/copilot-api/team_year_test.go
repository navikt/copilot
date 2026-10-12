package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"
)

func TestTeamYearOverviewBasis(t *testing.T) {
	cov := teamYearRow{MembershipFrom: "2026-05-06", GrossFrom: "2026-06-15", LastUsageDay: "2026-10-09"}
	row := func(month string, complete bool, grossUsers, netUsers int64) teamYearRow {
		r := cov
		r.Month, r.NetComplete, r.GrossUsers, r.Gross, r.NetUsers, r.Net, r.NoUsageNet = month+"-01", complete, grossUsers, 100.004, netUsers, 80.006, 1.5
		if complete {
			pct := 7.126
			r.ResidualPct = &pct
		}
		return r
	}
	rows := []teamYearRow{
		row("2026-04", false, 0, 0), // before membership history
		row("2026-05", true, 0, 9),  // premium requests: net only
		row("2026-06", true, 6, 4),  // under five payers: still shown
		row("2026-07", false, 4, 0), // gross, under five: still shown
		row("2026-08", true, 6, 5),  // net
		row("2026-09", true, 4, 5),  // net
		row("2026-10", true, 7, 7),  // current month never net
		row("2026-11", false, 0, 0), // future
	}
	rows[5].ResidualPct = nil // complete run from before the residual columns
	got := teamYearOverview("123", 2026, rows, time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC))
	want := []struct {
		month, basis string
		net, gross   bool
	}{
		{"2026-04", basisNone, false, false},
		{"2026-05", basisPremium, true, false},
		{"2026-06", basisNet, true, true},
		{"2026-07", basisGross, false, true},
		{"2026-08", basisNet, true, true},
		{"2026-09", basisNet, true, true},
		{"2026-10", basisGross, false, true},
	}
	if len(got.Months) != len(want) {
		t.Fatalf("months = %+v", got.Months)
	}
	for i, w := range want {
		m := got.Months[i]
		if m.Month != w.month || m.Basis != w.basis || (m.NetUSD != nil) != w.net || (m.GrossUSD != nil) != w.gross {
			t.Errorf("%s = %+v, want %+v", w.month, m, w)
		}
		hasNet := (w.basis == basisNet || w.basis == basisPremium) && w.month != "2026-09"
		if (m.ResidualPct != nil) != hasNet || (hasNet && *m.ResidualPct != 7.13) {
			t.Errorf("%s residual = %v", w.month, m.ResidualPct)
		}
		if (m.Basis == basisNone) != (m.Users == nil) {
			t.Errorf("%s users: %+v", w.month, m)
		}
	}
	if aug := got.Months[4]; *aug.NetUSD != 80.01 || *aug.GrossUSD != 100 || *aug.Users != 5 || *aug.NoUsageNetUSD != 1.5 {
		t.Errorf("august = %+v", aug)
	}
	if strings.Join(got.Coverage.NetMonths, ",") != "2026-06,2026-08,2026-09" || got.Coverage.GrossFrom != "2026-06-15" {
		t.Errorf("coverage = %+v", got.Coverage)
	}
}

func TestTeamYearLicences(t *testing.T) {
	row := func(month string, seats int64, licence float64) teamYearRow {
		return teamYearRow{Month: month + "-01", SeatMonths: seats, Licence: licence, SeatsSnapshot: "2026-10-11", TeamMembersFrom: "2026-10-11"}
	}
	now := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)
	got := teamYearOverview("123", 2026, []teamYearRow{
		row("2026-04", 9, 211), // before user_teams history: licences still shown
		row("2026-05", 4, 76),  // under five: shown too
		row("2026-06", 5, 95),
	}, now).Months
	if *got[1].SeatMonths != 4 || *got[1].LicenceUSDEstimate != 76 {
		t.Errorf("under five: %+v", got[1])
	}
	if *got[0].SeatMonths != 9 || *got[0].LicenceUSDEstimate != 211 || *got[2].LicenceUSDEstimate != 95 {
		t.Errorf("licences = %+v %+v", got[0], got[2])
	}
	if got[0].NetUSD != nil || got[0].GrossUSD != nil || got[0].TotalUSD != nil || got[0].TotalBasis != basisNone {
		t.Errorf("licences leaked into usage or total: %+v", got[0])
	}
	for _, missing := range []teamYearRow{
		{Month: "2026-07-01", SeatMonths: 9, Licence: 171, TeamMembersFrom: "2026-10-11"},
		{Month: "2026-07-01", SeatMonths: 9, Licence: 171, SeatsSnapshot: "2026-10-11"},
	} {
		if m := teamYearOverview("123", 2026, []teamYearRow{missing}, now).Months[0]; m.SeatMonths != nil {
			t.Errorf("no snapshot or members yet: %+v", m)
		}
	}
}

func TestTeamYearTotal(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	for _, c := range []struct {
		name string
		m    TeamYearMonth
		want *float64
	}{
		{"net with no-usage net", TeamYearMonth{Basis: basisNet, NetUSD: f(80.1), NoUsageNetUSD: f(2), LicenceUSDEstimate: f(152)}, f(234.1)},
		{"gross", TeamYearMonth{Basis: basisGross, GrossUSD: f(100), LicenceUSDEstimate: f(95)}, f(195)},
		{"net hidden", TeamYearMonth{Basis: basisNet, GrossUSD: f(100), LicenceUSDEstimate: f(95)}, nil},
		{"gross hidden", TeamYearMonth{Basis: basisGross, LicenceUSDEstimate: f(95)}, nil},
		{"licence hidden", TeamYearMonth{Basis: basisGross, GrossUSD: f(100)}, nil},
		{"no basis", TeamYearMonth{Basis: basisNone, LicenceUSDEstimate: f(95)}, nil},
	} {
		got := teamYearTotal(c.m)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: total = %v, want %v", c.name, got, c.want)
		}
	}
	cov := teamYearRow{MembershipFrom: "2026-05-06", GrossFrom: "2026-06-15", SeatsSnapshot: "2026-10-11", TeamMembersFrom: "2026-10-11"}
	cov.Month, cov.GrossUsers, cov.Gross, cov.SeatMonths, cov.Licence = "2026-07-01", 6, 100, 5, 95
	m := teamYearOverview("123", 2026, []teamYearRow{cov}, time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)).Months[0]
	if m.TotalUSD == nil || *m.TotalUSD != 195 || m.TotalBasis != basisGross {
		t.Errorf("overview total = %+v", m)
	}
}

func TestTeamYearQueryWithoutBillingTables(t *testing.T) {
	bqc, err := bigquery.NewClient(context.Background(), "p", option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	client := &BigQueryClient{client: bqc, projectID: "p", metricsDataset: "d"}
	for _, net := range []bool{true, false} {
		q := client.teamYearQuery("123", 2026, net, net, net)
		billing := strings.Contains(q.Q, "billing_user_monthly")
		if strings.Contains(q.Q, "copilot_seats") != net {
			t.Errorf("seats=%v: copilot_seats referenced wrongly", net)
		}
		if strings.Contains(q.Q, "%!") || (net && !strings.Contains(q.Q, "'enterprise',39.00,19.00")) {
			t.Errorf("seat prices not formatted into the query")
		}
		// pending_cancellation_date is the first day of the next billing
		// cycle, so the cancellation month itself is not a seat month.
		if net && !strings.Contains(q.Q, "DATE_SUB(pending_cancellation_date,INTERVAL 1 DAY)") {
			t.Errorf("cancellation month counted as a seat month")
		}
		// A reused slug must not pull in another team ID's members: match
		// membership to the team on the same day.
		if net && !strings.Contains(q.Q, "x.day=t.date AND x.team_slug=t.team_slug") {
			t.Errorf("team_members not matched to the team's slug on the same day")
		}
		if billing != net || !strings.Contains(q.Q, "user_metrics") || !strings.Contains(q.Q, "day>=DATE(@from) AND day<DATE(@to)") {
			t.Errorf("net=%v: billing tables referenced = %v", net, billing)
		}
		if len(q.Parameters) != 3 || q.Parameters[0].Value != "2026-01-01" || q.Parameters[1].Value != "2027-01-01" || q.Parameters[2].Value != "123" {
			t.Errorf("parameters = %+v", q.Parameters)
		}
	}
}

func TestTeamYearHandler(t *testing.T) {
	mock := &mockBigQueryClient{teamYear: &TeamYearOverview{TeamID: "123", Year: 2026, Months: []TeamYearMonth{{Month: "2026-08", Basis: basisNet}}}}
	h := newBigQueryHandlers(mock)
	for _, bad := range []string{"", "team=", "team=team-a", "team=0", "team=1/2", "team=1&year=2025", "team=1&year=x", "team=1&year=" + strconv.Itoa(time.Now().Year()+1)} {
		rec := httptest.NewRecorder()
		h.handleTeamYearOverview(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-year?"+bad, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status %d", bad, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.handleTeamYearOverview(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-year?team=123", nil))
	if rec.Code != http.StatusOK || mock.teamYearArgs[0] != "123" || mock.teamYearArgs[1] != time.Now().UTC().Year() {
		t.Fatalf("status %d, args %v", rec.Code, mock.teamYearArgs)
	}
	if body := rec.Body.String(); strings.Contains(body, "login") || strings.Contains(body, "user_id") {
		t.Errorf("body leaks identities: %s", body)
	}
}

// TestTeamYearMatchesMonthEndpoints is the read-only dev check that the year
// view equals team-gross and team-net for every visible team and month.
func TestTeamYearMatchesMonthEndpoints(t *testing.T) {
	if os.Getenv("VERIFY_TEAM_SPEND_BIGQUERY") != "true" {
		t.Skip("set VERIFY_TEAM_SPEND_BIGQUERY=true for the read-only dev check")
	}
	client, err := newBigQueryClient(&Config{GCPProjectID: "copilot-dev-e17a", CopilotMetricsDataset: "copilot_metrics"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	year := time.Now().UTC().Year()
	type amounts struct {
		gross, net float64
		grossUsers int64
		netUsers   int64
	}
	byTeam := map[string]map[string]*amounts{}
	get := func(id, month string) *amounts {
		if byTeam[id] == nil {
			byTeam[id] = map[string]*amounts{}
		}
		if byTeam[id][month] == nil {
			byTeam[id][month] = &amounts{}
		}
		return byTeam[id][month]
	}
	for m := 1; m <= int(time.Now().UTC().Month()); m++ {
		month := time.Date(year, time.Month(m), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
		gross, err := client.GetTeamGrossOverview(ctx, month)
		if err != nil {
			t.Fatal(err)
		}
		for _, team := range gross.Teams {
			a := get(team.TeamID, month)
			a.gross, a.grossUsers = team.AmountUSD, team.Users
		}
		net, err := client.GetTeamNetOverview(ctx, month)
		if err != nil && err != errTeamNetNotReady {
			t.Fatal(err)
		}
		if net != nil {
			for _, team := range net.Teams {
				a := get(team.TeamID, month)
				a.net, a.netUsers = team.AmountUSD, team.Users
			}
		}
	}
	ids := strings.Split(os.Getenv("VERIFY_TEAM_YEAR_IDS"), ",")
	if ids[0] == "" {
		ids = nil
		for id := range byTeam {
			ids = append(ids, id)
		}
	}
	compared := 0
	for _, id := range ids {
		overview, err := client.GetTeamYearOverview(ctx, id, year)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range overview.Months {
			want := byTeam[id][m.Month]
			if want == nil {
				// The year view has no threshold, so it may show amounts the
				// thresholded month endpoints hide. Parity only where they have the row.
				continue
			}
			if (want.grossUsers > 0) != (m.GrossUSD != nil) || (m.GrossUSD != nil && *m.GrossUSD != want.gross) {
				t.Errorf("%s %s gross %v, team-gross %v", id, m.Month, m.GrossUSD, want.gross)
			}
			if (m.Basis == basisNet || m.Basis == basisPremium) && ((want.netUsers > 0) != (m.NetUSD != nil) || (m.NetUSD != nil && (*m.NetUSD != want.net || *m.Users != want.netUsers))) {
				t.Errorf("%s %s net %v/%v, team-net %v/%d", id, m.Month, m.NetUSD, m.Users, want.net, want.netUsers)
			}
			if m.Basis == basisGross && m.Users != nil && *m.Users != want.grossUsers {
				t.Errorf("%s %s users %d, team-gross %d", id, m.Month, *m.Users, want.grossUsers)
			}
			compared++
		}
	}
	if compared == 0 {
		t.Fatal("nothing compared")
	}
	t.Logf("compared %d team-months across %d teams", compared, len(ids))
}

// A runs table without the residual columns (published before #1596) must not
// be named in the query: BigQuery fails the whole query on an unknown column.
func TestTeamYearQueryWithoutResidualColumns(t *testing.T) {
	bqc, err := bigquery.NewClient(context.Background(), "p", option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	client := &BigQueryClient{client: bqc, projectID: "p", metricsDataset: "d"}
	if q := client.teamYearQuery("123", 2026, true, false, true).Q; strings.Contains(q, "residual_net") || strings.Contains(q, "enterprise_net") {
		t.Error("query names residual columns the runs table lacks")
	}
	if q := client.teamYearQuery("123", 2026, true, true, true).Q; !strings.Contains(q, "SAFE_DIVIDE(residual_net,enterprise_net)") {
		t.Error("query drops residual_pct when the columns exist")
	}
	old := bigquery.Schema{{Name: "month"}, {Name: "status"}}
	if hasColumns(old, "enterprise_net", "residual_net") || !hasColumns(append(old, &bigquery.FieldSchema{Name: "enterprise_net"}, &bigquery.FieldSchema{Name: "residual_net"}), "enterprise_net", "residual_net") {
		t.Error("hasColumns wrong")
	}
}

// The row goes through decodeBQRow in production; a bigquery.Null* field there
// failed every team-year request ("cannot assign value of type float64").
func TestTeamYearRowDecodesResidual(t *testing.T) {
	schema := bigquery.Schema{{Name: "month", Type: bigquery.StringFieldType}, {Name: "residual_pct", Type: bigquery.FloatFieldType}}
	var r teamYearRow
	if err := decodeBQRow(schema, []bigquery.Value{"2026-03-01", 7.13}, &r); err != nil {
		t.Fatal(err)
	}
	if r.ResidualPct == nil || *r.ResidualPct != 7.13 {
		t.Fatalf("residual = %v", r.ResidualPct)
	}
	var empty teamYearRow
	if err := decodeBQRow(schema, []bigquery.Value{"2026-03-01", nil}, &empty); err != nil || empty.ResidualPct != nil {
		t.Fatalf("null residual: err=%v value=%v", err, empty.ResidualPct)
	}
}

// A team where nobody paid beyond the included premium requests (teamkuhr in
// January–May 2026) still shows every month, with zero net.
func TestTeamYearShowsMonthsWithoutPayers(t *testing.T) {
	var rows []teamYearRow
	for _, month := range []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05"} {
		rows = append(rows, teamYearRow{Month: month + "-01", NetComplete: true, NoUsageNet: 0.5, SeatMonths: 2, Licence: 38, SeatsSnapshot: "2026-10-11", TeamMembersFrom: "2026-10-11"})
	}
	for _, m := range teamYearOverview("123", 2026, rows, time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)).Months {
		if m.Basis != basisPremium || m.NetUSD == nil || *m.NetUSD != 0 || *m.Users != 0 || m.TotalUSD == nil || *m.TotalUSD != 38.5 {
			t.Errorf("%s = %+v", m.Month, m)
		}
	}
}
