package artifacts

import "strings"

// VersionTimestamp extracts the sortable date-time prefix from a version string.
// Version format: "2026.04.14-202800-a25f6c3" → "2026.04.14-202800"
func VersionTimestamp(v string) string {
	parts := strings.SplitN(v, "-", 3)
	if len(parts) >= 2 {
		return parts[0] + "-" + parts[1]
	}
	return v
}

// VersionParseable reports whether v carries a comparable date-time prefix.
// VersionNewer answers false for anything else, so callers that must tell
// "not newer" apart from "cannot tell" ask this first instead of inventing
// their own rule.
func VersionParseable(v string) bool {
	t := VersionTimestamp(v)
	return len(t) > 0 && t[0] >= '0' && t[0] <= '9'
}

// VersionNewer returns true if candidate is a newer version than current.
// Compares the date-time prefix (YYYY.MM.DD-HHMMSS) lexicographically,
// which works because the format is zero-padded and fixed-width.
// Returns false if either version is malformed (e.g. "dev", "").
func VersionNewer(candidate, current string) bool {
	if !VersionParseable(candidate) || !VersionParseable(current) {
		return false
	}
	return VersionTimestamp(candidate) > VersionTimestamp(current)
}
