package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/admin"
	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
	"github.com/navikt/copilot/apps/copilot-api/internal/config"
)

func TestCostAnomaliesAdminOnly(t *testing.T) {
	t.Setenv(auth.AdminGroupEnv, "admins")
	h := admin.New(admin.Deps{BQ: &adminQuerier{anomalies: []bqstore.Anomaly{{Rule: "A1", Date: "2026-09-21"}}}})
	mux := makeAPIRouter(&config.Config{}, nil, h, nil, nil, nil, nil)
	for _, tc := range []struct {
		groups []string
		want   int
	}{{nil, http.StatusNotFound}, {[]string{"admins"}, http.StatusOK}} {
		req := httptest.NewRequest("GET", "/api/v1/admin/billing/anomalies", nil)
		req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.User{Groups: tc.groups}))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("groups %v: status %d", tc.groups, rec.Code)
		}
		if tc.want == http.StatusOK {
			var got []bqstore.Anomaly
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || len(got) != 1 {
				t.Fatalf("body %s", rec.Body)
			}
		}
	}
}
