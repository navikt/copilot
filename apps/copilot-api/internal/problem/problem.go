// Package problem writes RFC 7807 problem responses, shared by the auth
// middleware and the API handlers.
package problem

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Detail is an RFC 7807 Problem Details response.
type Detail struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// NoCache marks a response as not cacheable.
func NoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

// Respond writes a problem response with the given status.
func Respond(w http.ResponseWriter, errorType, detail string, status int) {
	problem := Detail{
		Type:     "about:blank",
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: "",
	}

	if errorType != "" {
		problem.Type = "https://copilot-api.nav.no/errors/" + errorType
	}

	w.Header().Set("Content-Type", "application/problem+json")
	NoCache(w)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(problem); err != nil {
		slog.Error("Failed to encode problem details response", "error", err)
	}
}
