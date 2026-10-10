package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testSeat(login, pending string) copilotSeat {
	var s copilotSeat
	s.Assignee.Login = login
	s.PendingCancellationDate = pending
	return s
}

func TestCountSeats(t *testing.T) {
	got := countSeats("enterprise", "nav", 4, []copilotSeat{
		testSeat("alice", ""), testSeat("bob", "2026-11-01"), testSeat("", ""), testSeat("dave", "2026-11-01"),
	})
	want := SeatCounts{Scope: "enterprise", ScopeID: "nav", Total: 4, PendingCancellation: 2}
	if *got != want {
		t.Fatalf("countSeats = %+v, want %+v", *got, want)
	}
}

func TestFetchSeatCounts_FallsBackToOrg(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/enterprises/") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.URL.Path != "/orgs/navikt/copilot/billing/seats" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(copilotSeatResponse{TotalSeats: 2, Seats: []copilotSeat{testSeat("a", ""), testSeat("b", "2026-11-01")}})
	}))
	defer server.Close()

	client := &GitHubClient{
		httpClient: &http.Client{Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL}},
		enterprise: "nav",
		org:        "navikt",
	}
	got, err := client.FetchSeatCounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := SeatCounts{Scope: "organization", ScopeID: "navikt", Total: 2, PendingCancellation: 1}
	if *got != want {
		t.Fatalf("FetchSeatCounts = %+v, want %+v", *got, want)
	}
}

// TestIngestTodaySeatCounts_Idempotent runs the ingestion twice on the same day
// against a store keyed like the MERGE (snapshot_date, scope_id) and expects one row.
func TestIngestTodaySeatCounts_Idempotent(t *testing.T) {
	type key struct {
		day     string
		scopeID string
	}
	store := map[key]SeatCounts{}
	upsert := func(_ context.Context, day time.Time, c *SeatCounts) error {
		store[key{day.Format("2006-01-02"), c.ScopeID}] = *c
		return nil
	}
	total := 10
	fetch := func(context.Context) (*SeatCounts, error) {
		return &SeatCounts{Scope: "enterprise", ScopeID: "nav", Total: total}, nil
	}

	morning := time.Date(2026, 10, 10, 2, 0, 0, 0, time.UTC)
	ingestTodaySeatCounts(context.Background(), fetch, upsert, morning)
	total = 12
	ingestTodaySeatCounts(context.Background(), fetch, upsert, morning.Add(20*time.Hour))

	if len(store) != 1 {
		t.Fatalf("expected 1 row after re-run, got %d: %v", len(store), store)
	}
	if got := store[key{"2026-10-10", "nav"}].Total; got != 12 {
		t.Fatalf("re-run should replace the row: total = %d, want 12", got)
	}
}

func TestIngestTodaySeatCounts_FetchErrorWritesNothing(t *testing.T) {
	called := false
	ingestTodaySeatCounts(context.Background(),
		func(context.Context) (*SeatCounts, error) { return nil, errors.New("boom") },
		func(context.Context, time.Time, *SeatCounts) error { called = true; return nil },
		time.Now())
	if called {
		t.Fatal("upsert called after fetch error")
	}
}

func TestSeatCountsMergeSQL_KeysOnDateAndScope(t *testing.T) {
	sql := seatCountsMergeSQL("`p.d.seat_counts`")
	for _, want := range []string{
		"MERGE `p.d.seat_counts`",
		"ON t.snapshot_date = s.snapshot_date AND t.scope_id = s.scope_id",
		"WHEN MATCHED THEN UPDATE",
		"WHEN NOT MATCHED THEN INSERT",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("merge SQL missing %q:\n%s", want, sql)
		}
	}
}
