package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func readyHandler(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func metricsHandler() http.Handler {
	return promhttp.Handler()
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
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

func robotsTxtHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("User-agent: *\nAllow: /v0.1/\nDisallow: /health\nDisallow: /ready\nDisallow: /metrics\n"))
}

func securityTxtHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Contact: soc@nav.no\nPreferred-Languages: no, en\nCanonical: https://www.nav.no/.well-known/security.txt\nDisclosure: Full\n"))
}

func faviconHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
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

func navRegistryMeta(server StaticServerData) *NavRegistryMeta {
	if len(server.Tools) == 0 && len(server.Tags) == 0 && len(server.Examples) == 0 && len(server.SetupInstructions) == 0 && len(server.SandboxHosts) == 0 && len(server.ToolRisk) == 0 {
		return nil
	}
	return &NavRegistryMeta{
		Tools:             server.Tools,
		Tags:              server.Tags,
		Examples:          server.Examples,
		SetupInstructions: server.SetupInstructions,
		SandboxHosts:      server.SandboxHosts,
		ToolRisk:          server.ToolRisk,
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

	if config == nil || config.Store == nil {
		slog.Error("Store not initialized")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	servers, err := config.Store.GetServers()
	if err != nil {
		slog.Error("Error getting servers from store", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
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

	if config == nil || config.Store == nil {
		slog.Error("Store not initialized")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	server, err := config.Store.GetServer(serverName, version)
	if err != nil {
		slog.Error("Error getting server from store", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if server != nil {
		slog.Debug("Returning server", "name", serverName, "version", version)
		recordServerLookup(serverName, "found")
		setCORSHeaders(w)
		respondJSON(w, http.StatusOK, *server)
		return
	}
	recordServerLookup("unknown", "not_found")
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
