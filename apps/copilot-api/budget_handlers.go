package main

import (
	"errors"
	"log/slog"
	"net/http"
)

// BudgetHandlers handles the enterprise AI credit budget endpoint.
type BudgetHandlers struct {
	budgetClient *BudgetClient
	githubClient GitHubAPI
}

func newBudgetHandlers(budgetClient *BudgetClient, githubClient GitHubAPI) *BudgetHandlers {
	return &BudgetHandlers{
		budgetClient: budgetClient,
		githubClient: githubClient,
	}
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

	cacheControl(w, 30*60, true) // public — same for all users
	respondJSON(w, budget, http.StatusOK)
}
