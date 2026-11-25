package api

import (
	"context"
	"strings"
	"testing"

	"github.com/navikt/copilot/apps/copilot-survey/internal/survey"
)

// A closed survey's results and count come from final aggregates when it has
// them; an open survey's always from its rows.
func TestResultsReadFinalForClosed(t *testing.T) {
	rows := []survey.StoredAnswer{row(`{"a":1}`, `{"a":1}`)}
	api := adminAPI(t, rows)
	fake(api).final = func(_ context.Context, id string) (*survey.Tally, error) {
		if id != "old" {
			return nil, nil
		}
		return &survey.Tally{Responses: 6, Keys: map[string]map[int]*survey.VerTally{"a": {1: {N: 6, Counts: map[string]int{"2": 6}}}}}, nil
	}
	h := makeRouter(testAuthenticator(t), api)
	rec := do(h, "GET", "/api/v1/admin/surveys/old/results", adminToken, "")
	if !strings.Contains(rec.Body.String(), `"responses":6`) || !strings.Contains(rec.Body.String(), `{"value":"2","count":6,"suppressed":false}`) {
		t.Fatal(rec.Body.String())
	}
	rec = do(h, "GET", "/api/v1/admin/surveys", adminToken, "")
	if !strings.Contains(rec.Body.String(), `"id":"old","title":"Old","starts":"2025-01-01","ends":"2025-01-31","status":"stengt","responses":6`) ||
		!strings.Contains(rec.Body.String(), `"status":"åpen","responses":1`) {
		t.Fatal(rec.Body.String())
	}
}
