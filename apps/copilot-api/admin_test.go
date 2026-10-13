package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAdminRoutesGuarded walks every registered admin route.
// A new admin route that skips requireAdmin fails here.
func TestAdminRoutesGuarded(t *testing.T) {
	const group = "11111111-2222-3333-4444-555555555555"
	// nil BigQuery handlers: an admin who gets through sees the 503 stub.
	router := makeAPIRouter(&Config{}, nil, nil, nil, nil)
	cases := []struct {
		name string
		env  string
		user *User
		want int
	}{
		{"admin", group, &User{Groups: []string{group}}, http.StatusServiceUnavailable},
		{"non-admin", group, &User{Groups: []string{"other"}}, http.StatusNotFound},
		{"group unset", "", &User{Groups: []string{group}}, http.StatusNotFound},
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
				t.Setenv(adminGroupEnv, tc.env)
				req := httptest.NewRequest(http.MethodGet, p, nil)
				req = req.WithContext(context.WithValue(req.Context(), userContextKey, tc.user))
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
		user *User
		want int
	}{
		{"admin", group, &User{Groups: []string{"other", group}}, http.StatusOK},
		{"non-admin", group, &User{Groups: []string{"other"}}, http.StatusNotFound},
		{"no groups", group, &User{}, http.StatusNotFound},
		{"no user", group, nil, http.StatusNotFound},
		{"group unset", "", &User{Groups: []string{""}}, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(adminGroupEnv, tc.env)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/x", nil)
			if tc.user != nil {
				req = req.WithContext(context.WithValue(req.Context(), userContextKey, tc.user))
			}
			rec := httptest.NewRecorder()
			requireAdmin(ok).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestOldAdminPathsGone(t *testing.T) {
	const group = "11111111-2222-3333-4444-555555555555"
	t.Setenv(adminGroupEnv, group)
	router := makeAPIRouter(&Config{}, nil, nil, nil, nil)
	for _, p := range []string{
		"/api/v1/copilot/billing/admin-monthly",
		"/api/v1/copilot/billing/admin-active-users",
		"/api/v1/copilot/billing/admin-high-consumers",
		"/api/v1/copilot/billing/admin-budget-days",
		"/api/v1/copilot/billing/anomalies",
		"/api/v1/copilot/admin/cost-forecast",
	} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, &User{Groups: []string{group}}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", p, rec.Code)
		}
	}
}
