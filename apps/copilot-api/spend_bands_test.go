package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
)

func TestSpendBandsHandlerSendsOnlyAggregates(t *testing.T) {
	h := newBigQueryHandlers(&mockBigQueryClient{spendBands: bqstore.BuildSpendBands(
		[]bqstore.SpendUserRow{{Month: "2026-09", Net: 1, Gross: 2}, {Month: "2026-09", Net: 1, Gross: 2}, {Month: "2026-09", Net: 1, Gross: 2}, {Month: "2026-09", Net: 1, Gross: 2}, {Month: "2026-09", Net: 1, Gross: 2}}, nil, "")})
	rec := httptest.NewRecorder()
	h.handleSpendBands(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/spend-bands", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	for _, banned := range []string{"user_id", "login", "gross\""} {
		if strings.Contains(body, banned) {
			t.Errorf("response contains %q: %s", banned, body)
		}
	}
	var got bqstore.SpendBands
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Months[0].Bands[1].Users != 5 {
		t.Errorf("unexpected body %s (%v)", body, err)
	}
}
