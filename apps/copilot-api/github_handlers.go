package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// validGitHubUsername matches GitHub's username rules: alphanumeric + hyphens, 1-39 chars
var validGitHubUsername = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$`)

func isValidGitHubUsername(s string) bool {
	return validGitHubUsername.MatchString(s)
}

// navIdentOf returns user.NAVident for audit logging, or "" if user is nil
// (e.g. when the caller authenticated via a trusted-intermediary mechanism
// like X-On-Behalf-Of rather than a NAV employee's own token).
func navIdentOf(user *User) string {
	if user == nil {
		return ""
	}
	return user.NAVident
}

// GitHubAPI abstracts GitHub API operations for testability
type GitHubAPI interface {
	getCopilotBilling(ctx context.Context) (*CopilotBilling, error)
	getCopilotSeat(ctx context.Context, username string) (*CopilotSeat, error)
	assignUserToCopilot(ctx context.Context, username string) (*AssignResult, error)
	unassignUserFromCopilot(ctx context.Context, username string) (*UnassignResult, error)
	getUsernameBySamlIdentity(ctx context.Context, identity string) (string, error)
	getPremiumRequestUsage(ctx context.Context, org string, year, month int) (*PremiumRequestUsage, error)
	getRepositoryContributors(ctx context.Context, owner, repo string, paths []string) ([]Contributor, error)
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

	cacheControl(w, 900, false) // 15 min, private (billing summary)
	respondJSON(w, billing, http.StatusOK)
}

// handleGetSeat handles GET /api/v1/copilot/seats/{username}
func (h *GitHubHandlers) handleGetSeat(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !isValidGitHubUsername(username) {
		respondError(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	// Ownership check: only allow users to view their own seat data. Uses
	// the shared IdentityResolver-based check (see identity_middleware.go)
	// so this handler works identically whether the caller authenticated
	// via SAML or a trusted intermediary's X-On-Behalf-Of header.
	if !requireOwnership(w, r, username) {
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

	cacheControl(w, 300, false) // 5 min, private (user-specific)
	respondJSON(w, seat, http.StatusOK)
}

// handleAssignSeat handles POST /api/v1/copilot/seats
func (h *GitHubHandlers) handleAssignSeat(w http.ResponseWriter, r *http.Request) {
	// user is only used for audit logging (NAVident) below — the ownership
	// check itself is mechanism-agnostic (see requireOwnership).
	user, _ := getUserFromContext(r.Context())

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

	// Defense-in-depth: verify the requested username resolves to the authenticated caller.
	// The BFF already performs this check, but enforcing it here ensures the backend
	// cannot be used to manage seats for other users even if the BFF were bypassed.
	if !requireOwnership(w, r, req.Username) {
		return
	}

	result, err := h.githubClient.assignUserToCopilot(r.Context(), req.Username)
	if err != nil {
		slog.Error("Failed to assign seat",
			"username", req.Username,
			"actor_navident", navIdentOf(user),
			"error", err,
		)
		respondError(w, "github_error", "Failed to assign Copilot seat", http.StatusInternalServerError)
		return
	}

	// Audit log: use NAVident only — email is PII and must not be logged at INFO+
	slog.Info("Copilot seat assigned",
		"username", req.Username,
		"actor_navident", navIdentOf(user),
		"seats_created", result.SeatsCreated,
	)

	noCacheControl(w)
	respondJSON(w, map[string]interface{}{
		"seats_created": result.SeatsCreated,
		"username":      req.Username,
	}, http.StatusCreated)
}

// handleUnassignSeat handles DELETE /api/v1/copilot/seats/{username}
func (h *GitHubHandlers) handleUnassignSeat(w http.ResponseWriter, r *http.Request) {
	// user is only used for audit logging (NAVident) below — the ownership
	// check itself is mechanism-agnostic (see requireOwnership).
	user, _ := getUserFromContext(r.Context())

	username := r.PathValue("username")
	if !isValidGitHubUsername(username) {
		respondError(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	// Defense-in-depth: verify the requested username resolves to the authenticated caller.
	// The BFF already performs this check, but enforcing it here ensures the backend
	// cannot be used to manage seats for other users even if the BFF were bypassed.
	if !requireOwnership(w, r, username) {
		return
	}

	result, err := h.githubClient.unassignUserFromCopilot(r.Context(), username)
	if err != nil {
		slog.Error("Failed to unassign seat",
			"username", username,
			"actor_navident", navIdentOf(user),
			"error", err,
		)
		respondError(w, "github_error", "Failed to unassign Copilot seat", http.StatusInternalServerError)
		return
	}

	// Audit log: use NAVident only — email is PII and must not be logged at INFO+
	slog.Info("Copilot seat unassigned",
		"username", username,
		"actor_navident", navIdentOf(user),
		"seats_cancelled", result.SeatsCancelled,
	)

	noCacheControl(w)
	respondJSON(w, map[string]interface{}{
		"seats_cancelled": result.SeatsCancelled,
		"username":        username,
	}, http.StatusOK)
}

// handleGetUsernameBySAML handles GET /api/v1/copilot/saml/{identity}
// Restricted: callers can only look up their own email identity.
// Cache: 30 min (SAML identity mappings rarely change)
func (h *GitHubHandlers) handleGetUsernameBySAML(w http.ResponseWriter, r *http.Request) {
	identity := r.PathValue("identity")
	if identity == "" || len(identity) > 254 || strings.Contains(identity, "/") {
		respondError(w, "invalid_parameter", "Invalid SAML identity", http.StatusBadRequest)
		return
	}

	// Ownership check: only allow users to look up their own email.
	user, ok := getUserFromContext(r.Context())
	if !ok || user == nil {
		respondError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)
		return
	}
	if !strings.EqualFold(user.Email, identity) {
		respondError(w, "forbidden", "You can only look up your own identity", http.StatusForbidden)
		return
	}

	username, err := h.githubClient.getUsernameBySamlIdentity(r.Context(), identity)
	if err != nil {
		slog.Error("Failed to lookup SAML identity", "error", err)
		respondError(w, "github_error", "Failed to lookup SAML identity", http.StatusInternalServerError)
		return
	}

	if username == "" {
		cacheControl(w, 1800, false)
		respondJSON(w, map[string]interface{}{
			"identity": identity,
			"username": nil,
		}, http.StatusOK)
		return
	}

	cacheControl(w, 1800, false)
	respondJSON(w, map[string]interface{}{
		"identity": identity,
		"username": username,
	}, http.StatusOK)
}

// handlePremiumRequestUsage handles GET /api/v1/copilot/billing/premium
// Cache: 1 hour (billing data updated daily)
func (h *GitHubHandlers) handlePremiumRequestUsage(w http.ResponseWriter, r *http.Request) {
	org := r.URL.Query().Get("org")
	if org == "" || !isValidGitHubUsername(org) {
		respondError(w, "invalid_parameter", "Invalid organization name", http.StatusBadRequest)
		return
	}

	year := 0
	month := 0

	if yearStr := r.URL.Query().Get("year"); yearStr != "" {
		var err error
		year, err = strconv.Atoi(yearStr)
		if err != nil || year < 2000 || year > 2999 {
			respondError(w, "invalid_parameter", "Invalid year parameter", http.StatusBadRequest)
			return
		}
	}

	if monthStr := r.URL.Query().Get("month"); monthStr != "" {
		var err error
		month, err = strconv.Atoi(monthStr)
		if err != nil || month < 1 || month > 12 {
			respondError(w, "invalid_parameter", "Invalid month parameter", http.StatusBadRequest)
			return
		}
	}

	usage, err := h.githubClient.getPremiumRequestUsage(r.Context(), org, year, month)
	if err != nil {
		slog.Error("Failed to fetch premium request usage", "org", org, "error", err)
		respondError(w, "github_error", "Failed to fetch premium request usage", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, usage, http.StatusOK)
}

// handleRepositoryContributors handles GET /api/v1/copilot/repo-contributors
// Cache: 7 days (contributors list is stable)
func (h *GitHubHandlers) handleRepositoryContributors(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	repo := r.URL.Query().Get("repo")
	pathsJSON := r.URL.Query().Get("paths")

	if owner == "" || !isValidGitHubUsername(owner) {
		respondError(w, "invalid_parameter", "Invalid owner", http.StatusBadRequest)
		return
	}

	if repo == "" || len(repo) > 255 {
		respondError(w, "invalid_parameter", "Invalid repository name", http.StatusBadRequest)
		return
	}

	if pathsJSON == "" {
		respondError(w, "invalid_parameter", "Missing paths parameter", http.StatusBadRequest)
		return
	}

	var paths []string
	if err := json.Unmarshal([]byte(pathsJSON), &paths); err != nil {
		respondError(w, "invalid_parameter", "Invalid paths format", http.StatusBadRequest)
		return
	}

	if len(paths) == 0 || len(paths) > 50 {
		respondError(w, "invalid_parameter", "Must provide 1-50 paths", http.StatusBadRequest)
		return
	}

	for _, path := range paths {
		if len(path) > 512 {
			respondError(w, "invalid_parameter", "Path too long", http.StatusBadRequest)
			return
		}
	}

	contributors, err := h.githubClient.getRepositoryContributors(r.Context(), owner, repo, paths)
	if err != nil {
		slog.Error("Failed to fetch contributors", "owner", owner, "repo", repo, "error", err)
		respondError(w, "github_error", "Failed to fetch contributors", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 604800, false) // 7 days, private
	respondJSON(w, map[string]interface{}{
		"contributors": contributors,
	}, http.StatusOK)
}
