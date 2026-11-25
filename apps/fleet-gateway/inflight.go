package main

import (
	"net/http"

	"github.com/navikt/copilot/fleet-gateway/internal/httpx"
)

// writeOverCapacity answers 503 with Retry-After instead of queueing.
func writeOverCapacity(w http.ResponseWriter, what string) {
	httpx.MarkShed(w, "cap")
	w.Header().Set("Retry-After", "1")
	httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
		"error":   "too_many_requests",
		"message": "fleet-gateway is at its limit of concurrent " + what + "; retry shortly",
	})
}
