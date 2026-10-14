package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// GitHub's 2026-10-12 per-user report held 54.50040499999999, which the
// default PARSE_JSON (wide_number_mode 'exact') rejects. No BigQuery emulator
// here, so pin the SQL.
func TestAllocationReplacementSQLRoundsWideNumbers(t *testing.T) {
	sql := allocationReplacementSQL("t", "r")
	if !strings.Contains(sql, "PARSE_JSON(raw, wide_number_mode=>'round')") {
		t.Fatal("PARSE_JSON must use wide_number_mode=>'round'")
	}
	if !strings.Contains(sql, "WHERE day=@day AND (scope_id=@scopeID OR @complete)") {
		t.Fatal("DELETE must clear fallback scope rows when @complete is true")
	}
}

func TestIngestDay_UserMetricsStoreErrorFailsDay(t *testing.T) {
	day := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	fetcher := &mockFetcher{
		result: &FetchResult{Records: []json.RawMessage{json.RawMessage(`{"daily_active_users":30}`)}, Scope: "enterprise", ScopeID: "nav"},
		userMetricsResult: &FetchResult{
			Records: []json.RawMessage{json.RawMessage(`{"user_id":1,"user_login":"a","day":"2026-10-12","ai_credits_used":54.50040499999999}`)},
			Scope:   "enterprise", ScopeID: "nav",
		},
	}
	bqErr := errors.New("Input number: 54.50040499999999 cannot round-trip through string representation")
	err := ingestDay(context.Background(), fetcher, &mockStore{userMetricsErr: bqErr}, &Config{EnterpriseSlug: "nav"}, day)
	if !errors.Is(err, bqErr) {
		t.Fatalf("want per-user store error, got %v", err)
	}
}
