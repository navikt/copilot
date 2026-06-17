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

// VersionNewer returns true if candidate is a newer version than current.
// Compares the date-time prefix (YYYY.MM.DD-HHMMSS) lexicographically,
// which works because the format is zero-padded and fixed-width.
// Returns false if either version is malformed (e.g. "dev", "").
func VersionNewer(candidate, current string) bool {
	ct := VersionTimestamp(candidate)
	cu := VersionTimestamp(current)
	if len(ct) == 0 || ct[0] < '0' || ct[0] > '9' {
		return false
	}
	if len(cu) == 0 || cu[0] < '0' || cu[0] > '9' {
		return false
	}
	return ct > cu
}
