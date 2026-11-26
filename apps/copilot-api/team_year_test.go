package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
)

func TestTeamYearHandler(t *testing.T) {
	mock := &mockBigQueryClient{teamYear: &bqstore.TeamYearOverview{TeamID: "123", Year: 2026, Months: []bqstore.TeamYearMonth{{Month: "2026-08", Basis: bqstore.BasisNet}}}}
	h := newBigQueryHandlers(mock)
	for _, bad := range []string{"", "team=", "team=team-a", "team=0", "team=1/2", "team=1&year=2025", "team=1&year=x", "team=1&year=" + strconv.Itoa(time.Now().Year()+1)} {
		rec := httptest.NewRecorder()
		h.handleTeamYearOverview(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-year?"+bad, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status %d", bad, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.handleTeamYearOverview(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/team-year?team=123", nil))
	if rec.Code != http.StatusOK || mock.teamYearArgs[0] != "123" || mock.teamYearArgs[1] != time.Now().UTC().Year() {
		t.Fatalf("status %d, args %v", rec.Code, mock.teamYearArgs)
	}
	if body := rec.Body.String(); strings.Contains(body, "login") || strings.Contains(body, "user_id") {
		t.Errorf("body leaks identities: %s", body)
	}
}
