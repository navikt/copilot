package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	checkInterval    = 24 * time.Hour
	stalenessTimeout = 2 * time.Second
	staleThreshold   = 14
)

// stalenessCache persists the last update check result outside the repo.
type stalenessCache struct {
	LastChecked   string `json:"last_checked"`
	LatestVersion string `json:"latest_version"`
}

type stalenessAssessment struct {
	LatestVersion string
	Result        string
	UpToDate      bool
	SkewDays      int64
	HasSkew       bool
}

// cacheHome can be overridden in tests.
var cacheHome = ""

func cacheFilePath() string {
	if cacheHome != "" {
		return filepath.Join(cacheHome, "cache.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".nav-pilot", "cache.json")
}

func readCache() *stalenessCache {
	path := cacheFilePath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c stalenessCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil
	}
	return &c
}

func writeCache(c *stalenessCache) {
	path := cacheFilePath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	data = append(data, '\n')
	os.WriteFile(path, data, 0o644)
}

// checkStaleness returns the latest available version if the installed
// collection is outdated. Returns "" if up-to-date, check was skipped
// (within cooldown), or any error occurred (network, API, etc).
// Designed to be fast and never block — uses a 2s HTTP timeout.
func checkStaleness(installedVersion string) string {
	assessment := assessStaleness(installedVersion)
	if assessment.LatestVersion != "" && versionNewer(assessment.LatestVersion, installedVersion) {
		return assessment.LatestVersion
	}
	return ""
}

func assessStaleness(installedVersion string) stalenessAssessment {
	if installedVersion == "" || installedVersion == "dev" {
		return stalenessAssessment{Result: "dev"}
	}

	// Check cooldown
	cache := readCache()
	if cache != nil && cache.LastChecked != "" {
		if t, err := time.Parse(time.RFC3339, cache.LastChecked); err == nil {
			if time.Since(t) < checkInterval {
				// Within cooldown — use cached result
				return assessFromLatest(installedVersion, cache.LatestVersion, "cooldown")
			}
		}
	}

	// Use a short timeout client for staleness checks
	client := &http.Client{Timeout: stalenessTimeout}
	origClient := httpClient
	httpClient = client
	defer func() { httpClient = origClient }()

	latest, _, err := fetchLatestVersion()
	if err != nil {
		return stalenessAssessment{Result: "lookup_failed"}
	}

	// Only write cache on successful check
	writeCache(&stalenessCache{
		LastChecked:   time.Now().UTC().Format(time.RFC3339),
		LatestVersion: latest,
	})

	return assessFromLatest(installedVersion, latest, "")
}

func assessFromLatest(installedVersion, latestVersion, fallbackResult string) stalenessAssessment {
	result := "up_to_date"
	upToDate := true
	if latestVersion == "" {
		if fallbackResult != "" {
			return stalenessAssessment{Result: fallbackResult}
		}
		return stalenessAssessment{Result: "lookup_failed"}
	}

	skewDays, skewOk := versionSkewDays(latestVersion, installedVersion)
	if versionNewer(latestVersion, installedVersion) {
		upToDate = skewOk && skewDays <= staleThreshold
		if !upToDate {
			result = "stale"
		}
	}
	if fallbackResult != "" {
		result = fallbackResult
	}
	return stalenessAssessment{
		LatestVersion: latestVersion,
		Result:        result,
		UpToDate:      upToDate,
		SkewDays:      skewDays,
		HasSkew:       skewOk,
	}
}

func versionSkewDays(latestVersion, installedVersion string) (int64, bool) {
	latestTime, ok := parseVersionTimestamp(versionTimestamp(latestVersion))
	if !ok {
		return 0, false
	}
	installedTime, ok := parseVersionTimestamp(versionTimestamp(installedVersion))
	if !ok {
		return 0, false
	}
	diff := latestTime.Sub(installedTime)
	if diff < 0 {
		return 0, true
	}
	return int64(diff.Hours() / 24), true
}

func parseVersionTimestamp(ts string) (time.Time, bool) {
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
