package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRecordCopilotSession(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	in, err := newSessionInstruments(provider.Meter("nav-pilot"))
	if err != nil {
		t.Fatal(err)
	}
	tel := &otelTelemetry{provider: provider, version: "test", device: "dev", session: in}
	tel.RecordCopilotSession(CopilotSession{
		Client:          "copilot",
		Skills:          map[SkillKey]int64{{"klarsprak", "personal-copilot", "agent-invoked"}: 1},
		Tools:           map[ToolKey]int64{{"bash", "success"}: 2},
		Hooks:           map[HookKey]int64{{"preToolUse", "success"}: 3},
		Agents:          map[string]int64{"other": 1},
		Models:          map[string]SessionModel{"gpt-6-sol": {Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4, PremiumRequests: 1, NanoAIU: 5}},
		Turns:           4,
		DurationSeconds: 60,
		ShutdownType:    "routine",
		APIDurationMS:   100,
		APIModel:        "gpt-6-sol",
		Skipped:         1,
	})
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"nav_pilot_session_skill_loads_total": true, "nav_pilot_session_tool_calls_total": true,
		"nav_pilot_session_hook_runs_total": true, "nav_pilot_session_tokens_total": true,
		"nav_pilot_session_premium_requests_total": true, "nav_pilot_session_cost_nano_aiu_total": true,
		"nav_pilot_subagent_selected_total": true, "nav_pilot_session_events_skipped_total": true,
		"nav_pilot_session_turns": true, "nav_pilot_session_duration_seconds": true, "nav_pilot_session_api_duration_ms": true,
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			delete(want, m.Name)
			check := func(name string, has func(string) bool) {
				if !has("version") || !has("client") || has("device_id") {
					t.Errorf("%s: want version and client, no device_id", name)
				}
			}
			switch d := m.Data.(type) {
			case metricdata.Sum[int64]:
				for _, dp := range d.DataPoints {
					check(m.Name, func(k string) bool { return dp.Attributes.HasValue(attribute.Key(k)) })
				}
			case metricdata.Sum[float64]:
				for _, dp := range d.DataPoints {
					check(m.Name, func(k string) bool { return dp.Attributes.HasValue(attribute.Key(k)) })
				}
			case metricdata.Histogram[int64]:
				for _, dp := range d.DataPoints {
					check(m.Name, func(k string) bool { return dp.Attributes.HasValue(attribute.Key(k)) })
				}
			default:
				t.Errorf("%s: unexpected type %T", m.Name, d)
			}
		}
	}
	if len(want) != 0 {
		t.Errorf("not emitted: %v", want)
	}
}
