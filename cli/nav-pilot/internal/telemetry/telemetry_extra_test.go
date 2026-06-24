package telemetry

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// newFullTestTelemetry builds an otelTelemetry with ALL instruments so every
// Record* method can be exercised without a nil-pointer panic.
func newFullTestTelemetry(t *testing.T) (*otelTelemetry, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test-full")

	counter := func(name string) metric.Int64Counter {
		c, err := meter.Int64Counter(name)
		if err != nil {
			t.Fatalf("create counter %s: %v", name, err)
		}
		return c
	}
	hist := func(name string) metric.Int64Histogram {
		h, err := meter.Int64Histogram(name)
		if err != nil {
			t.Fatalf("create histogram %s: %v", name, err)
		}
		return h
	}
	gauge := func(name string) metric.Int64Gauge {
		g, err := meter.Int64Gauge(name)
		if err != nil {
			t.Fatalf("create gauge %s: %v", name, err)
		}
		return g
	}

	tel := &otelTelemetry{
		provider:           provider,
		commandTotal:       counter("nav_pilot_command_total"),
		commandDurationMS:  hist("nav_pilot_command_duration_ms"),
		commandErrorTotal:  counter("nav_pilot_command_error_total"),
		rtkLaunchTotal:     counter("nav_pilot_rtk_launch_total"),
		installItemsTotal:  counter("nav_pilot_install_items_total"),
		syncUpdatesTotal:   counter("nav_pilot_sync_updates_total"),
		syncConflictsTotal: counter("nav_pilot_sync_conflicts_total"),
		infoGauge:          gauge("nav_pilot_info"),
		installPresent:     gauge("nav_pilot_install_present"),
		installedItems:     gauge("nav_pilot_installed_items"),
		configInfo:         gauge("nav_pilot_config_info"),
		clientAvailable:    gauge("nav_pilot_client_available"),
		stalenessCheck:     counter("nav_pilot_staleness_check_total"),
		upToDate:           gauge("nav_pilot_up_to_date"),
		versionSkewDays:    hist("nav_pilot_version_skew_days"),
		version:            "test",
		device:             "dev-1",
		executionContext:   "organic",
		os:                 "linux",
		arch:               "amd64",
	}
	return tel, reader
}

func collectSum(t *testing.T, reader *sdkmetric.ManualReader, name string) []metricdata.DataPoint[int64] {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			s, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric %s is not an int64 sum", name)
			}
			return s.DataPoints
		}
	}
	t.Fatalf("metric %s not found", name)
	return nil
}

func TestMaxInt64(t *testing.T) {
	tests := []struct {
		a, b, want int64
	}{
		{0, 0, 0},
		{5, 3, 5},
		{-1, 0, 0},
		{-5, -3, -3},
		{10, 10, 10},
	}
	for _, tt := range tests {
		if got := maxInt64(tt.a, tt.b); got != tt.want {
			t.Errorf("maxInt64(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestNormalizeTelemetryDimension(t *testing.T) {
	tests := []struct {
		v, fallback, want string
	}{
		{"", "unknown", "unknown"},
		{"  ", "unknown", "unknown"},
		{"install", "unknown", "install"},
		{"sync", "unknown", "sync"},
		{"success", "unknown", "success"},
		{"error", "unknown", "error"},
		{"active", "unknown", "active"},
		{"darwin", "unknown", "darwin"},
		{"amd64", "unknown", "amd64"},
		{"claude-opus-4.8", "unknown", "claude-opus-4.8"}, // contains dot
		{"openai/gpt-4o", "unknown", "openai/gpt-4o"},     // contains dot? no, slash
		{"justtext", "unknown", "unknown"},                // no dot or dash -> fallback
		{"with-dash", "unknown", "with-dash"},             // has dash
		{"organic", "unknown", "organic"},
		{"ci_github_actions", "unknown", "ci_github_actions"},
	}
	for _, tt := range tests {
		got := normalizeTelemetryDimension(tt.v, tt.fallback)
		if got != tt.want {
			t.Errorf("normalizeTelemetryDimension(%q, %q) = %q, want %q", tt.v, tt.fallback, got, tt.want)
		}
	}
}

// --- NoopRecorder smoke tests for all methods ---

func TestNoopRecorder_AllMethods(t *testing.T) {
	var r Recorder = NoopRecorder{}
	r.RecordCommand("install", "interactive", "repo", "success", 100*time.Millisecond)
	r.RecordInstallItems("repo", "interactive", 3)
	r.RecordSyncUpdates("repo", "non_interactive", 2)
	r.RecordSyncConflicts("user", "non_interactive", 1)
	r.RecordInstallPresent("repo", "nav-pilot", true)
	r.RecordInstalledItems("repo", "agent", "active", 5)
	r.RecordStalenessCheck("copilot", "repo", "up_to_date")
	r.RecordUpToDate("copilot", "repo", true)
	r.RecordVersionSkewDays("copilot", "repo", 0)
	r.RecordConfig("opencode", "default", "auto", "", "", "none", false, false)
	r.RecordClientAvailable("copilot", false)
	r.RecordRTKLaunch("copilot", "applied")
	if err := r.Shutdown(context.Background()); err != nil {
		t.Errorf("NoopRecorder.Shutdown() = %v, want nil", err)
	}
}

// --- otelTelemetry Record* smoke tests ---

func TestOtelRecordCommand_Success(t *testing.T) {
	tel, _ := newFullTestTelemetry(t)
	// should not panic
	tel.RecordCommand("install", "interactive", "repo", "success", 50*time.Millisecond)
	tel.RecordCommand("install", "interactive", "repo", "error", 50*time.Millisecond)
}

func TestOtelRecordInstallItems(t *testing.T) {
	tel, _ := newFullTestTelemetry(t)
	tel.RecordInstallItems("repo", "interactive", 3)
	tel.RecordInstallItems("repo", "interactive", 0)  // count <=0 noop
	tel.RecordInstallItems("repo", "interactive", -1) // count <=0 noop
}

func TestOtelRecordSyncUpdates(t *testing.T) {
	tel, _ := newFullTestTelemetry(t)
	tel.RecordSyncUpdates("repo", "non_interactive", 2)
	tel.RecordSyncUpdates("repo", "non_interactive", 0) // noop
}

func TestOtelRecordSyncConflicts(t *testing.T) {
	tel, _ := newFullTestTelemetry(t)
	tel.RecordSyncConflicts("user", "non_interactive", 1)
	tel.RecordSyncConflicts("user", "non_interactive", 0) // noop
}

func TestOtelRecordInstallPresent(t *testing.T) {
	tel, _ := newFullTestTelemetry(t)
	tel.RecordInstallPresent("repo", "nav-pilot", true)
	tel.RecordInstallPresent("user", "nav-pilot", false)
}

func TestOtelRecordInstalledItems(t *testing.T) {
	tel, _ := newFullTestTelemetry(t)
	tel.RecordInstalledItems("repo", "agent", "active", 5)
	tel.RecordInstalledItems("repo", "agent", "active", 0)  // zero is ok
	tel.RecordInstalledItems("repo", "agent", "active", -1) // negative: noop
}

func TestOtelRecordStalenessCheck(t *testing.T) {
	tel, _ := newFullTestTelemetry(t)
	tel.RecordStalenessCheck("copilot", "repo", "up_to_date")
	tel.RecordStalenessCheck("opencode", "user", "stale")
}

func TestOtelRecordUpToDate(t *testing.T) {
	tel, _ := newFullTestTelemetry(t)
	tel.RecordUpToDate("copilot", "repo", true)
	tel.RecordUpToDate("copilot", "repo", false)
}

func TestOtelRecordVersionSkewDays(t *testing.T) {
	tel, _ := newFullTestTelemetry(t)
	tel.RecordVersionSkewDays("copilot", "repo", 7)
	tel.RecordVersionSkewDays("copilot", "repo", -1) // clamped to 0
}

func TestOtelRecordRTKLaunch(t *testing.T) {
	tel, reader := newFullTestTelemetry(t)
	tel.RecordRTKLaunch("copilot", "applied")
	tel.RecordRTKLaunch("copilot", "not_enabled")
	tel.RecordRTKLaunch("opencode", "non_interactive")
	tel.RecordRTKLaunch("pi", "rtk_missing")
	tel.RecordRTKLaunch("pi", "unexpected")

	dps := collectSum(t, reader, "nav_pilot_rtk_launch_total")
	if len(dps) != 5 {
		t.Fatalf("expected 5 datapoints, got %d", len(dps))
	}

	want := map[string]map[string]string{
		"copilot/applied": {
			"client":            "copilot",
			"result":            "applied",
			"version":           "test",
			"execution_context": "organic",
		},
		"copilot/not_enabled": {
			"client":            "copilot",
			"result":            "not_enabled",
			"version":           "test",
			"execution_context": "organic",
		},
		"opencode/non_interactive": {
			"client":            "opencode",
			"result":            "non_interactive",
			"version":           "test",
			"execution_context": "organic",
		},
		"pi/rtk_missing": {
			"client":            "pi",
			"result":            "rtk_missing",
			"version":           "test",
			"execution_context": "organic",
		},
		"pi/unknown": {
			"client":            "pi",
			"result":            "unknown",
			"version":           "test",
			"execution_context": "organic",
		},
	}

	for _, dp := range dps {
		client, _ := dp.Attributes.Value("client")
		result, _ := dp.Attributes.Value("result")
		key := client.AsString() + "/" + result.AsString()
		labels, ok := want[key]
		if !ok {
			t.Fatalf("unexpected datapoint labels: %s", key)
		}
		if dp.Value != 1 {
			t.Fatalf("datapoint %s has value %d, want 1", key, dp.Value)
		}
		for labelKey, labelWant := range labels {
			got, ok := dp.Attributes.Value(attribute.Key(labelKey))
			if !ok {
				t.Fatalf("datapoint %s missing label %q", key, labelKey)
			}
			if got.AsString() != labelWant {
				t.Fatalf("datapoint %s label %q = %q, want %q", key, labelKey, got.AsString(), labelWant)
			}
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing datapoints: %v", want)
	}
}

func TestNormalizeRTKLaunchResult(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "applied", want: "applied"},
		{in: "not_enabled", want: "not_enabled"},
		{in: "non_interactive", want: "non_interactive"},
		{in: "rtk_missing", want: "rtk_missing"},
		{in: " Applied ", want: "applied"},
		{in: "bogus", want: "unknown"},
		{in: "", want: "unknown"},
	}

	for _, tt := range tests {
		if got := normalizeRTKLaunchResult(tt.in); got != tt.want {
			t.Errorf("normalizeRTKLaunchResult(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOtelShutdown(t *testing.T) {
	tel, _ := newFullTestTelemetry(t)
	if err := tel.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown() = %v, want nil", err)
	}
}

func TestOtelRecordInfo(t *testing.T) {
	tel, _ := newFullTestTelemetry(t)
	// should not panic
	tel.recordInfo()
}

func TestInitTelemetry_Disabled(t *testing.T) {
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "0")
	r, err := InitTelemetry(context.Background(), "dev")
	if err != nil {
		t.Fatalf("InitTelemetry(disabled) = %v, want nil", err)
	}
	if _, ok := r.(NoopRecorder); !ok {
		t.Error("InitTelemetry(disabled) should return NoopRecorder")
	}
}

func TestCopilotOTelEndpointConfigured(t *testing.T) {
	if !CopilotOTelEndpointConfigured([]string{}) {
		t.Error("CopilotOTelEndpointConfigured(empty) = false; default endpoint always makes it true")
	}
}

func TestApplyOpenCodeOTelEnv(t *testing.T) {
	env, changed := ApplyOpenCodeOTelEnv([]string{}, "dev")
	if !changed {
		t.Fatal("expected env to be changed")
	}
	if v := LookupEnvValue(env, "OPENCODE_CLIENT"); v != "nav-pilot" {
		t.Errorf("OPENCODE_CLIENT = %q, want nav-pilot", v)
	}
	if got := LookupEnvValue(env, "OTEL_LOGS_EXPORTER"); got != "none" {
		t.Errorf("OTEL_LOGS_EXPORTER = %q, want none", got)
	}
}
