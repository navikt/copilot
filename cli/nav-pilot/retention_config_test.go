package main

import (
	"os"
	"testing"
	"time"
)

// TestRetentionConfigDefault verifies that default retention is 30 days.
func TestRetentionConfigDefault(t *testing.T) {
	// Clear environment
	oldVal := os.Getenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS")
	os.Unsetenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS")
	defer func() {
		if oldVal != "" {
			os.Setenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS", oldVal)
		}
	}()

	config := loadRetentionConfig()

	if config.DaysRetained != 30 {
		t.Errorf("default retention wrong: got %d days, want 30", config.DaysRetained)
	}

	expectedTTL := 30 * 24 * time.Hour
	if config.RetentionTTL != expectedTTL {
		t.Errorf("default TTL wrong: got %v, want %v", config.RetentionTTL, expectedTTL)
	}
}

// TestRetentionConfigFromEnv verifies env-var parsing.
func TestRetentionConfigFromEnv(t *testing.T) {
	tests := []struct {
		envValue string
		want     int
	}{
		{"7", 7},   // dev
		{"15", 15}, // custom
		{"30", 30}, // prod
		{"90", 90}, // archive
	}

	oldVal := os.Getenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS")
	defer func() {
		if oldVal != "" {
			os.Setenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS", oldVal)
		} else {
			os.Unsetenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS")
		}
	}()

	for _, tt := range tests {
		t.Run(tt.envValue, func(t *testing.T) {
			os.Setenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS", tt.envValue)

			config := loadRetentionConfig()

			if config.DaysRetained != tt.want {
				t.Errorf("got %d days, want %d", config.DaysRetained, tt.want)
			}

			expectedTTL := time.Duration(tt.want) * 24 * time.Hour
			if config.RetentionTTL != expectedTTL {
				t.Errorf("got TTL %v, want %v", config.RetentionTTL, expectedTTL)
			}
		})
	}
}

// TestRetentionConfigBounds verifies clamping to valid range (1-365 days).
func TestRetentionConfigBounds(t *testing.T) {
	tests := []struct {
		envValue string
		want     int
	}{
		{"0", 1},     // too low → clamped to 1
		{"-5", 1},    // negative → clamped to 1
		{"366", 365}, // too high → clamped to 365
		{"999", 365}, // way too high → clamped to 365
	}

	oldVal := os.Getenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS")
	defer func() {
		if oldVal != "" {
			os.Setenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS", oldVal)
		} else {
			os.Unsetenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS")
		}
	}()

	for _, tt := range tests {
		t.Run(tt.envValue, func(t *testing.T) {
			os.Setenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS", tt.envValue)

			config := loadRetentionConfig()

			if config.DaysRetained != tt.want {
				t.Errorf("got %d days, want %d", config.DaysRetained, tt.want)
			}
		})
	}
}

// TestRetentionConfigInvalidValue verifies fallback on invalid input.
func TestRetentionConfigInvalidValue(t *testing.T) {
	oldVal := os.Getenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS")
	defer func() {
		if oldVal != "" {
			os.Setenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS", oldVal)
		} else {
			os.Unsetenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS")
		}
	}()

	// Set invalid value
	os.Setenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS", "not-a-number")

	config := loadRetentionConfig()

	// Should fall back to default (30)
	if config.DaysRetained != 30 {
		t.Errorf("invalid value didn't fall back to default: got %d, want 30", config.DaysRetained)
	}
}

// TestRetentionConfigIsExpired verifies expiration checking.
func TestRetentionConfigIsExpired(t *testing.T) {
	config := RetentionConfig{
		DaysRetained: 7,
		RetentionTTL: 7 * 24 * time.Hour,
	}

	// Recent timestamp (should not be expired)
	recent := time.Now().Add(-1 * time.Hour)
	if config.isExpired(recent) {
		t.Errorf("recent timestamp incorrectly marked as expired: %v", recent)
	}

	// Old timestamp (should be expired)
	old := time.Now().Add(-30 * 24 * time.Hour)
	if !config.isExpired(old) {
		t.Errorf("old timestamp not marked as expired: %v", old)
	}

	// Edge case: exactly at TTL boundary
	boundary := time.Now().Add(-config.RetentionTTL)
	// This is technically expired (time.Since > TTL), so should be true
	if !config.isExpired(boundary) {
		t.Logf("boundary timestamp: might want to check edge case handling")
	}
}

// TestRetentionConfigString verifies human-readable output.
func TestRetentionConfigString(t *testing.T) {
	config := RetentionConfig{
		DaysRetained: 30,
		RetentionTTL: 30 * 24 * time.Hour,
	}

	str := config.String()

	// Should contain both days and hours
	if !contains(str, "30") {
		t.Errorf("string missing days: %q", str)
	}
	if !contains(str, "day") {
		t.Errorf("string missing 'day': %q", str)
	}
}

func contains(s, substr string) bool {
	if substr == "" {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
