package copilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"

	"github.com/navikt/copilot/apps/copilot-api/internal/github"
)

// validRepoName matches GitHub repository names: letters, digits, '.', '-'
// and '_', at most 100 characters.
var validRepoName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

// navIdentOf returns user.NAVident for audit logging, or "" if user is nil
// (e.g. when the caller authenticated via a trusted-intermediary mechanism
// like X-On-Behalf-Of rather than a NAV employee's own token).
func navIdentOf(user *auth.User) string {
	if user == nil {
		return ""
	}
	return user.NAVident
}

// GitHubAPI abstracts GitHub API operations for testability
type GitHubAPI interface {
	GetCopilotBilling(ctx context.Context) (*github.CopilotBilling, error)
	GetCopilotSeat(ctx context.Context, username string) (*github.CopilotSeat, error)
	AssignUserToCopilot(ctx context.Context, username string) (*github.AssignResult, error)
	UnassignUserFromCopilot(ctx context.Context, username string) (*github.UnassignResult, error)
	GetUsernameBySamlIdentity(ctx context.Context, identity string) (string, error)
	GetPremiumRequestUsage(ctx context.Context, org string, year, month int) (*github.PremiumRequestUsage, error)
	GetRepositoryContributors(ctx context.Context, owner, repo string, paths []string) ([]github.Contributor, error)
}

// GitHubHandlers wraps handlers that use GitHub API
type GitHubHandlers struct {
	githubClient GitHubAPI
}

func NewGitHubHandlers(githubClient GitHubAPI) *GitHubHandlers {
	return &GitHubHandlers{
		githubClient: githubClient,
	}
}

// HandleBilling handles GET /api/v1/copilot/billing
func (h *GitHubHandlers) HandleBilling(w http.ResponseWriter, r *http.Request) {
	billing, err := h.githubClient.GetCopilotBilling(r.Context())
	if err != nil {
		slog.Error("Failed to fetch billing", "error", err)
		problem.Respond(w, "github_error", "Failed to fetch Copilot billing data", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 900, false) // 15 min, private (billing summary)
	httpx.RespondJSON(w, billing, http.StatusOK)
}

// HandleGetSeat handles GET /api/v1/copilot/seats/{username}
func (h *GitHubHandlers) HandleGetSeat(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !auth.IsValidGitHubUsername(username) {
		problem.Respond(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	// Ownership check: only allow users to view their own seat data. Uses
	// the shared IdentityResolver-based check (see identity_middleware.go)
	// so this handler works identically whether the caller authenticated
	// via SAML or a trusted intermediary's X-On-Behalf-Of header.
	if !auth.RequireOwnership(w, r, username) {
		return
	}

	seat, err := h.githubClient.GetCopilotSeat(r.Context(), username)
	if err != nil {
		slog.Error("Failed to fetch seat", "error", err)
		problem.Respond(w, "github_error", "Failed to fetch Copilot seat data", http.StatusInternalServerError)
		return
	}

	if seat == nil {
		problem.Respond(w, "not_found", fmt.Sprintf("No Copilot seat found for user %s", username), http.StatusNotFound)
		return
	}

	httpx.CacheControl(w, 300, false) // 5 min, private (user-specific)
	httpx.RespondJSON(w, seat, http.StatusOK)
}

// HandleAssignSeat handles POST /api/v1/copilot/seats
func (h *GitHubHandlers) HandleAssignSeat(w http.ResponseWriter, r *http.Request) {
	// user supplies the groups for the check below and the NAVident for audit
	// logging; the ownership check is mechanism-agnostic (see requireOwnership).
	user, _ := auth.UserFromContext(r.Context())

	var req struct {
		Username string `json:"username"`
	}
	body := http.MaxBytesReader(w, r.Body, 1024)
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		problem.Respond(w, "invalid_request", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if !auth.IsValidGitHubUsername(req.Username) {
		problem.Respond(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	// Defense-in-depth: verify the requested username resolves to the authenticated caller.
	// The BFF already performs this check, but enforcing it here ensures the backend
	// cannot be used to manage seats for other users even if the BFF were bypassed.
	if !auth.RequireOwnership(w, r, req.Username) {
		return
	}

	// Defense-in-depth for the BFF's group check: only members of a group
	// listed under azure.application.claims.groups may activate a seat.
	// The admin group is in the claim only for requireAdmin and grants no seat.
	if user == nil || !slices.ContainsFunc(user.Groups, func(g string) bool { return g != "" && g != os.Getenv(auth.AdminGroupEnv) }) {
		problem.Respond(w, "forbidden", "User is not a member of any allowed group", http.StatusForbidden)
		return
	}

	result, err := h.githubClient.AssignUserToCopilot(r.Context(), req.Username)
	if err != nil {
		slog.Error("Failed to assign seat",
			"username", req.Username,
			"actor_navident", navIdentOf(user),
			"error", err,
		)
		problem.Respond(w, "github_error", "Failed to assign Copilot seat", http.StatusInternalServerError)
		return
	}

	// Audit log: use NAVident only — email is PII and must not be logged at INFO+
	slog.Info("Copilot seat assigned",
		"username", req.Username,
		"actor_navident", navIdentOf(user),
		"seats_created", result.SeatsCreated,
	)

	problem.NoCache(w)
	httpx.RespondJSON(w, map[string]interface{}{
		"seats_created": result.SeatsCreated,
		"username":      req.Username,
	}, http.StatusCreated)
}

// HandleUnassignSeat handles DELETE /api/v1/copilot/seats/{username}
func (h *GitHubHandlers) HandleUnassignSeat(w http.ResponseWriter, r *http.Request) {
	// user is only used for audit logging (NAVident) below — the ownership
	// check itself is mechanism-agnostic (see requireOwnership).
	user, _ := auth.UserFromContext(r.Context())

	username := r.PathValue("username")
	if !auth.IsValidGitHubUsername(username) {
		problem.Respond(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	// Defense-in-depth: verify the requested username resolves to the authenticated caller.
	// The BFF already performs this check, but enforcing it here ensures the backend
	// cannot be used to manage seats for other users even if the BFF were bypassed.
	if !auth.RequireOwnership(w, r, username) {
		return
	}

	result, err := h.githubClient.UnassignUserFromCopilot(r.Context(), username)
	if err != nil {
		slog.Error("Failed to unassign seat",
			"username", username,
			"actor_navident", navIdentOf(user),
			"error", err,
		)
		problem.Respond(w, "github_error", "Failed to unassign Copilot seat", http.StatusInternalServerError)
		return
	}

	// Audit log: use NAVident only — email is PII and must not be logged at INFO+
	slog.Info("Copilot seat unassigned",
		"username", username,
		"actor_navident", navIdentOf(user),
		"seats_cancelled", result.SeatsCancelled,
	)

	problem.NoCache(w)
	httpx.RespondJSON(w, map[string]interface{}{
		"seats_cancelled": result.SeatsCancelled,
		"username":        username,
	}, http.StatusOK)
}

// HandleGetUsernameBySAML handles GET /api/v1/copilot/saml/{identity}
// Restricted: callers can only look up their own email identity.
// Cache: 30 min (SAML identity mappings rarely change)
func (h *GitHubHandlers) HandleGetUsernameBySAML(w http.ResponseWriter, r *http.Request) {
	identity := r.PathValue("identity")
	if identity == "" || len(identity) > 254 || strings.Contains(identity, "/") {
		problem.Respond(w, "invalid_parameter", "Invalid SAML identity", http.StatusBadRequest)
		return
	}

	// Ownership check: only allow users to look up their own email.
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user == nil {
		problem.Respond(w, "unauthorized", "Authentication required", http.StatusUnauthorized)
		return
	}
	if !strings.EqualFold(user.Email, identity) {
		problem.Respond(w, "forbidden", "You can only look up your own identity", http.StatusForbidden)
		return
	}

	username, err := h.githubClient.GetUsernameBySamlIdentity(r.Context(), identity)
	if err != nil {
		slog.Error("Failed to lookup SAML identity", "error", err)
		problem.Respond(w, "github_error", "Failed to lookup SAML identity", http.StatusInternalServerError)
		return
	}

	if username == "" {
		httpx.CacheControl(w, 1800, false)
		httpx.RespondJSON(w, map[string]interface{}{
			"identity": identity,
			"username": nil,
		}, http.StatusOK)
		return
	}

	httpx.CacheControl(w, 1800, false)
	httpx.RespondJSON(w, map[string]interface{}{
		"identity": identity,
		"username": username,
	}, http.StatusOK)
}

// HandlePremiumRequestUsage handles GET /api/v1/copilot/billing/premium
// Cache: 1 hour (billing data updated daily)
func (h *GitHubHandlers) HandlePremiumRequestUsage(w http.ResponseWriter, r *http.Request) {
	org := r.URL.Query().Get("org")
	if org == "" || !auth.IsValidGitHubUsername(org) {
		problem.Respond(w, "invalid_parameter", "Invalid organization name", http.StatusBadRequest)
		return
	}

	year := 0
	month := 0

	if yearStr := r.URL.Query().Get("year"); yearStr != "" {
		var err error
		year, err = strconv.Atoi(yearStr)
		if err != nil || year < 2000 || year > 2999 {
			problem.Respond(w, "invalid_parameter", "Invalid year parameter", http.StatusBadRequest)
			return
		}
	}

	if monthStr := r.URL.Query().Get("month"); monthStr != "" {
		var err error
		month, err = strconv.Atoi(monthStr)
		if err != nil || month < 1 || month > 12 {
			problem.Respond(w, "invalid_parameter", "Invalid month parameter", http.StatusBadRequest)
			return
		}
	}

	usage, err := h.githubClient.GetPremiumRequestUsage(r.Context(), org, year, month)
	if err != nil {
		slog.Error("Failed to fetch premium request usage", "org", org, "error", err)
		problem.Respond(w, "github_error", "Failed to fetch premium request usage", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, usage, http.StatusOK)
}

// HandleRepositoryContributors handles GET /api/v1/copilot/repo-contributors
// Cache: 7 days (contributors list is stable)
func (h *GitHubHandlers) HandleRepositoryContributors(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	repo := r.URL.Query().Get("repo")
	pathsJSON := r.URL.Query().Get("paths")

	if owner == "" || !auth.IsValidGitHubUsername(owner) {
		problem.Respond(w, "invalid_parameter", "Invalid owner", http.StatusBadRequest)
		return
	}

	if !validRepoName.MatchString(repo) || repo == "." || repo == ".." {
		problem.Respond(w, "invalid_parameter", "Invalid repository name", http.StatusBadRequest)
		return
	}

	if pathsJSON == "" {
		problem.Respond(w, "invalid_parameter", "Missing paths parameter", http.StatusBadRequest)
		return
	}

	var paths []string
	if err := json.Unmarshal([]byte(pathsJSON), &paths); err != nil {
		problem.Respond(w, "invalid_parameter", "Invalid paths format", http.StatusBadRequest)
		return
	}

	if len(paths) == 0 || len(paths) > 50 {
		problem.Respond(w, "invalid_parameter", "Must provide 1-50 paths", http.StatusBadRequest)
		return
	}

	for _, path := range paths {
		if len(path) > 512 {
			problem.Respond(w, "invalid_parameter", "Path too long", http.StatusBadRequest)
			return
		}
	}

	contributors, err := h.githubClient.GetRepositoryContributors(r.Context(), owner, repo, paths)
	if errors.Is(err, github.ErrOwnerNotAllowed) {
		problem.Respond(w, "invalid_parameter", "Invalid owner", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("Failed to fetch contributors", "owner", owner, "repo", repo, "error", err)
		problem.Respond(w, "github_error", "Failed to fetch contributors", http.StatusInternalServerError)
		return
	}

	httpx.CacheControl(w, 604800, false) // 7 days, private
	httpx.RespondJSON(w, map[string]interface{}{
		"contributors": contributors,
	}, http.StatusOK)
}
