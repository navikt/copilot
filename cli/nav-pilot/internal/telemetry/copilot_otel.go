package telemetry

import "strings"

const copilotOTelEndpointOverride = "NAV_PILOT_COPILOT_OTEL_ENDPOINT"

func ApplyCopilotOTelEnv(env []string, cliVersion string) ([]string, bool) {
	changed := false
	endpoint := copilotOTelEndpoint(env)
	if endpoint == "" {
		return env, false
	}

	var updated bool
	env, updated = SetEnvValue(env, "OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)
	changed = changed || updated

	env, updated = SetEnvIfAbsent(env, "COPILOT_OTEL_ENABLED", "true")
	changed = changed || updated

	// device_id is the only pseudonymous identifier we inject; honour the
	// nav-pilot telemetry opt-out so an opted-out user is not re-identified
	// through Copilot's telemetry. launcher/version are non-identifying and
	// are always included.
	deviceID := ""
	if TelemetryEnabled() {
		deviceID = CopilotDeviceID()
	}
	env, updated = applyCopilotResourceAttributes(env, normalizeTelemetryDimension(cliVersion, "dev"), deviceID)
	changed = changed || updated

	return env, changed
}

// ApplyOpenCodeOTelEnv injects OTel env vars for opencode, reusing the same
// approach as ApplyCopilotOTelEnv. Also sets OPENCODE_CLIENT=nav-pilot.
func ApplyOpenCodeOTelEnv(env []string, cliVersion string) ([]string, bool) {
	changed := false
	endpoint := copilotOTelEndpoint(env)
	if endpoint == "" {
		return env, false
	}

	var updated bool
	env, updated = SetEnvValue(env, "OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)
	changed = changed || updated

	deviceID := ""
	if TelemetryEnabled() {
		deviceID = CopilotDeviceID()
	}
	env, updated = applyCopilotResourceAttributes(env, normalizeTelemetryDimension(cliVersion, "dev"), deviceID)
	changed = changed || updated

	env, updated = SetEnvIfAbsent(env, "OPENCODE_CLIENT", "nav-pilot")
	changed = changed || updated

	return env, changed
}

func CopilotOTelEndpointConfigured(env []string) bool {
	return copilotOTelEndpoint(env) != ""
}

// CopilotDeviceID resolves the pseudonymous nav-pilot device identifier for
// use as a Copilot resource attribute, falling back to "unknown" when it
// cannot be computed (mirrors the nav-pilot telemetry fallback).
func CopilotDeviceID() string {
	id, err := GetOrCreateDeviceID()
	if err != nil || strings.TrimSpace(id) == "" {
		return "unknown"
	}
	return id
}

// applyCopilotResourceAttributes merges nav-pilot context into Copilot's
// OTEL_RESOURCE_ATTRIBUTES so Copilot's spans and metrics can be attributed
// back to nav-pilot. It only appends keys that are not already present, so
// values set by the user or CI environment are never overwritten.
func applyCopilotResourceAttributes(env []string, version, deviceID string) ([]string, bool) {
	pairs := []struct{ key, value string }{
		{"nav.pilot.launcher", "nav-pilot"},
		{"nav.pilot.version", strings.TrimSpace(version)},
		{"nav.pilot.device_id", strings.TrimSpace(deviceID)},
	}

	existing := LookupEnvValue(env, "OTEL_RESOURCE_ATTRIBUTES")
	present := resourceAttributeKeys(existing)

	additions := make([]string, 0, len(pairs))
	for _, p := range pairs {
		if p.value == "" {
			continue
		}
		if _, ok := present[p.key]; ok {
			continue
		}
		additions = append(additions, p.key+"="+encodeResourceAttrValue(p.value))
	}
	if len(additions) == 0 {
		return env, false
	}

	merged := strings.Join(additions, ",")
	if trimmed := strings.TrimRight(strings.TrimSpace(existing), ","); trimmed != "" {
		merged = trimmed + "," + merged
	}

	env, _ = SetEnvValue(env, "OTEL_RESOURCE_ATTRIBUTES", merged)
	return env, true
}

// resourceAttributeKeys returns the set of keys already declared in an
// OTEL_RESOURCE_ATTRIBUTES value (comma-separated key=value pairs).
func resourceAttributeKeys(raw string) map[string]struct{} {
	keys := make(map[string]struct{})
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key := pair
		if i := strings.IndexByte(pair, '='); i >= 0 {
			key = strings.TrimSpace(pair[:i])
		}
		if key != "" {
			keys[key] = struct{}{}
		}
	}
	return keys
}

// encodeResourceAttrValue percent-encodes any character outside the
// unreserved set so commas, spaces or equals signs in a value cannot
// corrupt the OTEL_RESOURCE_ATTRIBUTES list.
func encodeResourceAttrValue(value string) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if strings.IndexByte(unreserved, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0F])
	}
	return b.String()
}

func copilotOTelEndpoint(env []string) string {
	if endpoint := normalizeCopilotOTelEndpoint(LookupEnvValue(env, copilotOTelEndpointOverride)); endpoint != "" {
		return endpoint
	}
	if endpoint := normalizeCopilotOTelEndpoint(LookupEnvValue(env, "OTEL_EXPORTER_OTLP_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	if endpoint := normalizeCopilotOTelEndpoint(LookupEnvValue(env, "NAV_PILOT_TELEMETRY_ENDPOINT")); endpoint != "" {
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

func LookupEnvValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix)
		}
	}
	return ""
}

func SetEnvValue(env []string, key, value string) ([]string, bool) {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			newValue := key + "=" + value
			if env[i] == newValue {
				return env, false
			}
			env[i] = newValue
			return env, true
		}
	}
	return append(env, key+"="+value), true
}

func SetEnvIfAbsent(env []string, key, value string) ([]string, bool) {
	if LookupEnvValue(env, key) != "" {
		return env, false
	}
	return append(env, key+"="+value), true
}
