package main

import "net/http"

// inFlightLimit caps concurrent MCP requests per pod. Each one may buffer up to
// 1 MiB in and 10 MiB out, so the cap bounds memory. Excess requests get 503
// with Retry-After instead of queueing.
func inFlightLimit(slots chan struct{}, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			next(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error":   "too_many_requests",
				"message": "fleet-gateway is at its limit of concurrent MCP requests; retry shortly",
			})
		}
	}
}
