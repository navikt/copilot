package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// healthHandler handles /health endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Warn("Failed to write health response", "error", err)
	}
}

// readyHandler handles /ready endpoint
func readyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !authMiddlewareReady.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte("AUTH_UNAVAILABLE")); err != nil {
			slog.Warn("Failed to write ready response", "error", err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Warn("Failed to write ready response", "error", err)
	}
}

// makeAPIRouter creates the main API router for /api/v1/
func makeAPIRouter(config *Config, bqHandlers *BigQueryHandlers, ghHandlers *GitHubHandlers) http.Handler {
	mux := http.NewServeMux()

	// BigQuery endpoints
	if bqHandlers != nil {
		mux.HandleFunc("GET /api/v1/copilot/usage/metrics", bqHandlers.handleDailyMetrics)
		mux.HandleFunc("GET /api/v1/copilot/adoption/summary", bqHandlers.handleAdoptionSummary)
		mux.HandleFunc("GET /api/v1/copilot/adoption/teams", bqHandlers.handleTeamAdoption)
		mux.HandleFunc("GET /api/v1/copilot/adoption/languages", bqHandlers.handleLanguageAdoption)
		mux.HandleFunc("GET /api/v1/copilot/adoption/staleness", bqHandlers.handleAdoptionStaleness)
		mux.HandleFunc("GET /api/v1/copilot/customizations/details", bqHandlers.handleCustomizationDetails)
		mux.HandleFunc("GET /api/v1/copilot/customizations/usage", bqHandlers.handleCustomizationUsage)
	}

	// GitHub API endpoints
	if ghHandlers != nil {
		mux.HandleFunc("GET /api/v1/copilot/billing", ghHandlers.handleBilling)
		mux.HandleFunc("GET /api/v1/copilot/seats/{username}", ghHandlers.handleGetSeat)
		mux.HandleFunc("POST /api/v1/copilot/seats", ghHandlers.handleAssignSeat)
		mux.HandleFunc("DELETE /api/v1/copilot/seats/{username}", ghHandlers.handleUnassignSeat)
		mux.HandleFunc("GET /api/v1/copilot/saml/{identity}", ghHandlers.handleGetUsernameBySAML)
	}

	// Placeholder endpoints - to be implemented in future phases
	mux.HandleFunc("/api/v1/copilot/usage/summary", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/trends", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/features", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/languages", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/editors", notImplementedHandler)
	mux.HandleFunc("/api/v1/copilot/usage/models", notImplementedHandler)
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
	if err := json.NewEncoder(w).Encode(problem); err != nil {
		slog.Error("Failed to encode problem details response", "error", err)
	}
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
