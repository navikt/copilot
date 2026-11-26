package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
)

func (m *mockBigQueryClient) GetUsageQuality(context.Context) ([]bqstore.UsageQualityDay, error) {
	return nil, nil
}

type qualityQueryMock struct {
	BigQueryQuerier
	rows []bqstore.UsageQualityDay
	err  error
}

func (m qualityQueryMock) GetUsageQuality(context.Context) ([]bqstore.UsageQualityDay, error) {
	return m.rows, m.err
}

func TestUsageQualityNullableCounts(t *testing.T) {
	for _, raw := range []string{
		`{"day":"2026-11-01"}`,
		`{"day":"2026-11-02","missing_client_telemetry_user_count":null,"below_usage_metrics_minimum_user_count":null}`,
	} {
		var day bqstore.UsageQualityDay
		if err := json.Unmarshal([]byte(raw), &day); err != nil {
			t.Fatal(err)
		}
		if day.MissingClientTelemetry != nil || day.BelowUsageMetricsMinimum != nil {
			t.Fatal("unavailable became zero")
		}
	}
	var day bqstore.UsageQualityDay
	if err := json.Unmarshal([]byte(`{"day":"2026-11-03","missing_client_telemetry_user_count":0,"below_usage_metrics_minimum_user_count":6}`), &day); err != nil {
		t.Fatal(err)
	}
	if day.MissingClientTelemetry == nil || *day.MissingClientTelemetry != 0 || day.BelowUsageMetricsMinimum == nil || *day.BelowUsageMetricsMinimum != 6 {
		t.Fatalf("counts: %+v", day)
	}
}

func TestUsageQualityHandler(t *testing.T) {
	zero := int64(0)
	for _, tc := range []struct {
		name   string
		mock   qualityQueryMock
		status int
		body   string
	}{
		{"empty", qualityQueryMock{}, 200, "[]"},
		{"counts", qualityQueryMock{rows: []bqstore.UsageQualityDay{{Day: "2026-11-03", Scope: "enterprise", ScopeID: "nav", MissingClientTelemetry: &zero}}}, 200, `"below_usage_metrics_minimum_user_count":null`},
		{"error", qualityQueryMock{err: errors.New("test query error")}, 500, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newBigQueryHandlers(tc.mock).handleUsageQuality(rec, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/quality", nil))
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.body) {
				t.Fatalf("%d %s", rec.Code, rec.Body.String())
			}
			if tc.status == 200 && !strings.Contains(rec.Header().Get("Cache-Control"), "private") {
				t.Fatal("quality response must use private caching")
			}
		})
	}
}
