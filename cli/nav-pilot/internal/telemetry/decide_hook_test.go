package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestDecideAndHookAttributesAreEnums is the content rule for the decide and
// hook instruments: every attribute is from a fixed set, except the model,
// which the caller has already reduced to a manifest id or "custom". The
// recorder is fed free text in every string field, the way a careless caller
// would pass a question, an option or a file path, and none of it may come out.
func TestDecideAndHookAttributesAreEnums(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("nav-pilot")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	tel := &otelTelemetry{provider: provider, version: "test", device: "device-under-test", executionContext: "organic"}
	var err error
	tel.decideResultTotal, err = meter.Int64Counter("nav_pilot_decide_result_total")
	must(err)
	tel.decideLatencyMS, err = meter.Int64Histogram("nav_pilot_decide_latency_ms")
	must(err)
	tel.decidePChoice, err = meter.Float64Histogram("nav_pilot_decide_p_choice")
	must(err)
	tel.hookLoopGuardTotal, err = meter.Int64Counter("nav_pilot_hook_loop_guard_total")
	must(err)
	tel.hookRedactTotal, err = meter.Int64Counter("nav_pilot_hook_redact_total")
	must(err)

	const model = "mlx-community/Some-Model-4bit"
	const leak = "Is this commit message about /Users/x/secret.txt? ghp_abc"
	for _, e := range []DecideEvent{
		{Result: "decided", Model: model, Evidence: true, EvidenceBytes: 500, Options: 2, Caller: "tty", Answered: true, MS: 300, PChoice: 0.93},
		{Result: "below_threshold", Model: model, Evidence: true, EvidenceBytes: 40 << 10, Options: 26, ThresholdUsed: true, Caller: "hook", Answered: true, MS: 900, PChoice: 0.6},
		{Result: "no_server", Options: 3, Caller: "script"},
		{Result: "timeout", Model: model, Options: 7, Caller: "script"},
		{Result: leak, Model: model, Options: 12, Caller: leak, Evidence: true, EvidenceBytes: 5000, Answered: true, MS: 10},
	} {
		tel.RecordDecide(e)
	}
	for _, r := range []string{"same_result", "cycle", "backstop", leak} {
		tel.RecordHookLoopGuard(r, "cloud")
		tel.RecordHookLoopGuard(r, leak)
	}
	for _, k := range []string{"secret", "fnr", "injection_note", leak} {
		tel.RecordHookRedact(k, 2)
	}

	allowed := map[string]map[string]bool{
		"result":            set("decided", "below_threshold", "no_server", "timeout", "error", "unknown"),
		"model":             set(model, "custom", "unset"),
		"evidence":          set("yes", "no"),
		"options":           set("2", "3-4", "5-11", "12+"),
		"threshold_used":    set("yes", "no"),
		"caller":            set("tty", "hook", "script", "unknown"),
		"evidence_size":     set("none", "<1k", "1-8k", "8-32k", "32k+"),
		"rule":              set("same_result", "cycle", "backstop", "unknown"),
		"session":           set("local", "cloud", "unknown"),
		"kind":              set("secret", "fnr", "injection_note", "unknown"),
		"version":           set("test"),
		"device_id":         set("device-under-test"),
		"execution_context": set("organic"),
	}

	var rm metricdata.ResourceMetrics
	must(reader.Collect(context.Background(), &rm))
	seen := map[string]bool{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			seen[m.Name] = true
			for _, attrs := range pointAttrs(t, m) {
				for _, kv := range attrs.ToSlice() {
					vals, ok := allowed[string(kv.Key)]
					if !ok {
						t.Errorf("%s: attribute %q is not in the allowed set", m.Name, kv.Key)
						continue
					}
					if !vals[kv.Value.AsString()] {
						t.Errorf("%s: %s=%q is not an allowed value", m.Name, kv.Key, kv.Value.AsString())
					}
				}
			}
		}
	}
	for _, want := range []string{"nav_pilot_decide_result_total", "nav_pilot_decide_latency_ms", "nav_pilot_decide_p_choice",
		"nav_pilot_hook_loop_guard_total", "nav_pilot_hook_redact_total"} {
		if !seen[want] {
			t.Errorf("never emitted %s", want)
		}
	}
}

func set(vs ...string) map[string]bool {
	m := map[string]bool{}
	for _, v := range vs {
		m[v] = true
	}
	return m
}

func pointAttrs(t *testing.T, m metricdata.Metrics) []attribute.Set {
	var out []attribute.Set
	switch d := m.Data.(type) {
	case metricdata.Sum[int64]:
		for _, dp := range d.DataPoints {
			out = append(out, dp.Attributes)
		}
	case metricdata.Histogram[int64]:
		for _, dp := range d.DataPoints {
			out = append(out, dp.Attributes)
		}
	case metricdata.Histogram[float64]:
		for _, dp := range d.DataPoints {
			out = append(out, dp.Attributes)
		}
	default:
		t.Fatalf("%s: unexpected data type %T", m.Name, m.Data)
	}
	return out
}

func TestDecideBuckets(t *testing.T) {
	for n, want := range map[int]string{2: "2", 3: "3-4", 4: "3-4", 5: "5-11", 11: "5-11", 12: "12+", 26: "12+"} {
		if got := optionsBucket(n); got != want {
			t.Errorf("optionsBucket(%d) = %q, want %q", n, got, want)
		}
	}
	for _, c := range []struct {
		has  bool
		n    int
		want string
	}{{false, 0, "none"}, {true, 0, "<1k"}, {true, 1023, "<1k"}, {true, 1024, "1-8k"}, {true, 8 << 10, "8-32k"}, {true, 32 << 10, "8-32k"}, {true, 32<<10 + 1, "32k+"}} {
		if got := evidenceSizeBucket(c.has, c.n); got != c.want {
			t.Errorf("evidenceSizeBucket(%v, %d) = %q, want %q", c.has, c.n, got, c.want)
		}
	}
}

// The alpha command names have a space and no dot or hyphen, so without their
// own entries they would all report as command="unknown".
func TestAlphaCommandNamesSurviveNormalisation(t *testing.T) {
	for _, c := range []string{"alpha", "alpha decide", "alpha decide eval", "alpha local init", "alpha local start",
		"alpha local stop", "alpha local restart", "alpha local status", "alpha local on", "alpha local off",
		"alpha local ask", "alpha local purge"} {
		if got := normalizeTelemetryDimension(c, "unknown"); got != c {
			t.Errorf("%q normalised to %q", c, got)
		}
	}
}
