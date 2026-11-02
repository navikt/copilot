// Package main implements the MCP Registry Writer service.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	Port            string
	LogLevel        slog.Level
	LoggedEndpoints map[string]bool
	DatabaseURL     string
	AdminUsers      map[string]bool
	TexasEndpoint   string
}

func loadConfig() *Config {
	config := &Config{
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     getDatabaseURL(),
		AdminUsers:      parseAdminUsers(os.Getenv("ADMIN_USERS")),
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

	loggedEndpointsStr := getEnv("LOGGED_ENDPOINTS", "/api/v1/entries")
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

func parseAdminUsers(val string) map[string]bool {
	admins := make(map[string]bool)
	if strings.TrimSpace(val) == "" {
		return admins
	}
	for _, user := range strings.Split(val, ",") {
		user = strings.TrimSpace(strings.ToLower(user))
		if user != "" {
			admins[user] = true
		}
	}
	return admins
}

// IsAdmin returns true if the specified actor email is in AdminUsers.
func (c *Config) IsAdmin(actor string) bool {
	if c == nil || c.AdminUsers == nil {
		return false
	}
	return c.AdminUsers[strings.TrimSpace(strings.ToLower(actor))]
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDatabaseURL() string {
	if u := os.Getenv("DB_URL"); u != "" {
		return u
	}
	host := os.Getenv("DB_HOST")
	if host == "" {
		return ""
	}
	port := getEnv("DB_PORT", "5432")
	user := os.Getenv("DB_USERNAME")
	pass := os.Getenv("DB_PASSWORD")
	db := os.Getenv("DB_DATABASE")
	if user != "" && pass != "" {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, db)
	}
	if user != "" {
		return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=disable", user, host, port, db)
	}
	return fmt.Sprintf("postgres://%s:%s/%s?sslmode=disable", host, port, db)
}

func getEndpointsList(endpoints map[string]bool) []string {
	list := make([]string, 0, len(endpoints))
	for endpoint := range endpoints {
		list = append(list, endpoint)
	}
	return list
}
