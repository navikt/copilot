package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// RetentionConfig holds telemetry retention policy for the current environment.
type RetentionConfig struct {
	DaysRetained int           // How many days to retain telemetry data
	RetentionTTL time.Duration // Equivalent duration for cache/context use
}

// loadRetentionConfig reads retention policy from environment or uses sensible defaults.
// Environment variable: NAV_PILOT_TELEMETRY_RETENTION_DAYS
//   - dev environment: 7 days (low overhead)
//   - prod environment: 30 days (standard)
//   - archive: 90 days (long-tail analysis)
//
// Default (if not set): 30 days
func loadRetentionConfig() RetentionConfig {
	// Try to read from environment
	retentionDaysStr := os.Getenv("NAV_PILOT_TELEMETRY_RETENTION_DAYS")
	if retentionDaysStr == "" {
		// No explicit setting; use default
		retentionDaysStr = "30"
	}

	// Parse as integer
	retentionDays, err := strconv.Atoi(retentionDaysStr)
	if err != nil {
		// Invalid value; use default
		debugLog("invalid NAV_PILOT_TELEMETRY_RETENTION_DAYS=%q; using default 30 days", retentionDaysStr)
		retentionDays = 30
	}

	// Sanity bounds: 1–365 days
	if retentionDays < 1 {
		debugLog("retention days too low (%d); clamping to 1", retentionDays)
		retentionDays = 1
	}
	if retentionDays > 365 {
		debugLog("retention days too high (%d); clamping to 365", retentionDays)
		retentionDays = 365
	}

	ttl := time.Duration(retentionDays) * 24 * time.Hour

	return RetentionConfig{
		DaysRetained: retentionDays,
		RetentionTTL: ttl,
	}
}

// String returns a human-readable representation of the retention policy.
func (r RetentionConfig) String() string {
	return fmt.Sprintf("%d days (~%.0fh)", r.DaysRetained, r.RetentionTTL.Hours())
}

// isExpired checks if a timestamp is older than the retention policy.
func (r RetentionConfig) isExpired(t time.Time) bool {
	return time.Since(t) > r.RetentionTTL
}
