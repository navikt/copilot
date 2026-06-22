package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/civil"
)

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
	teamUsage          []TeamUsageSummary
	teamUsageErr       error
	userMetrics        *UserMetricsSummary
	userMetricsErr     error
	monthlyTrends      []MonthlyTrend
	monthlyTrendsErr   error
	monthlyModels      []MonthlyModelUsage
	monthlyModelsErr   error
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

func (m *mockBigQueryClient) GetTeamUsageSummary(_ context.Context, _ int) ([]TeamUsageSummary, error) {
	return m.teamUsage, m.teamUsageErr
}

func (m *mockBigQueryClient) GetUserMetrics(_ context.Context, _ string, _ int) (*UserMetricsSummary, error) {
	return m.userMetrics, m.userMetricsErr
}

func (m *mockBigQueryClient) GetMonthlyTrends(_ context.Context, _ int) ([]MonthlyTrend, error) {
	return m.monthlyTrends, m.monthlyTrendsErr
}

func (m *mockBigQueryClient) GetMonthlyModelUsage(_ context.Context, _ int) ([]MonthlyModelUsage, error) {
	return m.monthlyModels, m.monthlyModelsErr
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
		mock := &mockBigQueryClient{teamAdoption: []TeamAdoption{{ScanDate: civil.Date{Year: 2026, Month: 6, Day: 2}, TeamSlug: "team-a", TeamName: "Team A", TeamRepos: 5}}}
		h := newBigQueryHandlers(mock)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/adoption/teams", nil)
		rec := httptest.NewRecorder()
		h.handleTeamAdoption(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status: got %d, want 200", rec.Code)
		}

		var result []TeamAdoption
		if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(result) != 1 || result[0].TeamSlug != "team-a" {
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
			name:       "team summary success",
			mock:       &mockBigQueryClient{teamUsage: []TeamUsageSummary{{TeamSlug: "team-a", TotalUsers: 2}}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-summary", nil),
			handle:     (*BigQueryHandlers).handleTeamUsageSummary,
			wantStatus: http.StatusOK,
		},
		{
			name:       "team summary error",
			mock:       &mockBigQueryClient{teamUsageErr: errors.New("bq")},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-summary", nil),
			handle:     (*BigQueryHandlers).handleTeamUsageSummary,
			wantStatus: http.StatusInternalServerError,
		},
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
			name:       "monthly models success",
			mock:       &mockBigQueryClient{monthlyModels: []MonthlyModelUsage{{Month: "2026-06", Model: "gpt", Interactions: 1}}},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/models", nil),
			handle:     (*BigQueryHandlers).handleMonthlyModelUsage,
			wantStatus: http.StatusOK,
		},
		{
			name:       "monthly models error",
			mock:       &mockBigQueryClient{monthlyModelsErr: errors.New("bq")},
			req:        httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/models", nil),
			handle:     (*BigQueryHandlers).handleMonthlyModelUsage,
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
	return req
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
