package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

// validGitHubUsername matches GitHub's username rules: alphanumeric + hyphens, 1-39 chars
var validGitHubUsername = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$`)

func isValidGitHubUsername(s string) bool {
	return validGitHubUsername.MatchString(s)
}

// GitHubAPI abstracts GitHub API operations for testability
type GitHubAPI interface {
	getCopilotBilling(ctx context.Context) (*CopilotBilling, error)
	getCopilotSeat(ctx context.Context, username string) (*CopilotSeat, error)
	assignUserToCopilot(ctx context.Context, username string) (*AssignResult, error)
	unassignUserFromCopilot(ctx context.Context, username string) (*UnassignResult, error)
	getUsernameBySamlIdentity(ctx context.Context, identity string) (string, error)
}

// GitHubHandlers wraps handlers that use GitHub API
type GitHubHandlers struct {
	githubClient GitHubAPI
}

func newGitHubHandlers(githubClient GitHubAPI) *GitHubHandlers {
	return &GitHubHandlers{
		githubClient: githubClient,
	}
}

// handleBilling handles GET /api/v1/copilot/billing
func (h *GitHubHandlers) handleBilling(w http.ResponseWriter, r *http.Request) {
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
	username := r.PathValue("username")
	if !isValidGitHubUsername(username) {
		respondError(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	seat, err := h.githubClient.getCopilotSeat(r.Context(), username)
	if err != nil {
		slog.Error("Failed to fetch seat", "username", username, "error", err)
		respondError(w, "github_error", "Failed to fetch Copilot seat data", http.StatusInternalServerError)
		return
	}

	if seat == nil {
		respondError(w, "not_found", fmt.Sprintf("No Copilot seat found for user %s", username), http.StatusNotFound)
		return
	}

	respondJSON(w, seat, http.StatusOK)
}

// handleAssignSeat handles POST /api/v1/copilot/seats
func (h *GitHubHandlers) handleAssignSeat(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user, ok := getUserFromContext(r.Context())
	if !ok || user == nil {
		respondError(w, "unauthorized", "User not authenticated", http.StatusUnauthorized)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	body := http.MaxBytesReader(w, r.Body, 1024)
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		respondError(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if !isValidGitHubUsername(req.Username) {
		respondError(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	result, err := h.githubClient.assignUserToCopilot(r.Context(), req.Username)
	if err != nil {
		slog.Error("Failed to assign seat",
			"username", req.Username,
			"actor", user.Email,
			"error", err,
		)
		respondError(w, "github_error", "Failed to assign Copilot seat", http.StatusInternalServerError)
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
	}, http.StatusCreated)
}

// handleUnassignSeat handles DELETE /api/v1/copilot/seats/{username}
func (h *GitHubHandlers) handleUnassignSeat(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user, ok := getUserFromContext(r.Context())
	if !ok || user == nil {
		respondError(w, "unauthorized", "User not authenticated", http.StatusUnauthorized)
		return
	}

	username := r.PathValue("username")
	if !isValidGitHubUsername(username) {
		respondError(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	result, err := h.githubClient.unassignUserFromCopilot(r.Context(), username)
	if err != nil {
		slog.Error("Failed to unassign seat",
			"username", username,
			"actor", user.Email,
			"error", err,
		)
		respondError(w, "github_error", "Failed to unassign Copilot seat", http.StatusInternalServerError)
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
	identity := r.PathValue("identity")
	if identity == "" || len(identity) > 254 || strings.Contains(identity, "/") {
		respondError(w, "invalid_parameter", "Invalid SAML identity", http.StatusBadRequest)
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
