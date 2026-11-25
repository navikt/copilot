package fetchers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/types"
)

func testSeat(login, pending string) copilotSeat {
	var s copilotSeat
	s.Assignee.Login = login
	for _, b := range []byte(login) {
		s.Assignee.ID = s.Assignee.ID*31 + int64(b)
	}
	s.PendingCancellationDate = pending
	return s
}

func TestCountSeats(t *testing.T) {
	got := countSeats("enterprise", "nav", 4, []copilotSeat{
		testSeat("alice", ""), testSeat("bob", "2026-11-01"), testSeat("", "2026-11-01"), testSeat("dave", "2026-11-01"),
		testSeat("bob", "2026-11-01"), // repeated across pages
	})
	want := types.SeatCounts{Scope: "enterprise", ScopeID: "nav", Total: 4, PendingCancellation: 2}
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
		HTTPClient: &http.Client{Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL}},
		Enterprise: "nav",
		Org:        "navikt",
	}
	got, err := client.FetchSeatCounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := types.SeatCounts{Scope: "organization", ScopeID: "navikt", Total: 2, PendingCancellation: 1}
	if *got != want {
		t.Fatalf("FetchSeatCounts = %+v, want %+v", *got, want)
	}
}
