package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// GitHubHandlers wraps handlers that use GitHub API
type GitHubHandlers struct {
	githubClient *GitHubClient
}

func newGitHubHandlers(githubClient *GitHubClient) *GitHubHandlers {
	return &GitHubHandlers{
		githubClient: githubClient,
	}
}

// handleBilling handles GET /api/v1/copilot/billing
func (h *GitHubHandlers) handleBilling(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, "method_not_allowed", "Only GET is allowed", http.StatusMethodNotAllowed)
		return
	}

	billing, err := h.githubClient.getCopilotBilling(r.Context())
	if err != nil {
		slog.Error("Failed to fetch billing", "error", err)
		respondError(w, "github_error", "Failed to fetch Copilot billing data", http.StatusInternalServerError)
		return
	}

	respondJSON(w, billing, http.StatusOK)
}

// handleGetSeat handles GET /api/v1/copilot/seats/{username}
func (h *GitHubHandlers) handleGetSeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, "method_not_allowed", "Only GET is allowed", http.StatusMethodNotAllowed)
		return
	}

	username := extractUsername(r.URL.Path)
	if username == "" {
		respondError(w, "invalid_parameter", "Username is required", http.StatusBadRequest)
		return
	}

	seat, err := h.githubClient.getCopilotSeat(r.Context(), username)
	if err != nil {
		slog.Error("Failed to fetch seat", "username", username, "error", err)
		respondError(w, "github_error", "Failed to fetch Copilot seat data", http.StatusInternalServerError)
		return
	}

	respondJSON(w, seat, http.StatusOK)
}

// handleAssignSeat handles POST /api/v1/copilot/seats
func (h *GitHubHandlers) handleAssignSeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, "method_not_allowed", "Only POST is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user from context
	user, ok := getUserFromContext(r.Context())
	if !ok || user == nil {
		respondError(w, "unauthorized", "User not authenticated", http.StatusUnauthorized)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		respondError(w, "invalid_parameter", "username is required", http.StatusBadRequest)
		return
	}

	result, err := h.githubClient.assignUserToCopilot(r.Context(), req.Username)
	if err != nil {
		slog.Error("Failed to assign seat",
			"username", req.Username,
			"actor", user.Email,
			"error", err,
		)
		respondError(w, "github_error", fmt.Sprintf("Failed to assign seat: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Audit log
	slog.Info("Copilot seat assigned",
		"username", req.Username,
		"actor", user.Email,
		"actor_navident", user.NAVident,
		"seats_created", result.SeatsCreated,
	)

	respondJSON(w, map[string]interface{}{
		"seats_created": result.SeatsCreated,
		"username":      req.Username,
	}, http.StatusOK)
}

// handleUnassignSeat handles DELETE /api/v1/copilot/seats/{username}
func (h *GitHubHandlers) handleUnassignSeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		respondError(w, "method_not_allowed", "Only DELETE is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get authenticated user from context
	user, ok := getUserFromContext(r.Context())
	if !ok || user == nil {
		respondError(w, "unauthorized", "User not authenticated", http.StatusUnauthorized)
		return
	}

	username := extractUsername(r.URL.Path)
	if username == "" {
		respondError(w, "invalid_parameter", "Username is required", http.StatusBadRequest)
		return
	}

	result, err := h.githubClient.unassignUserFromCopilot(r.Context(), username)
	if err != nil {
		slog.Error("Failed to unassign seat",
			"username", username,
			"actor", user.Email,
			"error", err,
		)
		respondError(w, "github_error", fmt.Sprintf("Failed to unassign seat: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Audit log
	slog.Info("Copilot seat unassigned",
		"username", username,
		"actor", user.Email,
		"actor_navident", user.NAVident,
		"seats_cancelled", result.SeatsCancelled,
	)

	respondJSON(w, map[string]interface{}{
		"seats_cancelled": result.SeatsCancelled,
		"username":        username,
	}, http.StatusOK)
}

// handleGetUsernameBySAML handles GET /api/v1/copilot/saml/{identity}
func (h *GitHubHandlers) handleGetUsernameBySAML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, "method_not_allowed", "Only GET is allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := extractUsername(r.URL.Path) // Reuse path extraction
	if identity == "" {
		respondError(w, "invalid_parameter", "SAML identity is required", http.StatusBadRequest)
		return
	}

	username, err := h.githubClient.getUsernameBySamlIdentity(r.Context(), identity)
	if err != nil {
		slog.Error("Failed to lookup SAML identity", "identity", identity, "error", err)
		respondError(w, "github_error", "Failed to lookup SAML identity", http.StatusInternalServerError)
		return
	}

	if username == "" {
		respondJSON(w, map[string]interface{}{
			"identity": identity,
			"username": nil,
		}, http.StatusOK)
		return
	}

	respondJSON(w, map[string]interface{}{
		"identity": identity,
		"username": username,
	}, http.StatusOK)
}

// extractUsername extracts username from path like /api/v1/copilot/seats/octocat
func extractUsername(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
