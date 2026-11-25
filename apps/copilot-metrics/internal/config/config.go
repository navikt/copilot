// Package config reads the job configuration from the environment.
package config

import (
	"cmp"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds the job settings read from the environment.
type Config struct {
	Port                       string
	LogLevel                   slog.Level
	EnterpriseSlug             string
	OrganizationSlug           string
	GitHubAppID                int64
	GitHubAppPrivateKey        string
	GitHubAppInstallationID    int64
	GitHubAppOrgInstallationID int64  // Optional: separate installation for org-level endpoints
	GitHubBillingToken         string // Classic PAT, used only when BillingPATFallback is true
	BillingPATFallback         bool   // BILLING_PAT_FALLBACK=true: billing uses the PAT instead of the app (transition only)
	BigQueryProjectID          string
	BigQueryDataset            string
	BigQueryTable              string
	BigQueryUserTeamsTable     string
	BigQueryUserMetricsTable   string
	BigQueryRepoMetricsTable   string
	SlackWebhookURL            string
}

// Load reads Config from the environment.
func Load() *Config {
	return &Config{
		Port:                       GetEnv("PORT", "8080"),
		LogLevel:                   parseLogLevel(GetEnv("LOG_LEVEL", "INFO")),
		EnterpriseSlug:             GetEnv("GITHUB_ENTERPRISE_SLUG", "nav"),
		OrganizationSlug:           GetEnv("GITHUB_ORG", "navikt"),
		GitHubAppID:                getEnvInt64("GITHUB_APP_ID", 0),
		GitHubAppPrivateKey:        strings.ReplaceAll(GetEnv("GITHUB_APP_PRIVATE_KEY", ""), `\n`, "\n"),
		GitHubAppInstallationID:    getEnvInt64("GITHUB_APP_INSTALLATION_ID", 0),
		GitHubAppOrgInstallationID: getEnvInt64("GITHUB_APP_ORG_INSTALLATION_ID", 0),
		GitHubBillingToken:         GetEnv("GITHUB_BILLING_TOKEN", ""),
		BillingPATFallback:         os.Getenv("BILLING_PAT_FALLBACK") == "true",
		BigQueryProjectID:          GetEnv("GCP_TEAM_PROJECT_ID", ""),
		BigQueryDataset:            GetEnv("BIGQUERY_DATASET", "copilot_metrics"),
		BigQueryTable:              GetEnv("BIGQUERY_TABLE", "usage_metrics"),
		BigQueryUserTeamsTable:     GetEnv("BIGQUERY_USER_TEAMS_TABLE", "user_teams"),
		BigQueryUserMetricsTable:   GetEnv("BIGQUERY_USER_METRICS_TABLE", "user_metrics"),
		BigQueryRepoMetricsTable:   GetEnv("BIGQUERY_REPO_METRICS_TABLE", "repository_metrics"),
		SlackWebhookURL:            cmp.Or(os.Getenv("apiUrl"), GetEnv("SLACK_WEBHOOK_URL", "")),
	}
}

// Validate reports a ConfigError when required settings are missing.
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

// ConfigError lists required environment variables that are missing.
type ConfigError struct {
	MissingVars []string
}

func (e *ConfigError) Error() string {
	return "missing required environment variables: " + strings.Join(e.MissingVars, ", ")
}

// GetEnv returns the environment variable key, or fallback when it is empty.
func GetEnv(key, fallback string) string {
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
