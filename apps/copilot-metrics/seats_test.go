package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/civil"
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
		orgHttpClient: &http.Client{Transport: &rewriteHostTransport{base: server.Client().Transport, target: server.URL}},
		enterprise:    "nav", org: "navikt",
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
	data, err := rowsToNDJSON(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"user_id":7`, `"created_at":"2026-03-14T10:00:00Z"`, `"pending_cancellation_date":null`, `"pending_cancellation_date":"2026-11-01"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s in %s", want, data)
		}
	}
}

func TestIngestTodayCopilotSeats(t *testing.T) {
	now := time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC)
	var stored int
	store := func(_ context.Context, day time.Time, rows []*SeatRow) error {
		if day.Format("20060102") != "20261011" {
			t.Errorf("day = %v", day)
		}
		for _, r := range rows {
			if r.SnapshotDate.String() != "2026-10-11" {
				t.Errorf("row stamped %s, partition is 2026-10-11", r.SnapshotDate)
			}
		}
		stored = len(rows)
		return nil
	}
	if err := ingestTodayCopilotSeats(context.Background(), func(context.Context) ([]*SeatRow, error) {
		// Fetched before midnight: the row's own day must not win.
		return []*SeatRow{{UserID: 1, SnapshotDate: civil.Date{Year: 2026, Month: 10, Day: 10}}}, nil
	}, store, now); err != nil || stored != 1 {
		t.Fatalf("err=%v stored=%d", err, stored)
	}
	if err := ingestTodayCopilotSeats(context.Background(), func(context.Context) ([]*SeatRow, error) {
		return nil, errSeatsNoPermission
	}, store, now); err != nil {
		t.Errorf("403 should be skipped: %v", err)
	}
	if err := ingestTodayCopilotSeats(context.Background(), func(context.Context) ([]*SeatRow, error) {
		return nil, errors.New("boom")
	}, store, now); err == nil {
		t.Error("fetch error should fail the job")
	}
}
