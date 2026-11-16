package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const (
	maxWriteBodyBytes = 1 << 20
	writeDBTimeout    = 10 * time.Second
)

// logSafe strips CR and LF characters from request-derived values passed to slog.
func logSafe(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	return strings.ReplaceAll(s, "\r", "")
}

func readWriteBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWriteBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "Request body too large"})
			return nil, false
		}
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to read request body"})
		return nil, false
	}
	return body, true
}

func decodeStrictJSON(body []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("unexpected trailing data")
	}
	return nil
}

// writeUnavailable answers 503 when the service has no connected database.
func writeUnavailable(w http.ResponseWriter, db *DB) bool {
	if db != nil {
		return false
	}
	respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Writes are unavailable: database not connected"})
	return true
}

type submitPayload struct {
	Entry json.RawMessage `json:"entry"`
}

func makeSubmitHandler(config *Config, dbRef *atomic.Pointer[DB], introspect IntrospectFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		caller, authErr := authenticate(r, introspect)
		if authErr != nil {
			slog.Warn("Authentication failed for submit", "error", authErr)
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}
		if !config.AllowsWriteAZP(caller.Azp) {
			slog.Warn("Write rejected: client is not an allowed writer", "azp", logSafe(caller.Azp))
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden: client is not allowed to write"})
			return
		}
		actor := caller.Actor

		body, ok := readWriteBody(w, r)
		if !ok {
			return
		}

		var payload submitPayload
		if parseErr := decodeStrictJSON(body, &payload); parseErr != nil || len(payload.Entry) == 0 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body: 'entry' object required"})
			return
		}

		sub, canonicalJSON, valErr := validateSubmission(payload.Entry)
		if valErr != nil {
			msg := submissionErrorMessage(valErr)
			slog.Warn("Validation failed for submit", "error", logSafe(msg), "actor", logSafe(actor))
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
			return
		}

		db := dbRef.Load()
		if writeUnavailable(w, db) {
			return
		}
		isAdmin := config.IsAdmin(caller.Groups)
		dbCtx, cancel := context.WithTimeout(r.Context(), writeDBTimeout)
		defer cancel()
		version, storeErr := db.Submit(dbCtx, actor, canonicalJSON, sub.Name, isAdmin)
		if storeErr != nil {
			if errors.Is(storeErr, ErrForbidden) {
				slog.Warn("Forbidden submit attempt for existing name", "name", logSafe(sub.Name), "actor", logSafe(actor))
				respondJSON(w, http.StatusForbidden, map[string]string{"error": storeErr.Error()})
				return
			}
			if errors.Is(storeErr, ErrRateLimitExceeded) {
				slog.Warn("Rate limit exceeded for actor", "actor", logSafe(actor))
				recordWriteMetric("submit", "rate_limited")
				respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": storeErr.Error()})
				return
			}
			slog.Error("Failed to store submitted entry", "error", storeErr, "name", logSafe(sub.Name), "actor", logSafe(actor))
			recordWriteMetric("submit", "error")
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
			return
		}

		recordWriteMetric("submit", "success")
		slog.Info("Server entry submitted", "name", logSafe(sub.Name), "version", version, "actor", logSafe(actor))
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

func makeEntryActionHandler(config *Config, dbRef *atomic.Pointer[DB], introspect IntrospectFunc) http.HandlerFunc {
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

		caller, authErr := authenticate(r, introspect)
		if authErr != nil {
			slog.Warn("Authentication failed for entry action", "action", action, "error", authErr)
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
			return
		}
		if !config.AllowsWriteAZP(caller.Azp) {
			slog.Warn("Write rejected: client is not an allowed writer", "azp", logSafe(caller.Azp))
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden: client is not allowed to write"})
			return
		}
		actor := caller.Actor

		if valErr := validateName(name, 0); valErr != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": valErr.Error()})
			return
		}

		if !config.IsAdmin(caller.Groups) {
			slog.Warn("Non-admin user attempted privileged action", "action", action, "actor", logSafe(actor), "name", logSafe(name))
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden: actor is not an admin"})
			return
		}

		body, ok := readWriteBody(w, r)
		if !ok {
			return
		}

		db := dbRef.Load()
		if writeUnavailable(w, db) {
			return
		}
		switch action {
		case "approve":
			handleApprove(w, r, db, actor, name, body)
		case "reject":
			handleReject(w, r, db, actor, name, body)
		}
	}
}

func handleApprove(w http.ResponseWriter, r *http.Request, db *DB, actor, name string, body []byte) {
	var payload approvePayload
	if err := decodeStrictJSON(body, &payload); err != nil || payload.Version <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body: positive 'version' integer required"})
		return
	}

	dbCtx, cancel := context.WithTimeout(r.Context(), writeDBTimeout)
	defer cancel()
	if err := db.Approve(dbCtx, actor, name, payload.Version); err != nil {
		if errors.Is(err, ErrVersionNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Version not found"})
			return
		}
		if errors.Is(err, ErrConflict) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		slog.Error("Failed to approve entry", "error", logSafe(err.Error()))
		recordWriteMetric("approve", "error")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	recordWriteMetric("approve", "success")
	slog.Info("Server entry approved") // name, version and actor are in registry_audit
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"name":    name,
		"version": payload.Version,
		"status":  "approved",
	})
}

func handleReject(w http.ResponseWriter, r *http.Request, db *DB, actor, name string, body []byte) {
	var payload rejectPayload
	if err := decodeStrictJSON(body, &payload); err != nil || payload.Version <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body: positive 'version' integer required"})
		return
	}

	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Reason is required for rejection"})
		return
	}

	dbCtx, cancel := context.WithTimeout(r.Context(), writeDBTimeout)
	defer cancel()
	if err := db.Reject(dbCtx, actor, name, payload.Version, reason); err != nil {
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
		slog.Error("Failed to reject entry", "error", logSafe(err.Error()))
		recordWriteMetric("reject", "error")
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	recordWriteMetric("reject", "success")
	slog.Info("Server entry rejected") // name, version and actor are in registry_audit
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"name":    name,
		"version": payload.Version,
		"status":  "rejected",
		"reason":  reason,
	})
}
