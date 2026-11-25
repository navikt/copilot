package jobs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/fetchers"
	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

// TestIngestTodaySeatCounts_Idempotent runs the ingestion twice on the same day
// against a store keyed like the MERGE (snapshot_date, scope_id) and expects one row.
func TestIngestTodaySeatCounts_Idempotent(t *testing.T) {
	type key struct {
		day     string
		scopeID string
	}
	store := map[key]types.SeatCounts{}
	upsert := func(_ context.Context, day time.Time, c *types.SeatCounts) error {
		store[key{day.Format("2006-01-02"), c.ScopeID}] = *c
		return nil
	}
	total := 10
	fetch := func(context.Context) (*types.SeatCounts, error) {
		return &types.SeatCounts{Scope: "enterprise", ScopeID: "nav", Total: total}, nil
	}

	morning := time.Date(2026, 10, 10, 2, 0, 0, 0, time.UTC)
	if err := ingestTodaySeatCounts(context.Background(), fetch, upsert, morning); err != nil {
		t.Fatal(err)
	}
	total = 12
	if err := ingestTodaySeatCounts(context.Background(), fetch, upsert, morning.Add(20*time.Hour)); err != nil {
		t.Fatal(err)
	}

	if len(store) != 1 {
		t.Fatalf("expected 1 row after re-run, got %d: %v", len(store), store)
	}
	if got := store[key{"2026-10-10", "nav"}].Total; got != 12 {
		t.Fatalf("re-run should replace the row: total = %d, want 12", got)
	}
}

func TestIngestTodaySeatCounts_FetchErrorWritesNothing(t *testing.T) {
	called := false
	err := ingestTodaySeatCounts(context.Background(),
		func(context.Context) (*types.SeatCounts, error) { return nil, errors.New("boom") },
		func(context.Context, time.Time, *types.SeatCounts) error { called = true; return nil },
		time.Now())
	if err == nil {
		t.Fatal("fetch error not returned")
	}
	if called {
		t.Fatal("upsert called after fetch error")
	}
}

func TestIngestTodaySeatCounts_ForbiddenIsSkipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Resource not accessible by integration"}`, http.StatusForbidden)
	}))
	defer srv.Close()
	c := &fetchers.GitHubClient{
		HTTPClient: &http.Client{Transport: &rewriteHostTransport{base: srv.Client().Transport, target: srv.URL}},
		Enterprise: "nav",
		Org:        "navikt",
	}
	called := false
	err := ingestTodaySeatCounts(context.Background(), c.FetchSeatCounts,
		func(context.Context, time.Time, *types.SeatCounts) error { called = true; return nil },
		time.Now())
	if err != nil || called {
		t.Fatalf("403 should be skipped without writing: err=%v called=%v", err, called)
	}
}
