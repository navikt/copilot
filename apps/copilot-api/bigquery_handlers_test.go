package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"

	"cloud.google.com/go/civil"

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
)

func ptr[T any](v T) *T { return &v }

// A month that has not happened yet must reach the page as null, not 0.
func TestHandleCohortRetentionNullForFutureMonths(t *testing.T) {
	h := newBigQueryHandlers(&mockBigQueryClient{cohortRetention: []bqstore.CohortRetention{
		{CohortMonth: "2026-08", CohortSize: 40, M1: ptr(int64(70))},
	}})
	rec := httptest.NewRecorder()
	h.handleCohortRetention(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/cohort-retention", nil))
	want := `[{"cohort_month":"2026-08","cohort_size":40,"m1":70,"m3":null,"m6":null}]`
	if got := rec.Body.String(); got != want+"\n" && got != want {
		t.Errorf("body: got %s, want %s", got, want)
	}
}

// mockBigQueryClient implements BigQueryQuerier for testing
type mockBigQueryClient struct {
	factAggregates       *bqstore.FactAggregates
	dailyMetrics         []bqstore.EnterpriseMetrics
	dailyMetricsErr      error
	adoptionSummary      *bqstore.AdoptionSummary
	adoptionSummaryErr   error
	teamAdoption         []bqstore.TeamAdoption
	teamAdoptionErr      error
	custDetails          []bqstore.CustomizationDetail
	custDetailsErr       error
	custUsage            []bqstore.CustomizationUsage
	custUsageErr         error
	langAdoption         []bqstore.LanguageAdoption
	langAdoptionErr      error
	stalenessFiles       []bqstore.StalenessFile
	stalenessErr         error
	teamGross            *bqstore.TeamGrossOverview
	teamGrossErr         error
	teamNet              *bqstore.TeamNetOverview
	anomalies            []bqstore.Anomaly
	spendBands           *bqstore.SpendBands
	teamNetErr           error
	grossByMonth         map[string]*bqstore.TeamGrossOverview
	netByMonth           map[string]*bqstore.TeamNetOverview
	teamYear             *bqstore.TeamYearOverview
	teamYearArgs         []any
	userTeams            []string
	userMetrics          *bqstore.UserMetricsSummary
	userMetricsErr       error
	monthlyTrends        []bqstore.MonthlyTrend
	monthlyTrendsErr     error
	monthlyBilling       []bqstore.MonthlyBillingUsage
	monthlyBillingErr    error
	billingModelDaily    []bqstore.BillingModelDailyCost
	billingModelErr      error
	billingForecast      *bqstore.BillingModelForecast
	billingForecastErr   error
	weeklyTrends         []bqstore.WeeklyTrend
	weeklyTrendsErr      error
	cohorts              []bqstore.AdoptionCohortDay
	cohortsErr           error
	usageDistribution    *bqstore.UsageDistribution
	usageDistErr         error
	repositoryUsage      []bqstore.RepositoryUsage
	repositoryUsageErr   error
	clientUsage          []bqstore.ClientUsage
	clientUsageErr       error
	editorUsage          []bqstore.EditorUsage
	editorUsageErr       error
	creditsPerUser       []bqstore.CreditsPerUserMonth
	creditsPerUserErr    error
	copilotPRs           []bqstore.CopilotPRMonth
	copilotPRsErr        error
	cohortRetention      []bqstore.CohortRetention
	cohortRetentionErr   error
	segments             *bqstore.UserSegments
	segmentsErr          error
	navPilotTelemetry    *bqstore.NavPilotTelemetry
	navPilotTelemetryErr error
}

func (m *mockBigQueryClient) GetNavPilotTelemetry(_ context.Context) (*bqstore.NavPilotTelemetry, error) {
	return m.navPilotTelemetry, m.navPilotTelemetryErr
}

func TestHandleNavPilotTelemetry(t *testing.T) {
	agg := bqstore.AggregateNavPilotTelemetry([]bqstore.MimirTelemetryRow{
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_reach", Metric: "nav_pilot_info", Operation: "7d", Devices: 217},
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_reach", Metric: "nav_pilot_info", Operation: "rtk_7d", Outcome: "true", Devices: 136},
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_reach", Metric: "nav_pilot_config_info", Client: "copilot", Devices: 160},
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_reach", Metric: "nav_pilot_config_info", Client: "opencode", Devices: 56},
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_reach", Metric: "nav_pilot_config_info", Client: "both", Devices: 13},
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_reach", Metric: "nav_pilot_installed_items", Source: "user", Devices: 162},
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_reach", Metric: "nav_pilot_installed_items", Source: "repo", Devices: 46},
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_reach", Metric: "nav_pilot_up_to_date", Operation: "cli", Outcome: "up_to_date", Devices: 158},
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_reach", Metric: "nav_pilot_up_to_date", Operation: "cli", Outcome: "stale", Devices: 41},
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_reach", Metric: "nav_pilot_local_dispatches_count", Operation: "7d", Devices: 6},
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_reach", Metric: "nav_pilot_command_duration_ms_count", Operation: "sync", Devices: 180},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_session_skill_loads_total", Skill: "klarsprak", Source: "nav", Trigger: "agent-invoked", Increase: 102},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_session_skill_loads_total", Skill: "klarsprak", Source: "nav", Trigger: "user-invoked", Increase: 9},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_subagent_selected_total", Agent: "nav-pilot", Increase: 206},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_session_tool_calls_total", Tool: "bash", Increase: 2880},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_session_turns", Client: "copilot", HistCount: 139, HistSum: 4726},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_session_turns", Client: "opencode", HistCount: 139, HistSum: 2557},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_local_dispatches", Client: "opencode", Outcome: "true", HistCount: 6, HistSum: 57},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_local_dispatches", Client: "opencode", Outcome: "false", HistCount: 111, HistSum: 0},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_local_dispatches", Client: "copilot", Outcome: "true", HistCount: 2, HistSum: 40},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_decide_result_total", Outcome: "decided", Increase: 18},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_session_tokens_total", Client: "copilot", TokenType: "cache_read", Increase: 202000000},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_session_tokens_total", Client: "copilot", TokenType: "input", Increase: 1350000},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_genai_session_tokens_total", Client: "opencode", TokenType: "cache_read", Increase: 180000000},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_session_hook_runs_total", Increase: 6224},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_hook_redact_total", Outcome: "secret", Increase: 14},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_hook_redact_total", Outcome: "fnr", Increase: 4},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_sync_updates_total", Increase: 1911},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_sync_conflicts_total", Increase: 41},
		{LatestDay: "2026-10-14", Family: "nav_pilot_session", Metric: "nav_pilot_launch_error_total", Increase: 102},
	})
	if agg.Devices7d != 217 || agg.CopilotDevices7d != 160 || agg.OpenCodeDevices7d != 56 || agg.BothClients7d != 13 || agg.RtkDevices7d != 136 || agg.CliUpToDate7d != 158 || agg.CliStale7d != 41 {
		t.Fatalf("unexpected reach: %+v", agg)
	}
	if agg.UserScopeDevices7d != 162 || agg.RepoScopeDevices7d != 46 {
		t.Fatalf("unexpected scope reach: %+v", agg)
	}
	if agg.AgentSkillLoads7d != 102 || agg.UserSkillLoads7d != 9 || len(agg.TopSkills7d) != 1 || agg.TopSkills7d[0].Loads != 111 {
		t.Fatalf("unexpected skill loads: %+v", agg)
	}
	if len(agg.TopAgents7d) != 1 || agg.TopAgents7d[0].Agent != "nav-pilot" || agg.TopAgents7d[0].Selections != 206 {
		t.Fatalf("unexpected agent selections: %+v", agg.TopAgents7d)
	}
	if len(agg.TopTools7d) != 1 || agg.TopTools7d[0].Tool != "bash" || agg.TopTools7d[0].Calls != 2880 {
		t.Fatalf("unexpected tool calls: %+v", agg.TopTools7d)
	}
	if agg.CopilotSessions7d != 139 || agg.CopilotTurns7d != 4726 || agg.OpenCodeSessions7d != 139 || agg.OpenCodeTurns7d != 2557 {
		t.Fatalf("unexpected session turns: %+v", agg)
	}
	if agg.Local7d.Devices7d != 6 || agg.Local7d.ObservedSessions != 119 || agg.Local7d.SessionsWithTraffic != 8 || agg.Local7d.SessionsWithoutTraffic != 111 || agg.Local7d.DispatchAttempts != 57 || agg.Local7d.DecideLocalCount != 18 {
		t.Fatalf("unexpected local telemetry: %+v", agg.Local7d)
	}
	if agg.CopilotTokens7d.CacheReadTokens != 202000000 || agg.CopilotTokens7d.InputTokens != 1350000 || agg.OpenCodeTokens7d.CacheReadTokens != 180000000 {
		t.Fatalf("unexpected session tokens: copilot=%+v opencode=%+v", agg.CopilotTokens7d, agg.OpenCodeTokens7d)
	}
	if agg.HookRuns7d != 6224 || agg.HookRedactions7d != 18 || agg.HookRedactSecrets7d != 14 || agg.HookRedactFnr7d != 4 || agg.SyncUpdates7d != 1911 || agg.SyncConflicts7d != 41 || agg.LaunchErrors7d != 102 {
		t.Fatalf("unexpected guardrail telemetry: %+v", agg)
	}

	legacyOnly := bqstore.AggregateNavPilotTelemetry([]bqstore.MimirTelemetryRow{
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_config", Metric: "nav_pilot_config_info", Client: "copilot", Devices: 90},
		{LatestDay: "2026-10-14", IsLatest: true, Family: "nav_pilot_config", Metric: "nav_pilot_config_info", Client: "opencode", Devices: 30},
	})
	if legacyOnly.CopilotDevices7d != 0 || legacyOnly.OpenCodeDevices7d != 0 {
		t.Fatalf("1d nav_pilot_config rows must not populate 7d reach fields: %+v", legacyOnly)
	}

	h := newBigQueryHandlers(&mockBigQueryClient{navPilotTelemetry: agg})
	rec := httptest.NewRecorder()
	h.handleNavPilotTelemetry(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/mimir/telemetry", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func (m *mockBigQueryClient) GetUserSegments(_ context.Context) (*bqstore.UserSegments, error) {
	return m.segments, m.segmentsErr
}

// The response carries display-ready shares and no field that could name a
// person or a team.
func TestHandleUserSegments(t *testing.T) {
	h := newBigQueryHandlers(&mockBigQueryClient{segments: &bqstore.UserSegments{
		Intensity: bqstore.BuildChart([]bqstore.SegmentRow{{Month: "2026-08", A: 60, B: 57, C: 3}}, bqstore.IntensityBands),
	}})
	rec := httptest.NewRecorder()
	h.handleUserSegments(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/segments", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"label":"Lett","shares":[50]`, `"label":"Middels","shares":[48]`, `"label":"Tung","shares":[2]`} {
		if !containsString(body, want) {
			t.Errorf("body lacks %s: %s", want, body)
		}
	}
	for _, banned := range []string{"user_id", "login", "slug", "team_id"} {
		if containsString(body, banned) {
			t.Errorf("body contains %q: %s", banned, body)
		}
	}

	h = newBigQueryHandlers(&mockBigQueryClient{segmentsErr: errors.New("bq")})
	rec = httptest.NewRecorder()
	h.handleUserSegments(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/segments", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("error status %d", rec.Code)
	}
}

func (m *mockBigQueryClient) GetCohortRetention(_ context.Context) ([]bqstore.CohortRetention, error) {
	return m.cohortRetention, m.cohortRetentionErr
}

func (m *mockBigQueryClient) GetDailyMetrics(_ context.Context, _ *int) ([]bqstore.EnterpriseMetrics, error) {
	return m.dailyMetrics, m.dailyMetricsErr
}

func (m *mockBigQueryClient) GetAdoptionSummary(_ context.Context) (*bqstore.AdoptionSummary, error) {
	return m.adoptionSummary, m.adoptionSummaryErr
}

func (m *mockBigQueryClient) GetTeamAdoption(_ context.Context) ([]bqstore.TeamAdoption, error) {
	return m.teamAdoption, m.teamAdoptionErr
}

func (m *mockBigQueryClient) GetCustomizationDetails(_ context.Context) ([]bqstore.CustomizationDetail, error) {
	return m.custDetails, m.custDetailsErr
}

func (m *mockBigQueryClient) GetCustomizationUsage(_ context.Context) ([]bqstore.CustomizationUsage, error) {
	return m.custUsage, m.custUsageErr
}

func (m *mockBigQueryClient) GetLanguageAdoption(_ context.Context) ([]bqstore.LanguageAdoption, error) {
	return m.langAdoption, m.langAdoptionErr
}

func (m *mockBigQueryClient) GetStalenessData(_ context.Context) ([]bqstore.StalenessFile, error) {
	return m.stalenessFiles, m.stalenessErr
}

func (m *mockBigQueryClient) GetTeamGrossOverview(_ context.Context, month string) (*bqstore.TeamGrossOverview, error) {
	if m.grossByMonth != nil {
		return m.grossByMonth[month], nil
	}
	return m.teamGross, m.teamGrossErr
}

func (m *mockBigQueryClient) GetTeamNetOverview(_ context.Context, month string) (*bqstore.TeamNetOverview, error) {
	if m.netByMonth != nil {
		return m.netByMonth[month], nil
	}
	return m.teamNet, m.teamNetErr
}

func (m *mockBigQueryClient) GetTeamYearOverview(_ context.Context, team string, year int) (*bqstore.TeamYearOverview, error) {
	m.teamYearArgs = []any{team, year}
	return m.teamYear, nil
}

func (m *mockBigQueryClient) GetAdminMonthlyOverview(_ context.Context, year int) (*bqstore.AdminMonthlyOverview, error) {
	return &bqstore.AdminMonthlyOverview{Year: year}, nil
}

func (m *mockBigQueryClient) GetUserTeams(_ context.Context, _ string) ([]string, error) {
	return m.userTeams, nil
}

func TestTeamGrossOverviewHandler(t *testing.T) {
	mock := &mockBigQueryClient{teamGross: &bqstore.TeamGrossOverview{
		Month: "2026-09", Teams: []bqstore.TeamSpend{{TeamID: "123", TeamSlug: "team-a", Users: 5, AmountUSD: 42, PerUserUSD: 8.4}},
		Comparison: bqstore.ComparisonIncomplete,
		Usage: map[string]bqstore.TeamUsageComposition{
			"123": {Providers: []string{"Anthropic", "OpenAI"}, Categories: []string{"Versatile", "Powerful", "Unclassified"}, Feature: "chat", Language: "go"},
		},
	}}
	h := newBigQueryHandlers(mock)
	for _, tc := range []struct {
		month string
		code  int
	}{
		{"2026-09", http.StatusOK},
		{"2026-13", http.StatusBadRequest},
	} {
		recorder := httptest.NewRecorder()
		h.handleTeamGrossOverview(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-gross?month="+tc.month, nil))
		if recorder.Code != tc.code {
			t.Errorf("month %q: status %d, want %d", tc.month, recorder.Code, tc.code)
		}
		if tc.code == http.StatusOK {
			var got bqstore.TeamGrossOverview
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(&got, mock.teamGross) {
				t.Errorf("unexpected team overview: %+v", got)
			}
		}
	}
}

func TestTeamNetOverviewHandler(t *testing.T) {
	h := newBigQueryHandlers(&mockBigQueryClient{teamNet: &bqstore.TeamNetOverview{
		Month: "2026-09", Teams: []bqstore.TeamSpend{bqstore.NewTeamSpend("123", "team-a", 5, 35)},
	}})
	for _, tc := range []struct {
		month string
		code  int
	}{
		{"2026-09", http.StatusOK},
		{"2026-13", http.StatusBadRequest},
	} {
		recorder := httptest.NewRecorder()
		h.handleTeamNetOverview(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-net?month="+tc.month, nil))
		if recorder.Code != tc.code {
			t.Errorf("month %q: status %d, want %d", tc.month, recorder.Code, tc.code)
		}
		if tc.code == http.StatusOK {
			var got bqstore.TeamNetOverview
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Teams) != 1 || got.Teams[0].AmountUSD != 35 || got.Teams[0].PerUserUSD != 7 {
				t.Errorf("unexpected team net overview: %+v", got)
			}
		}
	}
}

func TestHandleMyTeamsRequiresResolvedIdentity(t *testing.T) {
	h := newBigQueryHandlers(&mockBigQueryClient{userTeams: []string{"team-a"}})
	recorder := httptest.NewRecorder()
	h.handleMyTeams(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/my-teams", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/my-teams", nil)
	request = request.WithContext(context.WithValue(request.Context(), auth.ResolvedIdentityContextKey, &auth.ResolvedIdentity{GitHubUsername: "alice"}))
	h.handleMyTeams(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "[\"team-a\"]\n" {
		t.Fatalf("status %d, body %s", recorder.Code, recorder.Body.String())
	}
}

func (m *mockBigQueryClient) GetUserMetrics(_ context.Context, _ string, _ int) (*bqstore.UserMetricsSummary, error) {
	return m.userMetrics, m.userMetricsErr
}

func (m *mockBigQueryClient) GetMonthlyTrends(_ context.Context, _ int) ([]bqstore.MonthlyTrend, error) {
	return m.monthlyTrends, m.monthlyTrendsErr
}

func (m *mockBigQueryClient) GetMonthlyBillingUsage(_ context.Context, _ int) ([]bqstore.MonthlyBillingUsage, error) {
	return m.monthlyBilling, m.monthlyBillingErr
}

func (m *mockBigQueryClient) GetBillingModelDailyCosts(_ context.Context, _ string) ([]bqstore.BillingModelDailyCost, error) {
	return m.billingModelDaily, m.billingModelErr
}

func (m *mockBigQueryClient) GetBillingModelForecast(_ context.Context, _ string) (*bqstore.BillingModelForecast, error) {
	return m.billingForecast, m.billingForecastErr
}

func (m *mockBigQueryClient) GetUserWeeklyTrends(_ context.Context, _ string, _ int) ([]bqstore.WeeklyTrend, error) {
	return m.weeklyTrends, m.weeklyTrendsErr
}

func (m *mockBigQueryClient) GetUserDailyCredits(_ context.Context, _ string, _ int) ([]bqstore.DailyCredits, error) {
	return nil, nil
}

func (m *mockBigQueryClient) GetAdoptionCohorts(_ context.Context, _ int) ([]bqstore.AdoptionCohortDay, error) {
	return m.cohorts, m.cohortsErr
}

func (m *mockBigQueryClient) GetBillingMonthlyTrend(_ context.Context, _ int) ([]bqstore.BillingMonthlyTrend, error) {
	return nil, nil
}

func (m *mockBigQueryClient) GetBillingModelBreakdown(_ context.Context, _ int) ([]bqstore.BillingModelBreakdown, error) {
	return nil, nil
}

func (m *mockBigQueryClient) GetFactAggregates(_ context.Context) (*bqstore.FactAggregates, error) {
	return m.factAggregates, nil
}

func (m *mockBigQueryClient) GetDailySummary(_ context.Context) (*bqstore.DailySummary, error) {
	return nil, nil
}

func (m *mockBigQueryClient) GetUsageDistribution(_ context.Context, _ string, _ float64) (*bqstore.UsageDistribution, error) {
	return m.usageDistribution, m.usageDistErr
}

func (m *mockBigQueryClient) GetRepositoryUsage(_ context.Context) ([]bqstore.RepositoryUsage, error) {
	return m.repositoryUsage, m.repositoryUsageErr
}

func (m *mockBigQueryClient) GetClientUsage(_ context.Context) ([]bqstore.ClientUsage, error) {
	return m.clientUsage, m.clientUsageErr
}

func (m *mockBigQueryClient) GetCreditsPerUserMonthly(_ context.Context) ([]bqstore.CreditsPerUserMonth, error) {
	return m.creditsPerUser, m.creditsPerUserErr
}

func (m *mockBigQueryClient) GetCopilotPRsMonthly(_ context.Context) ([]bqstore.CopilotPRMonth, error) {
	return m.copilotPRs, m.copilotPRsErr
}

func TestHandleDailyMetrics(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		query            string
		mockMetrics      []bqstore.EnterpriseMetrics
		mockErr          error
		wantStatus       int
		wantBodyContains string
	}{
		{
			name:        "returns metrics on success",
			method:      http.MethodGet,
			mockMetrics: []bqstore.EnterpriseMetrics{{"day": "2024-01-01", "total_active_users": 10}},
			wantStatus:  http.StatusOK,
		},
		{
			name:        "returns empty list when no data",
			method:      http.MethodGet,
			mockMetrics: []bqstore.EnterpriseMetrics{},
			wantStatus:  http.StatusOK,
		},
		{
			name:             "rejects non-GET method",
			method:           http.MethodPost,
			wantStatus:       http.StatusMethodNotAllowed,
			wantBodyContains: "method_not_allowed",
		},
		{
			name:             "rejects invalid days param",
			method:           http.MethodGet,
			query:            "?days=abc",
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "invalid_parameter",
		},
		{
			name:             "rejects days=0",
			method:           http.MethodGet,
			query:            "?days=0",
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "invalid_parameter",
		},
		{
			name:             "rejects days=366",
			method:           http.MethodGet,
			query:            "?days=366",
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "invalid_parameter",
		},
		{
			name:             "returns 500 on backend error",
			method:           http.MethodGet,
			mockErr:          errors.New("bq connection failed"),
			wantStatus:       http.StatusInternalServerError,
			wantBodyContains: "internal_error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockBigQueryClient{dailyMetrics: tc.mockMetrics, dailyMetricsErr: tc.mockErr}
			h := newBigQueryHandlers(mock)

			req := httptest.NewRequest(tc.method, "/api/v1/copilot/usage/metrics"+tc.query, nil)
			rec := httptest.NewRecorder()
			h.handleDailyMetrics(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBodyContains != "" && !containsString(rec.Body.String(), tc.wantBodyContains) {
				t.Errorf("body %q does not contain %q", rec.Body.String(), tc.wantBodyContains)
			}
		})
	}
}

func TestHandleAdoptionSummary(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		summary    *bqstore.AdoptionSummary
		mockErr    error
		wantStatus int
	}{
		{
			name:       "returns summary when available",
			method:     http.MethodGet,
			summary:    &bqstore.AdoptionSummary{TotalRepos: 42},
			wantStatus: http.StatusOK,
		},
		{
			name:       "returns empty object when nil",
			method:     http.MethodGet,
			summary:    nil,
			wantStatus: http.StatusOK,
		},
		{
			name:       "rejects non-GET method",
			method:     http.MethodDelete,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "returns 500 on backend error",
			method:     http.MethodGet,
			mockErr:    errors.New("bq error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockBigQueryClient{adoptionSummary: tc.summary, adoptionSummaryErr: tc.mockErr}
			h := newBigQueryHandlers(mock)

			req := httptest.NewRequest(tc.method, "/api/v1/copilot/adoption/summary", nil)
			rec := httptest.NewRecorder()
			h.handleAdoptionSummary(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestHandleAdoptionStaleness(t *testing.T) {
	t.Run("computes summary from file list", func(t *testing.T) {
		files := []bqstore.StalenessFile{
			{Category: "instructions", FileName: "copilot-instructions.md", TotalRepos: 100, InSyncRepos: 80, OutOfSyncRepos: 20},
			{Category: "agents", FileName: "nav-pilot.agent.md", TotalRepos: 50, InSyncRepos: 50, OutOfSyncRepos: 0},
		}
		mock := &mockBigQueryClient{stalenessFiles: files}
		h := newBigQueryHandlers(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/staleness", nil)
		rec := httptest.NewRecorder()
		h.handleAdoptionStaleness(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", rec.Code)
		}

		var result bqstore.StalenessSummary
		if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if result.TotalFiles != 2 {
			t.Errorf("total_files: got %d, want 2", result.TotalFiles)
		}
		if result.TotalFileInstances != 150 {
			t.Errorf("total_file_instances: got %d, want 150", result.TotalFileInstances)
		}
		if result.InSyncCount != 130 {
			t.Errorf("in_sync_count: got %d, want 130", result.InSyncCount)
		}
		if result.OutOfSyncCount != 20 {
			t.Errorf("out_of_sync_count: got %d, want 20", result.OutOfSyncCount)
		}
		wantRate := 130.0 / 150.0
		if abs(result.SyncRate-wantRate) > 0.001 {
			t.Errorf("sync_rate: got %f, want %f", result.SyncRate, wantRate)
		}
		if len(result.Files) != 2 {
			t.Errorf("files length: got %d, want 2", len(result.Files))
		}
	})

	t.Run("handles empty file list with zero sync rate", func(t *testing.T) {
		mock := &mockBigQueryClient{stalenessFiles: []bqstore.StalenessFile{}}
		h := newBigQueryHandlers(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/staleness", nil)
		rec := httptest.NewRecorder()
		h.handleAdoptionStaleness(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", rec.Code)
		}

		var result bqstore.StalenessSummary
		if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if result.TotalFiles != 0 || result.TotalFileInstances != 0 || result.SyncRate != 0 {
			t.Errorf("expected zero-valued summary, got %+v", result)
		}
	})

	t.Run("rejects non-GET method", func(t *testing.T) {
		h := newBigQueryHandlers(&mockBigQueryClient{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/copilot/adoption/staleness", nil)
		rec := httptest.NewRecorder()
		h.handleAdoptionStaleness(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status: got %d, want 405", rec.Code)
		}
	})

	t.Run("returns 500 on backend error", func(t *testing.T) {
		mock := &mockBigQueryClient{stalenessErr: errors.New("bq down")}
		h := newBigQueryHandlers(mock)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/staleness", nil)
		rec := httptest.NewRecorder()
		h.handleAdoptionStaleness(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status: got %d, want 500", rec.Code)
		}
	})
}

func TestHandleTeamAdoption(t *testing.T) {
	t.Run("returns team adoption data", func(t *testing.T) {
		mock := &mockBigQueryClient{teamAdoption: []bqstore.TeamAdoption{{ScanDate: civil.Date{Year: 2026, Month: 6, Day: 2}, TeamSlug: "team-a", TeamName: "Team A", TeamRepos: 5, ActiveRepos: 5}}}
		h := newBigQueryHandlers(mock)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/teams", nil)
		rec := httptest.NewRecorder()
		h.handleTeamAdoption(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status: got %d, want 200", rec.Code)
		}

		var result TeamAdoptionOverview
		if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(result.Teams) != 1 || result.Teams[0].TeamSlug != "team-a" {
			t.Errorf("unexpected result: %+v", result)
		}
	})

	t.Run("returns 500 on error", func(t *testing.T) {
		mock := &mockBigQueryClient{teamAdoptionErr: errors.New("err")}
		h := newBigQueryHandlers(mock)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/teams", nil)
		rec := httptest.NewRecorder()
		h.handleTeamAdoption(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status: got %d, want 500", rec.Code)
		}
	})
}

func TestHandleNewStatsEndpoints(t *testing.T) {
	validDate := civil.Date{Year: 2026, Month: 6, Day: 2}
	tests := []struct {
		name       string
		mock       *mockBigQueryClient
		req        *http.Request
		handle     func(*BigQueryHandlers, http.ResponseWriter, *http.Request)
		wantStatus int
	}{
		{
			name:       "user metrics success",
			mock:       &mockBigQueryClient{userMetrics: &bqstore.UserMetricsSummary{UserLogin: "octocat", DaysInPeriod: 7}},
			req:        requestWithUsername("/api/v1/copilot/usage/user/octocat", "octocat"),
			handle:     (*BigQueryHandlers).handleUserMetrics,
			wantStatus: http.StatusOK,
		},
		{
			name:       "user metrics not found",
			mock:       &mockBigQueryClient{},
			req:        requestWithUsername("/api/v1/copilot/usage/user/octocat", "octocat"),
			handle:     (*BigQueryHandlers).handleUserMetrics,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "user metrics error",
			mock:       &mockBigQueryClient{userMetricsErr: errors.New("bq")},
			req:        requestWithUsername("/api/v1/copilot/usage/user/octocat", "octocat"),
			handle:     (*BigQueryHandlers).handleUserMetrics,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "user metrics invalid username",
			mock:       &mockBigQueryClient{},
			req:        requestWithUsername("/api/v1/copilot/usage/user/bad", "bad/user"),
			handle:     (*BigQueryHandlers).handleUserMetrics,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "monthly trends success",
			mock:       &mockBigQueryClient{monthlyTrends: []bqstore.MonthlyTrend{{Month: "2026-06", UniqueUsers: 10}}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/trends", nil),
			handle:     (*BigQueryHandlers).handleMonthlyTrends,
			wantStatus: http.StatusOK,
		},
		{
			name:       "monthly trends error",
			mock:       &mockBigQueryClient{monthlyTrendsErr: errors.New("bq")},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/trends", nil),
			handle:     (*BigQueryHandlers).handleMonthlyTrends,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "monthly billing success",
			mock:       &mockBigQueryClient{monthlyBilling: []bqstore.MonthlyBillingUsage{{Month: "2026-06", Model: "gpt", SKU: "premium", GrossRequests: 2}}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/billing/monthly", nil),
			handle:     (*BigQueryHandlers).handleMonthlyBillingUsage,
			wantStatus: http.StatusOK,
		},
		{
			name:       "monthly billing error",
			mock:       &mockBigQueryClient{monthlyBillingErr: errors.New("bq")},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/billing/monthly", nil),
			handle:     (*BigQueryHandlers).handleMonthlyBillingUsage,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "billing model daily success",
			mock:       &mockBigQueryClient{billingModelDaily: []bqstore.BillingModelDailyCost{{Day: "2026-06-01", Model: "gpt-5", NetAmount: 10.2}}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/billing/model-daily?month=2026-06", nil),
			handle:     (*BigQueryHandlers).handleBillingModelDaily,
			wantStatus: http.StatusOK,
		},
		{
			name:       "billing model daily invalid month",
			mock:       &mockBigQueryClient{},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/billing/model-daily?month=2026/06", nil),
			handle:     (*BigQueryHandlers).handleBillingModelDaily,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "billing model daily invalid calendar month",
			mock:       &mockBigQueryClient{},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/billing/model-daily?month=2026-13", nil),
			handle:     (*BigQueryHandlers).handleBillingModelDaily,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "billing model forecast success",
			mock:       &mockBigQueryClient{billingForecast: &bqstore.BillingModelForecast{Month: "2026-06", ProjectedEOMNetAmount: 120}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/billing/model-forecast?month=2026-06", nil),
			handle:     (*BigQueryHandlers).handleBillingModelForecast,
			wantStatus: http.StatusOK,
		},
		{
			name:       "billing model forecast error",
			mock:       &mockBigQueryClient{billingForecastErr: errors.New("bq")},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/billing/model-forecast?month=2026-06", nil),
			handle:     (*BigQueryHandlers).handleBillingModelForecast,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "billing model forecast invalid calendar month",
			mock:       &mockBigQueryClient{},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/billing/model-forecast?month=2026-13", nil),
			handle:     (*BigQueryHandlers).handleBillingModelForecast,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "weekly trends success",
			mock:       &mockBigQueryClient{weeklyTrends: []bqstore.WeeklyTrend{{Week: "2026-W23", Interactions: 3}}},
			req:        requestWithUsername("/api/v1/copilot/usage/user/octocat/weekly", "octocat"),
			handle:     (*BigQueryHandlers).handleUserWeeklyTrends,
			wantStatus: http.StatusOK,
		},
		{
			name:       "weekly trends error",
			mock:       &mockBigQueryClient{weeklyTrendsErr: errors.New("bq")},
			req:        requestWithUsername("/api/v1/copilot/usage/user/octocat/weekly", "octocat"),
			handle:     (*BigQueryHandlers).handleUserWeeklyTrends,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "cohorts success",
			mock:       &mockBigQueryClient{cohorts: []bqstore.AdoptionCohortDay{{Day: validDate, Phase: 1, UserCount: 4}}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/cohorts", nil),
			handle:     (*BigQueryHandlers).handleAdoptionCohorts,
			wantStatus: http.StatusOK,
		},
		{
			name:       "cohorts error",
			mock:       &mockBigQueryClient{cohortsErr: errors.New("bq")},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/cohorts", nil),
			handle:     (*BigQueryHandlers).handleAdoptionCohorts,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "repository usage success",
			mock:       &mockBigQueryClient{repositoryUsage: []bqstore.RepositoryUsage{{RepoName: "repo-a", RepoVisibility: "INTERNAL", PRCreatedByCopilot: 3}}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/repositories", nil),
			handle:     (*BigQueryHandlers).handleRepositoryUsage,
			wantStatus: http.StatusOK,
		},
		{
			name:       "repository usage error",
			mock:       &mockBigQueryClient{repositoryUsageErr: errors.New("bq")},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/repositories", nil),
			handle:     (*BigQueryHandlers).handleRepositoryUsage,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "client usage success",
			mock:       &mockBigQueryClient{clientUsage: []bqstore.ClientUsage{{Client: "some_new_client", Requests: 12, Sessions: 3}}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/clients", nil),
			handle:     (*BigQueryHandlers).handleClientUsage,
			wantStatus: http.StatusOK,
		},
		{
			name:       "client usage error",
			mock:       &mockBigQueryClient{clientUsageErr: errors.New("bq")},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/clients", nil),
			handle:     (*BigQueryHandlers).handleClientUsage,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "credits per user success",
			mock:       &mockBigQueryClient{creditsPerUser: []bqstore.CreditsPerUserMonth{{Month: "2026-06", Median: 40, Mean: 55, ActiveUsers: 600}}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/credits-per-user", nil),
			handle:     (*BigQueryHandlers).handleCreditsPerUserMonthly,
			wantStatus: http.StatusOK,
		},
		{
			name:       "credits per user error",
			mock:       &mockBigQueryClient{creditsPerUserErr: errors.New("bq")},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/credits-per-user", nil),
			handle:     (*BigQueryHandlers).handleCreditsPerUserMonthly,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "copilot prs success",
			mock:       &mockBigQueryClient{copilotPRs: []bqstore.CopilotPRMonth{{Month: "2026-08", CreatedByCopilot: 12, ReviewedByCopilot: 300, Days: 31}}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/copilot-prs", nil),
			handle:     (*BigQueryHandlers).handleCopilotPRsMonthly,
			wantStatus: http.StatusOK,
		},
		{
			name:       "copilot prs error",
			mock:       &mockBigQueryClient{copilotPRsErr: errors.New("bq")},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/copilot-prs", nil),
			handle:     (*BigQueryHandlers).handleCopilotPRsMonthly,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "cohort retention success",
			mock:       &mockBigQueryClient{cohortRetention: []bqstore.CohortRetention{{CohortMonth: "2026-08", CohortSize: 40, M1: ptr(int64(70))}}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/cohort-retention", nil),
			handle:     (*BigQueryHandlers).handleCohortRetention,
			wantStatus: http.StatusOK,
		},
		{
			name:       "cohort retention error",
			mock:       &mockBigQueryClient{cohortRetentionErr: errors.New("bq")},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/cohort-retention", nil),
			handle:     (*BigQueryHandlers).handleCohortRetention,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newBigQueryHandlers(tc.mock)
			rec := httptest.NewRecorder()
			tc.handle(h, rec, tc.req)
			if rec.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d; body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func requestWithUsername(path, username string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.SetPathValue("username", username)
	// Simulate IdentityMiddleware having already resolved the caller's
	// identity (see identity_middleware.go) — handleUserMetrics and friends
	// call requireOwnership, which reads this from context.
	ctx := context.WithValue(req.Context(), auth.ResolvedIdentityContextKey, &auth.ResolvedIdentity{GitHubUsername: username, Source: "test"})
	return req.WithContext(ctx)
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// containsString reports whether sub is in s
func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

func (m *mockBigQueryClient) GetCostDays(_ context.Context, _ string) ([]bqstore.CostDay, error) {
	return nil, nil
}

func (m *mockBigQueryClient) GetCostUserDays(_ context.Context, _ string) ([]bqstore.UserDay, error) {
	return nil, nil
}

func (m *mockBigQueryClient) GetCostAnomalies(_ context.Context) ([]bqstore.Anomaly, error) {
	return m.anomalies, nil
}

func (m *mockBigQueryClient) GetLimitSnapshots(_ context.Context) ([]bqstore.LimitChange, error) {
	return nil, nil
}

func (m *mockBigQueryClient) GetSpendBands(_ context.Context) (*bqstore.SpendBands, error) {
	return m.spendBands, nil
}

func (m *mockBigQueryClient) GetActiveUsersMonthly(_ context.Context) ([]bqstore.ActiveUsersMonth, error) {
	return []bqstore.ActiveUsersMonth{{Month: "2026-09", ActiveUsers: 719}}, nil
}

func (m *mockBigQueryClient) GetModelFamilyUsers(_ context.Context) ([]bqstore.ModelFamilyUsers, error) {
	return []bqstore.ModelFamilyUsers{}, nil
}

func (m *mockBigQueryClient) GetVolumeMonthly(_ context.Context, _ int) ([]bqstore.VolumeMonth, error) {
	return []bqstore.VolumeMonth{{Month: "2026-09", AICredits: 100}}, nil
}

func (m *mockBigQueryClient) GetBudgetDays(_ context.Context) ([]bqstore.BudgetDay, error) {
	return []bqstore.BudgetDay{}, nil
}

func (m *mockBigQueryClient) GetHighConsumers(_ context.Context) ([]bqstore.HighConsumerMonth, error) {
	return []bqstore.HighConsumerMonth{}, nil
}
