package main

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/navikt/copilot/apps/copilot-api/internal/auth"
)

// BudgetHandlers handles the enterprise AI credit budget endpoint.
type BudgetHandlers struct {
	budgetClient *BudgetClient
	raises       *budgetRaiseHandlers // optional: adds this month's raises
}

func newBudgetHandlers(budgetClient *BudgetClient) *BudgetHandlers {
	return &BudgetHandlers{
		budgetClient: budgetClient,
	}
}

// handleGetBudget handles GET /api/v1/copilot/budget.
// Relies on IdentityMiddleware (see identity_middleware.go) having already
// resolved the caller's GitHub username — mechanism-agnostic, works
// identically whether resolution came from SAML or X-On-Behalf-Of.
func (h *BudgetHandlers) handleGetBudget(w http.ResponseWriter, r *http.Request) {
	identity, ok := auth.GetResolvedIdentity(r.Context())
	if !ok {
		respondError(w, "unauthorized", "Caller identity could not be determined", http.StatusUnauthorized)
		return
	}

	budget, err := h.budgetClient.getUserBudget(r.Context(), identity.GitHubUsername)
	if err != nil {
		if errors.Is(err, errBudgetNotFound) {
			respondError(w, "not_found", "No budget data found", http.StatusNotFound)
			return
		}
		slog.Error("Failed to fetch user budget", "error", err)
		respondError(w, "budget_error", "Failed to fetch budget data", http.StatusInternalServerError)
		return
	}

	out := struct {
		*UserBudget
		Raises []raiseView `json:"raises"`
	}{UserBudget: budget}
	if h.raises != nil {
		now := h.raises.now().In(h.raises.loc)
		month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, h.raises.loc)
		// Raises are extra: a Postgres outage leaves them out, not the budget.
		if raises, err := h.raises.monthRaises(r.Context(), strings.ToLower(identity.GitHubUsername), month); err == nil {
			out.Raises = raises
		} else {
			slog.Warn("Budget raises unavailable for GET budget", "error", err)
		}
		noCacheControl(w) // changes right after a raise
	} else {
		cacheControl(w, 30*60, false)
	}
	respondJSON(w, out, http.StatusOK)
}

// handleGetGlobalBudget handles GET /api/v1/copilot/budget/global.
// Returns the enterprise-wide default AI credit budget (no identity resolution required).
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

	// The enterprise budget alone is internal: answer as before, not found.
	if budget.PerUserBudget == 0 && budget.TotalConsumed == 0 {
		respondError(w, "not_found", "No global budget data found", http.StatusNotFound)
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
