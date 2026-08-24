package agentpakke

import (
	"regexp"
	"strings"
)

// releaseVersionPattern is nav-pilot's release version format: the date-time
// core that makes versions comparable, plus the build sha the release tooling
// appends. It mirrors the "minNavPilotVersion" pattern in the published schema
// (schemas/agentpakke-v1.json) — keep the two in sync, since a manifest is
// checked by both this binary and an agentpakke repo's own schema lint.
var releaseVersionPattern = regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}-\d{6}(-[0-9a-zA-Z][0-9a-zA-Z.-]*)?$`)

// isReleaseVersionFormat reports whether v is a well-formed nav-pilot release
// version. It is the check applied to a manifest-declared minimum, which is a
// known construct and therefore fails closed when malformed — a value like a
// bare date compares as older than every real release and would silently
// disable the gate.
func isReleaseVersionFormat(v string) bool {
	return releaseVersionPattern.MatchString(v)
}

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
