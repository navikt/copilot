package main

import (
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Port                   string
	Environment            string
	LogLevel               slog.Level
	LoggedEndpoints        map[string]bool
	AzureClientID          string
	AzureIssuer            string
	AzureJWKSURI           string
	PreAuthorizedApps      string
	GitHubOrg              string
	GitHubAppID            string
	GitHubAppPrivateKey    string
	GitHubInstallationID   string
	GCPProjectID           string
	CopilotMetricsDataset  string
	CopilotMetricsTable    string
	CopilotAdoptionDataset string
	CacheTTLHours          int
}

func loadConfig() *Config {
	config := &Config{
		Port:                   getEnv("PORT", "8080"),
		Environment:            getEnv("NAIS_CLUSTER_NAME", "local"),
		LogLevel:               parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		LoggedEndpoints:        parseEndpoints(getEnv("LOGGED_ENDPOINTS", "/api/v1/")),
		AzureClientID:          getEnv("AZURE_APP_CLIENT_ID", ""),
		AzureIssuer:            getEnv("AZURE_OPENID_CONFIG_ISSUER", ""),
		AzureJWKSURI:           getEnv("AZURE_OPENID_CONFIG_JWKS_URI", ""),
		PreAuthorizedApps:      getEnv("AZURE_APP_PRE_AUTHORIZED_APPS", ""),
		GitHubOrg:              getEnv("GITHUB_ORG", "navikt"),
		GitHubAppID:            os.Getenv("GITHUB_APP_ID"),
		GitHubAppPrivateKey:    os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		GitHubInstallationID:   os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		GCPProjectID:           os.Getenv("GCP_TEAM_PROJECT_ID"),
		CopilotMetricsDataset:  getEnv("COPILOT_METRICS_DATASET", "copilot_metrics"),
		CopilotMetricsTable:    getEnv("COPILOT_METRICS_TABLE", "usage_metrics"),
		CopilotAdoptionDataset: getEnv("COPILOT_ADOPTION_DATASET", "copilot_adoption"),
		CacheTTLHours:          1,
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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

func getEndpointsList(endpoints map[string]bool) []string {
	list := make([]string, 0, len(endpoints))
	for endpoint := range endpoints {
		list = append(list, endpoint)
	}
	return list
}
