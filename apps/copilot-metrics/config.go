package main

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                    string
	LogLevel                slog.Level
	EnterpriseSlug          string
	OrganizationSlug        string
	GitHubAppID             int64
	GitHubAppPrivateKey     string
	GitHubAppInstallationID int64
	BigQueryProjectID       string
	BigQueryDataset         string
	BigQueryTable           string
	SlackWebhookURL         string
}

func loadConfig() *Config {
	return &Config{
		Port:                    getEnv("PORT", "8080"),
		LogLevel:                parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		EnterpriseSlug:          getEnv("GITHUB_ENTERPRISE_SLUG", "nav"),
		OrganizationSlug:        getEnv("GITHUB_ORG", "navikt"),
		GitHubAppID:             getEnvInt64("GITHUB_APP_ID", 0),
		GitHubAppPrivateKey:     getEnv("GITHUB_APP_PRIVATE_KEY", ""),
		GitHubAppInstallationID: getEnvInt64("GITHUB_APP_INSTALLATION_ID", 0),
		BigQueryProjectID:       getEnv("GCP_TEAM_PROJECT_ID", ""),
		BigQueryDataset:         getEnv("BIGQUERY_DATASET", "copilot_metrics"),
		BigQueryTable:           getEnv("BIGQUERY_TABLE", "usage_metrics"),
		SlackWebhookURL:         getEnv("SLACK_WEBHOOK_URL", ""),
	}
}

func (c *Config) Validate() error {
	var missing []string
	if c.GitHubAppID == 0 {
		missing = append(missing, "GITHUB_APP_ID")
	}
	if c.GitHubAppPrivateKey == "" {
		missing = append(missing, "GITHUB_APP_PRIVATE_KEY")
	}
	if c.GitHubAppInstallationID == 0 {
		missing = append(missing, "GITHUB_APP_INSTALLATION_ID")
	}
	if c.BigQueryProjectID == "" {
		missing = append(missing, "GCP_TEAM_PROJECT_ID")
	}
	if len(missing) > 0 {
		return &ConfigError{MissingVars: missing}
	}
	return nil
}

type ConfigError struct {
	MissingVars []string
}

func (e *ConfigError) Error() string {
	return "missing required environment variables: " + strings.Join(e.MissingVars, ", ")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if value := os.Getenv(key); value != "" {
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			slog.Warn("Invalid int64 value for environment variable, using fallback",
				"key", key,
				"value", value,
				"fallback", fallback,
				"error", err,
			)
			return fallback
		}
		return i
	}
	return fallback
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
