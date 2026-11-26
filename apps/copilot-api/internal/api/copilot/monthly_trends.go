package copilot

import (
	"log/slog"
	"net/http"

	"github.com/navikt/copilot/apps/copilot-api/internal/api/httpx"
	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

// HandleCreditsPerUserMonthly handles GET /api/v1/copilot/usage/credits-per-user
func (h *Handlers) HandleCreditsPerUserMonthly(w http.ResponseWriter, r *http.Request) {
	months, err := h.bqClient.GetCreditsPerUserMonthly(r.Context())
	if err != nil {
		slog.Error("Failed to fetch credits per user", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch credits per user", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, months, http.StatusOK)
}

// HandleCopilotPRsMonthly handles GET /api/v1/copilot/usage/copilot-prs
func (h *Handlers) HandleCopilotPRsMonthly(w http.ResponseWriter, r *http.Request) {
	months, err := h.bqClient.GetCopilotPRsMonthly(r.Context())
	if err != nil {
		slog.Error("Failed to fetch Copilot PRs per month", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch Copilot PRs per month", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, months, http.StatusOK)
}

// HandleCohortRetention handles GET /api/v1/copilot/usage/cohort-retention
func (h *Handlers) HandleCohortRetention(w http.ResponseWriter, r *http.Request) {
	cohorts, err := h.bqClient.GetCohortRetention(r.Context())
	if err != nil {
		slog.Error("Failed to fetch cohort retention", "error", err)
		problem.Respond(w, "internal_error", "Failed to fetch cohort retention", http.StatusInternalServerError)
		return
	}
	httpx.CacheControl(w, 3600, false)
	httpx.RespondJSON(w, cohorts, http.StatusOK)
}
