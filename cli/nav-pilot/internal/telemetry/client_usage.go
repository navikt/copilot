package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"

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

// newRunID names this process in the usage series.
//
// The series are cumulative and every process starts at zero. Two sessions on
// one device and repository writing the same series would read as counter
// resets, the fault that inflates Copilot's gen_ai_client_token_usage in
// Mimir. A random id per process keeps each series to one writer. It says
// nothing about the user and lives as long as one session.
func newRunID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (t *otelTelemetry) RecordClientUsage(u ClientUsage) {
	ctx := context.Background()
	common := []attribute.KeyValue{
		attribute.String("client", orUnset(u.Client)),
		attribute.String("nav_repo", orUnset(detectNavRepo())),
		attribute.String("run_id", t.runID),
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
