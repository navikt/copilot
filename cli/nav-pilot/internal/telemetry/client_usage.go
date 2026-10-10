package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ClientUsage is what one client session used, summed per model. Counts only:
// no prompt, response, path or user is ever part of it.
type ClientUsage struct {
	Client string
	Models map[ModelKey]ModelUsage
	Tools  map[string]int64
}

type ModelKey struct{ Provider, Model string }

type ModelUsage struct {
	Calls                                           int64
	Input, Output, Reasoning, CacheRead, CacheWrite int64
}

// RecordClientUsage adds one session's totals to the counters.
//
// No label per process or session. newOnlyExporter sends each export as what
// is new since the last one, so a session reaches Mimir as one sample of its
// own totals and sum_over_time adds the sessions up, concurrent ones included.
// That is delta in all but the flag: real OTLP delta needs deltatocumulative,
// which the Nav collector does not run, and nav-pilot's delta counters never
// arrived (see temporalityFor).
func (t *otelTelemetry) RecordClientUsage(u ClientUsage) {
	ctx := context.Background()
	common := []attribute.KeyValue{
		attribute.String("client", orUnset(u.Client)),
		attribute.String("nav_repo", orUnset(detectNavRepo())),
		attribute.String("version", t.version),
		attribute.String("device_id", t.device),
	}
	with := func(kv ...attribute.KeyValue) metric.MeasurementOption {
		return metric.WithAttributes(append(kv, common...)...)
	}
	for k, m := range u.Models {
		model := []attribute.KeyValue{
			attribute.String("gen_ai_provider_name", orUnset(k.Provider)),
			attribute.String("gen_ai_request_model", orUnset(k.Model)),
		}
		if m.Calls > 0 {
			t.genAICalls.Add(ctx, m.Calls, with(model...))
		}
		for typ, n := range map[string]int64{
			"input": m.Input, "output": m.Output, "reasoning": m.Reasoning,
			"cache_read": m.CacheRead, "cache_write": m.CacheWrite,
		} {
			if n > 0 {
				t.genAITokens.Add(ctx, n, with(append(model, attribute.String("gen_ai_token_type", typ))...))
			}
		}
	}
	for tool, n := range u.Tools {
		if n > 0 {
			t.genAIToolCalls.Add(ctx, n, with(attribute.String("gen_ai_tool_name", orUnset(tool))))
		}
	}
}
