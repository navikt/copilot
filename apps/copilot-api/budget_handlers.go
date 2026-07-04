package main

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

// BudgetHandlers handles the enterprise AI credit budget endpoint.
type BudgetHandlers struct {
	budgetClient *BudgetClient
	githubClient GitHubAPI

	// identityChain enables shadow-mode comparison logging, same as
	// BigQueryHandlers.identityChain (see identity.go and
	// logIdentityShadowComparison in bigquery_stats_handlers.go). Nil by
	// default (disabled).
	identityChain *IdentityResolverChain
}

func newBudgetHandlers(budgetClient *BudgetClient, githubClient GitHubAPI) *BudgetHandlers {
	return &BudgetHandlers{
		budgetClient: budgetClient,
		githubClient: githubClient,
	}
}

// setIdentityChain wires in the new IdentityResolver-based identity chain
// for shadow-mode comparison against the legacy SAML lookup in
// handleGetBudget. Passing nil disables shadow-mode comparison (the default).
func (h *BudgetHandlers) setIdentityChain(chain *IdentityResolverChain) {
	h.identityChain = chain
}

// handleGetBudget handles GET /api/v1/copilot/budget.
// Resolves the authenticated user's GitHub username via SAML and returns their AI credit budget.
func (h *BudgetHandlers) handleGetBudget(w http.ResponseWriter, r *http.Request) {
	user, ok := getUserFromContext(r.Context())
	if !ok {
		respondError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)
		return
	}

	username, err := h.githubClient.getUsernameBySamlIdentity(r.Context(), user.Email)
	if err != nil {
		slog.Error("Failed to resolve GitHub username via SAML", "error", err)
		respondError(w, "saml_error", "Failed to resolve GitHub identity", http.StatusInternalServerError)
		return
	}
	if username == "" {
		respondError(w, "not_found", "GitHub account not linked to Nav organisation", http.StatusNotFound)
		return
	}
	h.logIdentityShadowComparison(r, user, username)

	budget, err := h.budgetClient.getUserBudget(r.Context(), username)
	if err != nil {
		if errors.Is(err, errBudgetNotFound) {
			respondError(w, "not_found", "No budget data found", http.StatusNotFound)
			return
		}
		slog.Error("Failed to fetch user budget", "error", err)
		respondError(w, "budget_error", "Failed to fetch budget data", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 30*60, false)
	respondJSON(w, budget, http.StatusOK)
}

// handleGetGlobalBudget handles GET /api/v1/copilot/budget/global.
// Returns the enterprise-wide default AI credit budget (no SAML lookup required).
func (h *BudgetHandlers) handleGetGlobalBudget(w http.ResponseWriter, r *http.Request) {
	budget, err := h.budgetClient.getGlobalBudget(r.Context())
	if err != nil {
		if errors.Is(err, errBudgetNotFound) {
			respondError(w, "not_found", "No global budget data found", http.StatusNotFound)
			return
		}
		slog.Error("Failed to fetch global budget", "error", err)
		respondError(w, "budget_error", "Failed to fetch budget data", http.StatusInternalServerError)
		return
	}

	// Use the background-collected seat count for an accurate active user total.
	// The budget API only lists override users, not all licensed seats.
	metricsCollector.mu.RLock()
	activeSeats := metricsCollector.githubSeatsActive
	metricsCollector.mu.RUnlock()
	if activeSeats > 0 {
		budget.ActiveUsers = int(activeSeats)
	}

	cacheControl(w, 30*60, true) // public — same for all users
	respondJSON(w, budget, http.StatusOK)
}

// logIdentityShadowComparison compares the legacy SAML-resolved username
// used by handleGetBudget against what the new IdentityResolver-based chain
// would resolve, and logs any disagreement. Pure observability — see the
// equivalent method on BigQueryHandlers in bigquery_stats_handlers.go for
// full rationale. No-op unless setIdentityChain has been called.
func (h *BudgetHandlers) logIdentityShadowComparison(r *http.Request, user *User, legacyUsername string) {
	if h.identityChain == nil {
		return
	}
	identity, err := h.identityChain.Resolve(r.Context(), user, r)
	newUsername := ""
	if err == nil {
		newUsername = identity.GitHubUsername
	}

	if !strings.EqualFold(newUsername, legacyUsername) {
		slog.Warn("Identity resolution shadow-mode mismatch (budget)",
			"legacy_username", legacyUsername,
			"new_username", newUsername,
			"new_resolve_error", errString(err),
		)
		return
	}
	slog.Debug("Identity resolution shadow-mode match (budget)", "username", legacyUsername)
}
