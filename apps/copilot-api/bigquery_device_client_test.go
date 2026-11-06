package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"
)

func (m *mockBigQueryClient) GetEditorUsage(context.Context) ([]EditorUsage, error) {
	return m.editorUsage, m.editorUsageErr
}

func TestDeviceClientCoverageMapping(t *testing.T) {
	got := aggregateNavPilotTelemetry([]mimirTelemetryRow{
		{Family: "nav_pilot_reach", IsLatest: true, Metric: "nav_pilot_client_available", Operation: "7d", Client: "copilot", Devices: 210},
		{Family: "nav_pilot_reach", IsLatest: true, Metric: "nav_pilot_client_available", Operation: "7d", Client: "pi", Devices: 0},
		{Family: "nav_pilot_reach", IsLatest: false, Metric: "nav_pilot_client_available", Operation: "7d", Client: "opencode", Devices: 116},
		{Family: "nav_pilot_reach", IsLatest: true, Metric: "nav_pilot_client_available", Operation: "1d", Client: "opencode", Devices: 116},
		{Family: "nav_pilot_reach", IsLatest: true, Metric: "nav_pilot_install_present", Operation: "nav-pilot", Source: "user", Devices: 25},
		{Family: "nav_pilot_reach", IsLatest: true, Metric: "nav_pilot_install_present", Operation: "other", Source: "repo", Devices: 0},
	})
	if got.ClientAvailable7d["opencode"] != nil || *got.ClientAvailable7d["pi"] != 0 || *got.ClientAvailable7d["copilot"] != 210 {
		t.Fatalf("availability: %#v", got.ClientAvailable7d)
	}
	if len(got.InstallPresent7d) != 2 || got.InstallPresent7d[1].Collection != "nav-pilot" || got.InstallPresent7d[0].Devices != 0 {
		t.Fatalf("install presence: %#v", got.InstallPresent7d)
	}
	missing := aggregateNavPilotTelemetry(nil)
	if missing.InstallPresent7d != nil || missing.ClientAvailable7d["copilot"] != nil {
		t.Fatal("missing became zero")
	}
}

func TestEditorUsageNullableCounts(t *testing.T) {
	rows := []EditorUsage{{Editor: "vscode", Generations: bigquery.NullInt64{Int64: 0, Valid: true}}}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"generations":0`) || !strings.Contains(string(data), `"acceptances":null`) {
		t.Fatalf("response: %s", data)
	}
}

func TestEditorUsageQueryWindow(t *testing.T) {
	query := editorUsageQuery("`project.dataset.usage_metrics`")
	for _, want := range []string{"INTERVAL 28 DAY", "COUNT(DISTINCT day)", "GROUP BY editor", "COUNT(generations) = COUNT(*)", "scope = 'enterprise'", "LOWER(scope_id) = 'nav'", "ORDER BY loaded_at DESC", "FROM daily, UNNEST", "SAFE_CAST"} {
		if !strings.Contains(query, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(query, "active_users") {
		t.Fatal("activity must not sum users")
	}
}

func TestInstallPresenceDeterministicOrder(t *testing.T) {
	rows := []mimirTelemetryRow{
		{Family: "nav_pilot_reach", IsLatest: true, Metric: "nav_pilot_install_present", Operation: "z", Source: "user", Devices: 10},
		{Family: "nav_pilot_reach", IsLatest: true, Metric: "nav_pilot_install_present", Operation: "z", Source: "repo", Devices: 20},
		{Family: "nav_pilot_reach", IsLatest: true, Metric: "nav_pilot_install_present", Operation: "a", Source: "repo", Devices: 30},
	}
	want, err := json.Marshal(aggregateNavPilotTelemetry(rows).InstallPresent7d)
	if err != nil {
		t.Fatal(err)
	}
	rows[0], rows[2] = rows[2], rows[0]
	got, err := json.Marshal(aggregateNavPilotTelemetry(rows).InstallPresent7d)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) || !strings.HasPrefix(string(got), `[{"scope":"repo","collection":"a"`) {
		t.Fatalf("unstable order: %s / %s", got, want)
	}
}

func TestEditorUsageEmptyHandler(t *testing.T) {
	h := &BigQueryHandlers{bqClient: &mockBigQueryClient{}}
	w := httptest.NewRecorder()
	h.handleEditorUsage(w, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/editors", nil))
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("response: %d %s", w.Code, w.Body.String())
	}
}

func TestEditorUsageHandlerError(t *testing.T) {
	h := &BigQueryHandlers{bqClient: &mockBigQueryClient{editorUsageErr: errors.New("unavailable")}}
	w := httptest.NewRecorder()
	h.handleEditorUsage(w, httptest.NewRequest(http.MethodGet, "/api/v1/copilot/usage/editors", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}
