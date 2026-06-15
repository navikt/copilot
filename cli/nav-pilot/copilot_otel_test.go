package main

import "testing"

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
}
