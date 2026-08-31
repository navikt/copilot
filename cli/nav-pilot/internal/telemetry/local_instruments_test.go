package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestRecorderEmitsEveryLocalInstrument exercises the real recorder.
//
// The first version of this built its own MeterProvider and its own three
// instruments and proved the OTel SDK works, which was never in question. It
// would have passed with a dropped Add in RecordLocalServer, which is the
// regression it was supposed to catch. This one constructs the instruments the
// way telemetry.go does, calls the recorder's own methods, and reads what comes
// out.
func TestRecorderEmitsEveryLocalInstrument(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("nav-pilot")

	dispatches, err := meter.Int64Histogram("nav_pilot_local_dispatches")
	if err != nil {
		t.Fatal(err)
	}
	ready, err := meter.Int64Histogram("nav_pilot_local_ready_seconds")
	if err != nil {
		t.Fatal(err)
	}
	tel := &otelTelemetry{
		provider:          provider,
		localDispatches:   dispatches,
		localReadySeconds: ready,
		version:           "test",
		device:            "device-under-test",
	}

	// Zero dispatches is the value that matters most and the one most likely to
	// be optimised away by a future "do not record empty sessions" change.
	// Zero dispatches with traffic: the client saw the worker and declined,
	// which is the case the attribute exists to distinguish.
	tel.RecordLocalSession("opencode", "some/model", 0, true)
	// Both outcomes, because the failing one is the reason the attribute
	// exists: recorded only on success, this histogram cannot see the starts
	// that hung, and its slow tail is missing by construction.
	tel.RecordLocalReadySeconds("some/model", "ready", 4)
	tel.RecordLocalReadySeconds("some/model", "failed", 600)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			seen[m.Name] = true
		}
	}
	for _, want := range []string{
		"nav_pilot_local_dispatches",
		"nav_pilot_local_ready_seconds",
	} {
		if !seen[want] {
			t.Errorf("the recorder never emitted %s; collected %v", want, seen)
		}
	}

	// The outcome attribute has to reach the data point, not just the call.
	// Without it the histogram is a distribution of the starts that worked,
	// which is how the docs came to quote a startup time the fleet contradicts.
	outcomes := map[string]bool{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "nav_pilot_local_ready_seconds" {
				continue
			}
			hist, ok := m.Data.(metricdata.Histogram[int64])
			if !ok {
				t.Fatalf("nav_pilot_local_ready_seconds is %T, want a histogram", m.Data)
			}
			for _, dp := range hist.DataPoints {
				v, found := dp.Attributes.Value(attribute.Key("outcome"))
				if !found {
					t.Errorf("a ready_seconds point carries no outcome attribute: %v", dp.Attributes.ToSlice())
					continue
				}
				outcomes[v.AsString()] = true
			}
		}
	}
	for _, want := range []string{"ready", "failed"} {
		if !outcomes[want] {
			t.Errorf("no ready_seconds point with outcome=%q; got %v", want, outcomes)
		}
	}
}

// TestEverythingIsCumulative pins the export shape after the delta experiment.
//
// This does not protect a preference. Every delta counter nav-pilot emitted was
// lost downstream while cumulative histograms from the same processes arrived,
// so cumulative is the shape that demonstrably reaches the backend.
func TestEverythingIsCumulative(t *testing.T) {
	for _, kind := range []sdkmetric.InstrumentKind{
		sdkmetric.InstrumentKindCounter,
		sdkmetric.InstrumentKindHistogram,
		sdkmetric.InstrumentKindUpDownCounter,
	} {
		if got := temporalityFor(kind); got != metricdata.CumulativeTemporality {
			t.Errorf("kind %v exported as %v, want cumulative", kind, got)
		}
	}
}
