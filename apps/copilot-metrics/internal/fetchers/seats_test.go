package fetchers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navikt/copilot/apps/copilot-metrics/internal/storage"
)

func TestFetchCopilotSeatsRows(t *testing.T) {
	body := `{"total_seats":3,"seats":[
 {"assignee":{"id":7,"login":"a"},"plan_type":"business","created_at":"2026-03-14T10:00:00Z","pending_cancellation_date":null},
 {"assignee":{"id":8,"login":"b"},"plan_type":"enterprise","created_at":"2026-05-01T00:00:00Z","pending_cancellation_date":"2026-11-01"},
 {"assignee":{"slug":"team"},"created_at":"2026-01-01T00:00:00Z"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	client := &GitHubClient{
		OrgHTTPClient: &http.Client{Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL}},
		Enterprise:    "nav", Org: "navikt",
	}
	rows, err := client.FetchCopilotSeats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (seat without user skipped)", len(rows))
	}
	a, b := rows[0], rows[1]
	if a.UserID != 7 || a.Login != "a" || a.PlanType != "business" || a.ScopeID != "navikt" ||
		!a.CreatedAt.Valid || a.CreatedAt.Timestamp.Format("2006-01-02") != "2026-03-14" || a.PendingCancellationDate.Valid {
		t.Errorf("a = %+v", a)
	}
	if !b.PendingCancellationDate.Valid || b.PendingCancellationDate.Date.String() != "2026-11-01" {
		t.Errorf("b = %+v", b)
	}
	data, err := storage.RowsToNDJSON(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"user_id":7`, `"created_at":"2026-03-14T10:00:00Z"`, `"pending_cancellation_date":null`, `"pending_cancellation_date":"2026-11-01"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s in %s", want, data)
		}
	}
}
