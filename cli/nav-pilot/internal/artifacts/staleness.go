package artifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	checkInterval  = 24 * time.Hour
	staleThreshold = 14
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

func ParseVersionTimestamp(ts string) (time.Time, bool) {
	parts := strings.SplitN(ts, "-", 2)
	if len(parts) != 2 {
		return time.Time{}, false
	}
	datePart := strings.Split(parts[0], ".")
	if len(datePart) != 3 {
		return time.Time{}, false
	}
	if len(parts[1]) != 6 {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(datePart[0])
	if err != nil {
		return time.Time{}, false
	}
	month, err := strconv.Atoi(datePart[1])
	if err != nil {
		return time.Time{}, false
	}
	day, err := strconv.Atoi(datePart[2])
	if err != nil {
		return time.Time{}, false
	}
	hour, err := strconv.Atoi(parts[1][0:2])
	if err != nil {
		return time.Time{}, false
	}
	minute, err := strconv.Atoi(parts[1][2:4])
	if err != nil {
		return time.Time{}, false
	}
	second, err := strconv.Atoi(parts[1][4:6])
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC), true
}
