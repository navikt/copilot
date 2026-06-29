package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestTelemetryEnabled(t *testing.T) {
	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "")
	if !TelemetryEnabled() {
		t.Fatal("expected TelemetryEnabled to return true by default")
	}

	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "1")
	if !TelemetryEnabled() {
		t.Fatal("expected TelemetryEnabled to return true for 1")
	}

	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "0")
	if TelemetryEnabled() {
		t.Fatal("expected TelemetryEnabled to return false for 0")
	}

	t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "off")
	if TelemetryEnabled() {
		t.Fatal("expected TelemetryEnabled to return false for off")
	}
}

func TestNormalizeTelemetryDimension_AllowsStartupAndLaunch(t *testing.T) {
	if got := normalizeTelemetryDimension("startup", "unknown"); got != "startup" {
		t.Fatalf("normalizeTelemetryDimension(startup) = %q, want startup", got)
	}
	if got := normalizeTelemetryDimension("launch", "unknown"); got != "launch" {
		t.Fatalf("normalizeTelemetryDimension(launch) = %q, want launch", got)
	}
	for _, val := range []string{"network_error", "auth_error", "sync_failed"} {
		if got := normalizeTelemetryDimension(val, "unknown"); got != val {
			t.Fatalf("normalizeTelemetryDimension(%s) = %q, want %s", val, got, val)
		}
	}
}

func TestDetectExecutionContext(t *testing.T) {
	keys := []string{
		"NAV_PILOT_EXECUTION_CONTEXT",
		"GITHUB_ACTIONS",
		"CI",
		"GITLAB_CI",
		"JENKINS_URL",
		"BUILDKITE",
		"CIRCLECI",
		"TF_BUILD",
		"BUILD_ID",
	}

	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "override wins over github actions",
			env: map[string]string{
				"NAV_PILOT_EXECUTION_CONTEXT": "organic",
				"GITHUB_ACTIONS":              "true",
			},
			want: "organic",
		},
		{
			name: "github actions detected",
			env: map[string]string{
				"GITHUB_ACTIONS": "true",
			},
			want: "ci_github_actions",
		},
		{
			name: "generic ci detected",
			env: map[string]string{
				"CI": "true",
			},
			want: "ci_other",
		},
		{
			name: "generic ci env key detected",
			env: map[string]string{
				"GITLAB_CI": "1",
			},
			want: "ci_other",
		},
		{
			name: "unknown override allowed",
			env: map[string]string{
				"NAV_PILOT_EXECUTION_CONTEXT": "unknown",
			},
			want: "unknown",
		},
		{
			name: "invalid override falls back to github actions",
			env: map[string]string{
				"NAV_PILOT_EXECUTION_CONTEXT": "bogus",
				"GITHUB_ACTIONS":              "true",
			},
			want: "ci_github_actions",
		},
		{
			name: "organic by default",
			env:  map[string]string{},
			want: "organic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range keys {
				t.Setenv(key, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := detectExecutionContext(); got != tt.want {
				t.Fatalf("detectExecutionContext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOrUnset(t *testing.T) {
	if got := orUnset(""); got != "unset" {
		t.Errorf("orUnset(empty) = %q, want unset", got)
	}
	if got := orUnset("  "); got != "unset" {
		t.Errorf("orUnset(blank) = %q, want unset", got)
	}
	if got := orUnset("high"); got != "high" {
		t.Errorf("orUnset(high) = %q, want high", got)
	}
}

// newTestTelemetry builds an otelTelemetry backed by a ManualReader so tests
// can collect emitted metrics in-memory.
func newTestTelemetry(t *testing.T) (*otelTelemetry, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	meter := provider.Meter("test")
	gauge := func(name string) metric.Int64Gauge {
		g, err := meter.Int64Gauge(name)
		if err != nil {
			t.Fatalf("create gauge %s: %v", name, err)
		}
		return g
	}
	return &otelTelemetry{
		provider:         provider,
		configInfo:       gauge("nav_pilot_config_info"),
		clientAvailable:  gauge("nav_pilot_client_available"),
		version:          "test",
		device:           "dev-1",
		executionContext: "organic",
	}, reader
}

func collectGauge(t *testing.T, reader *sdkmetric.ManualReader, name string) []metricdata.DataPoint[int64] {
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
			g, ok := m.Data.(metricdata.Gauge[int64])
			if !ok {
				t.Fatalf("metric %s is not an int64 gauge", name)
			}
			return g.DataPoints
		}
	}
	return nil
}

func TestRecordConfig_EmitsLabels(t *testing.T) {
	tel, reader := newTestTelemetry(t)
	tel.RecordConfig("opencode", "autopilot", "claude-opus-4.8", "high", "", "debug", true, false)

	dps := collectGauge(t, reader, "nav_pilot_config_info")
	if len(dps) != 1 {
		t.Fatalf("expected 1 datapoint, got %d", len(dps))
	}
	if dps[0].Value != 1 {
		t.Fatalf("expected value 1, got %d", dps[0].Value)
	}
	want := map[string]string{
		"client":           "opencode",
		"config_mode":      "autopilot",
		"model":            "claude-opus-4.8",
		"reasoning_effort": "high",
		"context_tier":     "unset",
		"otel_log_level":   "debug",
		"allow_all_tools":  "true",
		"ask_user":         "false",
		"device_id":        "dev-1",
	}
	attrs := dps[0].Attributes
	for k, v := range want {
		got, ok := attrs.Value(attribute.Key(k))
		if !ok {
			t.Errorf("missing label %q", k)
			continue
		}
		if got.AsString() != v {
			t.Errorf("label %q = %q, want %q", k, got.AsString(), v)
		}
	}
}

func TestRecordClientAvailable_EmitsValue(t *testing.T) {
	tel, reader := newTestTelemetry(t)
	tel.RecordClientAvailable("copilot", true)
	tel.RecordClientAvailable("pi", false)

	dps := collectGauge(t, reader, "nav_pilot_client_available")
	got := map[string]int64{}
	for _, dp := range dps {
		c, _ := dp.Attributes.Value(attribute.Key("client"))
		got[c.AsString()] = dp.Value
	}
	if got["copilot"] != 1 {
		t.Errorf("copilot availability = %d, want 1", got["copilot"])
	}
	if got["pi"] != 0 {
		t.Errorf("pi availability = %d, want 0", got["pi"])
	}
}

func TestNoopRecordConfigAndClientAvailable(t *testing.T) {
	var rec Recorder = NoopRecorder{}
	rec.RecordConfig("copilot", "default", "", "", "", "none", false, true)
	rec.RecordClientAvailable("copilot", true)
}

func TestIsGenericCIEnvKeys(t *testing.T) {
	keys := []string{"GITLAB_CI", "JENKINS_URL", "BUILDKITE", "CIRCLECI", "TF_BUILD", "BUILD_ID"}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			t.Setenv("CI", "")
			t.Setenv("GITLAB_CI", "")
			t.Setenv("JENKINS_URL", "")
			t.Setenv("BUILDKITE", "")
			t.Setenv("CIRCLECI", "")
			t.Setenv("TF_BUILD", "")
			t.Setenv("BUILD_ID", "")
			t.Setenv(key, "1")
			if !isGenericCI() {
				t.Fatalf("expected isGenericCI() to return true when %s is set", key)
			}
		})
	}
}
