package main

import (
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
)

func TestRowsToNDJSON(t *testing.T) {
	rows := []*BillingUsageReportRow{{
		ReportDay:      civil.Date{Year: 2026, Month: 10, Day: 1},
		Organization:   "navikt",
		RepositoryName: bigquery.NullString{},
		RawRecord:      `{"a":1}`,
		LoadedAt:       time.Date(2026, 10, 1, 12, 0, 0, 123456789, time.UTC),
	}}
	got, err := rowsToNDJSON(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"report_day":"2026-10-01"`, `"raw_record":{"a":1}`, `"loaded_at":"2026-10-01T12:00:00.123456Z"`, `"repository_name":null`} {
		if !strings.Contains(string(got), want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

func TestRowsToNDJSONAllLoadedTypes(t *testing.T) {
	for name, f := range map[string]func() ([]byte, error){
		"usage":       func() ([]byte, error) { return rowsToNDJSON([]*UsageMetricsRow{{RawRecord: "{}"}}) },
		"billing":     func() ([]byte, error) { return rowsToNDJSON([]*BillingUsageRow{{RawRecord: "{}"}}) },
		"daily_model": func() ([]byte, error) { return rowsToNDJSON([]*BillingUsageDailyModelRow{{RawRecord: "{}"}}) },
		"budget":      func() ([]byte, error) { return rowsToNDJSON([]*BudgetSnapshotRow{{}}) },
		"user_budget": func() ([]byte, error) { return rowsToNDJSON([]*UserBudgetSnapshotRow{{}}) },
	} {
		if _, err := f(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
