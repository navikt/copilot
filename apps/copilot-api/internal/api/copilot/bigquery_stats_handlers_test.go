package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"

	"github.com/navikt/copilot/apps/copilot-api/internal/budget"
)

// mockBudgetGetter implements budget.GlobalBudgetGetter for testing resolveBudgetCredits.
type mockBudgetGetter struct {
	budget *budget.GlobalBudget
	err    error
}

func (m *mockBudgetGetter) GetGlobalBudget(_ context.Context) (*budget.GlobalBudget, error) {
	return m.budget, m.err
}

func TestResolveBudgetCredits(t *testing.T) {
	tests := []struct {
		name         string
		budgetClient budget.GlobalBudgetGetter
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
			budgetClient: &mockBudgetGetter{budget: &budget.GlobalBudget{PerUserBudget: 0}},
			want:         defaultPerUserBudgetCredits,
		},
		{
			name:         "negative per-user budget falls back to default",
			budgetClient: &mockBudgetGetter{budget: &budget.GlobalBudget{PerUserBudget: -10}},
			want:         defaultPerUserBudgetCredits,
		},
		{
			name:         "valid budget converts USD to credits",
			budgetClient: &mockBudgetGetter{budget: &budget.GlobalBudget{PerUserBudget: 400}},
			want:         40000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handlers{budgetClient: tc.budgetClient}
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
				usageDistribution: &bqstore.UsageDistribution{
					Month:    "2026-06",
					NumUsers: 10,
					CreditsHistogram: []bqstore.UsageHistogramBucket{
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
			h.HandleUsageDistribution(rec, req)
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
	shared := &bqstore.UsageDistribution{
		Month:              "2026-06",
		NumUsers:           10,
		TotalLicensedSeats: 999, // sentinel — should never be read back
		CreditsHistogram:   []bqstore.UsageHistogramBucket{{Bucket: "0%", NumUsers: 10}},
	}
	mock := &mockBigQueryClient{usageDistribution: shared}
	h := newBigQueryHandlers(mock)
	h.setSeatsGetter(func() int64 { return 42 })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/distribution", nil)
	rec := httptest.NewRecorder()
	h.HandleUsageDistribution(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}

	if shared.TotalLicensedSeats != 999 {
		t.Errorf("handler mutated shared/cached distribution: TotalLicensedSeats = %d, want 999 (unchanged)", shared.TotalLicensedSeats)
	}

	var got bqstore.UsageDistribution
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
		usageDistribution: &bqstore.UsageDistribution{Month: "2026-06", NumUsers: 10},
	}
	h := &Handlers{bqClient: mock}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/distribution", nil)
	rec := httptest.NewRecorder()
	h.HandleUsageDistribution(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, body=%s", rec.Code, rec.Body.String())
	}
	var got bqstore.UsageDistribution
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.TotalLicensedSeats != 0 {
		t.Errorf("TotalLicensedSeats = %d, want 0 with nil getter", got.TotalLicensedSeats)
	}
}
