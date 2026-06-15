package main

import (
	"strings"
)

const copilotOTelEndpointOverride = "NAV_PILOT_COPILOT_OTEL_ENDPOINT"

func applyCopilotOTelEnv(env []string) ([]string, bool) {
	changed := false
	endpoint := copilotOTelEndpoint(env)
	if endpoint == "" {
		return env, false
	}

	var updated bool
	env, updated = setEnvValue(env, "OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)
	changed = changed || updated

	env, updated = setEnvIfAbsent(env, "COPILOT_OTEL_ENABLED", "true")
	changed = changed || updated

	return env, changed
}

func copilotOTelEndpoint(env []string) string {
	if endpoint := normalizeCopilotOTelEndpoint(lookupEnvValue(env, copilotOTelEndpointOverride)); endpoint != "" {
		return endpoint
	}
	if endpoint := normalizeCopilotOTelEndpoint(lookupEnvValue(env, "OTEL_EXPORTER_OTLP_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	if endpoint := normalizeCopilotOTelEndpoint(lookupEnvValue(env, "NAV_PILOT_TELEMETRY_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	return normalizeCopilotOTelEndpoint(defaultTelemetryEndpoint)
}

func normalizeCopilotOTelEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}

	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/v1/metrics") {
		return strings.TrimSuffix(endpoint, "/v1/metrics")
	}
	return endpoint
}

func setEnvIfAbsent(env []string, key, value string) ([]string, bool) {
	if lookupEnvValue(env, key) != "" {
		return env, false
	}
	return append(env, key+"="+value), true
}
