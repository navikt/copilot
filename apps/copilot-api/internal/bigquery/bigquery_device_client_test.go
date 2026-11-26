package bigquery

import (
	"encoding/json"
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"
)

func TestDeviceClientCoverageMapping(t *testing.T) {
	got := AggregateNavPilotTelemetry([]MimirTelemetryRow{
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
	missing := AggregateNavPilotTelemetry(nil)
	if missing.InstallPresent7d != nil || missing.ClientAvailable7d["copilot"] != nil {
		t.Fatal("missing became zero")
	}
}

func TestEditorUsageNullableCounts(t *testing.T) {
	zero := int64(0)
	rows := []EditorUsage{{Editor: "vscode", Generations: &zero}}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"generations":0`) || !strings.Contains(string(data), `"acceptances":null`) {
		t.Fatalf("response: %s", data)
	}
}

func TestEditorUsageScannerCounts(t *testing.T) {
	schema := bigquery.Schema{
		{Name: "generations", Type: bigquery.IntegerFieldType},
		{Name: "acceptances", Type: bigquery.IntegerFieldType},
		{Name: "interactions", Type: bigquery.IntegerFieldType},
	}
	for _, tc := range []struct {
		name string
		row  []bigquery.Value
		want string
	}{
		{"counts", []bigquery.Value{int64(7), int64(3), int64(9)}, `"generations":7,"acceptances":3,"interactions":9`},
		{"zero and unknown", []bigquery.Value{int64(0), nil, nil}, `"generations":0,"acceptances":null,"interactions":null`},
		{"unknown", []bigquery.Value{nil, nil, nil}, `"generations":null,"acceptances":null,"interactions":null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got EditorUsage
			if err := DecodeRow(schema, tc.row, &got); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tc.want) {
				t.Fatalf("response: %s, want %s", data, tc.want)
			}
		})
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
	rows := []MimirTelemetryRow{
		{Family: "nav_pilot_reach", IsLatest: true, Metric: "nav_pilot_install_present", Operation: "z", Source: "user", Devices: 10},
		{Family: "nav_pilot_reach", IsLatest: true, Metric: "nav_pilot_install_present", Operation: "z", Source: "repo", Devices: 20},
		{Family: "nav_pilot_reach", IsLatest: true, Metric: "nav_pilot_install_present", Operation: "a", Source: "repo", Devices: 30},
	}
	want, err := json.Marshal(AggregateNavPilotTelemetry(rows).InstallPresent7d)
	if err != nil {
		t.Fatal(err)
	}
	rows[0], rows[2] = rows[2], rows[0]
	got, err := json.Marshal(AggregateNavPilotTelemetry(rows).InstallPresent7d)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) || !strings.HasPrefix(string(got), `[{"scope":"repo","collection":"a"`) {
		t.Fatalf("unstable order: %s / %s", got, want)
	}
}
