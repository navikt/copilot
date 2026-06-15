package main

import (
	"strings"
	"testing"
)

func TestNormalizeCopilotOTelEndpoint(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "base endpoint unchanged", in: "https://collector.nav.no", want: "https://collector.nav.no"},
		{name: "metrics suffix removed", in: "https://collector.nav.no/v1/metrics", want: "https://collector.nav.no"},
		{name: "metrics suffix removed with trailing slash", in: "https://collector.nav.no/v1/metrics/", want: "https://collector.nav.no"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeCopilotOTelEndpoint(tt.in); got != tt.want {
				t.Fatalf("normalizeCopilotOTelEndpoint(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCopilotOTelEndpointPrecedence(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		want string
	}{
		{
			name: "copilot endpoint override wins over generic otel endpoint",
			env:  []string{"NAV_PILOT_COPILOT_OTEL_ENDPOINT=https://copilot.example/v1/metrics", "OTEL_EXPORTER_OTLP_ENDPOINT=https://user.example/v1/metrics"},
			want: "https://copilot.example",
		},
		{
			name: "nav pilot telemetry endpoint fallback",
			env:  []string{"NAV_PILOT_TELEMETRY_ENDPOINT=https://shared.example/v1/metrics"},
			want: "https://shared.example",
		},
		{
			name: "default telemetry endpoint fallback",
			env:  []string{},
			want: "https://collector-internet.nav.cloud.nais.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := copilotOTelEndpoint(tt.env); got != tt.want {
				t.Fatalf("copilotOTelEndpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyCopilotOTelEnv(t *testing.T) {
	t.Run("sets endpoint and enabled when absent", func(t *testing.T) {
		env, changed := applyCopilotOTelEnv([]string{})
		if !changed {
			t.Fatal("expected env to be changed")
		}
		if got := lookupEnvValue(env, "OTEL_EXPORTER_OTLP_ENDPOINT"); got != "https://collector-internet.nav.cloud.nais.io" {
			t.Fatalf("OTEL_EXPORTER_OTLP_ENDPOINT = %q, want https://collector-internet.nav.cloud.nais.io", got)
		}
		if got := lookupEnvValue(env, "COPILOT_OTEL_ENABLED"); got != "true" {
			t.Fatalf("COPILOT_OTEL_ENABLED = %q, want true", got)
		}
	})

	t.Run("preserves existing otel endpoint", func(t *testing.T) {
		envIn := []string{"OTEL_EXPORTER_OTLP_ENDPOINT=https://already.example/v1/metrics"}
		envOut, changed := applyCopilotOTelEnv(envIn)
		if !changed {
			t.Fatal("expected env to be changed because endpoint is normalized and COPILOT_OTEL_ENABLED is added")
		}
		if got := lookupEnvValue(envOut, "OTEL_EXPORTER_OTLP_ENDPOINT"); got != "https://already.example" {
			t.Fatalf("OTEL_EXPORTER_OTLP_ENDPOINT = %q, want https://already.example", got)
		}
	})

	t.Run("respects existing copilot otel enabled setting", func(t *testing.T) {
		envIn := []string{"COPILOT_OTEL_ENABLED=false"}
		envOut, changed := applyCopilotOTelEnv(envIn)
		if !changed {
			t.Fatal("expected env to be changed because endpoint is added")
		}
		if got := lookupEnvValue(envOut, "COPILOT_OTEL_ENABLED"); got != "false" {
			t.Fatalf("COPILOT_OTEL_ENABLED = %q, want false", got)
		}
	})

	t.Run("injects nav-pilot resource attributes", func(t *testing.T) {
		envOut, _ := applyCopilotOTelEnv([]string{})
		got := lookupEnvValue(envOut, "OTEL_RESOURCE_ATTRIBUTES")
		if !strings.Contains(got, "nav.pilot.launcher=nav-pilot") {
			t.Fatalf("OTEL_RESOURCE_ATTRIBUTES = %q, want nav.pilot.launcher=nav-pilot", got)
		}
		if !strings.Contains(got, "nav.pilot.version=") {
			t.Fatalf("OTEL_RESOURCE_ATTRIBUTES = %q, want nav.pilot.version", got)
		}
		if !strings.Contains(got, "nav.pilot.device_id=") {
			t.Fatalf("OTEL_RESOURCE_ATTRIBUTES = %q, want nav.pilot.device_id", got)
		}
	})

	t.Run("omits device_id when telemetry is opted out", func(t *testing.T) {
		t.Setenv("NAV_PILOT_TELEMETRY_ENABLED", "false")
		envOut, _ := applyCopilotOTelEnv([]string{})
		got := lookupEnvValue(envOut, "OTEL_RESOURCE_ATTRIBUTES")
		if strings.Contains(got, "nav.pilot.device_id=") {
			t.Fatalf("device_id should be omitted when opted out: %q", got)
		}
		if !strings.Contains(got, "nav.pilot.launcher=nav-pilot") {
			t.Fatalf("launcher should still be present: %q", got)
		}
	})
}

func TestApplyCopilotResourceAttributes(t *testing.T) {
	t.Run("appends to existing attributes without clobbering", func(t *testing.T) {
		envIn := []string{"OTEL_RESOURCE_ATTRIBUTES=team=foo,nav.pilot.version=9.9.9"}
		envOut, changed := applyCopilotResourceAttributes(envIn, "1.2.3", "nav-pilot-abc123")
		if !changed {
			t.Fatal("expected env to be changed")
		}
		got := lookupEnvValue(envOut, "OTEL_RESOURCE_ATTRIBUTES")
		if !strings.HasPrefix(got, "team=foo,nav.pilot.version=9.9.9,") {
			t.Fatalf("existing attributes not preserved: %q", got)
		}
		if !strings.Contains(got, "nav.pilot.launcher=nav-pilot") {
			t.Fatalf("missing launcher attribute: %q", got)
		}
		if !strings.Contains(got, "nav.pilot.device_id=nav-pilot-abc123") {
			t.Fatalf("missing device_id attribute: %q", got)
		}
		if strings.Count(got, "nav.pilot.version=") != 1 {
			t.Fatalf("user-set nav.pilot.version was overwritten: %q", got)
		}
	})

	t.Run("skips empty values", func(t *testing.T) {
		envOut, changed := applyCopilotResourceAttributes([]string{}, "", "")
		if !changed {
			t.Fatal("expected launcher attribute to be added")
		}
		got := lookupEnvValue(envOut, "OTEL_RESOURCE_ATTRIBUTES")
		if got != "nav.pilot.launcher=nav-pilot" {
			t.Fatalf("OTEL_RESOURCE_ATTRIBUTES = %q, want only launcher", got)
		}
	})

	t.Run("percent-encodes unsafe characters", func(t *testing.T) {
		envOut, _ := applyCopilotResourceAttributes([]string{}, "1.0 beta,rc=1", "id")
		got := lookupEnvValue(envOut, "OTEL_RESOURCE_ATTRIBUTES")
		if !strings.Contains(got, "nav.pilot.version=1.0%20beta%2Crc%3D1") {
			t.Fatalf("value not percent-encoded: %q", got)
		}
	})

	t.Run("is idempotent across relaunch", func(t *testing.T) {
		env1, changed1 := applyCopilotResourceAttributes([]string{}, "1.2.3", "nav-pilot-abc123")
		if !changed1 {
			t.Fatal("expected first call to change env")
		}
		env2, changed2 := applyCopilotResourceAttributes(env1, "1.2.3", "nav-pilot-abc123")
		if changed2 {
			t.Fatal("expected second call to be a no-op")
		}
		if lookupEnvValue(env1, "OTEL_RESOURCE_ATTRIBUTES") != lookupEnvValue(env2, "OTEL_RESOURCE_ATTRIBUTES") {
			t.Fatal("relaunch changed the resource attributes")
		}
	})

	t.Run("recognises a bare existing key without value", func(t *testing.T) {
		envIn := []string{"OTEL_RESOURCE_ATTRIBUTES=nav.pilot.launcher"}
		envOut, _ := applyCopilotResourceAttributes(envIn, "1.2.3", "nav-pilot-abc123")
		got := lookupEnvValue(envOut, "OTEL_RESOURCE_ATTRIBUTES")
		if strings.Contains(got, "nav.pilot.launcher=nav-pilot") {
			t.Fatalf("bare existing key should not be re-added with a value: %q", got)
		}
	})

	t.Run("tolerates whitespace and trailing commas in existing value", func(t *testing.T) {
		envIn := []string{"OTEL_RESOURCE_ATTRIBUTES= team = foo ,"}
		envOut, changed := applyCopilotResourceAttributes(envIn, "1.2.3", "nav-pilot-abc123")
		if !changed {
			t.Fatal("expected env to be changed")
		}
		got := lookupEnvValue(envOut, "OTEL_RESOURCE_ATTRIBUTES")
		if strings.Contains(got, ",,") || strings.HasSuffix(got, ",") {
			t.Fatalf("malformed merged value: %q", got)
		}
		if !strings.Contains(got, "nav.pilot.launcher=nav-pilot") {
			t.Fatalf("missing launcher attribute: %q", got)
		}
	})
}

func TestEncodeResourceAttrValue(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "nav-pilot", want: "nav-pilot"},
		{in: "1.0.62-2", want: "1.0.62-2"},
		{in: "a,b", want: "a%2Cb"},
		{in: "a=b", want: "a%3Db"},
		{in: "a b", want: "a%20b"},
	}
	for _, tt := range tests {
		if got := encodeResourceAttrValue(tt.in); got != tt.want {
			t.Fatalf("encodeResourceAttrValue(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
