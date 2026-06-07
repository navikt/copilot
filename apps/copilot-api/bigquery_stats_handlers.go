package main

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func optionalIntParam(r *http.Request, name string, defaultValue, minValue, maxValue int) (int, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return defaultValue, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minValue || parsed > maxValue {
		return 0, false
	}
	return parsed, true
}

func isValidUsageUsername(username string) bool {
	return username != "" && len(username) <= 39 && !strings.Contains(username, "/") && isValidGitHubUsername(username)
}

func optionalMonthParam(r *http.Request, name string) (string, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return time.Now().UTC().Format("2006-01"), true
	}
	if !isValidYearMonth(value) {
		return "", false
	}
	return value, true
}

func (h *BigQueryHandlers) handleTeamUsageSummary(w http.ResponseWriter, r *http.Request) {
	days, ok := optionalIntParam(r, "days", 7, 1, 365)
	if !ok {
		respondError(w, "invalid_parameter", "days must be between 1 and 365", http.StatusBadRequest)
		return
	}

	usage, err := h.bqClient.GetTeamUsageSummary(r.Context(), days)
	if err != nil {
		slog.Error("Failed to fetch team usage summary", "error", err)
		respondError(w, "internal_error", "Failed to fetch team usage summary", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, usage, http.StatusOK)
}

func (h *BigQueryHandlers) handleUserMetrics(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !isValidUsageUsername(username) {
		respondError(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	days, ok := optionalIntParam(r, "days", 7, 1, 365)
	if !ok {
		respondError(w, "invalid_parameter", "days must be between 1 and 365", http.StatusBadRequest)
		return
	}

	metrics, err := h.bqClient.GetUserMetrics(r.Context(), username, days)
	if err != nil {
		slog.Error("Failed to fetch user metrics", "error", err)
		respondError(w, "internal_error", "Failed to fetch user metrics", http.StatusInternalServerError)
		return
	}
	if metrics == nil {
		respondError(w, "not_found", "No user metrics found", http.StatusNotFound)
		return
	}

	cacheControl(w, 300, false)
	respondJSON(w, metrics, http.StatusOK)
}

func (h *BigQueryHandlers) handleMonthlyTrends(w http.ResponseWriter, r *http.Request) {
	months, ok := optionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		respondError(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	trends, err := h.bqClient.GetMonthlyTrends(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch monthly trends", "error", err)
		respondError(w, "internal_error", "Failed to fetch monthly trends", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, trends, http.StatusOK)
}

func (h *BigQueryHandlers) handleMonthlyModelUsage(w http.ResponseWriter, r *http.Request) {
	months, ok := optionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		respondError(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	usage, err := h.bqClient.GetMonthlyModelUsage(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch monthly model usage", "error", err)
		respondError(w, "internal_error", "Failed to fetch monthly model usage", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, usage, http.StatusOK)
}

func (h *BigQueryHandlers) handleMonthlyBillingUsage(w http.ResponseWriter, r *http.Request) {
	months, ok := optionalIntParam(r, "months", 12, 1, 36)
	if !ok {
		respondError(w, "invalid_parameter", "months must be between 1 and 36", http.StatusBadRequest)
		return
	}

	usage, err := h.bqClient.GetMonthlyBillingUsage(r.Context(), months)
	if err != nil {
		slog.Error("Failed to fetch monthly billing usage", "error", err)
		respondError(w, "internal_error", "Failed to fetch monthly billing usage", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, usage, http.StatusOK)
}

func (h *BigQueryHandlers) handleBillingModelDaily(w http.ResponseWriter, r *http.Request) {
	month, ok := optionalMonthParam(r, "month")
	if !ok {
		respondError(w, "invalid_parameter", "month must be in YYYY-MM format", http.StatusBadRequest)
		return
	}

	usage, err := h.bqClient.GetBillingModelDailyCosts(r.Context(), month)
	if err != nil {
		slog.Error("Failed to fetch billing model daily costs", "error", err)
		respondError(w, "internal_error", "Failed to fetch billing model daily costs", http.StatusInternalServerError)
		return
	}
	if usage == nil {
		usage = []BillingModelDailyCost{}
	}

	cacheControl(w, 900, false)
	respondJSON(w, usage, http.StatusOK)
}

func (h *BigQueryHandlers) handleBillingModelForecast(w http.ResponseWriter, r *http.Request) {
	month, ok := optionalMonthParam(r, "month")
	if !ok {
		respondError(w, "invalid_parameter", "month must be in YYYY-MM format", http.StatusBadRequest)
		return
	}

	forecast, err := h.bqClient.GetBillingModelForecast(r.Context(), month)
	if err != nil {
		slog.Error("Failed to fetch billing model forecast", "error", err)
		respondError(w, "internal_error", "Failed to fetch billing model forecast", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 900, false)
	respondJSON(w, forecast, http.StatusOK)
}

func (h *BigQueryHandlers) handleUserWeeklyTrends(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !isValidUsageUsername(username) {
		respondError(w, "invalid_parameter", "Invalid GitHub username", http.StatusBadRequest)
		return
	}

	weeks, ok := optionalIntParam(r, "weeks", 12, 1, 52)
	if !ok {
		respondError(w, "invalid_parameter", "weeks must be between 1 and 52", http.StatusBadRequest)
		return
	}

	trends, err := h.bqClient.GetUserWeeklyTrends(r.Context(), username, weeks)
	if err != nil {
		slog.Error("Failed to fetch user weekly trends", "error", err)
		respondError(w, "internal_error", "Failed to fetch user weekly trends", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 300, false)
	respondJSON(w, trends, http.StatusOK)
}

func (h *BigQueryHandlers) handleAdoptionCohorts(w http.ResponseWriter, r *http.Request) {
	days, ok := optionalIntParam(r, "days", 90, 1, 365)
	if !ok {
		respondError(w, "invalid_parameter", "days must be between 1 and 365", http.StatusBadRequest)
		return
	}

	cohorts, err := h.bqClient.GetAdoptionCohorts(r.Context(), days)
	if err != nil {
		slog.Error("Failed to fetch adoption cohorts", "error", err)
		respondError(w, "internal_error", "Failed to fetch adoption cohorts", http.StatusInternalServerError)
		return
	}

	cacheControl(w, 3600, false)
	respondJSON(w, cohorts, http.StatusOK)
}
