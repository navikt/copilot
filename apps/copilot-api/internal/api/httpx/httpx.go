// Package httpx holds the small HTTP helpers shared by the API handler
// packages and internal/budget.
package httpx

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/navikt/copilot/apps/copilot-api/internal/problem"
)

// RespondJSON writes data as JSON with the given status.
func RespondJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}

// CacheControl sets HTTP cache headers for responses
// duration: cache duration in seconds (max-age and s-maxage)
// public: if true, cache is shareable; if false, cache is private to the user
func CacheControl(w http.ResponseWriter, duration int, public bool) {
	var policy string
	if public {
		policy = fmt.Sprintf("public, max-age=%d, s-maxage=%d", duration, duration)
	} else {
		policy = fmt.Sprintf("private, max-age=%d", duration)
	}
	w.Header().Set("Cache-Control", policy)
}

// NilSafe extracts a handler method from a potentially-nil handler struct.
// The caller is responsible for guarding against nil before using the result.
func NilSafe[T any](h *T, fn func(*T) http.HandlerFunc) http.HandlerFunc {
	if h == nil {
		return nil
	}
	return fn(h)
}

// RequireMethod answers 405 unless the request uses method.
func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	problem.Respond(w, "method_not_allowed", "Only GET is allowed", http.StatusMethodNotAllowed)
	return false
}

// OptionalIntParam reads an optional integer query parameter within
// [minValue, maxValue], falling back to defaultValue when it is absent.
func OptionalIntParam(r *http.Request, name string, defaultValue, minValue, maxValue int) (int, bool) {
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
