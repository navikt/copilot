package main

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	bqstore "github.com/navikt/copilot/apps/copilot-api/internal/bigquery"
	"github.com/navikt/copilot/apps/copilot-api/internal/cost"
)

// handleCostForecast handles GET /api/v1/admin/billing/cost-forecast.
func (h *BigQueryHandlers) handleCostForecast(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	days, err := h.bqClient.GetCostDays(r.Context(), month.Format("2006-01"))
	if err != nil {
		slog.Error("Failed to fetch cost days", "error", err)
		respondError(w, "internal_error", "Failed to fetch cost forecast", http.StatusInternalServerError)
		return
	}
	users, err := h.bqClient.GetCostUserDays(r.Context(), month.Format("2006-01"))
	if err != nil {
		slog.Error("Failed to fetch cost user days", "error", err)
		respondError(w, "internal_error", "Failed to fetch cost forecast", http.StatusInternalServerError)
		return
	}

	status, budget := cost.BudgetError, 0.0
	if h.budgetClient != nil {
		b, err := h.budgetClient.getGlobalBudget(r.Context())
		switch {
		case errors.Is(err, errBudgetNotFound):
			status = cost.BudgetMissing
		case err != nil:
			slog.Warn("Failed to fetch enterprise budget for cost forecast", "error", err)
		case b != nil && b.EnterpriseBudget > 0:
			status, budget = cost.BudgetSet, b.EnterpriseBudget
		default:
			status = cost.BudgetMissing
		}
	}

	out := cost.BuildCostForecast(days, users, month, budget, status)
	snaps, err := h.bqClient.GetLimitSnapshots(r.Context())
	if err != nil {
		// The markers are a nicety; the documented limits still give the history.
		slog.Warn("Failed to fetch limit snapshots", "error", err)
	}
	out.LimitChanges = bqstore.LimitChanges(snaps)
	out.YearForecast = []cost.YearForecastPoint{}
	if len(out.Models) > 0 && out.Models[0].Model == cost.ModelTrend {
		// The year chart is a nicety too: without it the month forecast still stands.
		var months []bqstore.AdminMonth
		var err error
		// Early in the year the fit needs last year's closed months too.
		for _, y := range []int{month.Year() - 1, month.Year()} {
			if y < month.Year() && int(month.Month()) > cost.YearFitMonths {
				continue
			}
			overview, oerr := h.bqClient.GetAdminMonthlyOverview(r.Context(), y)
			if oerr != nil {
				err = errors.Join(err, oerr)
				continue
			}
			months = append(months, overview.Months...)
		}
		users, uerr := h.bqClient.GetActiveUsersMonthly(r.Context())
		if err != nil || uerr != nil {
			slog.Warn("Failed to fetch year forecast data", "error", errors.Join(err, uerr))
		} else {
			out.YearForecast, out.YearEnd = cost.YearForecast(months, users, out.Models[0], month)
		}
	}
	cacheControl(w, 900, false)
	respondJSON(w, out, http.StatusOK)
}
