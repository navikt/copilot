package main

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port        string
	Environment string
	// EnableDevQuery is the second half of the double-lock guarding the
	// unauthenticated raw-SQL /dev/query endpoint (see main.go). Environment
	// defaults to "local" when NAIS_CLUSTER_NAME is unset, so gating on the
	// environment alone would fail OPEN: a misconfigured deployment missing
	// that env var would silently expose an unauthenticated SQL console.
	// Requiring an explicit ENABLE_DEV_QUERY=true opt-in ensures the endpoint
	// can never appear just because an env var went missing.
	EnableDevQuery         bool
	LogLevel               slog.Level
	LoggedEndpoints        map[string]bool
	AzureClientID          string
	AzureIssuer            string
	AzureJWKSURI           string
	PreAuthorizedApps      string
	GitHubOrg              string
	GitHubEnterprise       string
	GitHubAppID            string
	GitHubAppPrivateKey    string
	GitHubInstallationID   string
	GitHubBillingToken     string
	GCPProjectID           string
	CopilotMetricsDataset  string
	CopilotMetricsTable    string
	CopilotAdoptionDataset string
	CacheTTLHours          int
	VideoManifestURL       string
	VideoManifestPath      string
	VideoBucketPublic      string
	VideoPublicBaseURL     string
	VideoFeedCacheSeconds  int
}

func loadConfig() *Config {
	environment := getEnv("NAIS_CLUSTER_NAME", "local")
	config := &Config{
		Port:                   getEnv("PORT", "8080"),
		Environment:            getEnv("NAIS_CLUSTER_NAME", "local"),
		EnableDevQuery:         getEnv("ENABLE_DEV_QUERY", "") == "true",
		LogLevel:               parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		LoggedEndpoints:        parseEndpoints(getEnv("LOGGED_ENDPOINTS", "/api/v1/")),
		AzureClientID:          getEnv("AZURE_APP_CLIENT_ID", ""),
		AzureIssuer:            getEnv("AZURE_OPENID_CONFIG_ISSUER", ""),
		AzureJWKSURI:           getEnv("AZURE_OPENID_CONFIG_JWKS_URI", ""),
		PreAuthorizedApps:      getEnv("AZURE_APP_PRE_AUTHORIZED_APPS", ""),
		GitHubOrg:              getEnv("GITHUB_ORG", "navikt"),
		GitHubEnterprise:       getEnv("GITHUB_ENTERPRISE", "nav"),
		GitHubAppID:            os.Getenv("GITHUB_APP_ID"),
		GitHubAppPrivateKey:    os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		GitHubInstallationID:   os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		GitHubBillingToken:     os.Getenv("GITHUB_BILLING_TOKEN"),
		GCPProjectID:           os.Getenv("GCP_TEAM_PROJECT_ID"),
		CopilotMetricsDataset:  getEnv("COPILOT_METRICS_DATASET", "copilot_metrics"),
		CopilotMetricsTable:    getEnv("COPILOT_METRICS_TABLE", "usage_metrics"),
		CopilotAdoptionDataset: getEnv("COPILOT_ADOPTION_DATASET", "copilot_adoption"),
		CacheTTLHours:          1,
		VideoManifestURL:       getEnv("VIDEO_MANIFEST_URL", ""),
		VideoManifestPath:      getEnv("VIDEO_MANIFEST_PATH", ""),
		VideoBucketPublic:      selectVideoValue(environment, os.Getenv("VIDEO_BUCKET_PUBLIC_DEV"), os.Getenv("VIDEO_BUCKET_PUBLIC_PROD"), os.Getenv("VIDEO_BUCKET_PUBLIC")),
		VideoPublicBaseURL:     getEnv("VIDEO_PUBLIC_BASE_URL", ""),
		VideoFeedCacheSeconds:  getEnvInt("VIDEO_FEED_CACHE_SECONDS", 60),
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return n
}

func selectVideoValue(environment, devValue, prodValue, fallback string) string {
	switch normalizeEnvironment(environment) {
	case "dev":
		if devValue != "" {
			return devValue
		}
	case "prod":
		if prodValue != "" {
			return prodValue
		}
	}

	if fallback != "" {
		return fallback
	}
	if devValue != "" {
		return devValue
	}
	if prodValue != "" {
		return prodValue
	}
	return ""
}

func normalizeEnvironment(environment string) string {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "dev", "dev-gcp", "development":
		return "dev"
	case "prod", "prod-gcp", "production":
		return "prod"
	default:
		return ""
	}
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func parseEndpoints(input string) map[string]bool {
	endpoints := make(map[string]bool)
	for _, endpoint := range strings.Split(input, ",") {
		trimmed := strings.TrimSpace(endpoint)
		if trimmed != "" {
			endpoints[trimmed] = true
		}
	}
	return endpoints
}
