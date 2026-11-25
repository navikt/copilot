// Package config reads copilot-survey's settings from the environment.
package config

import (
	"log/slog"
	"os"
)

// Config comes from the environment NAIS injects (see .nais/app.yaml).
type Config struct {
	Port     string
	LogLevel slog.Level

	// CopilotAPIURL and CopilotAPIAudience reach copilot-api's
	// POST /internal/v1/saml/name-id with an M2M token from Texas.
	CopilotAPIURL      string
	CopilotAPIAudience string
	NaisTokenEndpoint  string

	// NaisTokenIntrospectionEndpoint validates incoming Entra tokens;
	// AzurePreAuthorizedApps names the apps allowed to call (copilot-cli,
	// my-copilot).
	NaisTokenIntrospectionEndpoint string
	AzurePreAuthorizedApps         string

	// DatabaseURL turns on submissions; each survey also needs its own key,
	// SURVEY_KEY_<ID> (see survey.Keys).
	DatabaseURL string
}

// Load reads Config from the environment.
func Load() *Config {
	cluster := getEnv("NAIS_CLUSTER_NAME", "local")
	audience := ""
	if cluster != "local" {
		audience = "api://" + cluster + ".copilot.copilot-api/.default"
	}
	return &Config{
		Port:                           getEnv("PORT", "8080"),
		LogLevel:                       parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		CopilotAPIURL:                  getEnv("COPILOT_API_URL", "http://copilot-api"),
		CopilotAPIAudience:             getEnv("COPILOT_API_AUDIENCE", audience),
		NaisTokenEndpoint:              os.Getenv("NAIS_TOKEN_ENDPOINT"),
		NaisTokenIntrospectionEndpoint: os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT"),
		AzurePreAuthorizedApps:         os.Getenv("AZURE_APP_PRE_AUTHORIZED_APPS"),
		DatabaseURL:                    os.Getenv("DB_URL"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseLogLevel(level string) slog.Level {
	var l slog.Level
	if l.UnmarshalText([]byte(level)) != nil {
		return slog.LevelInfo
	}
	return l
}
