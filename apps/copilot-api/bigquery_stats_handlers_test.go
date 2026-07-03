package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockBudgetGetter implements globalBudgetGetter for testing resolveBudgetCredits.
type mockBudgetGetter struct {
	budget *GlobalBudget
	err    error
}

func (m *mockBudgetGetter) getGlobalBudget(_ context.Context) (*GlobalBudget, error) {
	return m.budget, m.err
}

func TestVerifyUsernameOwnership(t *testing.T) {
	newRequestWithUser := func(user *User) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/user/hans", nil)
		if user != nil {
			req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))
		}
		return req
	}

	t.Run("copilot-cli azp with matching X-On-Behalf-Of succeeds", func(t *testing.T) {
		h := newBigQueryHandlers(&mockBigQueryClient{})
		h.setCopilotCLIClientID("copilot-cli-client-id")

		req := newRequestWithUser(&User{AZP: "copilot-cli-client-id"})
		req.Header.Set("X-On-Behalf-Of", "hans")
		rec := httptest.NewRecorder()

		if ok := h.verifyUsernameOwnership(rec, req, "hans"); !ok {
			t.Fatalf("expected ownership to be verified, got response %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("copilot-cli azp with mismatched X-On-Behalf-Of is denied", func(t *testing.T) {
		h := newBigQueryHandlers(&mockBigQueryClient{})
		h.setCopilotCLIClientID("copilot-cli-client-id")

		req := newRequestWithUser(&User{AZP: "copilot-cli-client-id"})
		req.Header.Set("X-On-Behalf-Of", "someone-else")
		rec := httptest.NewRecorder()

		if ok := h.verifyUsernameOwnership(rec, req, "hans"); ok {
			t.Fatal("expected ownership check to fail on username mismatch")
		}
		if rec.Code != http.StatusForbidden {
			t.Errorf("status: got %d, want %d", rec.Code, http.StatusForbidden)
		}
	})

	t.Run("copilot-cli azp with missing X-On-Behalf-Of is rejected", func(t *testing.T) {
		h := newBigQueryHandlers(&mockBigQueryClient{})
		h.setCopilotCLIClientID("copilot-cli-client-id")

		req := newRequestWithUser(&User{AZP: "copilot-cli-client-id"})
		rec := httptest.NewRecorder()

		if ok := h.verifyUsernameOwnership(rec, req, "hans"); ok {
			t.Fatal("expected ownership check to fail without X-On-Behalf-Of header")
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("non-copilot-cli azp falls back to SAML lookup", func(t *testing.T) {
		h := newBigQueryHandlers(&mockBigQueryClient{})
		h.setCopilotCLIClientID("copilot-cli-client-id")
		h.setGitHubClient(&mockGitHubClient{samlUsername: "hans"})
		h.environment = "prod"

		req := newRequestWithUser(&User{AZP: "my-copilot-client-id", Email: "hans@nav.no"})
		req.Header.Set("X-On-Behalf-Of", "attacker") // must be ignored for non-copilot-cli callers
		rec := httptest.NewRecorder()

		if ok := h.verifyUsernameOwnership(rec, req, "hans"); !ok {
			t.Fatalf("expected SAML-based ownership to be verified, got response %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("copilotCLIClientID unset ignores X-On-Behalf-Of entirely", func(t *testing.T) {
		h := newBigQueryHandlers(&mockBigQueryClient{})
		h.setGitHubClient(&mockGitHubClient{samlUsername: "hans"})
		h.environment = "prod"

		req := newRequestWithUser(&User{AZP: "copilot-cli-client-id", Email: "hans@nav.no"})
		req.Header.Set("X-On-Behalf-Of", "attacker")
		rec := httptest.NewRecorder()

		// copilotCLIClientID is empty, so even a matching azp must not bypass SAML.
		if ok := h.verifyUsernameOwnership(rec, req, "hans"); !ok {
			t.Fatalf("expected SAML-based ownership to be verified, got response %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestResolveBudgetCredits(t *testing.T) {
	tests := []struct {
		name         string
		budgetClient globalBudgetGetter
		want         float64
	}{
		{
			name:         "nil budget client falls back to default",
			budgetClient: nil,
			want:         defaultPerUserBudgetCredits,
		},
		{
			name:         "budget client error falls back to default",
			budgetClient: &mockBudgetGetter{err: errors.New("boom")},
			want:         defaultPerUserBudgetCredits,
		},
		{
			name:         "nil budget falls back to default",
			budgetClient: &mockBudgetGetter{budget: nil},
			want:         defaultPerUserBudgetCredits,
		},
		{
			name:         "zero per-user budget falls back to default",
			budgetClient: &mockBudgetGetter{budget: &GlobalBudget{PerUserBudget: 0}},
			want:         defaultPerUserBudgetCredits,
		},
		{
			name:         "negative per-user budget falls back to default",
			budgetClient: &mockBudgetGetter{budget: &GlobalBudget{PerUserBudget: -10}},
			want:         defaultPerUserBudgetCredits,
		},
		{
			name:         "valid budget converts USD to credits",
			budgetClient: &mockBudgetGetter{budget: &GlobalBudget{PerUserBudget: 400}},
			want:         40000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := &BigQueryHandlers{budgetClient: tc.budgetClient}
			got := h.resolveBudgetCredits(context.Background())
			if got != tc.want {
				t.Errorf("resolveBudgetCredits() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHandleUsageDistribution(t *testing.T) {
	tests := []struct {
		name       string
		mock       *mockBigQueryClient
		query      string
		wantStatus int
	}{
		{
			name: "success",
			mock: &mockBigQueryClient{
				usageDistribution: &UsageDistribution{
					Month:    "2026-06",
					NumUsers: 10,
					CreditsHistogram: []UsageHistogramBucket{
						{Bucket: "0%", NumUsers: 2},
						{Bucket: "1-9%", NumUsers: 8},
					},
				},
			},
			query:      "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "bq error",
			mock:       &mockBigQueryClient{usageDistErr: errors.New("bq")},
			query:      "",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "invalid month",
			mock:       &mockBigQueryClient{},
			query:      "?month=2026/06",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newBigQueryHandlers(tc.mock)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/distribution"+tc.query, nil)
			rec := httptest.NewRecorder()
			h.handleUsageDistribution(rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("status: got %d, want %d; body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

// TestHandleUsageDistributionDoesNotMutateCachedResponse guards against the
// data race fixed in handleUsageDistribution: the handler must copy the
// distribution before setting TotalLicensedSeats, since bqClient may return
// a shared pointer to a cached value across concurrent requests.
func TestHandleUsageDistributionDoesNotMutateCachedResponse(t *testing.T) {
	shared := &UsageDistribution{
		Month:              "2026-06",
		NumUsers:           10,
		TotalLicensedSeats: 999, // sentinel — should never be read back
		CreditsHistogram:   []UsageHistogramBucket{{Bucket: "0%", NumUsers: 10}},
	}
	mock := &mockBigQueryClient{usageDistribution: shared}
	h := newBigQueryHandlers(mock)
	h.setActiveSeatsGetter(func() int64 { return 42 })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/distribution", nil)
	rec := httptest.NewRecorder()
	h.handleUsageDistribution(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}

	if shared.TotalLicensedSeats != 999 {
		t.Errorf("handler mutated shared/cached distribution: TotalLicensedSeats = %d, want 999 (unchanged)", shared.TotalLicensedSeats)
	}

	var got UsageDistribution
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.TotalLicensedSeats != 42 {
		t.Errorf("response TotalLicensedSeats = %d, want 42", got.TotalLicensedSeats)
	}
}

// TestHandleUsageDistributionSeatsGetterDefaultsToNil verifies the handler
// doesn't panic and simply reports zero seats when no getter has been wired
// (e.g. a handler constructed without newBigQueryHandlers).
func TestHandleUsageDistributionSeatsGetterDefaultsToNil(t *testing.T) {
	mock := &mockBigQueryClient{
		usageDistribution: &UsageDistribution{Month: "2026-06", NumUsers: 10},
	}
	h := &BigQueryHandlers{bqClient: mock}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/distribution", nil)
	rec := httptest.NewRecorder()
	h.handleUsageDistribution(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var got UsageDistribution
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.TotalLicensedSeats != 0 {
		t.Errorf("TotalLicensedSeats = %d, want 0 with nil getter", got.TotalLicensedSeats)
	}
}

func TestIsValidYearMonth(t *testing.T) {
	tests := []struct {
		name  string
		month string
		want  bool
	}{
		{"valid month", "2026-06", true},
		{"valid january", "2026-01", true},
		{"valid december", "2026-12", true},
		{"invalid separator", "2026/06", false},
		{"invalid calendar month", "2026-13", false},
		{"invalid zero month", "2026-00", false},
		{"empty string", "", false},
		{"missing day would be wrong format anyway", "2026-06-01", false},
		{"garbage", "not-a-month", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isValidYearMonth(tc.month); got != tc.want {
				t.Errorf("isValidYearMonth(%q) = %v, want %v", tc.month, got, tc.want)
			}
		})
	}
}
