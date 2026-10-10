package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestTeamSpendBigQuery(t *testing.T) {
	if os.Getenv("VERIFY_TEAM_SPEND_BIGQUERY") != "true" {
		t.Skip("set VERIFY_TEAM_SPEND_BIGQUERY=true for the read-only September dev check")
	}
	client, err := newBigQueryClient(&Config{
		GCPProjectID: "copilot-dev-e17a", CopilotMetricsDataset: "copilot_metrics",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	gross, err := client.GetTeamGrossOverview(ctx, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if len(gross.Teams) != 122 || gross.SmallTeams != 55 || gross.DaysWithUsage != 30 {
		t.Fatalf("unexpected September gross coverage: %+v", gross)
	}
	if math.Abs(gross.DistinctGrossUSD-79489.149132719) > 0.01 {
		t.Fatalf("gross total = %f", gross.DistinctGrossUSD)
	}
	if len(gross.Usage) == 0 {
		t.Fatal("no team usage composition")
	}
	providers, categories, features, languages, unknownProviders, unknownCategories := 0, 0, 0, 0, 0, 0
	for id, usage := range gross.Usage {
		if len(usage.Providers) > 0 {
			providers++
		}
		if len(usage.Categories) > 0 {
			categories++
		}
		if usage.Feature != "" {
			features++
		}
		if usage.Language != "" {
			languages++
		}
		visible := false
		for _, team := range gross.Teams {
			if team.TeamID == id {
				visible = true
			}
		}
		if !visible || len(usage.Categories) > 4 {
			t.Fatal("composition escaped team suppression or rank limits")
		}
		for _, provider := range usage.Providers {
			if provider == unclassified {
				unknownProviders++
			}
		}
		for _, category := range usage.Categories {
			switch category {
			case unclassified:
				unknownCategories++
			case "Lightweight", "Versatile", "Powerful":
			default:
				t.Fatalf("unexpected category %q", category)
			}
		}
	}
	if len(gross.Usage) != 122 || providers != 122 || categories != 122 || features != 71 || languages != 88 {
		t.Fatalf("unexpected composition coverage: %d/%d/%d/%d", providers, categories, features, languages)
	}
	t.Logf("September composition: %d providers, %d categories, %d features, %d languages; Unclassified in %d provider summaries and %d category summaries", providers, categories, features, languages, unknownProviders, unknownCategories)
	grossRecorder := httptest.NewRecorder()
	newBigQueryHandlers(newCachedBigQueryClient(client, time.Minute)).handleTeamGrossOverview(grossRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-gross?month=2026-09", nil).WithContext(ctx))
	var httpGross TeamGrossOverview
	if err := json.Unmarshal(grossRecorder.Body.Bytes(), &httpGross); err != nil || grossRecorder.Code != http.StatusOK || !reflect.DeepEqual(httpGross.Usage, gross.Usage) {
		t.Fatalf("HTTP usage lost: %d %v", grossRecorder.Code, err)
	}
	net, err := client.GetTeamNetOverview(ctx, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if net == nil || len(net.Teams) != 119 || net.SmallTeams != 58 {
		t.Fatalf("unexpected September net coverage: %+v", net)
	}
	if math.Abs(net.KnownNetUSD-64388.330788755) > 0.01 || math.Abs(net.ResidualNetUSD-170.51492121) > 0.01 {
		t.Fatalf("net reconciliation: known %f, residual %f", net.KnownNetUSD, net.ResidualNetUSD)
	}
	for _, team := range net.Teams {
		if team.Users < minTeamContributors {
			t.Fatalf("returned suppressed team %q", team.TeamID)
		}
	}
	missing, err := client.GetTeamNetOverview(ctx, "2026-05")
	if err != nil || missing != nil {
		t.Fatalf("unfilled month = %+v, error %v", missing, err)
	}
	if os.Getenv("VERIFY_AUGUST_TEAM_SPEND") == "true" {
		august, err := client.GetTeamNetOverview(ctx, "2026-08")
		if err != nil {
			t.Fatal(err)
		}
		if august == nil || len(august.Teams) == 0 || math.Abs(august.KnownNetUSD-32163.250469783994) > 0.02 {
			t.Fatalf("August net reconciliation failed: %#v", august)
		}
		if math.Abs(august.EnterpriseNetUSD-august.KnownNetUSD-august.ResidualNetUSD) > 0.02 {
			t.Fatal("August residual does not reconcile")
		}
		t.Logf("August/September net comparison available: %d/%d visible teams", len(august.Teams), len(net.Teams))
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-net?month=2026-09", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	newBigQueryHandlers(newCachedBigQueryClient(client, time.Minute)).handleTeamNetOverview(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("net HTTP handler status %d", recorder.Code)
	}
	var decoded TeamNetOverview
	if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil || len(decoded.Teams) != 119 {
		t.Fatalf("net HTTP response decoding: %v, %d teams", err, len(decoded.Teams))
	}
}

func TestTeamGrossOverviewOptionalComposition(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	rows := []teamGrossRow{{
		TeamID: "visible", TeamSlug: "team-a", Users: 5, Gross: 42,
		SmallTeams: 2, SmallUsers: 3, SmallGross: 10, DistinctGross: 50,
		UnassignedGross: 8, LastDay: "2026-09-30", DaysWithUsage: 30,
	}}
	want := &TeamGrossOverview{
		Month: "2026-09", Teams: []TeamSpend{teamSpend("visible", "team-a", 5, 42)},
		SmallTeams: 2, DistinctGrossUSD: 50, LastUsageDay: "2026-09-30", DaysWithUsage: 30,
	}
	visibleUsage := TeamUsageComposition{Providers: []string{"OpenAI"}, Categories: []string{"Powerful"}, Feature: "chat", Language: "go"}
	for _, tc := range []struct {
		name  string
		usage map[string]TeamUsageComposition
		err   error
	}{
		{"failure", map[string]TeamUsageComposition{"visible": visibleUsage}, errors.New("sensitive upstream details")},
		{"nil", nil, nil},
		{"empty", map[string]TeamUsageComposition{}, nil},
		{"pruned", map[string]TeamUsageComposition{"visible": visibleUsage, "hidden": visibleUsage, "nav-it-github-users": visibleUsage}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			got := teamGrossOverview("2026-09", rows, tc.usage, tc.err)
			expected := *want
			if tc.err == nil {
				expected.Usage = map[string]TeamUsageComposition{"visible": {Providers: []string{}, Categories: []string{}}}
				if tc.name == "pruned" {
					expected.Usage = map[string]TeamUsageComposition{"visible": visibleUsage}
				}
			}
			if !reflect.DeepEqual(got, &expected) {
				t.Fatalf("overview = %+v, want %+v", got, &expected)
			}
			if tc.err != nil {
				var entry map[string]any
				if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
					t.Fatal(err)
				}
				if len(entry) != 3 || entry["msg"] != "Team usage composition unavailable" || entry["level"] != "WARN" {
					t.Fatalf("unsafe or unexpected log: %s", logs.String())
				}
			} else if logs.Len() != 0 {
				t.Fatalf("unexpected log: %s", logs.String())
			}
			recorder := httptest.NewRecorder()
			newBigQueryHandlers(&mockBigQueryClient{teamGross: got}).handleTeamGrossOverview(recorder,
				httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-gross?month=2026-09", nil))
			httpExpected := expected
			httpExpected.DaysWithUsage, httpExpected.DistinctGrossUSD, httpExpected.Comparison = 0, 0, comparisonIncomplete
			httpExpected.Teams = []TeamSpend{{TeamID: "visible", TeamSlug: "team-a", Users: 5, AmountUSD: 42, PerUserUSD: 8.4}}
			var decoded TeamGrossOverview
			if err := json.Unmarshal(recorder.Body.Bytes(), &decoded); err != nil || recorder.Code != http.StatusOK || !reflect.DeepEqual(&decoded, &httpExpected) {
				t.Fatalf("HTTP overview = %+v, status %d, error %v", decoded, recorder.Code, err)
			}
			if expected.Usage == nil {
				var body map[string]json.RawMessage
				if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if string(body["usage"]) != "null" {
					t.Fatalf("unavailable usage = %s, want null", body["usage"])
				}
			}
		})
	}
}

func TestTeamNetOverviewDoesNotAddOverlappingTeams(t *testing.T) {
	result := teamNetOverview("2026-09", []teamNetRow{
		{TeamID: "a", TeamSlug: "a", Users: 5, Net: 100, KnownNet: 100, EnterpriseNet: 102,
			SmallTeams: 2, SmallUsers: 3, SmallNet: 40, UnassignedNet: 10},
		{TeamID: "b", TeamSlug: "b", Users: 5, Net: 90, KnownNet: 100, EnterpriseNet: 102,
			SmallTeams: 2, SmallUsers: 3, SmallNet: 40, UnassignedNet: 10},
	})
	if result == nil || len(result.Teams) != 2 || result.KnownNetUSD != 100 || result.ResidualNetUSD != 2 {
		t.Fatalf("unexpected overlapping overview: %+v", result)
	}
	if result.Teams[0].AmountUSD+result.Teams[1].AmountUSD <= result.EnterpriseNetUSD {
		t.Fatal("test data must demonstrate that overlapping team amounts exceed enterprise total")
	}
	if teamNetOverview("2026-09", nil) != nil {
		t.Fatal("a month without completion marker must not appear as complete")
	}
}

func TestTeamOverviewsOmitTeamsBelowFive(t *testing.T) {
	gross := teamGrossOverview("2026-09", []teamGrossRow{
		{TeamID: "big", TeamSlug: "big", Users: 5, Gross: 10.006, SmallTeams: 1, SmallUsers: 4, SmallGross: 9},
		{TeamID: "small", TeamSlug: "small", Users: 4, Gross: 9, SmallTeams: 1, SmallUsers: 4, SmallGross: 9},
	}, nil, nil)
	net := teamNetOverview("2026-09", []teamNetRow{
		{TeamID: "big", TeamSlug: "big", Users: 6, Net: 10},
		{TeamID: "small", TeamSlug: "small", Users: 1, Net: 3},
	})
	for _, teams := range [][]TeamSpend{gross.Teams, net.Teams} {
		if len(teams) != 1 || teams[0].TeamID != "big" {
			t.Fatalf("teams = %+v, want only big", teams)
		}
	}
	if gross.Teams[0].AmountUSD != 10.01 || gross.Teams[0].PerUserUSD != 2 || net.Teams[0].PerUserUSD != 1.67 {
		t.Fatalf("rounding: gross %+v net %+v", gross.Teams[0], net.Teams[0])
	}
	// Complementary suppression: no hidden-team sum or total reaches the client.
	body, _ := json.Marshal(gross)
	var fields map[string]any
	_ = json.Unmarshal(body, &fields)
	for key := range fields {
		switch key {
		case "usage", "month", "teams", "small_teams", "last_usage_day", "comparison":
		default:
			t.Errorf("gross overview leaks %q", key)
		}
	}
	body, _ = json.Marshal(net)
	fields = nil
	_ = json.Unmarshal(body, &fields)
	for key := range fields {
		switch key {
		case "month", "teams", "small_teams", "loaded_at", "comparison":
		default:
			t.Errorf("net overview leaks %q", key)
		}
	}
}

func TestWithChanges(t *testing.T) {
	cur := []TeamSpend{teamSpend("a", "a", 5, 120), teamSpend("b", "b", 5, 105), teamSpend("c", "c", 5, 50), teamSpend("d", "d", 5, 10)}
	prev := []TeamSpend{teamSpend("a", "a", 5, 100), teamSpend("b", "b", 5, 100), teamSpend("c", "c", 4, 1)}
	got := withChanges(cur, prev)
	if got[0].ChangeUSD == nil || *got[0].ChangeUSD != 20 || !got[0].Highlight {
		t.Errorf("a = %+v, want +20 highlighted", got[0])
	}
	if got[1].ChangeUSD == nil || *got[1].ChangeUSD != 5 || got[1].Highlight {
		t.Errorf("b = %+v, want +5 not highlighted", got[1])
	}
	if got[2].ChangeUSD != nil || got[3].ChangeUSD != nil {
		t.Errorf("c/d must have no change: %+v %+v", got[2], got[3])
	}
	// Change uses raw amounts: 10.004 - 0.004 is exactly 10, not 10.00 - 0.00 rounded twice.
	if c := withChanges([]TeamSpend{teamSpend("e", "e", 5, 10.006)}, []TeamSpend{teamSpend("e", "e", 5, 0.004)}); *c[0].ChangeUSD != 10 {
		t.Errorf("e change = %v, want 10", *c[0].ChangeUSD)
	}
	if cur[0].ChangeUSD != nil {
		t.Error("withChanges mutated its input")
	}
}

func TestTeamGrossHandlerComparesFullMonths(t *testing.T) {
	sep := &TeamGrossOverview{Month: "2026-09", Teams: []TeamSpend{teamSpend("a", "a", 5, 130)}, DaysWithUsage: 30, LastUsageDay: "2026-09-30"}
	aug := &TeamGrossOverview{Month: "2026-08", Teams: []TeamSpend{teamSpend("a", "a", 5, 100)}, DaysWithUsage: 31, LastUsageDay: "2026-08-31"}
	h := newBigQueryHandlers(&mockBigQueryClient{
		grossByMonth: map[string]*TeamGrossOverview{"2026-09": sep, "2026-08": aug},
		netByMonth:   map[string]*TeamNetOverview{"2026-09": {Month: "2026-09", Teams: []TeamSpend{teamSpend("a", "a", 5, 90)}}},
	})
	rec := httptest.NewRecorder()
	h.handleTeamGrossOverview(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-gross?month=2026-09", nil))
	var got TeamGrossOverview
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Comparison != comparisonOK || got.Teams[0].ChangeUSD == nil || *got.Teams[0].ChangeUSD != 30 {
		t.Fatalf("gross = %+v", got)
	}
	rec = httptest.NewRecorder()
	h.handleTeamNetOverview(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-net?month=2026-09", nil))
	var net TeamNetOverview
	if err := json.Unmarshal(rec.Body.Bytes(), &net); err != nil {
		t.Fatal(err)
	}
	if net.Comparison != comparisonPreviousNetMiss || net.Teams[0].ChangeUSD != nil {
		t.Fatalf("net = %+v", net)
	}
}
