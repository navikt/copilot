package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func readyHandler(store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store != nil {
			if err := store.Ping(r.Context()); err != nil {
				respondJSON(w, http.StatusServiceUnavailable, map[string]string{
					"status": "unavailable",
					"error":  "database unreachable",
				})
				return
			}
		}
		respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func metricsHandler() http.Handler {
	return promhttp.Handler()
}

type submitPayload struct {
	Entry json.RawMessage `json:"entry"`
}

func makeSubmitHandler(config *Config, store *Store, introspect IntrospectFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		actor, authErr := authenticate(r, introspect)
		if authErr != nil {
			slog.Warn("Authentication failed for submit", "error", authErr)
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}

		body, readErr := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if readErr != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to read request body"})
			return
		}

		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()

		var payload submitPayload
		if parseErr := dec.Decode(&payload); parseErr != nil || len(payload.Entry) == 0 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body: 'entry' object required"})
			return
		}

		sub, canonicalJSON, valErr := validateServerEntry(payload.Entry)
		if valErr != nil {
			slog.Warn("Validation failed for submit", "error", valErr, "actor", actor)
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": valErr.Error()})
			return
		}

		isAdmin := config != nil && config.IsAdmin(actor)
		version, storeErr := store.Submit(r.Context(), actor, canonicalJSON, sub.Name, isAdmin)
		if storeErr != nil {
			if errors.Is(storeErr, ErrForbidden) {
				slog.Warn("Forbidden submit attempt for existing name", "name", sub.Name, "actor", actor)
				respondJSON(w, http.StatusForbidden, map[string]string{"error": storeErr.Error()})
				return
			}
			if errors.Is(storeErr, ErrRateLimitExceeded) {
				slog.Warn("Rate limit exceeded for actor", "actor", actor)
				recordWriteMetric("submit", "rate_limited")
				respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": storeErr.Error()})
				return
			}
			slog.Error("Failed to store submitted entry", "error", storeErr, "name", sub.Name, "actor", actor)
			recordWriteMetric("submit", "error")
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
			return
		}

		recordWriteMetric("submit", "success")
		slog.Info("Server entry submitted", "name", sub.Name, "version", version, "actor", actor)
		respondJSON(w, http.StatusCreated, map[string]interface{}{
			"name":    sub.Name,
			"version": version,
			"status":  "pending",
		})
	}
}

type approvePayload struct {
	Version int `json:"version"`
}

type rejectPayload struct {
	Version int    `json:"version"`
	Reason  string `json:"reason"`
}

func makeEntryActionHandler(config *Config, store *Store, introspect IntrospectFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		pathVal := r.PathValue("name")
		var action, name string

		switch {
		case strings.HasSuffix(pathVal, "/approve"):
			action = "approve"
			name = strings.TrimSuffix(pathVal, "/approve")
		case strings.HasSuffix(pathVal, "/reject"):
			action = "reject"
			name = strings.TrimSuffix(pathVal, "/reject")
		default:
			http.NotFound(w, r)
			return
		}

		actor, authErr := authenticate(r, introspect)
		if authErr != nil {
			slog.Warn("Authentication failed for entry action", "action", action, "error", authErr)
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}

		if valErr := validateServerName(name); valErr != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": valErr.Error()})
			return
		}

		if !config.IsAdmin(actor) {
			slog.Warn("Non-admin user attempted privileged action", "action", action, "actor", actor, "name", name)
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden: actor is not an admin"})
			return
		}

		body, readErr := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if readErr != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to read request body"})
			return
		}

		switch action {
		case "approve":
			handleApprove(w, r, store, actor, name, body)
		case "reject":
			handleReject(w, r, store, actor, name, body)
		}
	}
}

func handleApprove(w http.ResponseWriter, r *http.Request, store *Store, actor, name string, body []byte) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()

	var payload approvePayload
	if err := dec.Decode(&payload); err != nil || payload.Version <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body: positive 'version' integer required"})
		return
	}

	if err := store.Approve(r.Context(), actor, name, payload.Version); err != nil {
		if errors.Is(err, ErrVersionNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Version not found"})
			return
		}
		if errors.Is(err, ErrConflict) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		slog.Error("Failed to approve entry", "error", err, "name", name, "version", payload.Version)
		recordWriteMetric("approve", "error")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	recordWriteMetric("approve", "success")
	slog.Info("Server entry approved", "name", name, "version", payload.Version, "actor", actor)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"name":    name,
		"version": payload.Version,
		"status":  "approved",
	})
}

func handleReject(w http.ResponseWriter, r *http.Request, store *Store, actor, name string, body []byte) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()

	var payload rejectPayload
	if err := dec.Decode(&payload); err != nil || payload.Version <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body: positive 'version' integer required"})
		return
	}

	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Reason is required for rejection"})
		return
	}

	if err := store.Reject(r.Context(), actor, name, payload.Version, reason); err != nil {
		if errors.Is(err, ErrVersionNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Version not found"})
			return
		}
		if errors.Is(err, ErrAlreadyApproved) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrConflict) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		slog.Error("Failed to reject entry", "error", err, "name", name, "version", payload.Version)
		recordWriteMetric("reject", "error")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	recordWriteMetric("reject", "success")
	slog.Info("Server entry rejected", "name", name, "version", payload.Version, "reason", reason, "actor", actor)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"name":    name,
		"version": payload.Version,
		"status":  "rejected",
		"reason":  reason,
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}
