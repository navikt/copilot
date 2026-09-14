package artifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	checkInterval  = 24 * time.Hour
	staleThreshold = 14

	// ReleaseQuietPeriod is how long a release stays unannounced. This repo
	// releases several times a day, so without it a user is interrupted on
	// nearly every command by a version minutes old. Homebrew makes that
	// worse: navikt/homebrew-tap rebuilds the formula on an hourly cron, so
	// `brew upgrade` right after a release is a no-op. An explicit
	// `nav-pilot update` ignores the quiet period — that user asked.
	ReleaseQuietPeriod = 24 * time.Hour
)

// StalenessCache persists the last update check result outside the repo.
type StalenessCache struct {
	LastChecked   string `json:"last_checked"`
	LatestVersion string `json:"latest_version"`
	LastFailed    string `json:"last_failed,omitempty"`
}

type StalenessAssessment struct {
	LatestVersion string
	Result        string
	UpToDate      bool
	SkewDays      int64
	HasSkew       bool
}

// CacheHome can be overridden in tests.
var CacheHome string

func CacheFilePath() string {
	if CacheHome != "" {
		return filepath.Join(CacheHome, "cache.json")
	}
	// Same rule as the device id and the tier cache: the config file's
	// directory is nav-pilot's own state directory when one is named.
	if p := os.Getenv("NAV_PILOT_CONFIG"); p != "" {
		return filepath.Join(filepath.Dir(p), "cache.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".nav-pilot", "cache.json")
}

func ReadCache() *StalenessCache {
	path := CacheFilePath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c StalenessCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil
	}
	return &c
}

func WriteCache(c *StalenessCache) {
	path := CacheFilePath()
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	data = append(data, '\n')

	tmpFile, err := os.CreateTemp(dir, "cache-*.tmp")
	if err != nil {
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return
	}
	if err := tmpFile.Close(); err != nil {
		return
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmpPath, path)
}

// CheckStaleness returns the latest available version if the installed
// collection is outdated. Returns "" if up-to-date, check was skipped
// (within cooldown), or any error occurred (network, API, etc).
func CheckStaleness(installedVersion string, fetchFn func() (string, string, error)) string {
	assessment := AssessStaleness(installedVersion, fetchFn)
	if assessment.LatestVersion != "" && VersionNewer(assessment.LatestVersion, installedVersion) {
		return assessment.LatestVersion
	}
	return ""
}

func AssessStaleness(installedVersion string, fetchFn func() (string, string, error)) StalenessAssessment {
	if installedVersion == "" || installedVersion == "dev" {
		return StalenessAssessment{Result: "dev"}
	}

	cache := ReadCache()
	if cache != nil && cache.LastChecked != "" {
		if t, err := time.Parse(time.RFC3339, cache.LastChecked); err == nil {
			if cache.LastFailed != "" {
				if time.Since(t) < 1*time.Hour {
					return AssessFromLatest(installedVersion, cache.LatestVersion, "cooldown")
				}
			} else if time.Since(t) < checkInterval {
				return AssessFromLatest(installedVersion, cache.LatestVersion, "cooldown")
			}
		}
	}

	latest, _, err := fetchFn()
	if err != nil {
		var prevLatest string
		if cache != nil {
			prevLatest = cache.LatestVersion
		}
		WriteCache(&StalenessCache{
			LastChecked:   time.Now().UTC().Format(time.RFC3339),
			LatestVersion: prevLatest,
			LastFailed:    time.Now().UTC().Format(time.RFC3339),
		})
		return StalenessAssessment{Result: "lookup_failed"}
	}

	WriteCache(&StalenessCache{
		LastChecked:   time.Now().UTC().Format(time.RFC3339),
		LatestVersion: latest,
	})

	return AssessFromLatest(installedVersion, latest, "")
}

func AssessFromLatest(installedVersion, latestVersion, fallbackResult string) StalenessAssessment {
	result := "up_to_date"
	upToDate := true
	if latestVersion == "" {
		if fallbackResult != "" {
			return StalenessAssessment{Result: fallbackResult}
		}
		return StalenessAssessment{Result: "lookup_failed"}
	}

	skewDays, skewOK := VersionSkewDays(latestVersion, installedVersion)
	if VersionNewer(latestVersion, installedVersion) {
		upToDate = skewOK && skewDays <= staleThreshold
		if !upToDate {
			result = "stale"
		}
	}
	if fallbackResult != "" {
		result = fallbackResult
	}
	return StalenessAssessment{
		LatestVersion: latestVersion,
		Result:        result,
		UpToDate:      upToDate,
		SkewDays:      skewDays,
		HasSkew:       skewOK,
	}
}

// ReleaseIsFresh reports whether version was released less than
// [ReleaseQuietPeriod] ago, read off the YYYY.MM.DD-HHMMSS-sha stamp so no
// network call is needed. A version string it cannot parse is reported as not
// fresh: an unreadable stamp must not silence the nudge.
func ReleaseIsFresh(version string, now time.Time) bool {
	released, ok := ParseVersionTimestamp(VersionTimestamp(version))
	if !ok {
		return false
	}
	// A stamp in the future (clock skew) is newer still, so it counts as fresh.
	return now.Sub(released) < ReleaseQuietPeriod
}

func VersionSkewDays(latestVersion, installedVersion string) (int64, bool) {
	latestTime, ok := ParseVersionTimestamp(VersionTimestamp(latestVersion))
	if !ok {
		return 0, false
	}
	installedTime, ok := ParseVersionTimestamp(VersionTimestamp(installedVersion))
	if !ok {
		return 0, false
	}
	diff := latestTime.Sub(installedTime)
	if diff < 0 {
		return 0, true
	}
	return int64(diff.Hours() / 24), true
}

// versionTimestampLayout is the version stamp's date-time prefix:
// "2026.04.13-170138". Fixed width and zero-padded, which is what makes the
// lexical comparison in VersionNewer work.
const versionTimestampLayout = "2006.01.02-150405"

// ParseVersionTimestamp reads the date-time prefix of a version string. It
// rejects a field that is numeric but not a real calendar value (month 99, day
// 32, hour 24): time.Date would roll those forward into a plausible future
// date, and a release stamped in the future reads as fresh — which would let a
// nonsense tag silence the update nudge for good.
func ParseVersionTimestamp(ts string) (time.Time, bool) {
	t, err := time.Parse(versionTimestampLayout, ts)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
