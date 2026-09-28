// Package main implements copilot-survey, which owns Nav's user surveys: the
// definitions, the answers and the dedup that refuses a second answer. It
// has no ingress. copilot-cli calls it for nav-pilot, my-copilot for
// ki-utvikling.
package main

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"
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
	// SURVEY_KEY_<ID> (see surveyKeys).
	DatabaseURL string
}

func loadConfig() *Config {
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

// clientIDForApp finds the client id of a pre-authorized app by its NAIS
// name in AZURE_APP_PRE_AUTHORIZED_APPS ([{"name":"<cluster>:<ns>:<app>",
// "clientId":"…"}]). Empty when the app is missing, the input is malformed,
// or more than one entry matches, so trust never goes to the wrong app.
func clientIDForApp(preAuthorizedApps, app string) string {
	var apps []struct {
		Name     string `json:"name"`
		ClientID string `json:"clientId"`
	}
	if json.Unmarshal([]byte(preAuthorizedApps), &apps) != nil {
		return ""
	}
	var id string
	for _, a := range apps {
		if a.Name[strings.LastIndex(a.Name, ":")+1:] == app && a.ClientID != "" {
			if id != "" {
				return ""
			}
			id = a.ClientID
		}
	}
	return id
}

// surveyKeys reads each survey's secret key from SURVEY_KEY_<ID> (the id
// upper-cased, - as _), base64 of at least 32 bytes. A survey with a missing
// or short key takes no answers.
//
// A key still present after its survey closed is logged as a warning: it
// should have been deleted (see surveys/README.md, key lifecycle).
func surveyKeys(surveys []survey, now time.Time) map[string][]byte {
	keys := map[string][]byte{}
	for _, s := range surveys {
		name := "SURVEY_KEY_" + strings.ToUpper(strings.ReplaceAll(s.ID, "-", "_"))
		if !now.Before(s.closesOn()) {
			if os.Getenv(name) != "" {
				slog.Warn("survey key still present after the survey closed: delete it from the copilot-survey secret", "key", name)
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
	var l slog.Level
	if l.UnmarshalText([]byte(level)) != nil {
		return slog.LevelInfo
	}
	return l
}
