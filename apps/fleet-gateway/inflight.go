package main

import "net/http"

// writeOverCapacity answers 503 with Retry-After instead of queueing.
func writeOverCapacity(w http.ResponseWriter, what string) {
	markShed(w, "cap")
	w.Header().Set("Retry-After", "1")
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{
		"error":   "too_many_requests",
		"message": "fleet-gateway is at its limit of concurrent " + what + "; retry shortly",
	})
}
