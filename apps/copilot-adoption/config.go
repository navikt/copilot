package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                    string
	LogLevel                slog.Level
	OrganizationSlug        string
	GitHubAppID             int64
	GitHubAppPrivateKey     string
	GitHubAppInstallationID int64
	BigQueryProjectID       string
	BigQueryDataset         string
	BigQueryTable           string
	GraphQLBatchSize        int
	ScanConcurrency         int
	SlackWebhookURL         string
}

func loadConfig() *Config {
	return &Config{
		Port:                    getEnv("PORT", "8080"),
		LogLevel:                parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		OrganizationSlug:        getEnv("GITHUB_ORG", "navikt"),
		GitHubAppID:             getEnvInt64("GITHUB_APP_ID", 0),
		GitHubAppPrivateKey:     normalizeKey(getEnv("GITHUB_APP_PRIVATE_KEY", "")),
		GitHubAppInstallationID: getEnvInt64("GITHUB_APP_INSTALLATION_ID", 0),
		BigQueryProjectID:       getEnv("GCP_TEAM_PROJECT_ID", ""),
		BigQueryDataset:         getEnv("BIGQUERY_DATASET", "copilot_adoption"),
		BigQueryTable:           getEnv("BIGQUERY_TABLE", "repo_scan"),
		GraphQLBatchSize:        getEnvInt("GRAPHQL_BATCH_SIZE", 3),
		ScanConcurrency:         getEnvInt("SCAN_CONCURRENCY", 5),
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
	if c.GraphQLBatchSize < 1 || c.GraphQLBatchSize > 10 {
		return fmt.Errorf("GRAPHQL_BATCH_SIZE must be between 1 and 10, got %d", c.GraphQLBatchSize)
	}
	if c.ScanConcurrency < 1 || c.ScanConcurrency > 20 {
		return fmt.Errorf("SCAN_CONCURRENCY must be between 1 and 20, got %d", c.ScanConcurrency)
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

// normalizeKey replaces literal \n with newlines and strips surrounding quotes.
func normalizeKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, "\"'")
	key = strings.ReplaceAll(key, "\\n", "\n")
	return key
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

func getEnvInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		i, err := strconv.Atoi(value)
		if err != nil {
			slog.Warn("Invalid int value for environment variable, using fallback",
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
