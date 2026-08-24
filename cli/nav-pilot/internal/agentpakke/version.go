package agentpakke

import "strings"

// cliVersion is the running binary version, injected by package main at startup
// (mirrors internal/provider's SetVersion seam). Development builds leave it at
// "dev", which disables the minNavPilotVersion gate.
var cliVersion = "dev"

// SetVersion sets the running CLI version used for minNavPilotVersion checks.
// Called once from main().
func SetVersion(v string) { cliVersion = v }

// versionTimestamp extracts the sortable date-time prefix of a nav-pilot
// release version: "2026.04.14-202800-a25f6c3" → "2026.04.14-202800".
//
// Deliberately duplicated from internal/artifacts rather than imported: this
// package is a dependency of the install/sync layers that artifacts belongs to,
// and importing upward would close a cycle once stage 2 wires the manifest into
// them.
func versionTimestamp(v string) string {
	parts := strings.SplitN(v, "-", 3)
	if len(parts) >= 2 {
		return parts[0] + "-" + parts[1]
	}
	return v
}

// isReleaseVersion reports whether a version string carries the comparable
// YYYY.MM.DD-HHMMSS release shape. Development and unset builds ("dev", "",
// "v0.0.0-…") do not, and are exempt from version gating.
func isReleaseVersion(v string) bool {
	ts := versionTimestamp(strings.TrimSpace(v))
	if len(ts) == 0 || ts[0] < '0' || ts[0] > '9' {
		return false
	}
	return strings.Contains(ts, "-")
}

// versionOlder reports whether running is strictly older than required.
// The release format is zero-padded and fixed-width, so lexicographic
// comparison of the date-time prefix is a correct ordering.
func versionOlder(running, required string) bool {
	return versionTimestamp(running) < versionTimestamp(required)
}
