package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
)

// The route must sit behind requireAdmin: Nav's bill is for admins only.
func TestAdminMonthlyRouteRequiresAdmin(t *testing.T) {
	const group = "11111111-2222-3333-4444-555555555555"
	t.Setenv(adminGroupEnv, group)
	router := makeAPIRouter(&Config{}, &BigQueryHandlers{bqClient: &mockBigQueryClient{}}, nil, nil, auth.NewIdentityResolverChain(), nil)
	for _, tc := range []struct {
		groups []string
		want   int
	}{{[]string{group}, http.StatusOK}, {[]string{"other"}, http.StatusNotFound}} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/billing/monthly", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.User{Groups: tc.groups}))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("groups %v: got %d, want %d", tc.groups, rec.Code, tc.want)
		}
	}
}
