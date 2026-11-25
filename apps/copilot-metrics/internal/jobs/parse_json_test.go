package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/config"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

func TestIngestDay_UserMetricsStoreErrorFailsDay(t *testing.T) {
	day := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	fetcher := &mockFetcher{
		result: &types.FetchResult{Records: []json.RawMessage{json.RawMessage(`{"daily_active_users":30}`)}, Scope: "enterprise", ScopeID: "nav"},
		userMetricsResult: &types.FetchResult{
			Records: []json.RawMessage{json.RawMessage(`{"user_id":1,"user_login":"a","day":"2026-10-12","ai_credits_used":54.50040499999999}`)},
			Scope:   "enterprise", ScopeID: "nav",
		},
	}
	bqErr := errors.New("Input number: 54.50040499999999 cannot round-trip through string representation")
	err := ingestDay(context.Background(), fetcher, &mockStore{userMetricsErr: bqErr}, &config.Config{EnterpriseSlug: "nav"}, day)
	if !errors.Is(err, bqErr) {
		t.Fatalf("want per-user store error, got %v", err)
	}
}
