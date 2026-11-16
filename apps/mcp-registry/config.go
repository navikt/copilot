// Package main implements the MCP Registry server.
package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"slices"
	"strings"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	Port            string
	LogLevel        slog.Level
	LoggedEndpoints map[string]bool
	DomainInternal  string
	DomainExternal  string
	DatabaseURL     string
	// AdminGroupID is the Entra group object id whose members may approve and
	// reject entries. Empty means nobody is admin.
	AdminGroupID string
	// AllowedWriteAZP lists the client ids (token azp) allowed to call write
	// endpoints. Empty means no caller may write.
	AllowedWriteAZP []string
	TexasEndpoint   string
	Store           *Store
}

// IsAdmin reports whether groups contains the configured admin group.
func (c *Config) IsAdmin(groups []string) bool {
	return c != nil && c.AdminGroupID != "" && slices.Contains(groups, c.AdminGroupID)
}

// AllowsWriteAZP reports whether azp is an allowed writer client id. Fails
// closed: an empty list allows nobody.
func (c *Config) AllowsWriteAZP(azp string) bool {
	return c != nil && azp != "" && slices.Contains(c.AllowedWriteAZP, azp)
}

// parseAllowedWriteAZP splits the comma-separated ALLOWED_WRITE_AZP. An entry
// is a client id, or a Nais app name ("cluster:namespace:app") resolved to its
// client id through AZURE_APP_PRE_AUTHORIZED_APPS; unresolved names are dropped.
func parseAllowedWriteAZP(raw, preAuthorizedJSON string) []string {
	var preAuthorized []struct {
		Name     string `json:"name"`
		ClientID string `json:"clientId"`
	}
	if preAuthorizedJSON != "" {
		if err := json.Unmarshal([]byte(preAuthorizedJSON), &preAuthorized); err != nil {
			slog.Warn("Ignoring unparsable AZURE_APP_PRE_AUTHORIZED_APPS", "error", err)
		}
	}
	var out []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Count(item, ":") != 2 {
			out = append(out, item)
			continue
		}
		resolved := false
		for _, app := range preAuthorized {
			if app.Name == item && app.ClientID != "" {
				out = append(out, app.ClientID)
				resolved = true
			}
		}
		if !resolved {
			slog.Warn("Allowed writer app is not pre-authorized; ignoring", "app", item)
		}
	}
	return out
}

func loadConfig() *Config {
	config := &Config{
		Port:            getEnv("PORT", "8080"),
		DomainInternal:  getEnv("DOMAIN_INTERNAL", "intern.dev.nav.no"),
		DomainExternal:  getEnv("DOMAIN_EXTERNAL", "ekstern.dev.nav.no"),
		DatabaseURL:     getDatabaseURL(),
		AdminGroupID:    os.Getenv("ADMIN_GROUP_ID"),
		AllowedWriteAZP: parseAllowedWriteAZP(os.Getenv("ALLOWED_WRITE_AZP"), os.Getenv("AZURE_APP_PRE_AUTHORIZED_APPS")),
		TexasEndpoint:   os.Getenv("NAIS_TOKEN_INTROSPECTION_ENDPOINT"),
		LoggedEndpoints: make(map[string]bool),
	}

	logLevelStr := getEnv("LOG_LEVEL", "INFO")
	switch strings.ToUpper(logLevelStr) {
	case "DEBUG":
		config.LogLevel = slog.LevelDebug
	case "INFO":
		config.LogLevel = slog.LevelInfo
	case "WARN", "WARNING":
		config.LogLevel = slog.LevelWarn
	case "ERROR":
		config.LogLevel = slog.LevelError
	default:
		config.LogLevel = slog.LevelInfo
	}

	loggedEndpointsStr := getEnv("LOGGED_ENDPOINTS", "/v0.1/servers")
	if loggedEndpointsStr != "" {
		endpoints := strings.Split(loggedEndpointsStr, ",")
		for _, endpoint := range endpoints {
			endpoint = strings.TrimSpace(endpoint)
			if endpoint != "" {
				config.LoggedEndpoints[endpoint] = true
			}
		}
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDatabaseURL() string {
	return os.Getenv("DB_URL")
}

func getEndpointsList(endpoints map[string]bool) []string {
	list := make([]string, 0, len(endpoints))
	for endpoint := range endpoints {
		list = append(list, endpoint)
	}
	return list
}
