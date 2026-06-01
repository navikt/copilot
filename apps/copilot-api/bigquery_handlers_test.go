package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
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
		mock := &mockBigQueryClient{teamAdoption: []TeamAdoption{{TeamSlug: "team-a", TeamName: "Team A", TeamRepos: 5}}}
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
