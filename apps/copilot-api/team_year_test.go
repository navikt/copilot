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

func TestTeamYearOverviewBasisAndSuppression(t *testing.T) {
	cov := teamYearRow{MembershipFrom: "2026-05-06", GrossFrom: "2026-06-15", LastUsageDay: "2026-10-09"}
	row := func(month string, complete bool, grossUsers, netUsers int64) teamYearRow {
		r := cov
		r.Month, r.NetComplete, r.GrossUsers, r.Gross, r.NetUsers, r.Net, r.NoUsageNet = month+"-01", complete, grossUsers, 100.004, netUsers, 80.006, 1.5
		return r
	}
	rows := []teamYearRow{
		row("2026-04", false, 0, 0), // before membership history
		row("2026-05", true, 9, 9),  // net complete but before per-user gross
		row("2026-06", true, 6, 4),  // net hidden, gross shown
		row("2026-07", false, 4, 0), // gross, under five
		row("2026-08", true, 6, 5),  // net
		row("2026-09", true, 4, 5),  // net, gross hidden
		row("2026-10", true, 7, 7),  // current month never net
		row("2026-11", false, 0, 0), // future
	}
	got := teamYearOverview("123", 2026, rows, time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC))
	want := []struct {
		month, basis string
		hidden       bool
		net, gross   bool
	}{
		{"2026-04", basisNone, false, false, false},
		{"2026-05", basisNone, false, false, false},
		{"2026-06", basisNet, true, false, true},
		{"2026-07", basisGross, true, false, false},
		{"2026-08", basisNet, false, true, true},
		{"2026-09", basisNet, false, true, false},
		{"2026-10", basisGross, false, false, true},
	}
	if len(got.Months) != len(want) {
		t.Fatalf("months = %+v", got.Months)
	}
	for i, w := range want {
		m := got.Months[i]
		if m.Month != w.month || m.Basis != w.basis || m.Hidden != w.hidden || (m.NetUSD != nil) != w.net || (m.GrossUSD != nil) != w.gross {
			t.Errorf("%s = %+v, want %+v", w.month, m, w)
		}
		if (m.Hidden || m.Basis == basisNone) && (m.Users != nil || m.NoUsageNetUSD != nil) {
			t.Errorf("%s leaks counts: %+v", w.month, m)
		}
	}
	if aug := got.Months[4]; *aug.NetUSD != 80.01 || *aug.GrossUSD != 100 || *aug.Users != 5 || *aug.NoUsageNetUSD != 1.5 {
		t.Errorf("august = %+v", aug)
	}
	if strings.Join(got.Coverage.NetMonths, ",") != "2026-06,2026-08,2026-09" || got.Coverage.GrossFrom != "2026-06-15" {
		t.Errorf("coverage = %+v", got.Coverage)
	}
}

func TestTeamYearQueryWithoutBillingTables(t *testing.T) {
	bqc, err := bigquery.NewClient(context.Background(), "p", option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	client := &BigQueryClient{client: bqc, projectID: "p", metricsDataset: "d"}
	for _, net := range []bool{true, false} {
		q := client.teamYearQuery("123", 2026, net)
		billing := strings.Contains(q.Q, "billing_user_monthly")
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
				if m.GrossUSD != nil || m.NetUSD != nil {
					t.Errorf("%s %s shown but hidden by the month endpoints: %+v", id, m.Month, m)
				}
				continue
			}
			if (want.grossUsers > 0) != (m.GrossUSD != nil) || (m.GrossUSD != nil && *m.GrossUSD != want.gross) {
				t.Errorf("%s %s gross %v, team-gross %v", id, m.Month, m.GrossUSD, want.gross)
			}
			if m.Basis == basisNet && ((want.netUsers > 0) != (m.NetUSD != nil) || (m.NetUSD != nil && (*m.NetUSD != want.net || *m.Users != want.netUsers))) {
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
