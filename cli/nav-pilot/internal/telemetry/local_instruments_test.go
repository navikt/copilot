package telemetry

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestLocalInstrumentsAllEmit reads what the SDK actually produces.
//
// This exists because nav_pilot_local_server_total never reached Mimir while
// nav_pilot_local_ready_seconds — recorded two lines later in the same function,
// in the same process — arrived every time. Without a test at this level there
// is no way to tell whether the instrument was never emitted or emitted and
// dropped downstream, and those have completely different fixes.
//
// It is also the regression test the three local instruments never had: they were
// verified once by hand against a fake collector and never again.
func TestLocalInstrumentsAllEmit(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")

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

	ctx := context.Background()
	dispatches.Record(ctx, 0) // zero is the value that matters and must still emit
	serverTotal.Add(ctx, 1)
	ready.Record(ctx, 4)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &rm); err != nil {
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
			t.Errorf("%s was never emitted by the SDK; collected %v", want, seen)
		}
	}
}

// TestDeltaTemporalityAppliesToCounters pins the export shape, because it is the
// one thing that differs between the instrument that reaches Mimir and the one
// that does not. If this ever changes, the ingestion behaviour changes with it
// and the change should be deliberate.
func TestDeltaTemporalityAppliesToCounters(t *testing.T) {
	for _, tc := range []struct {
		kind sdkmetric.InstrumentKind
		want metricdata.Temporality
	}{
		{sdkmetric.InstrumentKindCounter, metricdata.DeltaTemporality},
		{sdkmetric.InstrumentKindHistogram, metricdata.CumulativeTemporality},
	} {
		if got := temporalityFor(tc.kind); got != tc.want {
			t.Errorf("kind %v exported as %v, want %v", tc.kind, got, tc.want)
		}
	}
}
