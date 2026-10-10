package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"cloud.google.com/go/civil"
)

func ptr[T any](v T) *T { return &v }

// A month that has not happened yet must reach the page as null, not 0.
func TestHandleCohortRetentionNullForFutureMonths(t *testing.T) {
	h := newBigQueryHandlers(&mockBigQueryClient{cohortRetention: []CohortRetention{
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
	dailyMetrics       []EnterpriseMetrics
	dailyMetricsErr    error
	adoptionSummary    *AdoptionSummary
	adoptionSummaryErr error
	teamAdoption       []TeamAdoption
	teamAdoptionErr    error
	custDetails        []CustomizationDetail
	custDetailsErr     error
	custUsage          []CustomizationUsage
	custUsageErr       error
	langAdoption       []LanguageAdoption
	langAdoptionErr    error
	stalenessFiles     []StalenessFile
	stalenessErr       error
	teamGross          *TeamGrossOverview
	teamGrossErr       error
	teamNet            *TeamNetOverview
	teamNetErr         error
	grossByMonth       map[string]*TeamGrossOverview
	netByMonth         map[string]*TeamNetOverview
	userTeams          []string
	userMetrics        *UserMetricsSummary
	userMetricsErr     error
	monthlyTrends      []MonthlyTrend
	monthlyTrendsErr   error
	monthlyBilling     []MonthlyBillingUsage
	monthlyBillingErr  error
	billingModelDaily  []BillingModelDailyCost
	billingModelErr    error
	billingForecast    *BillingModelForecast
	billingForecastErr error
	weeklyTrends       []WeeklyTrend
	weeklyTrendsErr    error
	cohorts            []AdoptionCohortDay
	cohortsErr         error
	usageDistribution  *UsageDistribution
	usageDistErr       error
	repositoryUsage    []RepositoryUsage
	repositoryUsageErr error
	creditsPerUser     []CreditsPerUserMonth
	creditsPerUserErr  error
	copilotPRs         []CopilotPRMonth
	copilotPRsErr      error
	cohortRetention    []CohortRetention
	cohortRetentionErr error
	segments           *UserSegments
	segmentsErr        error
}

func (m *mockBigQueryClient) GetUserSegments(_ context.Context) (*UserSegments, error) {
	return m.segments, m.segmentsErr
}

// The response carries display-ready shares and no field that could name a
// person or a team.
func TestHandleUserSegments(t *testing.T) {
	h := newBigQueryHandlers(&mockBigQueryClient{segments: &UserSegments{
		Intensity: buildChart([]segmentRow{{Month: "2026-08", A: 60, B: 57, C: 3}}, intensityBands),
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

func (m *mockBigQueryClient) GetCohortRetention(_ context.Context) ([]CohortRetention, error) {
	return m.cohortRetention, m.cohortRetentionErr
}

func (m *mockBigQueryClient) GetDailyMetrics(_ context.Context, _ *int) ([]EnterpriseMetrics, error) {
	return m.dailyMetrics, m.dailyMetricsErr
}

func (m *mockBigQueryClient) GetAdoptionSummary(_ context.Context) (*AdoptionSummary, error) {
	return m.adoptionSummary, m.adoptionSummaryErr
}

func (m *mockBigQueryClient) GetTeamAdoption(_ context.Context) ([]TeamAdoption, error) {
	return m.teamAdoption, m.teamAdoptionErr
}

func (m *mockBigQueryClient) GetCustomizationDetails(_ context.Context) ([]CustomizationDetail, error) {
	return m.custDetails, m.custDetailsErr
}

func (m *mockBigQueryClient) GetCustomizationUsage(_ context.Context) ([]CustomizationUsage, error) {
	return m.custUsage, m.custUsageErr
}

func (m *mockBigQueryClient) GetLanguageAdoption(_ context.Context) ([]LanguageAdoption, error) {
	return m.langAdoption, m.langAdoptionErr
}

func (m *mockBigQueryClient) GetStalenessData(_ context.Context) ([]StalenessFile, error) {
	return m.stalenessFiles, m.stalenessErr
}

func (m *mockBigQueryClient) GetTeamGrossOverview(_ context.Context, month string) (*TeamGrossOverview, error) {
	if m.grossByMonth != nil {
		return m.grossByMonth[month], nil
	}
	return m.teamGross, m.teamGrossErr
}

func (m *mockBigQueryClient) GetTeamNetOverview(_ context.Context, month string) (*TeamNetOverview, error) {
	if m.netByMonth != nil {
		return m.netByMonth[month], nil
	}
	return m.teamNet, m.teamNetErr
}

func (m *mockBigQueryClient) GetUserTeams(_ context.Context, _ string) ([]string, error) {
	return m.userTeams, nil
}

func TestTeamGrossOverviewHandler(t *testing.T) {
	mock := &mockBigQueryClient{teamGross: &TeamGrossOverview{
		Month: "2026-09", Teams: []TeamSpend{{TeamID: "123", TeamSlug: "team-a", Users: 5, AmountUSD: 42, PerUserUSD: 8.4}},
		Comparison: comparisonIncomplete,
		Usage: map[string]TeamUsageComposition{
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
			var got TeamGrossOverview
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
	h := newBigQueryHandlers(&mockBigQueryClient{teamNet: &TeamNetOverview{
		Month: "2026-09", Teams: []TeamSpend{teamSpend("123", "team-a", 5, 35)},
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
			var got TeamNetOverview
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
	request = request.WithContext(context.WithValue(request.Context(), resolvedIdentityContextKey, &ResolvedIdentity{GitHubUsername: "alice"}))
	h.handleMyTeams(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "[\"team-a\"]\n" {
		t.Fatalf("status %d, body %s", recorder.Code, recorder.Body.String())
	}
}

func (m *mockBigQueryClient) GetUserMetrics(_ context.Context, _ string, _ int) (*UserMetricsSummary, error) {
	return m.userMetrics, m.userMetricsErr
}

func (m *mockBigQueryClient) GetMonthlyTrends(_ context.Context, _ int) ([]MonthlyTrend, error) {
	return m.monthlyTrends, m.monthlyTrendsErr
}

func (m *mockBigQueryClient) GetMonthlyBillingUsage(_ context.Context, _ int) ([]MonthlyBillingUsage, error) {
	return m.monthlyBilling, m.monthlyBillingErr
}

func (m *mockBigQueryClient) GetBillingModelDailyCosts(_ context.Context, _ string) ([]BillingModelDailyCost, error) {
	return m.billingModelDaily, m.billingModelErr
}

func (m *mockBigQueryClient) GetBillingModelForecast(_ context.Context, _ string) (*BillingModelForecast, error) {
	return m.billingForecast, m.billingForecastErr
}

func (m *mockBigQueryClient) GetUserWeeklyTrends(_ context.Context, _ string, _ int) ([]WeeklyTrend, error) {
	return m.weeklyTrends, m.weeklyTrendsErr
}

func (m *mockBigQueryClient) GetUserDailyCredits(_ context.Context, _ string, _ int) ([]DailyCredits, error) {
	return nil, nil
}

func (m *mockBigQueryClient) GetAdoptionCohorts(_ context.Context, _ int) ([]AdoptionCohortDay, error) {
	return m.cohorts, m.cohortsErr
}

func (m *mockBigQueryClient) GetBillingMonthlyTrend(_ context.Context, _ int) ([]BillingMonthlyTrend, error) {
	return nil, nil
}

func (m *mockBigQueryClient) GetBillingModelBreakdown(_ context.Context, _ int) ([]BillingModelBreakdown, error) {
	return nil, nil
}

func (m *mockBigQueryClient) GetDailySummary(_ context.Context) (*DailySummary, error) {
	return nil, nil
}

func (m *mockBigQueryClient) GetUsageDistribution(_ context.Context, _ string, _ float64) (*UsageDistribution, error) {
	return m.usageDistribution, m.usageDistErr
}

func (m *mockBigQueryClient) GetRepositoryUsage(_ context.Context) ([]RepositoryUsage, error) {
	return m.repositoryUsage, m.repositoryUsageErr
}

func (m *mockBigQueryClient) GetCreditsPerUserMonthly(_ context.Context) ([]CreditsPerUserMonth, error) {
	return m.creditsPerUser, m.creditsPerUserErr
}

func (m *mockBigQueryClient) GetCopilotPRsMonthly(_ context.Context) ([]CopilotPRMonth, error) {
	return m.copilotPRs, m.copilotPRsErr
}

func TestHandleDailyMetrics(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		query            string
		mockMetrics      []EnterpriseMetrics
		mockErr          error
		wantStatus       int
		wantBodyContains string
	}{
		{
			name:        "returns metrics on success",
			method:      http.MethodGet,
			mockMetrics: []EnterpriseMetrics{{"day": "2024-01-01", "total_active_users": 10}},
			wantStatus:  http.StatusOK,
		},
		{
			name:        "returns empty list when no data",
			method:      http.MethodGet,
			mockMetrics: []EnterpriseMetrics{},
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
		summary    *AdoptionSummary
		mockErr    error
		wantStatus int
	}{
		{
			name:       "returns summary when available",
			method:     http.MethodGet,
			summary:    &AdoptionSummary{TotalRepos: 42},
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
		files := []StalenessFile{
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

		var result StalenessSummary
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
		mock := &mockBigQueryClient{stalenessFiles: []StalenessFile{}}
		h := newBigQueryHandlers(mock)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/staleness", nil)
		rec := httptest.NewRecorder()
		h.handleAdoptionStaleness(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", rec.Code)
		}

		var result StalenessSummary
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
		mock := &mockBigQueryClient{teamAdoption: []TeamAdoption{{ScanDate: civil.Date{Year: 2026, Month: 6, Day: 2}, TeamSlug: "team-a", TeamName: "Team A", TeamRepos: 5, ActiveRepos: 5}}}
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
			mock:       &mockBigQueryClient{userMetrics: &UserMetricsSummary{UserLogin: "octocat", DaysInPeriod: 7}},
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
			mock:       &mockBigQueryClient{monthlyTrends: []MonthlyTrend{{Month: "2026-06", UniqueUsers: 10}}},
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
			mock:       &mockBigQueryClient{monthlyBilling: []MonthlyBillingUsage{{Month: "2026-06", Model: "gpt", SKU: "premium", GrossRequests: 2}}},
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
			mock:       &mockBigQueryClient{billingModelDaily: []BillingModelDailyCost{{Day: "2026-06-01", Model: "gpt-5", NetAmount: 10.2}}},
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
			mock:       &mockBigQueryClient{billingForecast: &BillingModelForecast{Month: "2026-06", ProjectedEOMNetAmount: 120}},
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
			mock:       &mockBigQueryClient{weeklyTrends: []WeeklyTrend{{Week: "2026-W23", Interactions: 3}}},
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
			mock:       &mockBigQueryClient{cohorts: []AdoptionCohortDay{{Day: validDate, Phase: 1, UserCount: 4}}},
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
			mock:       &mockBigQueryClient{repositoryUsage: []RepositoryUsage{{RepoName: "repo-a", RepoVisibility: "INTERNAL", PRCreatedByCopilot: 3}}},
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
			name:       "credits per user success",
			mock:       &mockBigQueryClient{creditsPerUser: []CreditsPerUserMonth{{Month: "2026-06", Median: 40, Mean: 55, ActiveUsers: 600}}},
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
			mock:       &mockBigQueryClient{copilotPRs: []CopilotPRMonth{{Month: "2026-08", CreatedByCopilot: 12, ReviewedByCopilot: 300, Days: 31}}},
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
			mock:       &mockBigQueryClient{cohortRetention: []CohortRetention{{CohortMonth: "2026-08", CohortSize: 40, M1: ptr(int64(70))}}},
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
	ctx := context.WithValue(req.Context(), resolvedIdentityContextKey, &ResolvedIdentity{GitHubUsername: username, Source: "test"})
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
