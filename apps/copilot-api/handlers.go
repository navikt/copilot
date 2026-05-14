package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// healthHandler handles /health endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// readyHandler handles /ready endpoint
func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// makeAPIRouter creates the main API router for /api/v1/
func makeAPIRouter(config *Config, bqHandlers *BigQueryHandlers) http.Handler {
	mux := http.NewServeMux()

	// BigQuery endpoints
	if bqHandlers != nil {
		mux.HandleFunc("/api/v1/copilot/usage/metrics", bqHandlers.handleDailyMetrics)
		mux.HandleFunc("/api/v1/copilot/adoption/summary", bqHandlers.handleAdoptionSummary)
		mux.HandleFunc("/api/v1/copilot/adoption/teams", bqHandlers.handleTeamAdoption)
		mux.HandleFunc("/api/v1/copilot/adoption/languages", bqHandlers.handleLanguageAdoption)
		mux.HandleFunc("/api/v1/copilot/customizations/details", bqHandlers.handleCustomizationDetails)
		mux.HandleFunc("/api/v1/copilot/customizations/usage", bqHandlers.handleCustomizationUsage)
	}

	// Placeholder endpoints - to be implemented in Phase 4
	mux.HandleFunc("/api/v1/copilot/usage/summary", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/trends", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/features", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/languages", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/editors", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/models", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/billing/summary", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/billing/premium", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/seats/", notImplementedHandler)
	mux.HandleFunc("/api/v1/mcp/servers", notImplementedHandler)

	return mux
}

func notImplementedHandler(w http.ResponseWriter, r *http.Request) {
	respondError(w, "not_implemented", "This endpoint is not yet implemented", http.StatusNotImplemented)
}

// RFC 7807 Problem Details response
type ProblemDetail struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

func respondError(w http.ResponseWriter, errorType, detail string, status int) {
	problem := ProblemDetail{
		Type:     "about:blank",
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: "",
	}

	if errorType != "" {
		problem.Type = "https://copilot-api.nav.no/errors/" + errorType
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(problem)
}

func respondJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(config *Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shouldLog := false
		for endpoint := range config.LoggedEndpoints {
			if strings.HasPrefix(r.URL.Path, endpoint) {
				shouldLog = true
				break
			}
		}

		if shouldLog {
			slog.Info("HTTP request",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)
		}

		next.ServeHTTP(w, r)
	})
}
