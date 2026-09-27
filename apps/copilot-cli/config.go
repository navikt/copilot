// Package main implements copilot-cli, a NAIS-hosted gateway for nav-pilot
// and ki-utvikling. It signs callers in with a GitHub token (nav-pilot, navikt
// org members only) or an Entra ID OBO token (my-copilot), forwards usage
// requests to copilot-api, and takes answers to user surveys.
package main

import (
	"encoding/base64"
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

	// NaisTokenIntrospectionEndpoint and AzurePreAuthorizedApps turn on the
	// Entra ID sign-in path; NAIS injects both.
	NaisTokenIntrospectionEndpoint string
	AzurePreAuthorizedApps         string

	// DatabaseURL turns on survey submissions; each survey also needs its
	// own key, SURVEY_KEY_<ID> (see surveyKeys).
	DatabaseURL string

	// The nav-pilot GitHub App's installation credentials, for looking up a
	// GitHub user's Nav e-mail in navikt's SAML SSO (see samlLookup).
	GitHubAppID             string
	GitHubAppInstallationID string
	GitHubAppPrivateKey     string

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
		CopilotAPIAudience:    getEnv("COPILOT_API_AUDIENCE", audienceForCluster(cluster)),
		NaisTokenEndpoint:     os.Getenv("NAIS_TOKEN_ENDPOINT"),
		OrgMembershipCacheTTL: 5 * time.Minute,

		GitHubClientID:                 os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:             os.Getenv("GITHUB_CLIENT_SECRET"),
		NaisTokenIntrospectionEndpoint: os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT"),
		AzurePreAuthorizedApps:         os.Getenv("AZURE_APP_PRE_AUTHORIZED_APPS"),
		DatabaseURL:                    os.Getenv("DB_URL"),
		GitHubAppID:                    os.Getenv("GITHUB_APP_ID"),
		GitHubAppInstallationID:        os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		GitHubAppPrivateKey:            os.Getenv("GITHUB_APP_PRIVATE_KEY"),
	}
}

// audienceForCluster derives the default Entra ID audience for the
// copilot-api backend, following the api://<cluster>.<namespace>.<app>/.default
// convention used across this monorepo.
func audienceForCluster(cluster string) string {
	if cluster == "" || cluster == "local" {
		return ""
	}
	return "api://" + cluster + ".copilot.copilot-api/.default"
}

// surveyKeys reads each survey's secret key from SURVEY_KEY_<ID> (the id
// upper-cased, - as _), base64 of at least 32 bytes. A survey with a missing
// or short key takes no answers.
//
// A key still present after its survey closed is logged as a warning: it
// should have been deleted (see README, key lifecycle).
func surveyKeys(surveys []survey, now time.Time) map[string][]byte {
	keys := map[string][]byte{}
	for _, s := range surveys {
		name := "SURVEY_KEY_" + strings.ToUpper(strings.ReplaceAll(s.ID, "-", "_"))
		if !now.Before(s.closesOn()) {
			if os.Getenv(name) != "" {
				slog.Warn("survey key still present after the survey closed: delete it from the copilot-cli secret", "key", name)
			}
			continue
		}
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv(name)))
		if err == nil && len(key) >= 32 {
			keys[s.ID] = key
		}
	}
	return keys
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
