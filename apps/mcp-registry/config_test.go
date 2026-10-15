package main

import (
	"log/slog"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOGGED_ENDPOINTS", "")

	config := loadConfig()

	if config.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", config.Port)
	}

	if config.LogLevel != slog.LevelInfo {
		t.Errorf("expected default log level INFO, got %s", config.LogLevel.String())
	}

	expectedEndpoints := map[string]bool{
		"/v0.1/servers": true,
	}

	if len(config.LoggedEndpoints) != len(expectedEndpoints) {
		t.Errorf("expected %d logged endpoints, got %d", len(expectedEndpoints), len(config.LoggedEndpoints))
	}

	for endpoint := range expectedEndpoints {
		if !config.LoggedEndpoints[endpoint] {
			t.Errorf("expected endpoint %s to be logged by default", endpoint)
		}
	}
}

func TestLoadConfig_CustomPort(t *testing.T) {
	t.Setenv("PORT", "3000")

	config := loadConfig()

	if config.Port != "3000" {
		t.Errorf("expected port 3000, got %s", config.Port)
	}
}

func TestLoadConfig_LogLevels(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectedLevel slog.Level
	}{
		{"debug lowercase", "debug", slog.LevelDebug},
		{"debug uppercase", "DEBUG", slog.LevelDebug},
		{"info lowercase", "info", slog.LevelInfo},
		{"info uppercase", "INFO", slog.LevelInfo},
		{"warn lowercase", "warn", slog.LevelWarn},
		{"warn uppercase", "WARN", slog.LevelWarn},
		{"warning", "WARNING", slog.LevelWarn},
		{"error lowercase", "error", slog.LevelError},
		{"error uppercase", "ERROR", slog.LevelError},
		{"invalid defaults to info", "invalid", slog.LevelInfo},
		{"empty defaults to info", "", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tt.envValue)

			config := loadConfig()

			if config.LogLevel != tt.expectedLevel {
				t.Errorf("expected log level %s, got %s", tt.expectedLevel.String(), config.LogLevel.String())
			}
		})
	}
}

func TestLoadConfig_CustomEndpoints(t *testing.T) {
	tests := []struct {
		name              string
		envValue          string
		setEnv            bool
		expectedEndpoints map[string]bool
	}{
		{
			name:     "single endpoint",
			envValue: "/health",
			setEnv:   true,
			expectedEndpoints: map[string]bool{
				"/health": true,
			},
		},
		{
			name:     "multiple endpoints",
			envValue: "/health,/ready,/metrics",
			setEnv:   true,
			expectedEndpoints: map[string]bool{
				"/health":  true,
				"/ready":   true,
				"/metrics": true,
			},
		},
		{
			name:     "endpoints with spaces",
			envValue: "/health, /ready , /metrics",
			setEnv:   true,
			expectedEndpoints: map[string]bool{
				"/health":  true,
				"/ready":   true,
				"/metrics": true,
			},
		},
		{
			name:     "default endpoints when not set",
			envValue: "",
			setEnv:   false,
			expectedEndpoints: map[string]bool{
				"/v0.1/servers": true,
			},
		},
		{
			name:     "all endpoints",
			envValue: "/,/health,/ready,/metrics,/allowlist,/mcp-config",
			setEnv:   true,
			expectedEndpoints: map[string]bool{
				"/":           true,
				"/health":     true,
				"/ready":      true,
				"/metrics":    true,
				"/allowlist":  true,
				"/mcp-config": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv("LOGGED_ENDPOINTS", tt.envValue)
			} else {
				t.Setenv("LOGGED_ENDPOINTS", "")
			}

			config := loadConfig()

			if len(config.LoggedEndpoints) != len(tt.expectedEndpoints) {
				t.Errorf("expected %d logged endpoints, got %d", len(tt.expectedEndpoints), len(config.LoggedEndpoints))
			}

			for endpoint := range tt.expectedEndpoints {
				if !config.LoggedEndpoints[endpoint] {
					t.Errorf("expected endpoint %s to be logged", endpoint)
				}
			}

			for endpoint := range config.LoggedEndpoints {
				if !tt.expectedEndpoints[endpoint] {
					t.Errorf("unexpected endpoint %s in logged endpoints", endpoint)
				}
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "returns env value when set",
			key:          "TEST_KEY",
			defaultValue: "default",
			envValue:     "custom",
			expected:     "custom",
		},
		{
			name:         "returns default when env not set",
			key:          "TEST_KEY_UNSET",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "returns empty string when both empty",
			key:          "TEST_KEY_EMPTY",
			defaultValue: "",
			envValue:     "",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			}

			result := getEnv(tt.key, tt.defaultValue)

			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetEndpointsList(t *testing.T) {
	tests := []struct {
		name      string
		endpoints map[string]bool
		expected  int
	}{
		{
			name:      "empty map",
			endpoints: map[string]bool{},
			expected:  0,
		},
		{
			name: "single endpoint",
			endpoints: map[string]bool{
				"/health": true,
			},
			expected: 1,
		},
		{
			name: "multiple endpoints",
			endpoints: map[string]bool{
				"/health":  true,
				"/ready":   true,
				"/metrics": true,
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getEndpointsList(tt.endpoints)

			if len(result) != tt.expected {
				t.Errorf("expected %d endpoints, got %d", tt.expected, len(result))
			}

			for endpoint := range tt.endpoints {
				found := false
				for _, e := range result {
					if e == endpoint {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("endpoint %s not found in result list", endpoint)
				}
			}
		})
	}
}
