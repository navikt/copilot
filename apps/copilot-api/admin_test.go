package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
	"github.com/navikt/copilot/apps/copilot-api/internal/config"
)

// TestAdminRoutesGuarded walks every registered admin route.
// A new admin route that skips requireAdmin fails here.
func TestAdminRoutesGuarded(t *testing.T) {
	const group = "11111111-2222-3333-4444-555555555555"
	// nil BigQuery handlers: an admin who gets through sees the 503 stub.
	router := makeAPIRouter(&config.Config{}, nil, nil, nil, nil, nil, nil)
	cases := []struct {
		name string
		env  string
		user *auth.User
		want int
	}{
		{"admin", group, &auth.User{Groups: []string{group}}, http.StatusServiceUnavailable},
		{"non-admin", group, &auth.User{Groups: []string{"other"}}, http.StatusNotFound},
		{"group unset", "", &auth.User{Groups: []string{group}}, http.StatusNotFound},
	}
	var paths []string
	for _, r := range adminRoutes() {
		if !strings.HasPrefix(r.path, "/api/v1/admin/") {
			t.Errorf("%s is not under /api/v1/admin/", r.path)
		}
		paths = append(paths, r.path)
	}
	for _, tc := range cases {
		for _, p := range paths {
			t.Run(tc.name+" "+p, func(t *testing.T) {
				t.Setenv(auth.AdminGroupEnv, tc.env)
				req := httptest.NewRequest(http.MethodGet, p, nil)
				req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, tc.user))
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				if rec.Code != tc.want {
					t.Fatalf("got %d, want %d", rec.Code, tc.want)
				}
			})
		}
	}
}

func TestRequireAdmin(t *testing.T) {
	const group = "11111111-2222-3333-4444-555555555555"
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	cases := []struct {
		name string
		env  string
		user *auth.User
		want int
	}{
		{"admin", group, &auth.User{Groups: []string{"other", group}}, http.StatusOK},
		{"non-admin", group, &auth.User{Groups: []string{"other"}}, http.StatusNotFound},
		{"no groups", group, &auth.User{}, http.StatusNotFound},
		{"no user", group, nil, http.StatusNotFound},
		{"group unset", "", &auth.User{Groups: []string{""}}, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(auth.AdminGroupEnv, tc.env)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/x", nil)
			if tc.user != nil {
				req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, tc.user))
			}
			rec := httptest.NewRecorder()
			auth.RequireAdmin(ok).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestOldAdminPathsGone(t *testing.T) {
	const group = "11111111-2222-3333-4444-555555555555"
	t.Setenv(auth.AdminGroupEnv, group)
	router := makeAPIRouter(&config.Config{}, nil, nil, nil, nil, nil, nil)
	for _, p := range []string{
		"/api/v1/copilot/billing/admin-monthly",
		"/api/v1/copilot/billing/admin-active-users",
		"/api/v1/copilot/billing/admin-high-consumers",
		"/api/v1/copilot/billing/admin-budget-days",
		"/api/v1/copilot/billing/anomalies",
		"/api/v1/copilot/admin/cost-forecast",
	} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.User{Groups: []string{group}}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", p, rec.Code)
		}
	}
}

// adminQuerier answers the queries behind the admin routes these router
// tests call; any other query panics on the nil embedded interface.
type adminQuerier struct {
	bqstore.BigQueryQuerier
	anomalies []bqstore.Anomaly
}

func (m *adminQuerier) GetAdminMonthlyOverview(_ context.Context, year int) (*bqstore.AdminMonthlyOverview, error) {
	return &bqstore.AdminMonthlyOverview{Year: year}, nil
}

func (m *adminQuerier) GetCostAnomalies(_ context.Context) ([]bqstore.Anomaly, error) {
	return m.anomalies, nil
}

func (m *adminQuerier) GetActiveUsersMonthly(_ context.Context) ([]bqstore.ActiveUsersMonth, error) {
	return []bqstore.ActiveUsersMonth{{Month: "2026-09", ActiveUsers: 719}}, nil
}

func (m *adminQuerier) GetBudgetDays(_ context.Context) ([]bqstore.BudgetDay, error) {
	return []bqstore.BudgetDay{}, nil
}

func (m *adminQuerier) GetHighConsumers(_ context.Context) ([]bqstore.HighConsumerMonth, error) {
	return []bqstore.HighConsumerMonth{}, nil
}
