package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockGitHubClient implements GitHubAPI for testing
type mockGitHubClient struct {
	billing         *CopilotBilling
	billingErr      error
	seat            *CopilotSeat
	seatErr         error
	assignResult    *AssignResult
	assignErr       error
	unassignResult  *UnassignResult
	unassignErr     error
	samlUsername    string
	samlErr         error
	premiumUsage    *PremiumRequestUsage
	premiumErr      error
	contributors    []Contributor
	contributorsErr error
}

func (m *mockGitHubClient) getCopilotBilling(_ context.Context) (*CopilotBilling, error) {
	return m.billing, m.billingErr
}

func (m *mockGitHubClient) getCopilotSeat(_ context.Context, _ string) (*CopilotSeat, error) {
	return m.seat, m.seatErr
}

func (m *mockGitHubClient) assignUserToCopilot(_ context.Context, _ string) (*AssignResult, error) {
	return m.assignResult, m.assignErr
}

func (m *mockGitHubClient) unassignUserFromCopilot(_ context.Context, _ string) (*UnassignResult, error) {
	return m.unassignResult, m.unassignErr
}

func (m *mockGitHubClient) getUsernameBySamlIdentity(_ context.Context, _ string) (string, error) {
	return m.samlUsername, m.samlErr
}

func (m *mockGitHubClient) getPremiumRequestUsage(_ context.Context, _ string, _ int, _ int) (*PremiumRequestUsage, error) {
	return m.premiumUsage, m.premiumErr
}

func (m *mockGitHubClient) getRepositoryContributors(_ context.Context, _ string, _ string, _ []string) ([]Contributor, error) {
	return m.contributors, m.contributorsErr
}

// userContext injects a User into the request context (test helper)
func userContext(req *http.Request, u *User) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), userContextKey, u))
}

func TestIsValidGitHubUsername(t *testing.T) {
	valid := []string{"octocat", "nav-it", "a", "user123", "A-B-C"}
	invalid := []string{"", "-leading", "trailing-", "inv@lid", strings.Repeat("a", 40), "has/slash"}

	for _, u := range valid {
		if !isValidGitHubUsername(u) {
			t.Errorf("expected %q to be valid", u)
		}
	}
	for _, u := range invalid {
		if isValidGitHubUsername(u) {
			t.Errorf("expected %q to be invalid", u)
		}
	}
}

func TestHandleBilling(t *testing.T) {
	t.Run("returns billing data on success", func(t *testing.T) {
		mock := &mockGitHubClient{billing: &CopilotBilling{}}
		mock.billing.SeatBreakdown.Total = 100
		h := newGitHubHandlers(mock)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/billing", nil)
		rec := httptest.NewRecorder()
		h.handleBilling(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status: got %d, want 200", rec.Code)
		}
	})

	t.Run("returns 500 on error", func(t *testing.T) {
		mock := &mockGitHubClient{billingErr: errors.New("github error")}
		h := newGitHubHandlers(mock)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/billing", nil)
		rec := httptest.NewRecorder()
		h.handleBilling(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status: got %d, want 500", rec.Code)
		}
	})
}

func TestHandleGetSeat(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		seat       *CopilotSeat
		seatErr    error
		wantStatus int
	}{
		{
			name:       "returns seat for valid user",
			username:   "octocat",
			seat:       &CopilotSeat{PlanType: "business"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "returns 404 when seat not found",
			username:   "octocat",
			seat:       nil,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "rejects invalid username",
			username:   "inv@lid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 500 on backend error",
			username:   "octocat",
			seatErr:    errors.New("github down"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockGitHubClient{seat: tc.seat, seatErr: tc.seatErr}
			h := newGitHubHandlers(mock)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/seats/"+tc.username, nil)
			req.SetPathValue("username", tc.username)
			rec := httptest.NewRecorder()
			h.handleGetSeat(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestHandleAssignSeat(t *testing.T) {
	actor := &User{Email: "actor@nav.no", NAVident: "A123456"}

	t.Run("assigns seat when username matches caller SAML identity", func(t *testing.T) {
		mock := &mockGitHubClient{
			samlUsername: "octocat",
			assignResult: &AssignResult{SeatsCreated: 1},
		}
		h := newGitHubHandlers(mock)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/copilot/seats",
			strings.NewReader(`{"username":"octocat"}`))
		req = userContext(req, actor)
		rec := httptest.NewRecorder()
		h.handleAssignSeat(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("status: got %d, want 201", rec.Code)
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		h := newGitHubHandlers(&mockGitHubClient{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/copilot/seats",
			strings.NewReader(`{"username":"octocat"}`))
		rec := httptest.NewRecorder()
		h.handleAssignSeat(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want 401", rec.Code)
		}
	})

	t.Run("rejects invalid username in body", func(t *testing.T) {
		h := newGitHubHandlers(&mockGitHubClient{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/copilot/seats",
			strings.NewReader(`{"username":"inv@lid"}`))
		req = userContext(req, actor)
		rec := httptest.NewRecorder()
		h.handleAssignSeat(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rec.Code)
		}
	})

	t.Run("rejects malformed JSON", func(t *testing.T) {
		h := newGitHubHandlers(&mockGitHubClient{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/copilot/seats",
			strings.NewReader(`not-json`))
		req = userContext(req, actor)
		rec := httptest.NewRecorder()
		h.handleAssignSeat(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rec.Code)
		}
	})

	t.Run("returns 500 when SAML lookup fails", func(t *testing.T) {
		mock := &mockGitHubClient{samlErr: errors.New("saml unavailable")}
		h := newGitHubHandlers(mock)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/copilot/seats",
			strings.NewReader(`{"username":"octocat"}`))
		req = userContext(req, actor)
		rec := httptest.NewRecorder()
		h.handleAssignSeat(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status: got %d, want 500", rec.Code)
		}
	})

	t.Run("returns 403 when caller has no linked GitHub account", func(t *testing.T) {
		mock := &mockGitHubClient{samlUsername: ""}
		h := newGitHubHandlers(mock)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/copilot/seats",
			strings.NewReader(`{"username":"octocat"}`))
		req = userContext(req, actor)
		rec := httptest.NewRecorder()
		h.handleAssignSeat(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status: got %d, want 403", rec.Code)
		}
	})

	t.Run("returns 403 when username does not match caller identity", func(t *testing.T) {
		mock := &mockGitHubClient{samlUsername: "other-user"}
		h := newGitHubHandlers(mock)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/copilot/seats",
			strings.NewReader(`{"username":"octocat"}`))
		req = userContext(req, actor)
		rec := httptest.NewRecorder()
		h.handleAssignSeat(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status: got %d, want 403", rec.Code)
		}
	})
}

func TestHandleUnassignSeat(t *testing.T) {
	actor := &User{Email: "actor@nav.no", NAVident: "A123456"}

	t.Run("unassigns seat when username matches caller SAML identity", func(t *testing.T) {
		mock := &mockGitHubClient{
			samlUsername:   "octocat",
			unassignResult: &UnassignResult{SeatsCancelled: 1},
		}
		h := newGitHubHandlers(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/copilot/seats/octocat", nil)
		req.SetPathValue("username", "octocat")
		req = userContext(req, actor)
		rec := httptest.NewRecorder()
		h.handleUnassignSeat(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status: got %d, want 200", rec.Code)
		}
	})

	t.Run("rejects unauthenticated request", func(t *testing.T) {
		h := newGitHubHandlers(&mockGitHubClient{})
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/copilot/seats/octocat", nil)
		req.SetPathValue("username", "octocat")
		rec := httptest.NewRecorder()
		h.handleUnassignSeat(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want 401", rec.Code)
		}
	})

	t.Run("rejects invalid username in path", func(t *testing.T) {
		h := newGitHubHandlers(&mockGitHubClient{})
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/copilot/seats/inv@lid", nil)
		req.SetPathValue("username", "inv@lid")
		req = userContext(req, actor)
		rec := httptest.NewRecorder()
		h.handleUnassignSeat(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rec.Code)
		}
	})

	t.Run("returns 500 when SAML lookup fails", func(t *testing.T) {
		mock := &mockGitHubClient{samlErr: errors.New("saml unavailable")}
		h := newGitHubHandlers(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/copilot/seats/octocat", nil)
		req.SetPathValue("username", "octocat")
		req = userContext(req, actor)
		rec := httptest.NewRecorder()
		h.handleUnassignSeat(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status: got %d, want 500", rec.Code)
		}
	})

	t.Run("returns 403 when caller has no linked GitHub account", func(t *testing.T) {
		mock := &mockGitHubClient{samlUsername: ""}
		h := newGitHubHandlers(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/copilot/seats/octocat", nil)
		req.SetPathValue("username", "octocat")
		req = userContext(req, actor)
		rec := httptest.NewRecorder()
		h.handleUnassignSeat(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status: got %d, want 403", rec.Code)
		}
	})

	t.Run("returns 403 when username does not match caller identity", func(t *testing.T) {
		mock := &mockGitHubClient{samlUsername: "other-user"}
		h := newGitHubHandlers(mock)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/copilot/seats/octocat", nil)
		req.SetPathValue("username", "octocat")
		req = userContext(req, actor)
		rec := httptest.NewRecorder()
		h.handleUnassignSeat(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status: got %d, want 403", rec.Code)
		}
	})
}

func TestHandleGetUsernameBySAML(t *testing.T) {
	tests := []struct {
		name         string
		identity     string
		mockUsername string
		mockErr      error
		wantStatus   int
	}{
		{
			name:         "returns username for known identity",
			identity:     "user@nav.no",
			mockUsername: "octocat",
			wantStatus:   http.StatusOK,
		},
		{
			name:       "returns null username for unknown identity",
			identity:   "unknown@nav.no",
			wantStatus: http.StatusOK,
		},
		{
			name:       "rejects empty identity",
			identity:   "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rejects identity with slash",
			identity:   "user/name@nav.no",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 500 on backend error",
			identity:   "user@nav.no",
			mockErr:    errors.New("github error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockGitHubClient{samlUsername: tc.mockUsername, samlErr: tc.mockErr}
			h := newGitHubHandlers(mock)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/saml/"+tc.identity, nil)
			req.SetPathValue("identity", tc.identity)
			rec := httptest.NewRecorder()
			h.handleGetUsernameBySAML(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestHandlePremiumRequestUsage(t *testing.T) {
	tests := []struct {
		name       string
		org        string
		year       string
		month      string
		mockData   *PremiumRequestUsage
		mockErr    error
		wantStatus int
	}{
		{
			name:       "returns data with optional year and month",
			org:        "nav",
			mockData:   &PremiumRequestUsage{},
			wantStatus: http.StatusOK,
		},
		{
			name:       "returns data with only year",
			org:        "nav",
			year:       "2024",
			mockData:   &PremiumRequestUsage{},
			wantStatus: http.StatusOK,
		},
		{
			name:       "returns data with only month",
			org:        "nav",
			month:      "6",
			mockData:   &PremiumRequestUsage{},
			wantStatus: http.StatusOK,
		},
		{
			name:       "rejects missing org parameter",
			month:      "6",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rejects invalid year format",
			org:        "nav",
			year:       "invalid",
			month:      "6",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rejects invalid month value (too high)",
			org:        "nav",
			year:       "2024",
			month:      "13",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rejects invalid month value (zero)",
			org:        "nav",
			year:       "2024",
			month:      "0",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 500 on backend error",
			org:        "nav",
			year:       "2024",
			month:      "6",
			mockErr:    errors.New("github error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockGitHubClient{premiumUsage: tc.mockData, premiumErr: tc.mockErr}
			h := newGitHubHandlers(mock)

			query := ""
			if tc.org != "" {
				query += "org=" + tc.org
			}
			if tc.year != "" {
				if query != "" {
					query += "&"
				}
				query += "year=" + tc.year
			}
			if tc.month != "" {
				if query != "" {
					query += "&"
				}
				query += "month=" + tc.month
			}

			url := "/api/v1/copilot/billing/premium"
			if query != "" {
				url += "?" + query
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()
			h.handlePremiumRequestUsage(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestHandleRepositoryContributors(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		repo       string
		paths      string
		mockData   []Contributor
		mockErr    error
		wantStatus int
	}{
		{
			name:       "returns contributors on success",
			owner:      "navikt",
			repo:       "copilot",
			paths:      `["src/main.go","src/api.go"]`,
			mockData:   []Contributor{{Login: "user1"}, {Login: "user2"}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "rejects missing owner parameter",
			repo:       "copilot",
			paths:      `["src/main.go"]`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rejects missing repo parameter",
			owner:      "navikt",
			paths:      `["src/main.go"]`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rejects missing paths parameter",
			owner:      "navikt",
			repo:       "copilot",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rejects invalid JSON in paths",
			owner:      "navikt",
			repo:       "copilot",
			paths:      "not-json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "returns 500 on backend error",
			owner:      "navikt",
			repo:       "copilot",
			paths:      `["src/main.go"]`,
			mockErr:    errors.New("github error"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockGitHubClient{contributors: tc.mockData, contributorsErr: tc.mockErr}
			h := newGitHubHandlers(mock)

			query := ""
			if tc.owner != "" {
				query += "owner=" + tc.owner
			}
			if tc.repo != "" {
				if query != "" {
					query += "&"
				}
				query += "repo=" + tc.repo
			}
			if tc.paths != "" {
				if query != "" {
					query += "&"
				}
				query += "paths=" + tc.paths
			}

			url := "/api/v1/copilot/repo-contributors"
			if query != "" {
				url += "?" + query
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()
			h.handleRepositoryContributors(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}
