// Package main implements copilot-cli, a NAIS-hosted gateway for nav-pilot.
// It signs callers in with a GitHub token (navikt org members only) and
// forwards usage requests to copilot-api and survey requests to
// copilot-survey.
package main

import (
	"log/slog"
	"os"
	"strings"
	"time"
)

// Config holds all runtime configuration for copilot-cli, sourced from
// environment variables injected by NAIS (see .nais/*.yaml).
type Config struct {
	Port        string
	Environment string
	LogLevel    slog.Level

	// GitHubOrg is the org membership required to use the CLI (navikt).
	GitHubOrg string

	// CopilotAPIURL is the internal NAIS service URL for copilot-api.
	CopilotAPIURL string
	// CopilotAPIAudience is the Entra ID scope used when exchanging an M2M
	// token via the Texas sidecar, e.g. api://<cluster>.copilot.copilot-api/.default
	CopilotAPIAudience string

	// NaisTokenEndpoint is the Texas sidecar endpoint used for client_credentials
	// (machine-to-machine) token exchange. Empty when running locally without
	// a Texas sidecar.
	NaisTokenEndpoint string

	// GitHubClientID and GitHubClientSecret are the nav-pilot GitHub App's
	// credentials. Without them the GitHub sign-in path is off.
	GitHubClientID     string
	GitHubClientSecret string

	// CopilotSurveyURL and CopilotSurveyAudience reach copilot-survey the
	// same way.
	CopilotSurveyURL      string
	CopilotSurveyAudience string

	// OrgMembershipCacheTTL controls how long a verified org membership is
	// cached, keyed by a SHA-256 hash of the caller's GitHub token, to avoid
	// hammering the GitHub API.
	OrgMembershipCacheTTL time.Duration
}

func loadConfig() *Config {
	cluster := getEnv("NAIS_CLUSTER_NAME", "local")

	return &Config{
		Port:                  getEnv("PORT", "8080"),
		Environment:           cluster,
		LogLevel:              parseLogLevel(getEnv("LOG_LEVEL", "INFO")),
		GitHubOrg:             getEnv("GITHUB_ORG", "navikt"),
		CopilotAPIURL:         getEnv("COPILOT_API_URL", "http://copilot-api"),
		CopilotAPIAudience:    getEnv("COPILOT_API_AUDIENCE", audienceForCluster(cluster, "copilot-api")),
		CopilotSurveyURL:      getEnv("COPILOT_SURVEY_URL", "http://copilot-survey"),
		CopilotSurveyAudience: getEnv("COPILOT_SURVEY_AUDIENCE", audienceForCluster(cluster, "copilot-survey")),
		NaisTokenEndpoint:     os.Getenv("NAIS_TOKEN_ENDPOINT"),
		OrgMembershipCacheTTL: 5 * time.Minute,

		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
	}
}

// audienceForCluster derives the default Entra ID audience for an app in the
// copilot namespace, following the api://<cluster>.<namespace>.<app>/.default
// convention used across this monorepo.
func audienceForCluster(cluster, app string) string {
	if cluster == "" || cluster == "local" {
		return ""
	}
	return "api://" + cluster + ".copilot." + app + "/.default"
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
