package jobs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/fetchers"
)

type fakeSeries struct {
	labels map[string]string
	value  string
}

// fakeMimir answers each query with the series registered for the first
// matching substring, and records every query and its time.
func fakeMimir(t *testing.T, answers map[string][]fakeSeries) (*fetchers.MimirClient, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Scope-OrgID") != "nais" {
			http.Error(w, "no org", http.StatusUnauthorized)
			return
		}
		q := r.URL.Query().Get("query")
		seen = append(seen, q+" @"+r.URL.Query().Get("time"))
		var res []map[string]any
		for sub, series := range answers {
			if strings.Contains(q, sub) {
				for _, s := range series {
					res = append(res, map[string]any{"metric": s.labels, "value": []any{0, s.value}})
				}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "vector", "result": res}})
	}))
	t.Cleanup(srv.Close)
	return &fetchers.MimirClient{BaseURL: srv.URL, HTTP: srv.Client()}, &seen
}

func TestRunMimirDailyEmptyDayFailsWithoutWipe(t *testing.T) {
	m, _ := fakeMimir(t, map[string][]fakeSeries{})
	d := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	// A nil BigQueryClient proves ReplaceMimirDay was not called, as any write attempt panics.
	err := runMimirDaily(context.Background(), m, nil, d, d)
	if err == nil || !strings.Contains(err.Error(), "Mimir returned no rows") {
		t.Fatalf("expected 'Mimir returned no rows' error, got %v", err)
	}
}
