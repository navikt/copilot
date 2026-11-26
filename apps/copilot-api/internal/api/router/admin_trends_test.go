package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/admin"
	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
	"github.com/navikt/copilot/apps/copilot-api/internal/config"
)

func TestAdminTrendRoutesNeedAdmin(t *testing.T) {
	const group = "11111111-2222-3333-4444-555555555555"
	t.Setenv(auth.AdminGroupEnv, group)
	router := API(&config.Config{}, nil, admin.New(admin.Deps{BQ: &adminQuerier{}}), nil, nil, nil, nil)
	for _, path := range []string{"/api/v1/admin/billing/active-users", "/api/v1/admin/billing/high-consumers", "/api/v1/admin/billing/budget-days"} {
		for _, tc := range []struct {
			groups []string
			want   int
		}{{nil, http.StatusNotFound}, {[]string{group}, http.StatusOK}} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.User{Groups: tc.groups}))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("%s groups=%v: %d, want %d", path, tc.groups, rec.Code, tc.want)
			}
			if tc.want == http.StatusOK && !strings.HasPrefix(rec.Header().Get("Cache-Control"), "private") {
				t.Errorf("%s: cache-control %q, want private", path, rec.Header().Get("Cache-Control"))
			}
		}
	}
}
