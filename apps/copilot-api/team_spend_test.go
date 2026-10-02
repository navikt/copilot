package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
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
	missing, err := client.GetTeamNetOverview(ctx, "2026-08")
	if err != nil || missing != nil {
		t.Fatalf("unfilled month = %+v, error %v", missing, err)
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
	if result.Teams[0].NetUSD+result.Teams[1].NetUSD <= result.EnterpriseNetUSD {
		t.Fatal("test data must demonstrate that overlapping team amounts exceed enterprise total")
	}
	if teamNetOverview("2026-09", nil) != nil {
		t.Fatal("a month without completion marker must not appear as complete")
	}
}
