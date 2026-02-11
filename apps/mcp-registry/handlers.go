package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func readyHandler(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, "# HELP mcp_registry_requests_total Total number of requests\n"); err != nil {
		slog.Error("Failed to write metrics help", "error", err)
		return
	}
	if _, err := fmt.Fprintf(w, "# TYPE mcp_registry_requests_total counter\n"); err != nil {
		slog.Error("Failed to write metrics type", "error", err)
		return
	}
	if _, err := fmt.Fprintf(w, "mcp_registry_requests_total 0\n"); err != nil {
		slog.Error("Failed to write metrics value", "error", err)
	}
}

func rootHandler(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"service":     "Nav MCP Registry",
		"version":     "1.0.0",
		"description": "Nav internal MCP server registry providing approved servers for GitHub Copilot",
		"endpoints": map[string]string{
			"servers":        "/v0.1/servers",
			"server_version": "/v0.1/servers/{serverName}/versions/{version}",
			"server_latest":  "/v0.1/servers/{serverName}/latest",
			"health":         "/health",
			"ready":          "/ready",
			"metrics":        "/metrics",
		},
	})
}

func optionsHandler(w http.ResponseWriter, _ *http.Request) {
	setCORSHeaders(w)
	w.WriteHeader(http.StatusNoContent)
}

func substituteVariables(data []byte, config *Config) []byte {
	result := string(data)
	result = strings.ReplaceAll(result, "{{domain_internal}}", config.DomainInternal)
	result = strings.ReplaceAll(result, "{{domain_external}}", config.DomainExternal)
	return []byte(result)
}

func makeServersListHandler(config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serversListHandler(w, r, config)
	}
}

func serversListHandler(w http.ResponseWriter, r *http.Request, config *Config) {
	if r.Method == http.MethodOptions {
		optionsHandler(w, r)
		return
	}
	if r.Method != http.MethodGet {
		slog.Warn("Method not allowed", "method", r.Method, "path", r.URL.Path)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := os.ReadFile("allowlist.json")
	if err != nil {
		slog.Error("Error reading allowlist.json", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data = substituteVariables(data, config)

	fileInfo, err := os.Stat("allowlist.json")
	if err != nil {
		slog.Error("Error getting file info", "error", err, "file", "allowlist.json")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var staticData StaticRegistryData
	if err := json.Unmarshal(data, &staticData); err != nil {
		slog.Error("Unexpected error parsing allowlist.json", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	updatedAt := fileInfo.ModTime().UTC()
	servers := make([]ServerResponse, 0, len(staticData.Servers))
	for _, s := range staticData.Servers {
		publishedAt := updatedAt
		if s.PublishedAt != "" {
			if parsed, err := time.Parse(time.RFC3339, s.PublishedAt); err == nil {
				publishedAt = parsed
			}
		}
		status := s.Status
		if status == "" {
			status = StatusActive
		}
		servers = append(servers, ServerResponse{
			Server: ServerJSON{
				Schema:      CurrentSchemaURL,
				Name:        s.Name,
				Description: s.Description,
				Version:     s.Version,
				Remotes:     s.Remotes,
			},
			Meta: ResponseMeta{
				Official: &RegistryExtensions{
					Status:      status,
					PublishedAt: publishedAt,
					UpdatedAt:   updatedAt,
					IsLatest:    true,
				},
			},
		})
	}

	response := ServerListResponse{
		Servers: servers,
		Metadata: Metadata{
			Count: len(servers),
		},
	}

	slog.Debug("Returning servers list", "server_count", len(servers))
	setCORSHeaders(w)
	respondJSON(w, http.StatusOK, response)
}

func makeServerVersionHandler(config *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serverVersionHandler(w, r, config)
	}
}

func serverVersionHandler(w http.ResponseWriter, r *http.Request, config *Config) {
	if r.Method == http.MethodOptions {
		optionsHandler(w, r)
		return
	}
	if r.Method != http.MethodGet {
		slog.Warn("Method not allowed", "method", r.Method, "path", r.URL.Path)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v0.1/servers/")

	var serverName, version string
	if parts := strings.Split(path, "/versions/"); len(parts) == 2 {
		var err error
		serverName, err = url.PathUnescape(parts[0])
		if err != nil {
			slog.Warn("Invalid server name encoding", "encoded", parts[0], "error", err)
			http.Error(w, "Invalid server name encoding", http.StatusBadRequest)
			return
		}
		version = parts[1]
	} else if strings.HasSuffix(path, "/latest") {
		encoded := strings.TrimSuffix(path, "/latest")
		var err error
		serverName, err = url.PathUnescape(encoded)
		if err != nil {
			slog.Warn("Invalid server name encoding", "encoded", encoded, "error", err)
			http.Error(w, "Invalid server name encoding", http.StatusBadRequest)
			return
		}
		version = "latest"
	} else {
		slog.Warn("Invalid path format", "path", r.URL.Path)
		http.Error(w, "Invalid path format", http.StatusBadRequest)
		return
	}

	data, err := os.ReadFile("allowlist.json")
	if err != nil {
		slog.Error("Error reading allowlist.json", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data = substituteVariables(data, config)

	fileInfo, err := os.Stat("allowlist.json")
	if err != nil {
		slog.Error("Error getting file info", "error", err, "file", "allowlist.json")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var staticData StaticRegistryData
	if err := json.Unmarshal(data, &staticData); err != nil {
		slog.Error("Unexpected error parsing allowlist.json", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	updatedAt := fileInfo.ModTime().UTC()
	for _, s := range staticData.Servers {
		if s.Name == serverName && (version == "latest" || s.Version == version) {
			publishedAt := updatedAt
			if s.PublishedAt != "" {
				if parsed, err := time.Parse(time.RFC3339, s.PublishedAt); err == nil {
					publishedAt = parsed
				}
			}
			status := s.Status
			if status == "" {
				status = StatusActive
			}
			response := ServerResponse{
				Server: ServerJSON{
					Schema:      CurrentSchemaURL,
					Name:        s.Name,
					Description: s.Description,
					Version:     s.Version,
					Remotes:     s.Remotes,
				},
				Meta: ResponseMeta{
					Official: &RegistryExtensions{
						Status:      status,
						PublishedAt: publishedAt,
						UpdatedAt:   updatedAt,
						IsLatest:    true,
					},
				},
			}
			slog.Debug("Returning server", "name", serverName, "version", version)
			setCORSHeaders(w)
			respondJSON(w, http.StatusOK, response)
			return
		}
	}

	slog.Warn("Server not found", "name", serverName, "version", version)
	http.Error(w, "Server not found", http.StatusNotFound)
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}
