package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRecordClientUsage(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("nav-pilot")
	tel := &otelTelemetry{provider: provider, version: "test", device: "dev"}
	tel.genAITokens, _ = meter.Int64Counter("nav_pilot_genai_session_tokens_total")
	tel.genAICalls, _ = meter.Int64Counter("nav_pilot_genai_session_calls_total")
	tel.genAIToolCalls, _ = meter.Int64Counter("nav_pilot_genai_session_tool_calls_total")
	tel.RecordClientUsage(ClientUsage{
		Client: "opencode",
		Models: map[ModelKey]ModelUsage{{Provider: "github-copilot", Model: "m"}: {Calls: 2, Input: 10, Output: 5}},
		Tools:  map[string]int64{"bash": 3},
	})
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	wantKeys := map[string][]string{
		"nav_pilot_genai_session_tokens_total":     {"client", "nav_repo", "version", "device_id", "gen_ai_provider_name", "gen_ai_request_model", "gen_ai_token_type"},
		"nav_pilot_genai_session_calls_total":      {"client", "nav_repo", "version", "device_id", "gen_ai_provider_name", "gen_ai_request_model"},
		"nav_pilot_genai_session_tool_calls_total": {"client", "nav_repo", "version", "device_id", "gen_ai_tool_name"},
	}
	tokenTypes := map[string]bool{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			keys, ok := wantKeys[m.Name]
			if !ok {
				t.Fatalf("unexpected metric %s", m.Name)
			}
			delete(wantKeys, m.Name)
			for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
				if dp.Attributes.Len() != len(keys) {
					t.Fatalf("%s attrs = %v, want %v", m.Name, dp.Attributes.ToSlice(), keys)
				}
				for _, k := range keys {
					if _, ok := dp.Attributes.Value(attribute.Key(k)); !ok {
						t.Fatalf("%s missing %s", m.Name, k)
					}
				}
				if v, ok := dp.Attributes.Value("gen_ai_token_type"); ok {
					tokenTypes[v.AsString()] = true
				}
			}
		}
	}
	if len(wantKeys) != 0 {
		t.Fatalf("missing metrics %v", wantKeys)
	}
	if len(tokenTypes) != 2 || !tokenTypes["input"] || !tokenTypes["output"] {
		t.Fatalf("token types = %v, want only input and output (zero types emit nothing)", tokenTypes)
	}
}
