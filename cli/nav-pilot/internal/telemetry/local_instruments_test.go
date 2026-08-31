package telemetry

import (
	"context"
	"testing"

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
	serverTotal, err := meter.Int64Counter("nav_pilot_local_server_total")
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
		localServerTotal:  serverTotal,
		localReadySeconds: ready,
		version:           "test",
		device:            "device-under-test",
	}

	// Zero dispatches is the value that matters most and the one most likely to
	// be optimised away by a future "do not record empty sessions" change.
	tel.RecordLocalSession("opencode", "some/model", 0)
	tel.RecordLocalServer("some/model", "ready")
	tel.RecordLocalReadySeconds("some/model", 4)

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
		"nav_pilot_local_server_total",
		"nav_pilot_local_ready_seconds",
	} {
		if !seen[want] {
			t.Errorf("the recorder never emitted %s; collected %v", want, seen)
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
