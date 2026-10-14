// Package main implements the MCP Registry server.
package main

import (
	"log/slog"
	"os"
	"strings"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	Port            string
	LogLevel        slog.Level
	LoggedEndpoints map[string]bool
	DomainInternal  string
	DomainExternal  string
}

func loadConfig() *Config {
	config := &Config{
		Port:            getEnv("PORT", "8080"),
		DomainInternal:  getEnv("DOMAIN_INTERNAL", "intern.dev.nav.no"),
		DomainExternal:  getEnv("DOMAIN_EXTERNAL", "ekstern.dev.nav.no"),
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

func getEndpointsList(endpoints map[string]bool) []string {
	list := make([]string, 0, len(endpoints))
	for endpoint := range endpoints {
		list = append(list, endpoint)
	}
	return list
}
